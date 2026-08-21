package account

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/RoseKhlifa/FarmBot/internal/game/ace"
	"github.com/RoseKhlifa/FarmBot/internal/game/session"
	"github.com/RoseKhlifa/FarmBot/internal/game/transport"
	"github.com/RoseKhlifa/FarmBot/internal/game/tsdk"
	"github.com/RoseKhlifa/FarmBot/internal/store"
)

var (
	ErrRuntimeStarted = errors.New("account runtime is already started")
	ErrRuntimeStopped = errors.New("account runtime is stopped")
)

// Config contains the account-scoped login settings. Session options remain
// grouped in the game/session package so Runtime does not duplicate protocol
// configuration fields or introduce another global configuration object.
type Config struct {
	AccountID string
	LoginCode string
	Session   session.Options
}

// Scheduler is the narrow lifecycle contract Runtime needs. P4-02 supplies
// the concrete scheduler; keeping this as an interface prevents Runtime from
// owning scheduling policy or a package-level registry.
type Scheduler interface {
	Start(context.Context) error
	Stop()
}

// Dependencies are all replaceable account-local collaborators. Stats is
// intentionally an interface from the store package so Runtime remains easy
// to test without opening SQLite.
type Dependencies struct {
	Stats         store.StatsRepo
	Scheduler     func(*Runtime) Scheduler
	Login         func(context.Context, string, session.Options) (*session.Session, error)
	TSDK          func(context.Context, tsdk.Options) *tsdk.Runtime
	Initialize    func(context.Context, *Runtime, *session.Session) error
	StatusChanged func(StatusSnapshot)
	Event         func(string, transport.Event)
}

// Runtime is the isolated container for one account's live resources. No
// field is package-global: every account gets its own status, session,
// scheduler and cancellation context.
type Runtime struct {
	accountID string
	ws        *transport.Client
	session   *session.Session
	tsdk      *tsdk.Runtime
	ace       *ace.Service
	sched     Scheduler
	status    *StatusState
	stats     store.StatsRepo
	cfg       *Config
	domains   map[string]any
	closers   []func() error
	handlers  []func(context.Context, transport.Event) error

	mu        sync.Mutex
	ctx       context.Context
	cancel    context.CancelFunc
	startDone chan struct{}
	starting  bool
	started   bool
	stopping  bool
	stopDone  chan struct{}

	login        func(context.Context, string, session.Options) (*session.Session, error)
	tsdkNew      func(context.Context, tsdk.Options) *tsdk.Runtime
	schedulerNew func(*Runtime) Scheduler
	initialize   func(context.Context, *Runtime, *session.Session) error
	event        func(string, transport.Event)
}

// NewRuntime creates an idle account runtime. It does not open a network
// connection, initialize WASM, or start any goroutine.
func NewRuntime(cfg Config, deps Dependencies) *Runtime {
	if cfg.AccountID == "" {
		cfg.AccountID = cfg.Session.AccountID
	}
	cfg.Session.AccountID = cfg.AccountID
	login := deps.Login
	if login == nil {
		login = session.LoginWithOptions
	}
	tsdkNew := deps.TSDK
	if tsdkNew == nil {
		tsdkNew = tsdk.New
	}
	status := NewStatusState(cfg.AccountID)
	status.SetOnChange(deps.StatusChanged)
	return &Runtime{
		accountID:    cfg.AccountID,
		status:       status,
		stats:        deps.Stats,
		cfg:          &cfg,
		domains:      make(map[string]any),
		login:        login,
		tsdkNew:      tsdkNew,
		schedulerNew: deps.Scheduler,
		initialize:   deps.Initialize,
		event:        deps.Event,
	}
}

// New is a short constructor alias for callers that prefer the package's
// conventional name.
func New(cfg Config, deps Dependencies) *Runtime { return NewRuntime(cfg, deps) }

