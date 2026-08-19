// Package transport owns one game-gateway WebSocket connection.
//
// It deliberately stops at the protocol boundary: connection lifecycle,
// framing, encryption, request correlation, and notification delivery live
// here; heartbeats, reconnect policy, and domain actions belong to callers.
package transport

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/RoseKhlifa/FarmBot/internal/game/pb"
	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"
)

const (
	// Runtime-owned cadence values are kept here as protocol timing contracts.
	// The transport does not start timers for them.
	AceDataInterval       = 5 * time.Second
	AceHeartbeatInterval  = 25 * time.Second
	AceSpeedCheckInterval = 30 * time.Second
	AceStatusInterval     = 150 * time.Second
	AceFunctionInterval   = 180 * time.Second
	GameHeartbeatInterval = 25 * time.Second

	defaultRequestTimeout = 20 * time.Second
	maxPendingRequests    = 50
)

var (
	ErrNotConnected = errors.New("game gateway is not connected")
	ErrClosed       = errors.New("game gateway transport is closed")
	ErrNoCipher     = errors.New("game gateway cipher is not configured")
)

// Cipher is the account-local TSDK surface required by the gateway.
// Runtime satisfies this interface without making transport depend on wazero.
type Cipher interface {
	Encrypt([]byte) ([]byte, error)
	Decrypt([]byte) ([]byte, error)
}

// InitInfoProvider is implemented by tsdk.Runtime after BindUser. It keeps
// the transport decoupled from the concrete wazero runtime while preserving
// the gateway's one-shot _get_encrypted_init_info credential flow.
type InitInfoProvider interface {
	GetEncryptedInitInfo() (string, error)
}

// UserState is connection-local account state. It is intentionally a value
// owned by Client instead of a package-level singleton.
type UserState struct {
	GID      int64
	Name     string
	Level    int64
	Gold     int64
	Exp      int64
	Coupon   int64
	GoldBean int64
	OpenID   string
	Avatar   string
}

// Command identifies one game RPC. Response, when non-nil, is populated by
// SendMsg after the response body has been decrypted and protobuf-decoded.
// Service/Method are accepted as short aliases for ServiceName/MethodName.
type Command struct {
	ServiceName string
	MethodName  string
	Service     string
	Method      string
	Response    proto.Message
	Decode      func([]byte) (proto.Message, error)
	Timeout     time.Duration
}

func (c Command) serviceName() string {
	if c.ServiceName != "" {
		return c.ServiceName
	}
	return c.Service
}

func (c Command) methodName() string {
	if c.MethodName != "" {
		return c.MethodName
	}
	return c.Method
}

// Options configures one Client. A Client must not be shared by accounts.
type Options struct {
	Cipher           Cipher
	Header           http.Header
	Dialer           *websocket.Dialer
	DefaultTimeout   time.Duration
	EventBuffer      int
	InitialAuthToken string
	// OnEvent is called synchronously by the reader goroutine. Keep callbacks
	// short; Events is preferable when consumers need independent pacing.
	OnEvent func(Event)
}

// Client is a single account's gateway connection.
type Client struct {
	url    string
	config Options

	connMu sync.RWMutex
	conn   *websocket.Conn

	writeMu sync.Mutex
	stateMu sync.RWMutex
	state   UserState

	seq       atomic.Int64
	serverSeq atomic.Int64
	pendingMu sync.Mutex
	pending   map[int64]*pendingRequest

	authMu           sync.Mutex
	initialAuthToken string

	events  chan Event
	onEvent func(Event)

	closeOnce  sync.Once
	closed     chan struct{}
	closeErrMu sync.RWMutex
	closeErr   error
	eventMu    sync.RWMutex
	eventsDone bool
}

type pendingRequest struct {
	result chan responseFrame
}

type responseFrame struct {
	message *pb.Message
	err     error
}

// New constructs an unconnected client. Use Connect or Dial to establish the
// WebSocket. The returned client owns its own UserState and event channel.
func New(url string, options Options) *Client {
	if options.DefaultTimeout <= 0 {
		options.DefaultTimeout = defaultRequestTimeout
	}
	if options.EventBuffer <= 0 {
		options.EventBuffer = 64
	}
	return &Client{
		url:              url,
		config:           options,
		pending:          make(map[int64]*pendingRequest),
		initialAuthToken: options.InitialAuthToken,
		events:           make(chan Event, options.EventBuffer),
		onEvent:          options.OnEvent,
		closed:           make(chan struct{}),
	}
}

