// Package realtime provides the account-scoped WebSocket event hub used by
// the HTTP API. It intentionally speaks a small JSON envelope instead of the
// Socket.IO protocol so the frontend can migrate to a native WebSocket client.
package realtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/RoseKhlifa/FarmBot/internal/httpapi/middleware"
	"github.com/RoseKhlifa/FarmBot/internal/store"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

const (
	StatusUpdateEvent        = "status:update"
	LogNewEvent              = "log:new"
	AccountLogNewEvent       = "account-log:new"
	LogsSnapshotEvent        = "logs:snapshot"
	AccountLogsSnapshotEvent = "account-logs:snapshot"
	SubscribedEvent          = "subscribed"
	ReadyEvent               = "ready"
	PongEvent                = "pong"
	allAccountsGroup         = "all"
	defaultLogSnapshotLimit  = 100
)

var (
	ErrHubClosed           = errors.New("realtime hub is closed")
	ErrUnauthorized        = errors.New("realtime WebSocket authorization failed")
	ErrAccountAccessDenied = errors.New("realtime account access denied")
	ErrAccountRequired     = errors.New("realtime account subscription is required")
)

// SnapshotProvider supplies the initial state sent after a successful
// subscription. The callbacks return any JSON-marshalable value so the HTTP
// layer does not need to depend on one concrete log or provider type.
type SnapshotProvider struct {
	Status      func(context.Context, string) (any, error)
	Logs        func(context.Context, string, int) (any, error)
	AccountLogs func(context.Context, int) (any, error)
}

// Config contains all account-local collaborators for a Hub. Sessions and
// account authorization are injected from P6-02; no token or account state is
// kept in package globals.
type Config struct {
	Sessions         *middleware.SessionManager
	SessionManager   *middleware.SessionManager
	AccountAccess    middleware.AccountAccessConfig
	ResolveAccount   func(context.Context, string) (string, error)
	AuthorizeAccount func(context.Context, store.User, string) (bool, error)
	Snapshot         SnapshotProvider

	CheckOrigin      func(*http.Request) bool
	ReadLimit        int64
	WriteWait        time.Duration
	PingPeriod       time.Duration
	LogSnapshotLimit int
}

type envelope struct {
	Event string `json:"event"`
	Data  any    `json:"data"`
}

type inboundEnvelope struct {
	Event     string          `json:"event"`
	Data      json.RawMessage `json:"data"`
	AccountID string          `json:"accountId"`
}

type subscribeData struct {
	AccountID string `json:"accountId"`
}

type client struct {
	hub  *Hub
	conn *websocket.Conn
	user store.User

	writeMu   sync.Mutex
	done      chan struct{}
	closeOnce sync.Once

	// subscription is empty when the client is subscribed to the all group.
	subscription string
}

// Hub owns all realtime connections for one HTTP application instance.
// groups maps a canonical account ID to clients; the special "all" group
// receives broadcasts for every account and is available only to elevated
// sessions.
type Hub struct {
	mu      sync.RWMutex
	clients map[*client]struct{}
	groups  map[string]map[*client]struct{}
	closed  bool
	config  Config
}

// NewHub creates an idle account-scoped realtime hub.
func NewHub(cfg Config) *Hub {
	if cfg.ReadLimit <= 0 {
		cfg.ReadLimit = 64 << 10
	}
	if cfg.WriteWait <= 0 {
		cfg.WriteWait = 10 * time.Second
	}
	if cfg.PingPeriod <= 0 {
		cfg.PingPeriod = 30 * time.Second
	}
	if cfg.LogSnapshotLimit <= 0 {
		cfg.LogSnapshotLimit = defaultLogSnapshotLimit
	}
	return &Hub{
		clients: make(map[*client]struct{}),
		groups:  make(map[string]map[*client]struct{}),
		config:  cfg,
	}
}

// New is a short constructor alias used by composition roots.
func New(cfg Config) *Hub { return NewHub(cfg) }

