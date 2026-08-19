// Package session assembles one account's authenticated game session.
package session

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/RoseKhlifa/FarmBot/internal/config"
	"github.com/RoseKhlifa/FarmBot/internal/game/ace"
	"github.com/RoseKhlifa/FarmBot/internal/game/pb"
	"github.com/RoseKhlifa/FarmBot/internal/game/transport"
	"github.com/RoseKhlifa/FarmBot/internal/game/tsdk"
	"google.golang.org/protobuf/proto"
)

const (
	userService  = "gamepb.userpb.UserService"
	plantService = "gamepb.plantpb.PlantService"
	loginMethod  = "Login"
	landsMethod  = "AllLands"

	defaultOrigin    = "https://gate-obt.nqf.qq.com"
	defaultUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) " +
		"AppleWebKit/537.36 (KHTML, like Gecko) Chrome/132.0.0.0 Safari/537.36 " +
		"MicroMessenger/7.0.20.1781(0x6700143B) NetType/WIFI MiniProgramEnv/Windows " +
		"WindowsWechat/WMPF WindowsWechat(0x63090a13)"
)

var ErrOffline = errors.New("game session is offline")

var _ SecurityRuntime = (*tsdk.Runtime)(nil)

// SecurityRuntime is the combined P2-02 surface needed during login and by
// the account-local ACE service. *tsdk.Runtime implements this interface.
type SecurityRuntime interface {
	ace.Runtime
	Init(context.Context) error
	Destroy() error
	BindUser(string) error
	GetEncryptedInitInfo() (string, error)
	Encrypt([]byte) ([]byte, error)
	Decrypt([]byte) ([]byte, error)
}

// Options supplies account identity and protocol overrides. Zero values use
// the process configuration. Runtime and ACEIntervals are primarily useful to
// deterministic tests; production callers normally provide AccountID and UIN.
type Options struct {
	AccountID     string
	UIN           string
	GatewayURL    string
	ClientVersion string
	Platform      string
	OS            string
	UserAgent     string
	LoginDevice   *pb.DeviceInfo
	TSDK          tsdk.Options
	Runtime       SecurityRuntime
	Transport     transport.Options
	ACEIntervals  ace.Intervals
	Logger        func(level, message string)
}

// Session owns the authenticated transport, TSDK runtime, and ACE lifecycle
// for exactly one account. Close must be called when the account stops.
type Session struct {
	OpenID string
	UIN    string
	GID    int64

	mu            sync.RWMutex
	online        bool
	terminated    bool
	state         transport.UserState
	clientVersion string
	initialLands  *pb.AllLandsReply

	client  *transport.Client
	runtime SecurityRuntime
	ace     *ace.Service
	cancel  context.CancelFunc

	closeOnce sync.Once
	closeErr  error
}

// Login authenticates code against the configured game gateway.
func Login(ctx context.Context, code string) (*Session, error) {
	return LoginWithOptions(ctx, code, Options{})
}

