package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/RoseKhlifa/FarmBot/internal/store"
)

const (
	DefaultSessionPersistenceKey = "httpapi_admin_sessions"
	DefaultSessionCleanupPeriod  = 5 * time.Minute
)

var (
	ErrSessionNotFound = errors.New("admin session not found")
	ErrSessionBanned   = errors.New("admin session user is banned")
	ErrSessionExpired  = errors.New("admin session user is expired")
	ErrSessionManager  = errors.New("admin session manager is closed")
)

// Session is the authenticated identity associated with one admin token.
// User is copied at creation and refreshed from UserRepo when configured.
type Session struct {
	Token      string     `json:"token"`
	User       store.User `json:"user"`
	CreatedAt  time.Time  `json:"createdAt"`
	LastSeenAt time.Time  `json:"lastSeenAt"`
}

// SessionManagerConfig controls the account-local session manager. ConfigRepo
// is optional; when present, sessions survive process restarts in SQLite.
type SessionManagerConfig struct {
	ConfigRepo     store.ConfigRepo
	UserRepo       store.UserRepo
	PersistenceKey string
	Clock          func() time.Time
	CleanupPeriod  time.Duration
}

// SessionManager owns session state for one HTTP application instance. It is
// deliberately not package-global so multiple servers and tests stay isolated.
type SessionManager struct {
	mu             sync.RWMutex
	sessions       map[string]Session
	config         store.ConfigRepo
	users          store.UserRepo
	persistenceKey string
	now            func() time.Time
	cleanupPeriod  time.Duration
	stop           chan struct{}
	done           chan struct{}
	closeOnce      sync.Once
}

type persistedSessions struct {
	Sessions []persistedSession `json:"sessions"`
}

// persistedUser intentionally excludes password hashes and salts. A session
// needs authorization state after restart, never credential material.
type persistedUser struct {
	Username           string `json:"username"`
	Role               string `json:"role"`
	Status             string `json:"status"`
	ExpireAt           *int64 `json:"expireAt,omitempty"`
	AccountLimit       int    `json:"accountLimit,omitempty"`
	CardCode           string `json:"cardCode,omitempty"`
	CardJSON           string `json:"cardJson,omitempty"`
	MustChangePassword bool   `json:"mustChangePassword,omitempty"`
}

type persistedSession struct {
	Token      string        `json:"token"`
	User       persistedUser `json:"user"`
	CreatedAt  time.Time     `json:"createdAt"`
	LastSeenAt time.Time     `json:"lastSeenAt"`
}

// NewSessionManager creates a session manager and best-effort loads persisted
// sessions. A missing or malformed optional persistence record is treated as
// an empty session set so a stale database value cannot prevent server start.
func NewSessionManager(cfg SessionManagerConfig) (*SessionManager, error) {
	clock := cfg.Clock
	if clock == nil {
		clock = time.Now
	}
	key := strings.TrimSpace(cfg.PersistenceKey)
	if key == "" {
		key = DefaultSessionPersistenceKey
	}
	period := cfg.CleanupPeriod
	if period <= 0 {
		period = DefaultSessionCleanupPeriod
	}
	m := &SessionManager{
		sessions:       make(map[string]Session),
		config:         cfg.ConfigRepo,
		users:          cfg.UserRepo,
		persistenceKey: key,
		now:            clock,
		cleanupPeriod:  period,
	}
	if err := m.load(context.Background()); err != nil {
		return nil, err
	}
	return m, nil
}

// NewSessionStore is an alias retained for callers that prefer store naming.
func NewSessionStore(cfg SessionManagerConfig) (*SessionManager, error) {
	return NewSessionManager(cfg)
}