// NewClient is an explicit alias for New.
func NewClient(url string, options Options) *Client { return New(url, options) }

// Dial constructs and connects a client in one operation.
func Dial(ctx context.Context, url string, options Options) (*Client, error) {
	client := New(url, options)
	if err := client.Connect(ctx); err != nil {
		return nil, err
	}
	return client, nil
}

// Connect opens the gateway WebSocket and starts its reader loop.
func (c *Client) Connect(ctx context.Context) error {
	if c == nil {
		return errors.New("nil game gateway client")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	c.connMu.Lock()
	if c.conn != nil {
		c.connMu.Unlock()
		return errors.New("game gateway client is already connected")
	}
	select {
	case <-c.closed:
		c.connMu.Unlock()
		return ErrClosed
	default:
	}
	dialer := c.config.Dialer
	if dialer == nil {
		dialer = websocket.DefaultDialer
	}
	conn, _, err := dialer.DialContext(ctx, c.url, c.config.Header)
	if err != nil {
		c.connMu.Unlock()
		return fmt.Errorf("connect game gateway: %w", err)
	}
	c.conn = conn
	c.seq.Store(1)
	c.serverSeq.Store(0)
	c.connMu.Unlock()
	go c.readLoop(conn)
	return nil
}

// SetInitialAuthToken installs the one-shot credential produced by
// tsdk.Runtime.GetEncryptedInitInfo after account binding. The next request
// consumes it; all later requests use a random gateway token.
func (c *Client) SetInitialAuthToken(token string) {
	c.authMu.Lock()
	c.initialAuthToken = strings.TrimSpace(token)
	c.authMu.Unlock()
}

// UseEncryptedInitInfo obtains and installs the one-shot gateway credential
// from an account-local TSDK runtime.
func (c *Client) UseEncryptedInitInfo(provider InitInfoProvider) error {
	if provider == nil {
		return errors.New("nil encrypted init info provider")
	}
	token, err := provider.GetEncryptedInitInfo()
	if err != nil {
		return fmt.Errorf("get encrypted init info: %w", err)
	}
	if strings.TrimSpace(token) == "" {
		return errors.New("encrypted init info is empty")
	}
	c.SetInitialAuthToken(token)
	return nil
}

// State returns a snapshot of this connection's account state.
func (c *Client) State() UserState {
	c.stateMu.RLock()
	defer c.stateMu.RUnlock()
	return c.state
}

// UpdateState applies a caller-provided state snapshot.
func (c *Client) UpdateState(state UserState) {
	c.stateMu.Lock()
	c.state = state
	c.stateMu.Unlock()
}

// Events returns the per-client notification stream. It is closed when the
// connection closes. Event delivery is best-effort when the buffer is full;
// the read loop must never stall the gateway connection on a slow consumer.
func (c *Client) Events() <-chan Event { return c.events }

// ServerSeq returns the greatest server sequence observed on this connection.
func (c *Client) ServerSeq() int64 { return c.serverSeq.Load() }

// SendMsg is the sole RPC primitive. req is protobuf-marshaled, TSDK-encrypted
// before framing, and the response is decrypted and decoded into cmd.Response
// (or through cmd.Decode).
func (c *Client) SendMsg(ctx context.Context, cmd Command, req proto.Message) (proto.Message, error) {
	frame, err := c.send(ctx, cmd, req)
	if err != nil {
		return nil, err
	}
	body, err := c.decrypt(frame.message.Body)
	if err != nil {
		return nil, fmt.Errorf("decrypt %s.%s response: %w", cmd.serviceName(), cmd.methodName(), err)
	}
	if cmd.Decode != nil {
		return cmd.Decode(body)
	}
	if cmd.Response == nil {
		return nil, nil
	}
	if err := proto.Unmarshal(body, cmd.Response); err != nil {
		return nil, fmt.Errorf("decode %s.%s response: %w", cmd.serviceName(), cmd.methodName(), err)
	}
	return cmd.Response, nil
}

// SendMsgRaw is useful for protocol messages whose body is intentionally
// opaque (activity payloads, or a message being migrated incrementally).
func (c *Client) SendMsgRaw(ctx context.Context, cmd Command, req proto.Message) ([]byte, *pb.Meta, error) {
	frame, err := c.send(ctx, cmd, req)
	if err != nil {
		return nil, nil, err
	}
	body, err := c.decrypt(frame.message.Body)
	if err != nil {
		return nil, frame.message.Meta, fmt.Errorf("decrypt %s.%s response: %w", cmd.serviceName(), cmd.methodName(), err)
	}
	return body, frame.message.Meta, nil
}

func (c *Client) send(ctx context.Context, cmd Command, req proto.Message) (responseFrame, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	service, method := cmd.serviceName(), cmd.methodName()
	if service == "" || method == "" {
		return responseFrame{}, errors.New("game gateway command requires service and method")
	}
	var cancel context.CancelFunc
	ctx, cancel = context.WithTimeout(ctx, c.timeout(cmd))
	defer cancel()
	if !c.connected() {
		return responseFrame{}, ErrNotConnected
	}
	body, err := marshalRequest(req)
	if err != nil {
		return responseFrame{}, fmt.Errorf("encode %s.%s request: %w", service, method, err)
	}
	body, err = c.encrypt(body)
	if err != nil {
		return responseFrame{}, fmt.Errorf("encrypt %s.%s request: %w", service, method, err)
	}
	seq := c.seq.Add(1) - 1
	pending := &pendingRequest{result: make(chan responseFrame, 1)}
	c.pendingMu.Lock()
	if len(c.pending) >= maxPendingRequests {
		c.pendingMu.Unlock()
		return responseFrame{}, fmt.Errorf("request queue is full: %s.%s (pending=%d)", service, method, maxPendingRequests)
	}
	c.pending[seq] = pending
	c.pendingMu.Unlock()

	message := &pb.Message{
		Meta: &pb.Meta{
			ServiceName: service,
			MethodName:  method,
			MessageType: int32(pb.MessageType_Request),
			ClientSeq:   seq,
			ServerSeq:   c.serverSeq.Load(),
		},
		Body:      body,
		AuthToken: c.nextAuthToken(),
	}
	encoded, err := proto.Marshal(message)
	if err != nil {
		c.removePending(seq)
		return responseFrame{}, fmt.Errorf("encode %s.%s frame: %w", service, method, err)
	}
	if err := c.write(encoded); err != nil {
		c.removePending(seq)
		return responseFrame{}, fmt.Errorf("send %s.%s: %w", service, method, err)
	}

	select {
	case result := <-pending.result:
		return result, result.err
	case <-ctx.Done():
		c.removePending(seq)
		return responseFrame{}, fmt.Errorf("%s.%s request seq=%d: %w", service, method, seq, ctx.Err())
	case <-c.closed:
		c.removePending(seq)
		if err := c.closeError(); err != nil {
			return responseFrame{}, err
		}
		return responseFrame{}, ErrClosed
	}
}

func (c *Client) timeout(cmd Command) time.Duration {
	if cmd.Timeout > 0 {
		return cmd.Timeout
	}
	return c.config.DefaultTimeout
}

func marshalRequest(req proto.Message) ([]byte, error) {
	if req == nil {
		return nil, nil
	}
	return proto.Marshal(req)
}

func (c *Client) encrypt(body []byte) ([]byte, error) {
	if len(body) == 0 {
		return nil, nil
	}
	if c.config.Cipher == nil {
		return nil, ErrNoCipher
	}
	return c.config.Cipher.Encrypt(body)
}

func (c *Client) decrypt(body []byte) ([]byte, error) {
	if len(body) == 0 {
		return nil, nil
	}
	if c.config.Cipher == nil {
		return nil, ErrNoCipher
	}
	return c.config.Cipher.Decrypt(body)
}

func (c *Client) nextAuthToken() string {
	c.authMu.Lock()
	defer c.authMu.Unlock()
	if c.initialAuthToken != "" {
		token := c.initialAuthToken
		c.initialAuthToken = ""
		return token
	}
	return randomGatewayToken()
}

func randomGatewayToken() string {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	length, err := rand.Int(rand.Reader, big.NewInt(64))
	if err != nil {
		// crypto/rand failures are exceptional; a fixed-length token preserves
		// the wire contract without silently returning an empty credential.
		return strings.Repeat("A", 64) + "="
	}
	var token strings.Builder
	token.Grow(int(length.Int64()) + 65)
	for i := int64(0); i < length.Int64()+64; i++ {
		index, err := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
		if err != nil {
			return strings.Repeat("A", 64) + "="
		}
		token.WriteByte(alphabet[index.Int64()])
	}
	token.WriteByte('=')
	return token.String()
}

func (c *Client) connected() bool {
	c.connMu.RLock()
	connected := c.conn != nil
	c.connMu.RUnlock()
	return connected
}

func (c *Client) write(data []byte) error {
	c.connMu.RLock()
	conn := c.conn
	c.connMu.RUnlock()
	if conn == nil {
		return ErrNotConnected
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return conn.WriteMessage(websocket.BinaryMessage, data)
}

func (c *Client) readLoop(conn *websocket.Conn) {
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			c.finishConnection(conn, err)
			return
		}
		c.handleFrame(data)
	}
}

