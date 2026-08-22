package account

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	appconfig "github.com/RoseKhlifa/FarmBot/internal/config"
	"github.com/RoseKhlifa/FarmBot/internal/game/session"
	"github.com/RoseKhlifa/FarmBot/internal/game/tsdk"
	"github.com/RoseKhlifa/FarmBot/internal/store"
	"github.com/RoseKhlifa/FarmBot/internal/yyb"
)

var (
	ErrManagerClosed       = errors.New("account manager is closed")
	ErrAccountAlreadyStart = errors.New("account runtime is already managed")
	ErrAccountStopping     = errors.New("account runtime is stopping")
	ErrAccountNotFound     = errors.New("account was not found")
	// ErrAccountOffline indicates that an account exists but has no active
	// runtime. Read-only handlers can use it to return an empty snapshot while
	// mutating operations should continue to surface the offline state.
	ErrAccountOffline = errors.New("account is offline")
)

// ReconnectConfig controls the finite reconnect state machine. Delay is the
// time before the next attempt; StableAfter clears the attempt counter after a
// successful reconnect remains online for that duration.
type ReconnectConfig struct {
	Enabled      bool
	Delay        time.Duration
	MaxAttempts  int
	StableAfter  time.Duration
	PollInterval time.Duration
}

func defaultReconnectConfig() ReconnectConfig {
	return ReconnectConfig{
		Enabled:      true,
		Delay:        5 * time.Minute,
		MaxAttempts:  3,
		StableAfter:  10 * time.Minute,
		PollInterval: 100 * time.Millisecond,
	}
}

func (c ReconnectConfig) normalized() ReconnectConfig {
	defaults := defaultReconnectConfig()
	if c.Delay <= 0 {
		c.Delay = defaults.Delay
	}
	if c.MaxAttempts <= 0 {
		c.MaxAttempts = defaults.MaxAttempts
	}
	if c.StableAfter <= 0 {
		c.StableAfter = defaults.StableAfter
	}
	if c.PollInterval <= 0 {
		c.PollInterval = defaults.PollInterval
	}
	return c
}

// AccountLoader is an optional persistence adapter. When omitted, Manager
// uses Accounts and its Get/GetConfig methods directly.
type AccountLoader func(context.Context, string) (store.Account, *store.AccountConfig, error)

// RuntimeSpec is the fully resolved input used to construct one Runtime.
// AccountConfig stays available for domain wiring without making Runtime own
// persistence concerns.
type RuntimeSpec struct {
	Account       store.Account
	AccountConfig *store.AccountConfig
	RuntimeConfig Config
	Dependencies  Dependencies
	Reconnect     ReconnectConfig
}

// ManagerConfig contains account-manager collaborators. Yyb is an in-process
// service; no HTTP client, URL, bearer token, or package-level registry is
// accepted here.
type ManagerConfig struct {
	Accounts store.AccountRepo
	Config   store.ConfigRepo
	Yyb      yyb.Service
	AppID    string
	// CodeProvider optionally resolves a fresh login code for a stored account.
	// It is used for third-party YYB accounts while the built-in Yyb service
	// remains the default for ordinary YYB accounts.
	CodeProvider func(context.Context, store.Account, *store.AccountConfig, string) (string, error)
	Load         AccountLoader

	RuntimeDependencies Dependencies
	Reconnect           ReconnectConfig
	Context             context.Context

	RuntimeFactory func(RuntimeSpec) *Runtime
	StartRuntime   func(context.Context, *Runtime) error
	StopRuntime    func(*Runtime) error
}

type reconnectEntry struct {
	attempts    int
	generation  uint64
	policy      ReconnectConfig
	timer       *time.Timer
	stableTimer *time.Timer
}

type startAttempt struct {
	cancel context.CancelFunc
	done   chan struct{}
}