// Start authenticates the account, starts its session-owned ACE lifecycle,
// then starts the injected scheduler. A failed start releases every resource
// created during that attempt before returning the error.
func (r *Runtime) Start(parent context.Context) error {
	if r == nil {
		return errors.New("account runtime is nil")
	}
	if parent == nil {
		parent = context.Background()
	}

	r.mu.Lock()
	if r.started || r.starting {
		r.mu.Unlock()
		return ErrRuntimeStarted
	}
	if r.stopping {
		r.mu.Unlock()
		return ErrRuntimeStopped
	}
	if r.cfg == nil || r.accountID == "" {
		r.mu.Unlock()
		return errors.New("account runtime requires an account ID")
	}
	if r.cfg.LoginCode == "" {
		r.mu.Unlock()
		return errors.New("account runtime requires a login code")
	}
	r.ctx, r.cancel = context.WithCancel(parent)
	r.startDone = make(chan struct{})
	r.starting = true
	startDone := r.startDone
	ctx := r.ctx
	cfg := *r.cfg
	login := r.login
	tsdkNew := r.tsdkNew
	schedulerNew := r.schedulerNew
	initialize := r.initialize
	r.status.MarkStarting(time.Now())
	r.mu.Unlock()

	var ownedTSDK *tsdk.Runtime
	loginOptions := cfg.Session
	loginOptions.AccountID = r.accountID
	if loginOptions.TSDK.AccountID == "" {
		loginOptions.TSDK.AccountID = r.accountID
	}
	if loginOptions.Runtime == nil {
		ownedTSDK = tsdkNew(ctx, loginOptions.TSDK)
		loginOptions.Runtime = ownedTSDK
	}
	loggedIn, err := login(ctx, cfg.LoginCode, loginOptions)
	if err != nil {
		_ = cleanupRuntimeResources(nil, ownedTSDK, nil, nil, nil)
		r.finishStart(startDone, err)
		return fmt.Errorf("login account %q: %w", r.accountID, err)
	}
	if loggedIn == nil {
		err = errors.New("login returned a nil session")
		_ = cleanupRuntimeResources(nil, ownedTSDK, nil, nil, nil)
		r.finishStart(startDone, err)
		return err
	}
	if !loggedIn.Online() {
		err = errors.New("login returned an offline session")
		_ = cleanupRuntimeResources(loggedIn, ownedTSDK, nil, nil, nil)
		r.finishStart(startDone, err)
		return err
	}

	var scheduler Scheduler
	if schedulerNew != nil {
		scheduler = schedulerNew(r)
		if scheduler != nil {
			if err = scheduler.Start(ctx); err != nil {
				_ = cleanupRuntimeResources(loggedIn, ownedTSDK, nil, nil, scheduler)
				r.finishStart(startDone, err)
				return fmt.Errorf("start account scheduler: %w", err)
			}
		}
	}

	r.mu.Lock()
	r.session = loggedIn
	r.tsdk = ownedTSDK
	r.sched = scheduler
	r.mu.Unlock()

	if initialize != nil {
		if err = initialize(ctx, r, loggedIn); err != nil {
			domainErr := r.detachAndCloseDomains()
			r.clearStartResources()
			cleanupErr := cleanupRuntimeResources(loggedIn, ownedTSDK, nil, nil, scheduler)
			err = errors.Join(err, domainErr, cleanupErr)
			r.finishStart(startDone, err)
			return fmt.Errorf("initialize account domains: %w", err)
		}
	}

	r.mu.Lock()
	if ctx.Err() != nil || !r.starting {
		r.mu.Unlock()
		domainErr := r.detachAndCloseDomains()
		r.clearStartResources()
		_ = cleanupRuntimeResources(loggedIn, ownedTSDK, nil, nil, scheduler)
		err = errors.Join(ctx.Err(), domainErr)
		r.finishStart(startDone, err)
		return err
	}
	r.started = true
	r.starting = false
	r.status.MarkOnline(loggedIn.State(), time.Now())
	r.mu.Unlock()
	close(startDone)
	go r.watchSession(ctx, loggedIn)
	return nil
}