// RegisterRoutes installs the native WebSocket endpoint. The server skeleton
// intentionally remains unaware of this package; app wiring calls this method
// through ServerOptions.RegisterRoutes.
func (h *Hub) RegisterRoutes(router gin.IRoutes) {
	if h == nil || router == nil {
		return
	}
	router.GET("/ws", func(c *gin.Context) {
		h.HandleWebSocket(c.Writer, c.Request)
	})
}

// Register is a concise alias for RegisterRoutes.
func (h *Hub) Register(router gin.IRoutes) { h.RegisterRoutes(router) }

// RegisterRoutes is also available as a package-level helper for simple
// composition roots.
func RegisterRoutes(router gin.IRoutes, h *Hub) {
	if h != nil {
		h.RegisterRoutes(router)
	}
}

// Handler returns an http.Handler for tests or non-Gin composition roots.
func (h *Hub) Handler() http.Handler { return http.HandlerFunc(h.HandleWebSocket) }

// ServeHTTP lets Hub satisfy net/http.Handler directly.
func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.HandleWebSocket(w, r)
}

// HandleWebSocket authenticates, upgrades, subscribes and serves one client.
func (h *Hub) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	if h == nil {
		http.Error(w, ErrHubClosed.Error(), http.StatusServiceUnavailable)
		return
	}
	if err := h.isOpen(); err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	user, err := h.authenticate(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	requested := requestAccountID(r)
	subscription, err := h.resolveSubscription(r.Context(), user, requested)
	if err != nil {
		status := http.StatusForbidden
		if errors.Is(err, ErrAccountRequired) {
			status = http.StatusBadRequest
		}
		http.Error(w, err.Error(), status)
		return
	}

	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin:     h.config.CheckOrigin,
	}
	if upgrader.CheckOrigin == nil {
		upgrader.CheckOrigin = func(*http.Request) bool { return true }
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	connection := &client{hub: h, conn: conn, user: user, done: make(chan struct{})}
	h.addClient(connection, subscription)
	if err := connection.send(SubscribedEvent, map[string]any{"accountId": displaySubscription(subscription)}); err != nil {
		connection.close()
		return
	}
	h.sendSnapshots(connection, subscription)
	if err := connection.send(ReadyEvent, map[string]any{"ok": true, "ts": time.Now().UnixMilli()}); err != nil {
		connection.close()
		return
	}
	connection.serve()
}

// HandleWS is the short name used by some composition roots.
func (h *Hub) HandleWS(w http.ResponseWriter, r *http.Request) {
	h.HandleWebSocket(w, r)
}

// Broadcast sends an event to the exact account group and the elevated
// all-accounts group. It returns the number of client writes attempted.
func (h *Hub) Broadcast(accountID, event string, payload any) int {
	if h == nil || strings.TrimSpace(accountID) == "" || strings.TrimSpace(event) == "" {
		return 0
	}
	clients := h.clientsFor(accountID)
	count := 0
	for _, connection := range clients {
		if connection.send(event, payload) == nil {
			count++
		} else {
			connection.close()
		}
	}
	return count
}

// BroadcastStatus preserves the legacy status:update payload shape.
func (h *Hub) BroadcastStatus(accountID string, status any) int {
	accountID = strings.TrimSpace(accountID)
	return h.Broadcast(accountID, StatusUpdateEvent, map[string]any{
		"accountId": accountID,
		"status":    status,
	})
}

// BroadcastLog publishes a log entry whose account ID is carried by the
// entry's accountId/account_id/id field.
func (h *Hub) BroadcastLog(entry any) int {
	return h.broadcastEntry(LogNewEvent, entry)
}

// BroadcastAccountLog publishes the account-log stream with the legacy event
// name and payload unchanged.
func (h *Hub) BroadcastAccountLog(entry any) int {
	return h.broadcastEntry(AccountLogNewEvent, entry)
}

// PublishStatus/PublishLog/PublishAccountLog are descriptive aliases for
// runtime and logger integrations.
func (h *Hub) PublishStatus(accountID string, status any) int {
	return h.BroadcastStatus(accountID, status)
}

func (h *Hub) PublishLog(entry any) int { return h.BroadcastLog(entry) }

func (h *Hub) PublishAccountLog(entry any) int { return h.BroadcastAccountLog(entry) }