// Manager owns all account Runtime instances and their reconnect state. The
// maps and timers belong to this Manager instance, so separate applications or
// tests cannot affect one another.
type Manager struct {
	mu sync.RWMutex

	accounts     store.AccountRepo
	config       store.ConfigRepo
	yyb          yyb.Service
	appID        string
	codeProvider func(context.Context, store.Account, *store.AccountConfig, string) (string, error)
	load         AccountLoader

	runtimeDeps    Dependencies
	runtimeFactory func(RuntimeSpec) *Runtime
	startRuntime   func(context.Context, *Runtime) error
	stopRuntime    func(*Runtime) error

	ctx      context.Context
	cancel   context.CancelFunc
	closed   bool
	draining atomic.Bool

	runtimes  map[string]*Runtime
	starting  map[string]*startAttempt
	stopping  map[string]chan struct{}
	policies  map[string]ReconnectConfig
	reconnect map[string]*reconnectEntry
	policy    ReconnectConfig
}

// NewManager creates an idle account manager. No account is loaded or
// network connection is opened until Start is called.
func NewManager(cfg ManagerConfig) *Manager {
	parent := cfg.Context
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithCancel(parent)
	policy := cfg.Reconnect
	if policy == (ReconnectConfig{}) {
		policy = defaultReconnectConfig()
	} else {
		policy = policy.normalized()
	}
	deps := cfg.RuntimeDependencies
	if deps.Scheduler == nil {
		deps.Scheduler = func(*Runtime) Scheduler { return NewRuntimeScheduler() }
	}

	m := &Manager{
		accounts:     cfg.Accounts,
		config:       cfg.Config,
		yyb:          cfg.Yyb,
		appID:        strings.TrimSpace(cfg.AppID),
		codeProvider: cfg.CodeProvider,
		load:         cfg.Load,
		runtimeDeps:  deps,
		startRuntime: cfg.StartRuntime,
		stopRuntime:  cfg.StopRuntime,
		ctx:          ctx,
		cancel:       cancel,
		runtimes:     make(map[string]*Runtime),
		starting:     make(map[string]*startAttempt),
		stopping:     make(map[string]chan struct{}),
		policies:     make(map[string]ReconnectConfig),
		reconnect:    make(map[string]*reconnectEntry),
		policy:       policy,
	}
	if cfg.RuntimeFactory != nil {
		m.runtimeFactory = cfg.RuntimeFactory
	} else {
		m.runtimeFactory = func(spec RuntimeSpec) *Runtime {
			return NewRuntime(spec.RuntimeConfig, spec.Dependencies)
		}
	}
	return m
}

// NewAccountManager is a descriptive constructor alias.
func NewAccountManager(cfg ManagerConfig) *Manager { return NewManager(cfg) }

// Start loads account configuration, refreshes an in-process yyb login code
// when applicable, creates the account Runtime, and starts it. Start is safe
// against concurrent duplicate calls.
func (m *Manager) Start(accountID string) error {
	return m.StartContext(context.Background(), accountID)
}

// StartContext is Start with an optional caller context for login and setup.
func (m *Manager) StartContext(parent context.Context, accountID string) error {
	if m != nil && m.draining.Load() {
		return ErrManagerClosed
	}
	return m.startContext(parent, accountID, false)
}

// BeginDrain prevents new account starts while graceful shutdown drains
// existing runtimes.
func (m *Manager) BeginDrain() {
	if m != nil {
		m.draining.Store(true)
	}
}

func (m *Manager) IsDraining() bool { return m != nil && m.draining.Load() }