// Stop cancels the account context, waits for an in-progress login to finish,
// then releases the scheduler and session-owned protocol resources. It is
// safe to call repeatedly and from concurrent shutdown paths.
func (r *Runtime) Stop() error {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	if !r.started && !r.starting {
		r.mu.Unlock()
		return nil
	}
	if r.stopping {
		done := r.stopDone
		r.mu.Unlock()
		if done != nil {
			<-done
		}
		return nil
	}
	r.stopping = true
	r.stopDone = make(chan struct{})
	stopDone := r.stopDone
	cancel := r.cancel
	startDone := r.startDone
	r.status.MarkStopping(time.Now())
	r.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if startDone != nil {
		<-startDone
	}

	r.mu.Lock()
	sess, sdk, aceService, ws, scheduler := r.session, r.tsdk, r.ace, r.ws, r.sched
	closers := append([]func() error(nil), r.closers...)
	r.session, r.tsdk, r.ace, r.ws, r.sched = nil, nil, nil, nil, nil
	r.closers = nil
	r.handlers = nil
	r.domains = make(map[string]any)
	r.started = false
	r.starting = false
	r.cancel = nil
	r.ctx = nil
	r.mu.Unlock()

	domainErr := closeRuntimeDomains(closers)
	err := errors.Join(domainErr, cleanupRuntimeResources(sess, sdk, aceService, ws, scheduler))
	r.status.MarkOffline(err, time.Now())
	r.mu.Lock()
	r.stopping = false
	close(stopDone)
	r.stopDone = nil
	r.mu.Unlock()
	return err
}

// Close is an alias for Stop so Runtime can be used as a lifecycle resource.
func (r *Runtime) Close() error { return r.Stop() }

// AccountID returns the stable account key owned by this runtime.
func (r *Runtime) AccountID() string {
	if r == nil {
		return ""
	}
	return r.accountID
}

// Context returns the current account context, or nil while idle.
func (r *Runtime) Context() context.Context {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.ctx
}

// Status returns a race-free per-account status snapshot.
func (r *Runtime) Status() StatusSnapshot {
	if r == nil || r.status == nil {
		return StatusSnapshot{}
	}
	return r.status.Snapshot()
}

// Session returns the authenticated session while Runtime owns it. Callers
// must not close it directly; use Stop so all resources are coordinated.
func (r *Runtime) Session() *session.Session {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.session
}

// Stats returns the injected account statistics repository.
func (r *Runtime) Stats() store.StatsRepo {
	if r == nil {
		return nil
	}
	return r.stats
}

// SetGold and AddOperation are the narrow status mutation hooks exposed to
// domain services.
func (r *Runtime) SetGold(value int64) {
	if r != nil && r.status != nil {
		r.status.SetGold(value)
	}
}

func (r *Runtime) AddOperation(name string, delta float64) {
	if r != nil && r.status != nil {
		r.status.AddOperation(name, delta)
	}
}

// SetDomain installs one account-local domain service for later P5 wiring.
func (r *Runtime) SetDomain(name string, service any) {
	if r == nil {
		return
	}
	name = stringsTrim(name)
	if name == "" {
		return
	}
	r.mu.Lock()
	r.domains[name] = service
	r.mu.Unlock()
}

// Domain returns an injected account-local domain service.
func (r *Runtime) Domain(name string) any {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.domains[stringsTrim(name)]
}

// DomainScheduler exposes the account-local named scheduler to domain
// composition without exposing lifecycle ownership.
func (r *Runtime) DomainScheduler() DomainScheduler {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	scheduler, ok := r.sched.(interface {
		Every(string, time.Duration, time.Duration, any) error
		StopTask(string) bool
	})
	if !ok {
		return nil
	}
	return domainSchedulerAdapter{scheduler: scheduler}
}

type domainSchedulerAdapter struct {
	scheduler interface {
		Every(string, time.Duration, time.Duration, any) error
		StopTask(string) bool
	}
}

func (s domainSchedulerAdapter) Every(name string, interval, jitter time.Duration, callback any) error {
	return s.scheduler.Every(name, interval, jitter, callback)
}

func (s domainSchedulerAdapter) Stop(name string) bool { return s.scheduler.StopTask(name) }