// ConnectionCount reports the number of currently registered clients.
func (h *Hub) ConnectionCount() int {
	if h == nil {
		return 0
	}
	h.mu.RLock()
	count := len(h.clients)
	h.mu.RUnlock()
	return count
}

// SubscriberCount reports the number of clients in one account group. The
// special "all" value reports the all-accounts group.
func (h *Hub) SubscriberCount(accountID string) int {
	if h == nil {
		return 0
	}
	group := strings.TrimSpace(accountID)
	if group == "" {
		group = allAccountsGroup
	}
	h.mu.RLock()
	count := len(h.groups[group])
	h.mu.RUnlock()
	return count
}

// Close stops accepting new clients and closes every active connection.
func (h *Hub) Close() error {
	if h == nil {
		return nil
	}
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return nil
	}
	h.closed = true
	clients := make([]*client, 0, len(h.clients))
	for connection := range h.clients {
		clients = append(clients, connection)
	}
	h.mu.Unlock()
	for _, connection := range clients {
		connection.close()
	}
	return nil
}

func (h *Hub) isOpen() error {
	h.mu.RLock()
	closed := h.closed
	h.mu.RUnlock()
	if closed {
		return ErrHubClosed
	}
	return nil
}

func (h *Hub) authenticate(r *http.Request) (store.User, error) {
	sessions := h.config.Sessions
	if sessions == nil {
		sessions = h.config.SessionManager
	}
	if sessions == nil {
		return store.User{}, ErrUnauthorized
	}
	token := requestToken(r)
	if token == "" {
		return store.User{}, ErrUnauthorized
	}
	session, err := sessions.Lookup(r.Context(), token)
	if err != nil {
		return store.User{}, fmt.Errorf("lookup realtime session: %w", err)
	}
	return session.User, nil
}

func (h *Hub) resolveSubscription(ctx context.Context, user store.User, requested string) (string, error) {
	requested = strings.TrimSpace(requested)
	if requested == "" || strings.EqualFold(requested, allAccountsGroup) {
		if middleware.HasAdminRole(user.Role) {
			return "", nil
		}
		return "", ErrAccountRequired
	}

	canonical := requested
	resolver := h.config.ResolveAccount
	if resolver == nil {
		resolver = h.config.AccountAccess.Resolve
	}
	if resolver != nil {
		resolved, err := resolver(ctx, requested)
		if err != nil {
			return "", fmt.Errorf("resolve realtime account: %w", err)
		}
		if strings.TrimSpace(resolved) != "" {
			canonical = strings.TrimSpace(resolved)
		}
	} else {
		repo := h.config.AccountAccess.Repo
		if repo == nil {
			repo = h.config.AccountAccess.AccountRepo
		}
		if repo != nil {
			accounts, err := repo.List(ctx)
			if err != nil {
				return "", fmt.Errorf("list realtime accounts: %w", err)
			}
			if resolved := middleware.ResolveAccountReference(accounts, requested); resolved != "" {
				canonical = resolved
			}
		}
	}
	if middleware.HasAdminRole(user.Role) {
		return canonical, nil
	}

	allowed := h.config.AuthorizeAccount
	if allowed == nil {
		allowed = h.config.AccountAccess.CanAccess
	}
	if allowed != nil {
		ok, err := allowed(ctx, user, canonical)
		if err != nil {
			return "", fmt.Errorf("check realtime account access: %w", err)
		}
		if ok {
			return canonical, nil
		}
		return "", ErrAccountAccessDenied
	}
	if h.config.AccountAccess.AccountsForUser != nil {
		accounts, err := h.config.AccountAccess.AccountsForUser(ctx, user.Username)
		if err != nil {
			return "", fmt.Errorf("list user realtime accounts: %w", err)
		}
		if middleware.ResolveAccountReference(accounts, canonical) == canonical {
			return canonical, nil
		}
		return "", ErrAccountAccessDenied
	}
	repo := h.config.AccountAccess.Repo
	if repo == nil {
		repo = h.config.AccountAccess.AccountRepo
	}
	if repo != nil {
		accounts, err := repo.GetByUser(ctx, user.Username)
		if err != nil {
			return "", fmt.Errorf("list user realtime accounts: %w", err)
		}
		if middleware.ResolveAccountReference(accounts, canonical) == canonical {
			return canonical, nil
		}
	}
	return "", ErrAccountAccessDenied
}