// GenerateAdminToken returns a 24-byte cryptographically random token encoded
// as lowercase hexadecimal, matching the Node session token entropy.
func GenerateAdminToken() (string, error) {
	bytes := make([]byte, 24)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate admin token: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

// Create stores an authenticated user and returns the bearer token.
func (m *SessionManager) Create(ctx context.Context, user store.User) (string, error) {
	if m == nil {
		return "", ErrSessionManager
	}
	if strings.TrimSpace(user.Username) == "" {
		return "", errors.New("admin session user is missing a username")
	}
	if err := validateSessionUser(user, m.now()); err != nil {
		return "", err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	for attempt := 0; attempt < 4; attempt++ {
		token, err := GenerateAdminToken()
		if err != nil {
			return "", err
		}
		session := Session{Token: token, User: user, CreatedAt: m.now(), LastSeenAt: m.now()}
		m.mu.Lock()
		if _, exists := m.sessions[token]; exists {
			m.mu.Unlock()
			continue
		}
		m.sessions[token] = session
		m.mu.Unlock()
		if err := m.persist(ctx); err != nil {
			m.mu.Lock()
			delete(m.sessions, token)
			m.mu.Unlock()
			return "", err
		}
		return token, nil
	}
	return "", errors.New("unable to allocate unique admin session token")
}

func (m *SessionManager) CreateSession(ctx context.Context, user store.User) (string, error) {
	return m.Create(ctx, user)
}

func (m *SessionManager) CreateAdminSession(ctx context.Context, user store.User) (string, error) {
	return m.Create(ctx, user)
}

func (m *SessionManager) HasToken(token string) bool {
	_, ok := m.Get(token)
	return ok
}

func (m *SessionManager) GetSession(token string) (Session, bool) {
	return m.Get(token)
}

// Get returns a snapshot without refreshing the backing user repository.
// Middleware should use Lookup so disabled or expired users are invalidated.
func (m *SessionManager) Get(token string) (Session, bool) {
	if m == nil {
		return Session{}, false
	}
	m.mu.RLock()
	session, ok := m.sessions[strings.TrimSpace(token)]
	m.mu.RUnlock()
	return session, ok
}

// Lookup validates a token and refreshes the user snapshot when UserRepo is
// configured. Invalid sessions are removed immediately.
func (m *SessionManager) Lookup(ctx context.Context, token string) (Session, error) {
	if m == nil {
		return Session{}, ErrSessionManager
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return Session{}, ErrSessionNotFound
	}
	if ctx == nil {
		ctx = context.Background()
	}
	session, ok := m.Get(token)
	if !ok {
		return Session{}, ErrSessionNotFound
	}
	if m.users != nil {
		fresh, err := m.users.Get(ctx, session.User.Username)
		if err != nil {
			if errors.Is(err, store.ErrUserNotFound) {
				_ = m.Invalidate(ctx, token)
				return Session{}, ErrSessionNotFound
			}
			return Session{}, fmt.Errorf("refresh admin session user: %w", err)
		}
		if fresh == nil {
			_ = m.Invalidate(ctx, token)
			return Session{}, ErrSessionNotFound
		}
		session.User = *fresh
	}
	if err := validateSessionUser(session.User, m.now()); err != nil {
		_ = m.Invalidate(ctx, token)
		return Session{}, err
	}
	session.LastSeenAt = m.now()
	m.mu.Lock()
	if current, exists := m.sessions[token]; exists {
		current.User = session.User
		current.LastSeenAt = session.LastSeenAt
		session = current
		m.sessions[token] = current
	}
	m.mu.Unlock()
	return session, nil
}

// Invalidate removes one token and persists the new session set.
func (m *SessionManager) Invalidate(ctx context.Context, token string) error {
	if m == nil {
		return ErrSessionManager
	}
	if ctx == nil {
		ctx = context.Background()
	}
	m.mu.Lock()
	_, existed := m.sessions[strings.TrimSpace(token)]
	delete(m.sessions, strings.TrimSpace(token))
	m.mu.Unlock()
	if !existed {
		return nil
	}
	return m.persist(ctx)
}

func (m *SessionManager) Delete(ctx context.Context, token string) error {
	return m.Invalidate(ctx, token)
}

func (m *SessionManager) InvalidateSession(ctx context.Context, token string) error {
	return m.Invalidate(ctx, token)
}

// UpdateSessions mirrors the legacy session update hook used after account
// renewal. Updates are applied atomically to the in-memory map and persisted
// once after all matching sessions have changed.
func (m *SessionManager) UpdateSessions(ctx context.Context, predicate func(Session) bool, update func(*Session)) error {
	if m == nil {
		return ErrSessionManager
	}
	if update == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	changed := false
	m.mu.Lock()
	for token, session := range m.sessions {
		if predicate != nil && !predicate(session) {
			continue
		}
		update(&session)
		session.Token = token
		m.sessions[token] = session
		changed = true
	}
	m.mu.Unlock()
	if !changed {
		return nil
	}
	return m.persist(ctx)
}

// InvalidateAll removes sessions matching predicate. A nil predicate removes
// every session.
func (m *SessionManager) InvalidateAll(ctx context.Context, predicate func(Session) bool) error {
	if m == nil {
		return ErrSessionManager
	}
	if ctx == nil {
		ctx = context.Background()
	}
	changed := false
	m.mu.Lock()
	for token, session := range m.sessions {
		if predicate == nil || predicate(session) {
			delete(m.sessions, token)
			changed = true
		}
	}
	m.mu.Unlock()
	if !changed {
		return nil
	}
	return m.persist(ctx)
}

// Cleanup removes sessions whose current user is disabled or expired. A
// transient repository error leaves that session in place for the next sweep.
func (m *SessionManager) Cleanup(ctx context.Context) error {
	if m == nil {
		return ErrSessionManager
	}
	if ctx == nil {
		ctx = context.Background()
	}
	sessions := m.snapshot()
	removed := false
	for token, session := range sessions {
		user := session.User
		if m.users != nil {
			fresh, err := m.users.Get(ctx, user.Username)
			if err != nil || fresh == nil {
				continue
			}
			user = *fresh
		}
		if err := validateSessionUser(user, m.now()); err != nil {
			m.mu.Lock()
			delete(m.sessions, token)
			m.mu.Unlock()
			removed = true
		}
	}
	if removed {
		return m.persist(ctx)
	}
	return nil
}

// Start begins the five-minute invalid-session sweep. Calling it more than
// once is harmless; Close stops the sweep and is safe to call repeatedly.
func (m *SessionManager) Start(ctx context.Context) {
	if m == nil {
		return
	}
	m.mu.Lock()
	if m.stop != nil {
		m.mu.Unlock()
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	stop := make(chan struct{})
	done := make(chan struct{})
	m.stop, m.done = stop, done
	period := m.cleanupPeriod
	m.mu.Unlock()
	go func() {
		defer close(done)
		ticker := time.NewTicker(period)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				_ = m.Cleanup(ctx)
			case <-stop:
				return
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (m *SessionManager) Close() {
	if m == nil {
		return
	}
	m.closeOnce.Do(func() {
		m.mu.RLock()
		stop, done := m.stop, m.done
		m.mu.RUnlock()
		if stop != nil {
			close(stop)
			<-done
		}
	})
}

func (m *SessionManager) Count() int {
	if m == nil {
		return 0
	}
	m.mu.RLock()
	count := len(m.sessions)
	m.mu.RUnlock()
	return count
}

func (m *SessionManager) snapshot() map[string]Session {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make(map[string]Session, len(m.sessions))
	for token, session := range m.sessions {
		result[token] = session
	}
	return result
}

func (m *SessionManager) load(ctx context.Context) error {
	if m.config == nil {
		return nil
	}
	raw, err := m.config.GetGlobal(ctx, m.persistenceKey)
	if err != nil || len(raw) == 0 {
		return nil
	}
	var payload persistedSessions
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil
	}
	for _, saved := range payload.Sessions {
		session := fromPersistedSession(saved)
		if strings.TrimSpace(session.Token) == "" || strings.TrimSpace(session.User.Username) == "" {
			continue
		}
		if err := validateSessionUser(session.User, m.now()); err != nil {
			continue
		}
		m.sessions[session.Token] = session
	}
	return nil
}

func (m *SessionManager) persist(ctx context.Context) error {
	if m.config == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	payload := persistedSessions{Sessions: make([]persistedSession, 0)}
	for _, session := range m.snapshot() {
		payload.Sessions = append(payload.Sessions, toPersistedSession(session))
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode admin sessions: %w", err)
	}
	if err := m.config.SetGlobal(ctx, m.persistenceKey, raw); err != nil {
		return fmt.Errorf("persist admin sessions: %w", err)
	}
	return nil
}

func toPersistedSession(session Session) persistedSession {
	return persistedSession{
		Token: session.Token, CreatedAt: session.CreatedAt, LastSeenAt: session.LastSeenAt,
		User: persistedUser{
			Username: session.User.Username, Role: session.User.Role, Status: session.User.Status,
			ExpireAt: session.User.ExpireAt, AccountLimit: session.User.AccountLimit,
			CardCode: session.User.CardCode, CardJSON: session.User.CardJSON,
			MustChangePassword: session.User.MustChangePassword,
		},
	}
}

func fromPersistedSession(saved persistedSession) Session {
	return Session{
		Token: saved.Token, CreatedAt: saved.CreatedAt, LastSeenAt: saved.LastSeenAt,
		User: store.User{
			Username: saved.User.Username, Role: saved.User.Role, Status: saved.User.Status,
			ExpireAt: saved.User.ExpireAt, AccountLimit: saved.User.AccountLimit,
			CardCode: saved.User.CardCode, CardJSON: saved.User.CardJSON,
			MustChangePassword: saved.User.MustChangePassword,
		},
	}
}

func validateSessionUser(user store.User, now time.Time) error {
	if strings.TrimSpace(user.Status) != "" && !strings.EqualFold(strings.TrimSpace(user.Status), "active") {
		return ErrSessionBanned
	}
	// Administrative roles do not expire through card entitlement, but they
	// must still honor an explicit disabled/banned status above.
	if isElevatedRole(user.Role) {
		return nil
	}
	nowMillis := now.UnixMilli()
	if user.ExpireAt != nil && *user.ExpireAt > 0 && *user.ExpireAt <= nowMillis {
		return ErrSessionExpired
	}
	if strings.TrimSpace(user.CardJSON) == "" {
		return nil
	}
	var card struct {
		Enabled   *bool           `json:"enabled"`
		ExpiresAt json.RawMessage `json:"expiresAt"`
	}
	if json.Unmarshal([]byte(user.CardJSON), &card) != nil {
		return nil
	}
	if card.Enabled != nil && !*card.Enabled {
		return ErrSessionBanned
	}
	if expiry := parseMillis(card.ExpiresAt); expiry > 0 && expiry <= nowMillis {
		return ErrSessionExpired
	}
	return nil
}

func parseMillis(raw json.RawMessage) int64 {
	value := strings.TrimSpace(string(raw))
	if value == "" || value == "null" {
		return 0
	}
	if value[0] == '"' {
		var quoted string
		if json.Unmarshal(raw, &quoted) == nil {
			value = strings.TrimSpace(quoted)
		}
	}
	var number float64
	if _, err := fmt.Sscan(value, &number); err != nil {
		return 0
	}
	return int64(number)
}