// SchedulerStatus returns a stable scheduler snapshot for HTTP and realtime
// diagnostics.
func (r *Runtime) SchedulerStatus() SchedulerStatus {
	if r == nil {
		return SchedulerStatus{}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	provider, _ := r.sched.(interface{ Status() SchedulerStatus })
	if provider == nil {
		return SchedulerStatus{}
	}
	return provider.Status()
}

// AddCloser registers one account-local domain resource. Closers run in
// reverse registration order before the session and scheduler are released.
func (r *Runtime) AddCloser(closer func() error) {
	if r == nil || closer == nil {
		return
	}
	r.mu.Lock()
	r.closers = append(r.closers, closer)
	r.mu.Unlock()
}

// AddEventHandler registers a synchronous subscriber for the Runtime-owned
// session event stream. Runtime is the sole channel consumer and fans events
// out to every domain subscriber.
func (r *Runtime) AddEventHandler(handler func(context.Context, transport.Event) error) {
	if r == nil || handler == nil {
		return
	}
	r.mu.Lock()
	r.handlers = append(r.handlers, handler)
	r.mu.Unlock()
}

func (r *Runtime) finishStart(done chan struct{}, err error) {
	r.mu.Lock()
	if r.starting {
		r.starting = false
	}
	if err != nil {
		r.started = false
		r.cancel = nil
		r.ctx = nil
		r.status.MarkError(err, time.Now())
	}
	r.mu.Unlock()
	close(done)
}

func (r *Runtime) watchSession(ctx context.Context, sess *session.Session) {
	events := sess.Events()
	if events == nil {
		return
	}
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-events:
			if !ok {
				r.status.MarkOffline(nil, time.Now())
				return
			}
			r.status.UpdateUser(sess.State())
			r.dispatchEvent(ctx, event)
			switch event.Type {
			case transport.EventDisconnect, transport.EventKickout:
				err := event.Err
				if err == nil && event.Reason != "" {
					err = errors.New(event.Reason)
				}
				r.status.MarkOffline(err, time.Now())
				return
			}
		}
	}
}

func (r *Runtime) dispatchEvent(ctx context.Context, event transport.Event) {
	if r != nil && r.event != nil {
		r.event(r.accountID, event)
	}
	r.mu.Lock()
	handlers := append([]func(context.Context, transport.Event) error(nil), r.handlers...)
	r.mu.Unlock()
	for _, handler := range handlers {
		_ = handler(ctx, event)
	}
}

func (r *Runtime) clearStartResources() {
	r.mu.Lock()
	r.session, r.tsdk, r.ace, r.ws, r.sched = nil, nil, nil, nil, nil
	r.mu.Unlock()
}

func (r *Runtime) detachAndCloseDomains() error {
	r.mu.Lock()
	closers := append([]func() error(nil), r.closers...)
	r.closers = nil
	r.handlers = nil
	r.domains = make(map[string]any)
	r.mu.Unlock()
	return closeRuntimeDomains(closers)
}

func closeRuntimeDomains(closers []func() error) error {
	errorsList := make([]error, 0)
	for index := len(closers) - 1; index >= 0; index-- {
		if err := closers[index](); err != nil {
			errorsList = append(errorsList, err)
		}
	}
	return errors.Join(errorsList...)
}

func cleanupRuntimeResources(sess *session.Session, sdk *tsdk.Runtime, aceService *ace.Service, ws *transport.Client, scheduler Scheduler) error {
	if scheduler != nil {
		scheduler.Stop()
	}
	var firstErr error
	if sess != nil {
		firstErr = sess.Close()
	}
	if aceService != nil {
		if err := aceService.Close(); firstErr == nil {
			firstErr = err
		}
	}
	if ws != nil {
		if err := ws.Close(); firstErr == nil {
			firstErr = err
		}
	}
	if sdk != nil && sess == nil {
		if err := sdk.Destroy(); firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func stringsTrim(value string) string {
	start, end := 0, len(value)
	for start < end && value[start] <= ' ' {
		start++
	}
	for end > start && value[end-1] <= ' ' {
		end--
	}
	return value[start:end]
}