func (h *Hub) addClient(connection *client, subscription string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		connection.close()
		return
	}
	connection.subscription = subscription
	h.clients[connection] = struct{}{}
	h.addToGroupLocked(connection, subscription)
}

func (h *Hub) replaceSubscription(connection *client, subscription string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.removeFromGroupLocked(connection)
	connection.subscription = subscription
	h.addToGroupLocked(connection, subscription)
}

func (h *Hub) addToGroupLocked(connection *client, subscription string) {
	group := subscription
	if group == "" {
		group = allAccountsGroup
	}
	if h.groups[group] == nil {
		h.groups[group] = make(map[*client]struct{})
	}
	h.groups[group][connection] = struct{}{}
}

func (h *Hub) removeClient(connection *client) {
	h.mu.Lock()
	delete(h.clients, connection)
	h.removeFromGroupLocked(connection)
	h.mu.Unlock()
}

func (h *Hub) removeFromGroupLocked(connection *client) {
	group := connection.subscription
	if group == "" {
		group = allAccountsGroup
	}
	if members := h.groups[group]; members != nil {
		delete(members, connection)
		if len(members) == 0 {
			delete(h.groups, group)
		}
	}
}

func (h *Hub) clientsFor(accountID string) []*client {
	h.mu.RLock()
	defer h.mu.RUnlock()
	seen := make(map[*client]struct{})
	clients := make([]*client, 0)
	for _, group := range []string{strings.TrimSpace(accountID), allAccountsGroup} {
		for connection := range h.groups[group] {
			if _, exists := seen[connection]; exists {
				continue
			}
			seen[connection] = struct{}{}
			clients = append(clients, connection)
		}
	}
	return clients
}

func (h *Hub) broadcastEntry(event string, entry any) int {
	accountID := accountIDFromPayload(entry)
	if accountID == "" {
		return 0
	}
	return h.Broadcast(accountID, event, entry)
}

func (h *Hub) sendSnapshots(connection *client, subscription string) {
	provider := h.config.Snapshot
	ctx := context.Background()
	displayID := displaySubscription(subscription)
	if subscription != "" && provider.Status != nil {
		if status, err := provider.Status(ctx, subscription); err == nil {
			_ = connection.send(StatusUpdateEvent, map[string]any{"accountId": subscription, "status": status})
		}
	}
	if provider.Logs != nil {
		if logs, err := provider.Logs(ctx, subscription, h.config.LogSnapshotLimit); err == nil {
			_ = connection.send(LogsSnapshotEvent, map[string]any{"accountId": displayID, "logs": logs})
		}
	}
	if provider.AccountLogs != nil {
		if logs, err := provider.AccountLogs(ctx, h.config.LogSnapshotLimit); err == nil {
			_ = connection.send(AccountLogsSnapshotEvent, map[string]any{"logs": logs})
		}
	}
}

func (c *client) serve() {
	defer c.close()
	defer c.hub.removeClient(c)
	c.conn.SetReadLimit(c.hub.config.ReadLimit)
	if c.hub.config.PingPeriod > 0 {
		_ = c.conn.SetReadDeadline(time.Now().Add(c.hub.config.PingPeriod * 2))
		c.conn.SetPongHandler(func(string) error {
			return c.conn.SetReadDeadline(time.Now().Add(c.hub.config.PingPeriod * 2))
		})
		go c.pingLoop()
	}
	for {
		messageType, data, err := c.conn.ReadMessage()
		if err != nil {
			return
		}
		if messageType != websocket.TextMessage && messageType != websocket.BinaryMessage {
			continue
		}
		c.handle(data)
	}
}

func (c *client) pingLoop() {
	ticker := time.NewTicker(c.hub.config.PingPeriod)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			c.writeMu.Lock()
			_ = c.conn.SetWriteDeadline(time.Now().Add(c.hub.config.WriteWait))
			err := c.conn.WriteMessage(websocket.PingMessage, nil)
			c.writeMu.Unlock()
			if err != nil {
				c.close()
				return
			}
		case <-c.done:
			return
		}
	}
}