func (c *Client) handleFrame(data []byte) {
	message := new(pb.Message)
	if err := proto.Unmarshal(data, message); err != nil {
		c.emit(Event{Type: EventDecodeError, Err: fmt.Errorf("decode gateway frame: %w", err), Raw: append([]byte(nil), data...)})
		return
	}
	meta := message.Meta
	if meta == nil {
		c.emit(Event{Type: EventDecodeError, Err: errors.New("gateway frame has no metadata"), Raw: append([]byte(nil), data...)})
		return
	}
	for {
		old := c.serverSeq.Load()
		if meta.ServerSeq <= old || c.serverSeq.CompareAndSwap(old, meta.ServerSeq) {
			break
		}
	}
	switch pb.MessageType(meta.MessageType) {
	case pb.MessageType_Response:
		c.pendingMu.Lock()
		pending := c.pending[meta.ClientSeq]
		if pending != nil {
			delete(c.pending, meta.ClientSeq)
		}
		c.pendingMu.Unlock()
		if pending != nil {
			if meta.ErrorCode != 0 {
				pending.result <- responseFrame{message: message, err: &GatewayError{Meta: meta}}
			} else {
				pending.result <- responseFrame{message: message}
			}
		}
	case pb.MessageType_Notify:
		dispatchNotify(c, meta, message.Body)
	}
}