func (m *Manager) startContext(parent context.Context, accountID string, reconnect bool) (err error) {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return errors.New("account ID is required")
	}
	if parent == nil {
		parent = context.Background()
	}
	if err := parent.Err(); err != nil {
		return err
	}

	m.mu.Lock()
	if m.closed || m.draining.Load() {
		m.mu.Unlock()
		return ErrManagerClosed
	}
	if _, ok := m.runtimes[accountID]; ok {
		m.mu.Unlock()
		return ErrAccountAlreadyStart
	}
	if _, ok := m.starting[accountID]; ok {
		m.mu.Unlock()
		return ErrAccountAlreadyStart
	}
	if _, ok := m.stopping[accountID]; ok {
		m.mu.Unlock()
		return ErrAccountStopping
	}
	if !reconnect {
		m.cancelReconnectLocked(accountID, true)
	}
	startCtx, cancel := context.WithCancel(m.ctx)
	parentStop := context.AfterFunc(parent, cancel)
	attempt := &startAttempt{cancel: cancel, done: make(chan struct{})}
	m.starting[accountID] = attempt
	m.mu.Unlock()
	defer func() {
		parentStop()
		m.finishStart(accountID, attempt)
	}()

	spec, err := m.loadSpec(startCtx, accountID)
	if err != nil {
		return err
	}
	if err := startCtx.Err(); err != nil {
		return err
	}
	runtime := m.runtimeFactory(spec)
	if runtime == nil {
		return errors.New("runtime factory returned nil")
	}
	starter := m.startRuntime
	if starter == nil {
		starter = func(ctx context.Context, runtime *Runtime) error { return runtime.Start(ctx) }
	}
	if err := starter(startCtx, runtime); err != nil {
		// YYB login codes are short-lived and single-use. A code can be
		// accepted by the provider but rejected by the game gateway with
		// 1000016 when it was issued from a stale session or consumed by an
		// earlier request. Refresh the provider code once and retry the whole
		// account start; never loop because a persistent permission error needs
		// to remain visible to the caller.
		if isYYBAccount(spec.Account) && isGameLoginPermissionError(err) && startCtx.Err() == nil {
			firstErr := err
			_ = m.stopOne(runtime)
			retrySpec, refreshErr := m.loadSpec(startCtx, accountID)
			if refreshErr == nil {
				retryRuntime := m.runtimeFactory(retrySpec)
				if retryRuntime == nil {
					refreshErr = errors.New("runtime factory returned nil")
				} else if retryErr := starter(startCtx, retryRuntime); retryErr == nil {
					runtime = retryRuntime
					spec = retrySpec
					err = nil
				} else {
					_ = m.stopOne(retryRuntime)
					refreshErr = retryErr
				}
			}
			if err != nil {
				if refreshErr != nil {
					return fmt.Errorf("start account %q: %w; refreshed-code retry failed: %v", accountID, firstErr, refreshErr)
				}
				return fmt.Errorf("start account %q: %w", accountID, firstErr)
			}
		}
		if err != nil {
			return fmt.Errorf("start account %q: %w", accountID, err)
		}
	}
	if err := startCtx.Err(); err != nil {
		_ = m.stopOne(runtime)
		return err
	}

	m.mu.Lock()
	if m.closed || startCtx.Err() != nil {
		m.mu.Unlock()
		_ = m.stopOne(runtime)
		if err := startCtx.Err(); err != nil {
			return err
		}
		return ErrManagerClosed
	}
	m.runtimes[accountID] = runtime
	m.policies[accountID] = spec.Reconnect
	if reconnect {
		m.armStableResetLocked(accountID, runtime, spec.Reconnect)
	}
	m.mu.Unlock()

	go m.monitor(accountID, runtime, spec.Reconnect.PollInterval)
	return nil
}

// Stop removes an account from management before stopping it, preventing a
// manual stop from being mistaken for an unexpected disconnect. It is
// idempotent for an account that is already stopped or unknown.
func (m *Manager) Stop(accountID string) error {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return nil
	}
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return ErrManagerClosed
	}
	if attempt := m.starting[accountID]; attempt != nil {
		delete(m.starting, accountID)
		attempt.cancel()
		done := attempt.done
		m.mu.Unlock()
		<-done
		return nil
	}
	if done := m.stopping[accountID]; done != nil {
		m.mu.Unlock()
		<-done
		return nil
	}
	runtime := m.runtimes[accountID]
	if runtime == nil {
		m.cancelReconnectLocked(accountID, true)
		m.mu.Unlock()
		return nil
	}
	delete(m.runtimes, accountID)
	delete(m.policies, accountID)
	m.cancelReconnectLocked(accountID, true)
	stopDone := make(chan struct{})
	m.stopping[accountID] = stopDone
	m.mu.Unlock()
	return m.stopDetached(accountID, runtime, stopDone)
}

// Restart performs a manual stop followed by a fresh account load and start.
// Manual restarts reset the reconnect attempt counter.
func (m *Manager) Restart(accountID string) error {
	return m.RestartContext(context.Background(), accountID)
}