// LoginWithOptions is Login with account-scoped dependency and protocol
// configuration. It exists so the account Runtime can preserve stable account
// identity while the two-argument Login API remains the default entry point.
func LoginWithOptions(ctx context.Context, code string, options Options) (_ *Session, err error) {
	if ctx == nil {
		ctx = context.Background()
	}
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, errors.New("login code is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	resolved, err := resolveOptions(code, options)
	if err != nil {
		return nil, err
	}
	lifecycleCtx, cancel := context.WithCancel(ctx)
	session := &Session{UIN: strings.TrimSpace(resolved.UIN), cancel: cancel}
	defer func() {
		if err != nil {
			_ = session.Close()
		}
	}()

	runtime := resolved.Runtime
	if runtime == nil {
		runtime = tsdk.New(lifecycleCtx, resolved.TSDK)
	}
	session.runtime = runtime
	if err := runtime.Init(lifecycleCtx); err != nil {
		return nil, fmt.Errorf("initialize TSDK: %w", err)
	}

	gatewayURL, err := buildGatewayURL(resolved, code)
	if err != nil {
		return nil, err
	}
	transportOptions := resolved.Transport
	transportOptions.Cipher = runtime
	transportOptions.InitialAuthToken = ""
	transportOptions.Header = loginHeaders(transportOptions.Header, resolved.UserAgent)
	previousOnEvent := transportOptions.OnEvent
	transportOptions.OnEvent = func(event transport.Event) {
		if previousOnEvent != nil {
			previousOnEvent(event)
		}
		if event.Type == transport.EventDisconnect || event.Type == transport.EventKickout {
			if session.markOffline() {
				go func() { _ = session.Close() }()
			}
		}
	}
	client, err := transport.Dial(lifecycleCtx, gatewayURL, transportOptions)
	if err != nil {
		return nil, err
	}
	session.client = client

	loginReply, err := sendLogin(lifecycleCtx, client, resolved)
	if err != nil {
		return nil, err
	}
	basic := loginReply.GetBasic()
	if basic == nil {
		return nil, errors.New("login reply has no basic account information")
	}
	openID := strings.TrimSpace(basic.GetOpenId())
	if openID == "" {
		return nil, errors.New("login reply has no openid")
	}
	if err := runtime.BindUser(openID); err != nil {
		return nil, fmt.Errorf("bind TSDK user: %w", err)
	}
	if err := client.UseEncryptedInitInfo(runtime); err != nil {
		return nil, err
	}

	state := transport.UserState{
		GID:    basic.GetGid(),
		Name:   basic.GetName(),
		Level:  basic.GetLevel(),
		Gold:   basic.GetGold(),
		Exp:    basic.GetExp(),
		OpenID: openID,
		Avatar: basic.GetAvatarUrl(),
	}
	client.UpdateState(state)
	lands, err := requestInitialLands(lifecycleCtx, client)
	if err != nil {
		return nil, err
	}

	if !session.activate(openID, resolved.ClientVersion, state, lands) {
		return nil, ErrOffline
	}

	aceService, err := ace.New(ace.Options{
		Runtime:       runtime,
		Sender:        aceTransportSender{client: client},
		IsConnected:   session.Online,
		UserHeartbeat: session.heartbeat,
		Logger:        ace.Logger(resolved.Logger),
		Intervals:     resolved.ACEIntervals,
	})
	if err != nil {
		return nil, fmt.Errorf("create ACE service: %w", err)
	}
	session.ace = aceService
	if err := aceService.Start(lifecycleCtx); err != nil {
		return nil, fmt.Errorf("start ACE service: %w", err)
	}
	if !session.Online() {
		return nil, ErrOffline
	}

	go func() {
		<-lifecycleCtx.Done()
		_ = session.Close()
	}()
	return session, nil
}

func resolveOptions(code string, options Options) (Options, error) {
	processConfig := config.Load()
	if options.GatewayURL == "" {
		options.GatewayURL = processConfig.ServerURL
	}
	if options.ClientVersion == "" {
		options.ClientVersion = processConfig.ClientVersion
	}
	if options.Platform == "" {
		options.Platform = processConfig.Platform
	}
	if options.OS == "" {
		options.OS = processConfig.OS
	}
	if options.UserAgent == "" {
		options.UserAgent = defaultUserAgent
	}
	if options.AccountID == "" {
		options.AccountID = strings.TrimSpace(options.UIN)
	}
	if options.AccountID == "" {
		digest := sha256.Sum256([]byte(code))
		options.AccountID = fmt.Sprintf("session-%x", digest[:8])
	}
	if options.TSDK.AccountID == "" {
		options.TSDK.AccountID = options.AccountID
	}
	if options.TSDK.GameID == 0 {
		options.TSDK.GameID = uint32(processConfig.TSDK.GameID)
	}
	if options.TSDK.AppKey == "" {
		options.TSDK.AppKey = processConfig.TSDK.AppKey
	}
	if options.TSDK.Enabled == nil {
		enabled := processConfig.TSDK.AceEnabled
		options.TSDK.Enabled = &enabled
	}
	if options.TSDK.DataDir == "" {
		options.TSDK.DataDir = filepath.Join(processConfig.DataDir, "tsdk", options.TSDK.AccountID)
	}
	if options.TSDK.Logger == nil && options.Logger != nil {
		options.TSDK.Logger = tsdk.Logger(options.Logger)
	}
	return options, nil
}

func buildGatewayURL(options Options, code string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(options.GatewayURL))
	if err != nil {
		return "", fmt.Errorf("parse game gateway URL: %w", err)
	}
	if parsed.Scheme != "ws" && parsed.Scheme != "wss" {
		return "", fmt.Errorf("game gateway URL requires ws or wss scheme: %q", parsed.Scheme)
	}
	if parsed.Host == "" {
		return "", errors.New("game gateway URL has no host")
	}
	query := parsed.Query()
	query.Set("platform", options.Platform)
	query.Set("os", options.OS)
	query.Set("ver", options.ClientVersion)
	query.Set("code", code)
	query.Set("openID", "")
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func loginHeaders(existing http.Header, userAgent string) http.Header {
	headers := existing.Clone()
	if headers == nil {
		headers = make(http.Header)
	}
	if headers.Get("User-Agent") == "" {
		headers.Set("User-Agent", userAgent)
	}
	if headers.Get("Origin") == "" {
		headers.Set("Origin", defaultOrigin)
	}
	return headers
}

func sendLogin(ctx context.Context, client *transport.Client, options Options) (*pb.LoginReply, error) {
	device := options.LoginDevice
	if device == nil {
		device = &pb.DeviceInfo{
			ClientVersion: options.ClientVersion,
			SysSoftware:   "iOS 26.2.1",
			Network:       "wifi",
			Memory:        7672,
			DeviceId:      "iPhone X<iPhone18,3>",
		}
	} else {
		device = proto.Clone(device).(*pb.DeviceInfo)
		if device.ClientVersion == "" {
			device.ClientVersion = options.ClientVersion
		}
	}
	request := &pb.LoginRequest{
		DeviceInfo: device,
		SceneId:    "1256",
		ReportData: &pb.ReportData{MinigameChannel: "other", MinigamePlatid: 2},
	}
	response, err := client.SendMsg(ctx, transport.Command{
		ServiceName: userService,
		MethodName:  loginMethod,
		Response:    new(pb.LoginReply),
	}, request)
	if err != nil {
		return nil, fmt.Errorf("game login handshake: %w", err)
	}
	reply, ok := response.(*pb.LoginReply)
	if !ok {
		return nil, fmt.Errorf("game login returned %T, want *pb.LoginReply", response)
	}
	return reply, nil
}

func requestInitialLands(ctx context.Context, client *transport.Client) (*pb.AllLandsReply, error) {
	response, err := client.SendMsg(ctx, transport.Command{
		ServiceName: plantService,
		MethodName:  landsMethod,
		Response:    new(pb.AllLandsReply),
	}, &pb.AllLandsRequest{})
	if err != nil {
		return nil, fmt.Errorf("request initial AllLands: %w", err)
	}
	reply, ok := response.(*pb.AllLandsReply)
	if !ok {
		return nil, fmt.Errorf("initial AllLands returned %T, want *pb.AllLandsReply", response)
	}
	return reply, nil
}

func (s *Session) heartbeat(ctx context.Context) error {
	if !s.Online() {
		return ErrOffline
	}
	_, err := s.client.SendMsg(ctx, transport.Command{
		ServiceName: userService,
		MethodName:  "Heartbeat",
		Response:    new(pb.HeartbeatReply),
	}, &pb.HeartbeatRequest{Gid: s.GID, ClientVersion: s.clientVersion})
	return err
}

// Online reports whether the login handshake is complete and the underlying
// gateway has not disconnected or kicked the account.
func (s *Session) Online() bool {
	if s == nil {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.online
}

// IsOnline is an explicit predicate alias for callers that prefer boolean
// method naming over the concise Online form.
func (s *Session) IsOnline() bool { return s.Online() }

func (s *Session) markOffline() bool {
	s.mu.Lock()
	wasOnline := s.online
	s.online = false
	s.terminated = true
	s.mu.Unlock()
	return wasOnline
}

func (s *Session) activate(openID, clientVersion string, state transport.UserState, lands *pb.AllLandsReply) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.terminated {
		return false
	}
	s.OpenID = openID
	s.GID = state.GID
	s.state = state
	s.clientVersion = clientVersion
	s.initialLands = lands
	s.online = true
	return true
}

// State returns the account state captured from LoginReply.
func (s *Session) State() transport.UserState {
	if s == nil {
		return transport.UserState{}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state
}

// InitialLands returns a copy of the AllLands reply that consumed the
// one-time TSDK initialization credential.
func (s *Session) InitialLands() *pb.AllLandsReply {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.initialLands == nil {
		return nil
	}
	return proto.Clone(s.initialLands).(*pb.AllLandsReply)
}

// SendMsg exposes the authenticated account connection to typed domain
// services without allowing them to own the transport lifecycle.
func (s *Session) SendMsg(ctx context.Context, command transport.Command, request proto.Message) (proto.Message, error) {
	if s == nil || !s.Online() {
		return nil, ErrOffline
	}
	return s.client.SendMsg(ctx, command, request)
}

// SendMsgRaw preserves opaque protobuf bodies for protocol migrations that do
// not yet have a generated response type.
func (s *Session) SendMsgRaw(ctx context.Context, command transport.Command, request proto.Message) ([]byte, *pb.Meta, error) {
	if s == nil || !s.Online() {
		return nil, nil, ErrOffline
	}
	return s.client.SendMsgRaw(ctx, command, request)
}

// Events exposes gateway notifications while ownership remains with Session.
func (s *Session) Events() <-chan transport.Event {
	if s == nil || s.client == nil {
		return nil
	}
	return s.client.Events()
}

// ACEStatus returns a race-free snapshot of this session's ACE lifecycle.
func (s *Session) ACEStatus() ace.Status {
	if s == nil || s.ace == nil {
		return ace.Status{}
	}
	return s.ace.Status()
}

// Close stops ACE before releasing the gateway and TSDK instance. It is safe
// to call repeatedly and is also triggered when ctx is canceled.
func (s *Session) Close() error {
	if s == nil {
		return nil
	}
	s.closeOnce.Do(func() {
		s.markOffline()
		if s.cancel != nil {
			s.cancel()
		}
		if s.ace != nil {
			s.ace.Stop()
		}
		if s.client != nil {
			s.closeErr = s.client.Close()
		}
		if s.runtime != nil {
			if err := s.runtime.Destroy(); s.closeErr == nil {
				s.closeErr = err
			}
		}
	})
	return s.closeErr
}

type aceTransportSender struct {
	client *transport.Client
}

var _ ace.Sender = aceTransportSender{}

func (s aceTransportSender) Send(ctx context.Context, service, method string, body []byte, timeout time.Duration) ([]byte, error) {
	var request pb.AntiDataRequest
	if err := proto.Unmarshal(body, &request); err != nil {
		return nil, fmt.Errorf("decode ACE request: %w", err)
	}
	reply, _, err := s.client.SendMsgRaw(ctx, transport.Command{
		ServiceName: service,
		MethodName:  method,
		Timeout:     timeout,
	}, &request)
	return reply, err
}