func (c *Client) removePending(seq int64) {
	c.pendingMu.Lock()
	delete(c.pending, seq)
	c.pendingMu.Unlock()
}

func (c *Client) finishConnection(conn *websocket.Conn, err error) {
	c.connMu.Lock()
	if c.conn != conn {
		c.connMu.Unlock()
		return
	}
	c.conn = nil
	c.connMu.Unlock()
	if err != nil {
		c.closeErrMu.Lock()
		c.closeErr = err
		c.closeErrMu.Unlock()
	}
	c.pendingMu.Lock()
	pending := c.pending
	c.pending = make(map[int64]*pendingRequest)
	c.pendingMu.Unlock()
	for _, request := range pending {
		request.result <- responseFrame{err: fmt.Errorf("gateway disconnected: %w", connectionError(err))}
	}
	c.emit(Event{Type: EventDisconnect, Err: err})
}

func connectionError(err error) error {
	if err == nil {
		return ErrClosed
	}
	return err
}

// Close stops this connection, rejects pending calls, and closes Events.
func (c *Client) Close() error {
	if c == nil {
		return nil
	}
	var closeErr error
	c.closeOnce.Do(func() {
		close(c.closed)
		c.connMu.Lock()
		conn := c.conn
		c.conn = nil
		c.connMu.Unlock()
		if conn != nil {
			closeErr = conn.Close()
		}
		c.pendingMu.Lock()
		pending := c.pending
		c.pending = make(map[int64]*pendingRequest)
		c.pendingMu.Unlock()
		for _, request := range pending {
			request.result <- responseFrame{err: ErrClosed}
		}
		c.emit(Event{Type: EventDisconnect, Err: ErrClosed})
		c.eventMu.Lock()
		c.eventsDone = true
		close(c.events)
		c.eventMu.Unlock()
	})
	return closeErr
}

func (c *Client) closeError() error {
	c.closeErrMu.RLock()
	defer c.closeErrMu.RUnlock()
	return c.closeErr
}