func (m *Manager) RestartContext(parent context.Context, accountID string) error {
	if err := m.Stop(accountID); err != nil && !errors.Is(err, ErrManagerClosed) {
		return err
	}
	return m.StartContext(parent, accountID)
}

// Get returns the currently managed Runtime for accountID, or nil when the
// account is stopped or has not been started.
func (m *Manager) Get(accountID string) *Runtime {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.runtimes[strings.TrimSpace(accountID)]
}

// List returns active runtimes in stable account-ID order.
func (m *Manager) List() []*Runtime {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	ids := make([]string, 0, len(m.runtimes))
	for id := range m.runtimes {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	result := make([]*Runtime, 0, len(ids))
	for _, id := range ids {
		result = append(result, m.runtimes[id])
	}
	m.mu.RUnlock()
	return result
}

// NotifyOffline lets transport/session adapters report a disconnect without
// coupling them to a string RPC dispatcher. The monitor uses the same path.
func (m *Manager) NotifyOffline(accountID, reason string) error {
	accountID = strings.TrimSpace(accountID)
	m.mu.RLock()
	runtime := m.runtimes[accountID]
	m.mu.RUnlock()
	if runtime == nil {
		return ErrAccountNotFound
	}
	m.handleOffline(accountID, runtime, reason)
	return nil
}

func (m *Manager) NotifyKickout(accountID, reason string) error {
	return m.NotifyOffline(accountID, "kickout:"+strings.TrimSpace(reason))
}

// Close stops every managed account and cancels pending reconnect timers.
func (m *Manager) Close() error {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return nil
	}
	m.closed = true
	m.cancel()
	runtimes := make([]*Runtime, 0, len(m.runtimes))
	for id, runtime := range m.runtimes {
		delete(m.runtimes, id)
		runtimes = append(runtimes, runtime)
	}
	attempts := make([]*startAttempt, 0, len(m.starting))
	for id, attempt := range m.starting {
		delete(m.starting, id)
		attempt.cancel()
		attempts = append(attempts, attempt)
	}
	stopping := make([]chan struct{}, 0, len(m.stopping))
	for _, done := range m.stopping {
		stopping = append(stopping, done)
	}
	for id := range m.reconnect {
		m.cancelReconnectLocked(id, true)
	}
	m.mu.Unlock()

	for _, attempt := range attempts {
		<-attempt.done
	}
	for _, done := range stopping {
		<-done
	}
	var firstErr error
	for _, runtime := range runtimes {
		if err := m.stopOne(runtime); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (m *Manager) finishStart(accountID string, attempt *startAttempt) {
	m.mu.Lock()
	if current := m.starting[accountID]; current == attempt {
		delete(m.starting, accountID)
	}
	m.mu.Unlock()
	close(attempt.done)
}

func (m *Manager) stopOne(runtime *Runtime) error {
	if runtime == nil {
		return nil
	}
	if m.stopRuntime != nil {
		return m.stopRuntime(runtime)
	}
	return runtime.Stop()
}

func (m *Manager) stopDetached(accountID string, runtime *Runtime, done chan struct{}) error {
	err := m.stopOne(runtime)
	m.mu.Lock()
	if current := m.stopping[accountID]; current == done {
		delete(m.stopping, accountID)
		close(done)
	}
	m.mu.Unlock()
	return err
}

func (m *Manager) loadSpec(ctx context.Context, accountID string) (RuntimeSpec, error) {
	var (
		account store.Account
		config  *store.AccountConfig
		err     error
	)
	if m.load != nil {
		account, config, err = m.load(ctx, accountID)
	} else {
		if m.accounts == nil {
			return RuntimeSpec{}, errors.New("account repository is required")
		}
		var loaded *store.Account
		loaded, err = m.accounts.Get(ctx, accountID)
		if err == nil && loaded != nil {
			account = *loaded
			config, err = m.accounts.GetConfig(ctx, accountID)
			if errors.Is(err, sql.ErrNoRows) {
				err = nil
			}
		}
	}
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return RuntimeSpec{}, fmt.Errorf("%w: %s", ErrAccountNotFound, accountID)
		}
		return RuntimeSpec{}, fmt.Errorf("load account %q: %w", accountID, err)
	}
	if strings.TrimSpace(account.ID) == "" {
		return RuntimeSpec{}, fmt.Errorf("%w: %s", ErrAccountNotFound, accountID)
	}

	code := strings.TrimSpace(account.Code)
	if isYYBAccount(account) && (m.yyb != nil || m.codeProvider != nil) {
		ref := strings.TrimSpace(account.YYBOpenID)
		if ref == "" {
			ref = strings.TrimSpace(account.OpenID)
		}
		if ref == "" {
			return RuntimeSpec{}, fmt.Errorf("account %q has no yyb openid", accountID)
		}
		if m.codeProvider != nil {
			code, err = m.codeProvider(ctx, account, config, m.appID)
		} else {
			code, err = m.yyb.GetCode(ctx, ref, m.appID)
		}
		if err != nil {
			return RuntimeSpec{}, fmt.Errorf("refresh yyb code for account %q: %w", accountID, err)
		}
		code = strings.TrimSpace(code)
	}
	if code == "" {
		return RuntimeSpec{}, fmt.Errorf("account %q has no login code", accountID)
	}

	policy := m.policy
	if m.config != nil {
		if raw, configErr := m.config.GetWXConfig(ctx); configErr == nil {
			policy = applyReconnectJSON(policy, raw)
		} else if !errors.Is(configErr, sql.ErrNoRows) {
			return RuntimeSpec{}, fmt.Errorf("load global reconnect config: %w", configErr)
		}
	}
	if config != nil {
		policy = applyReconnectJSON(policy, config.ConfigJSON)
	}
	policy = policy.normalized()

	platform := strings.TrimSpace(account.Platform)
	if platform == "" {
		platform = "qq"
	}
	// The reference worker receives the persisted system protocol settings
	// before it opens the gateway connection. Keep the account platform
	// account-scoped (YYB accounts use "wx"), while honoring the stored gateway,
	// client version, and OS values for every account. Falling back to process
	// defaults is intentional for fresh installations with no row yet.
	system := loadSystemProtocolConfig(ctx, m.config)
	device, userAgent := m.deviceProtocol(ctx)
	sessionOptions := session.Options{
		AccountID:     account.ID,
		UIN:           strings.TrimSpace(account.UIN),
		Platform:      platform,
		GatewayURL:    system.GatewayURL,
		ClientVersion: system.ClientVersion,
		OS:            system.OS,
		UserAgent:     userAgent,
		TSDK:          tsdk.Options{Device: device},
	}
	return RuntimeSpec{
		Account:       account,
		AccountConfig: config,
		RuntimeConfig: Config{
			AccountID: account.ID,
			LoginCode: code,
			Session:   sessionOptions,
		},
		Dependencies: m.runtimeDeps,
		Reconnect:    policy,
	}, nil
}