func (c *client) handle(data []byte) {
	var frame inboundEnvelope
	if err := json.Unmarshal(data, &frame); err != nil {
		return
	}
	switch strings.TrimSpace(frame.Event) {
	case "subscribe":
		requested := strings.TrimSpace(frame.AccountID)
		if len(frame.Data) > 0 && string(frame.Data) != "null" {
			var payload subscribeData
			if json.Unmarshal(frame.Data, &payload) == nil && strings.TrimSpace(payload.AccountID) != "" {
				requested = strings.TrimSpace(payload.AccountID)
			}
		}
		subscription, err := c.hub.resolveSubscription(context.Background(), c.user, requested)
		if err != nil {
			_ = c.send(SubscribedEvent, map[string]any{"accountId": displaySubscription(c.currentSubscription()), "error": err.Error()})
			return
		}
		c.hub.replaceSubscription(c, subscription)
		_ = c.send(SubscribedEvent, map[string]any{"accountId": displaySubscription(subscription)})
		c.hub.sendSnapshots(c, subscription)
	case "ping":
		_ = c.send(PongEvent, map[string]any{"ts": time.Now().UnixMilli()})
	}
}

func (c *client) currentSubscription() string {
	c.hub.mu.RLock()
	subscription := c.subscription
	c.hub.mu.RUnlock()
	return subscription
}

func (c *client) send(event string, payload any) error {
	if c == nil || c.conn == nil {
		return ErrHubClosed
	}
	data, err := json.Marshal(envelope{Event: event, Data: payload})
	if err != nil {
		return fmt.Errorf("encode realtime event %q: %w", event, err)
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if err := c.conn.SetWriteDeadline(time.Now().Add(c.hub.config.WriteWait)); err != nil {
		return err
	}
	return c.conn.WriteMessage(websocket.TextMessage, data)
}

func (c *client) close() {
	if c == nil {
		return
	}
	c.closeOnce.Do(func() {
		close(c.done)
		_ = c.conn.Close()
	})
}

func requestToken(r *http.Request) string {
	if r == nil {
		return ""
	}
	if token := strings.TrimSpace(r.Header.Get(middleware.AdminTokenHeader)); token != "" {
		return token
	}
	if r.URL == nil {
		return ""
	}
	for _, key := range []string{"token", "adminToken", "admin_token"} {
		if token := strings.TrimSpace(r.URL.Query().Get(key)); token != "" {
			return token
		}
	}
	return ""
}

func requestAccountID(r *http.Request) string {
	if r == nil {
		return ""
	}
	if accountID := strings.TrimSpace(r.Header.Get(middleware.AccountIDHeader)); accountID != "" {
		return accountID
	}
	if r.URL == nil {
		return ""
	}
	for _, key := range []string{"accountId", "account_id", "account"} {
		if accountID := strings.TrimSpace(r.URL.Query().Get(key)); accountID != "" {
			return accountID
		}
	}
	return ""
}

func displaySubscription(subscription string) string {
	if strings.TrimSpace(subscription) == "" {
		return allAccountsGroup
	}
	return strings.TrimSpace(subscription)
}

func accountIDFromPayload(payload any) string {
	if payload == nil {
		return ""
	}
	if values, ok := payload.(map[string]any); ok {
		for _, key := range []string{"accountId", "account_id", "id"} {
			if value, exists := values[key]; exists {
				if accountID := strings.TrimSpace(fmt.Sprint(value)); accountID != "" && accountID != "<nil>" {
					return accountID
				}
			}
		}
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	var values map[string]any
	if json.Unmarshal(data, &values) != nil {
		return ""
	}
	for _, key := range []string{"accountId", "account_id", "id"} {
		if value, exists := values[key]; exists {
			if accountID := strings.TrimSpace(fmt.Sprint(value)); accountID != "" && accountID != "<nil>" {
				return accountID
			}
		}
	}
	return ""
}