func (m *Manager) monitor(accountID string, runtime *Runtime, interval time.Duration) {
	if interval <= 0 {
		interval = m.policy.PollInterval
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.mu.RLock()
			current := m.runtimes[accountID]
			m.mu.RUnlock()
			if current != runtime {
				return
			}
			snapshot := runtime.Status()
			if snapshot.Phase == PhaseOffline {
				m.handleOffline(accountID, runtime, snapshot.LastError)
				return
			}
		}
	}
}

func (m *Manager) handleOffline(accountID string, runtime *Runtime, reason string) {
	m.mu.Lock()
	if m.closed || m.runtimes[accountID] != runtime {
		m.mu.Unlock()
		return
	}
	delete(m.runtimes, accountID)
	policy := m.policies[accountID]
	delete(m.policies, accountID)
	if policy == (ReconnectConfig{}) {
		policy = m.policy
	}
	entry := m.reconnect[accountID]
	if entry == nil {
		entry = &reconnectEntry{policy: policy}
	}
	stopDone := make(chan struct{})
	m.stopping[accountID] = stopDone
	entry.policy = policy
	if !policy.Enabled || entry.attempts >= policy.MaxAttempts {
		delete(m.reconnect, accountID)
		m.mu.Unlock()
		_ = m.stopDetached(accountID, runtime, stopDone)
		return
	}
	entry.attempts++
	entry.generation++
	generation := entry.generation
	if entry.timer != nil {
		entry.timer.Stop()
	}
	attempt := entry.attempts
	entry.timer = time.AfterFunc(policy.Delay, func() {
		m.reconnectAttempt(accountID, uint64(attempt), generation)
	})
	m.reconnect[accountID] = entry
	m.mu.Unlock()

	_ = reason // retained by the event boundary for logging integrations
	_ = m.stopDetached(accountID, runtime, stopDone)
}

func (m *Manager) reconnectAttempt(accountID string, attempt uint64, generation uint64) {
	m.mu.Lock()
	entry := m.reconnect[accountID]
	if m.closed || entry == nil || entry.generation != generation || uint64(entry.attempts) != attempt {
		m.mu.Unlock()
		return
	}
	entry.timer = nil
	policy := entry.policy
	m.mu.Unlock()

	err := m.startContext(m.ctx, accountID, true)
	if err == nil {
		return
	}
	if errors.Is(err, ErrManagerClosed) || errors.Is(err, ErrAccountNotFound) || errors.Is(err, context.Canceled) {
		m.mu.Lock()
		if current := m.reconnect[accountID]; current == entry {
			delete(m.reconnect, accountID)
		}
		m.mu.Unlock()
		return
	}
	m.scheduleRetry(accountID, attempt, policy)
}

func (m *Manager) scheduleRetry(accountID string, attempt uint64, policy ReconnectConfig) {
	m.mu.Lock()
	entry := m.reconnect[accountID]
	if m.closed || entry == nil || uint64(entry.attempts) != attempt {
		m.mu.Unlock()
		return
	}
	if !policy.Enabled || entry.attempts >= policy.MaxAttempts {
		delete(m.reconnect, accountID)
		m.mu.Unlock()
		return
	}
	entry.attempts++
	entry.generation++
	generation := entry.generation
	nextAttempt := entry.attempts
	entry.policy = policy
	entry.timer = time.AfterFunc(policy.Delay, func() {
		m.reconnectAttempt(accountID, uint64(nextAttempt), generation)
	})
	m.mu.Unlock()
}

func (m *Manager) armStableResetLocked(accountID string, runtime *Runtime, policy ReconnectConfig) {
	entry := m.reconnect[accountID]
	if entry == nil || entry.attempts == 0 || policy.StableAfter <= 0 {
		return
	}
	if entry.stableTimer != nil {
		entry.stableTimer.Stop()
	}
	entry.generation++
	generation := entry.generation
	entry.stableTimer = time.AfterFunc(policy.StableAfter, func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		current := m.reconnect[accountID]
		if current == entry && current.generation == generation && m.runtimes[accountID] == runtime {
			current.attempts = 0
			current.stableTimer = nil
		}
	})
}

func (m *Manager) cancelReconnectLocked(accountID string, reset bool) {
	entry := m.reconnect[accountID]
	if entry == nil {
		return
	}
	if entry.timer != nil {
		entry.timer.Stop()
	}
	if entry.stableTimer != nil {
		entry.stableTimer.Stop()
	}
	entry.generation++
	if reset {
		delete(m.reconnect, accountID)
	}
}

func isYYBAccount(account store.Account) bool {
	return strings.EqualFold(strings.TrimSpace(account.LoginType), "yyb") || strings.TrimSpace(account.YYBOpenID) != ""
}

func isGameLoginPermissionError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "code=1000016")
}

type deviceProtocolConfig struct {
	Enabled     bool   `json:"enabled"`
	UserAgent   string `json:"userAgent"`
	DeviceModel string `json:"deviceModel"`
	DeviceBrand string `json:"deviceBrand"`
}

type systemProtocolConfig struct {
	GatewayURL    string
	ClientVersion string
	OS            string
}

func loadSystemProtocolConfig(ctx context.Context, repo store.ConfigRepo) systemProtocolConfig {
	defaults := appconfig.Load()
	result := systemProtocolConfig{
		GatewayURL:    defaults.ServerURL,
		ClientVersion: defaults.ClientVersion,
		OS:            defaults.OS,
	}
	if repo == nil {
		return result
	}
	raw, err := repo.GetSystemConfig(ctx)
	if err != nil {
		// Older JSON imports used the legacy key before the typed config
		// repository was introduced. Keep those installations functional.
		raw, err = repo.GetGlobal(ctx, "legacy:systemConfig")
	}
	if err != nil {
		return result
	}
	var stored struct {
		ServerURL     string `json:"serverUrl"`
		ClientVersion string `json:"clientVersion"`
		OS            string `json:"os"`
	}
	if json.Unmarshal(raw, &stored) != nil {
		return result
	}
	if value := strings.TrimSpace(stored.ServerURL); value != "" {
		result.GatewayURL = value
	}
	if value := strings.TrimSpace(stored.ClientVersion); value != "" {
		result.ClientVersion = value
	}
	if value := strings.TrimSpace(stored.OS); value != "" {
		result.OS = value
	}
	return result
}

func (m *Manager) deviceProtocol(ctx context.Context) (tsdk.DeviceInfo, string) {
	// Match the reference Node client's DEFAULT_DEVICE_PROTOCOL. Its
	// getDeviceProtocol() always returns these iPhone values when the user has
	// not saved a custom device; the "enabled" flag only gates the custom
	// User-Agent, not the model/brand fed to the TSDK runtime. The game has no
	// Linux client, so the fingerprint must present an iPhone regardless of the
	// host OS the bot runs on. The optional setting still overrides whichever
	// fields it explicitly supplies.
	device := tsdk.DeviceInfo{Model: "iPhone 15 Pro Max", Brand: "Apple"}
	if m == nil || m.config == nil {
		return device, ""
	}
	raw, err := m.config.GetGlobal(ctx, "deviceProtocol")
	if err != nil {
		// JSON migrations retain unknown legacy settings under this namespace.
		// Read that key as a compatibility fallback without making an optional
		// setting failure abort account startup.
		raw, err = m.config.GetGlobal(ctx, "legacy:deviceProtocol")
		if err != nil {
			return device, ""
		}
	}
	var cfg deviceProtocolConfig
	if json.Unmarshal(raw, &cfg) != nil {
		return device, ""
	}
	if value := strings.TrimSpace(cfg.DeviceModel); value != "" {
		device.Model = value
	}
	if value := strings.TrimSpace(cfg.DeviceBrand); value != "" {
		device.Brand = value
	}
	if cfg.Enabled {
		return device, strings.TrimSpace(cfg.UserAgent)
	}
	return device, ""
}

func applyReconnectJSON(base ReconnectConfig, raw json.RawMessage) ReconnectConfig {
	if len(raw) == 0 {
		return base
	}
	var values map[string]json.RawMessage
	if err := json.Unmarshal(raw, &values); err != nil {
		return base
	}
	if nested, ok := values["reconnect"]; ok {
		var nestedValues map[string]json.RawMessage
		if json.Unmarshal(nested, &nestedValues) == nil {
			for key, value := range nestedValues {
				values[key] = value
			}
		}
	}
	for _, key := range []string{"autoReconnect", "auto_reconnect"} {
		if value, ok := values[key]; ok {
			var enabled bool
			if json.Unmarshal(value, &enabled) == nil {
				base.Enabled = enabled
			}
			break
		}
	}
	for _, key := range []string{"reconnectDelayMin", "reconnect_delay_min"} {
		if value, ok := values[key]; ok {
			var minutes float64
			if json.Unmarshal(value, &minutes) == nil && minutes > 0 {
				base.Delay = time.Duration(minutes * float64(time.Minute))
			}
			break
		}
	}
	for _, key := range []string{"reconnectMaxAttempts", "reconnect_max_attempts"} {
		if value, ok := values[key]; ok {
			var attempts int
			if json.Unmarshal(value, &attempts) == nil && attempts > 0 {
				base.MaxAttempts = attempts
			}
			break
		}
	}
	return base
}
