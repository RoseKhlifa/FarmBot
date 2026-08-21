package account

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/RoseKhlifa/FarmBot/internal/game/transport"
)

const (
	LoopFarm   = "farm"
	LoopHelp   = "help"
	LoopSteal  = "steal"
	LoopDaily  = "daily"
	LoopStatus = "status_sync"
)

var (
	ErrLoopsStarted   = errors.New("account loops are already started")
	ErrLoopsStopped   = errors.New("account loops are stopped")
	ErrLoopsScheduler = errors.New("account loops require a named scheduler")
)

// LoopIntervals contains the cadence and jitter for each Runtime-owned loop.
// Jitter is passed to Scheduler.Every as a symmetric duration around the base
// interval. The defaults preserve the staggered ranges used by worker.js.
type LoopIntervals struct {
	Farm        time.Duration
	FarmJitter  time.Duration
	Help        time.Duration
	HelpJitter  time.Duration
	Steal       time.Duration
	StealJitter time.Duration
	Daily       time.Duration
	Status      time.Duration
}

// DefaultLoopIntervals returns conservative account-local cadences. Callers
// can override any non-zero field through LoopOptions.
func DefaultLoopIntervals() LoopIntervals {
	return LoopIntervals{
		Farm:        4 * time.Second,
		FarmJitter:  1 * time.Second,
		Help:        17*time.Second + 500*time.Millisecond,
		HelpJitter:  2*time.Second + 500*time.Millisecond,
		Steal:       12*time.Second + 500*time.Millisecond,
		StealJitter: 2*time.Second + 500*time.Millisecond,
		Daily:       60 * time.Second,
		Status:      5 * time.Second,
	}
}

// LoopScheduler is the scheduling capability required by LoopController. It
// is intentionally smaller than the Runtime lifecycle interface so tests can
// supply a deterministic fake without opening a network or account session.
type LoopScheduler interface {
	Every(name string, interval, jitter time.Duration, fn any) error
}

// DomainScheduler is the named-task surface exposed to account domains.
type DomainScheduler interface {
	LoopScheduler
	Stop(string) bool
}

// LoopHooks are account-local domain seams. The loop layer only invokes these
// callbacks; farm, friend, warehouse and task algorithms remain elsewhere.
// Action fields accept func(context.Context), func(context.Context) error,
// func(), and their error-returning variants. Event fields additionally accept
// a transport.Event argument.
type LoopHooks struct {
	Farm  any
	Help  any
	Steal any
	Daily any

	OnSell          any
	OnFarmHarvested any
	OnKickout       any
	OnDisconnect    any
	OnTaskInfo      any
}

// LoopOptions configures one LoopController. Events and Scheduler are
// optional when a Runtime already owns a logged-in Session and scheduler.
type LoopOptions struct {
	Intervals LoopIntervals
	Hooks     LoopHooks
	Events    <-chan transport.Event
	Scheduler LoopScheduler
}

// LoopController owns the automation loops for exactly one Runtime. It is a
// value assembled by the account manager, rather than a package singleton, so
// stopping one account cannot affect another account's timers or events.
type LoopController struct {
	runtime   *Runtime
	options   LoopOptions
	intervals LoopIntervals
	hooks     loopHooks
	scheduler LoopScheduler
	events    <-chan transport.Event

	mu       sync.Mutex
	ctx      context.Context
	cancel   context.CancelFunc
	started  bool
	names    []string
	eventsWG sync.WaitGroup
}

type loopHooks struct {
	farm, help, steal, daily                                     taskFunc
	onSell, onFarmHarvested, onKickout, onDisconnect, onTaskInfo eventFunc
}

type eventFunc func(context.Context, transport.Event) error

// NewLoopController creates an idle controller. It does not start timers or
// consume transport events until Start is called.
func NewLoopController(runtime *Runtime, options LoopOptions) (*LoopController, error) {
	if runtime == nil {
		return nil, errors.New("loop controller requires a Runtime")
	}
	intervals := mergeLoopIntervals(options.Intervals)
	hooks, err := normalizeLoopHooks(options.Hooks)
	if err != nil {
		return nil, err
	}
	scheduler := options.Scheduler
	if scheduler == nil {
		runtime.mu.Lock()
		if runtime.sched != nil {
			scheduler, _ = runtime.sched.(LoopScheduler)
		}
		runtime.mu.Unlock()
	}
	return &LoopController{
		runtime:   runtime,
		options:   options,
		intervals: intervals,
		hooks:     hooks,
		scheduler: scheduler,
		events:    options.Events,
	}, nil
}

// NewAutomationLoops is an explicit alias for callers using the domain term.
func NewAutomationLoops(runtime *Runtime, options LoopOptions) (*LoopController, error) {
	return NewLoopController(runtime, options)
}

// NewLoops creates a controller from an existing Runtime's account context.
// It is useful to keep manager wiring concise while retaining explicit
// ownership of the returned controller.
func (r *Runtime) NewLoops(options LoopOptions) (*LoopController, error) {
	return NewLoopController(r, options)
}

// Start registers all five scheduler entries, launches the event bridge, and
// performs the initial daily routine/status refresh. Start is idempotent only
// at the caller level: a second call returns ErrLoopsStarted.
func (c *LoopController) Start(parent context.Context) error {
	if c == nil {
		return errors.New("loop controller is nil")
	}
	if parent == nil {
		parent = context.Background()
	}

	c.mu.Lock()
	if c.started {
		c.mu.Unlock()
		return ErrLoopsStarted
	}
	if c.runtime == nil {
		c.mu.Unlock()
		return errors.New("loop controller has no Runtime")
	}
	if c.scheduler == nil {
		c.mu.Unlock()
		return ErrLoopsScheduler
	}
	ctx, cancel := context.WithCancel(parent)
	c.ctx, c.cancel = ctx, cancel
	c.started = true
	c.names = nil
	scheduler := c.scheduler
	events := c.events
	c.mu.Unlock()

	if starter, ok := scheduler.(interface{ Start(context.Context) error }); ok {
		if err := starter.Start(ctx); err != nil {
			_ = c.Stop()
			return fmt.Errorf("start account scheduler: %w", err)
		}
	}
	registrations := []struct {
		name     string
		interval time.Duration
		jitter   time.Duration
		fn       taskFunc
	}{
		{LoopFarm, c.intervals.Farm, c.intervals.FarmJitter, c.hooks.farm},
		{LoopHelp, c.intervals.Help, c.intervals.HelpJitter, c.hooks.help},
		{LoopSteal, c.intervals.Steal, c.intervals.StealJitter, c.hooks.steal},
		{LoopDaily, c.intervals.Daily, 0, c.hooks.daily},
		{LoopStatus, c.intervals.Status, 0, c.statusTick},
	}
	for _, registration := range registrations {
		fn := registration.fn
		if fn == nil {
			fn = func(context.Context) error { return nil }
		}
		name := registration.name
		callback := func(loopCtx context.Context) {
			c.markNextAction(name)
			_ = callTask(loopCtx, fn)
		}
		if err := scheduler.Every(name, registration.interval, registration.jitter, callback); err != nil {
			stopNamedTask(scheduler, name)
			_ = c.Stop()
			return fmt.Errorf("register account loop %q: %w", name, err)
		}
		c.mu.Lock()
		c.names = append(c.names, name)
		c.mu.Unlock()
	}

	if c.hooks.daily != nil {
		go func() { _ = callTask(ctx, c.hooks.daily) }()
	}
	if events != nil {
		c.mu.Lock()
		c.events = events
		c.mu.Unlock()
		c.eventsWG.Add(1)
		go c.eventLoop(ctx, events)
	}
	return nil
}

// Stop cancels callbacks, removes named entries where the concrete scheduler
// exposes that operation, and waits for the event bridge to exit.
func (c *LoopController) Stop() error {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	if !c.started {
		c.mu.Unlock()
		return nil
	}
	c.started = false
	cancel := c.cancel
	names := append([]string(nil), c.names...)
	scheduler := c.scheduler
	c.cancel = nil
	c.ctx = nil
	c.names = nil
	c.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	for _, name := range names {
		stopNamedTask(scheduler, name)
	}
	c.eventsWG.Wait()
	if c.runtime != nil && c.runtime.status != nil {
		c.runtime.status.SetNextAction("", time.Time{})
	}
	return nil
}

// Close makes LoopController usable as a lifecycle resource.
func (c *LoopController) Close() error { return c.Stop() }

// Started reports whether this controller currently owns active loop entries.
func (c *LoopController) Started() bool {
	if c == nil {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.started
}

// HandleEvent routes one transport event synchronously. It is exported so
// tests and a future Runtime fan-out can feed events without sharing a channel
// consumer with Runtime.watchSession.
func (c *LoopController) HandleEvent(ctx context.Context, event transport.Event) error {
	if c == nil {
		return errors.New("loop controller is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if c.runtime != nil && c.runtime.status != nil {
		switch event.Type {
		case transport.EventSell:
			c.runtime.status.AddOperation("sell", 1)
		case transport.EventFarmHarvested:
			c.runtime.status.AddOperation("farm_harvested", 1)
		case transport.EventTaskInfoNotify:
			c.runtime.status.AddOperation("task_info_notify", 1)
		case transport.EventKickout, transport.EventDisconnect:
			err := event.Err
			if err == nil && event.Reason != "" {
				err = errors.New(event.Reason)
			}
			c.runtime.status.MarkOffline(err, time.Now())
		}
	}

	var handler eventFunc
	switch event.Type {
	case transport.EventSell:
		handler = c.hooks.onSell
	case transport.EventFarmHarvested:
		handler = c.hooks.onFarmHarvested
	case transport.EventKickout:
		handler = c.hooks.onKickout
	case transport.EventDisconnect:
		handler = c.hooks.onDisconnect
	case transport.EventTaskInfoNotify:
		handler = c.hooks.onTaskInfo
	}
	if handler == nil {
		return nil
	}
	return handler(ctx, event)
}

// Status returns the current per-account status snapshot. Runtime remains the
// source of truth; this method is a convenience for loop owners.
func (c *LoopController) Status() StatusSnapshot {
	if c == nil || c.runtime == nil {
		return StatusSnapshot{}
	}
	return c.runtime.Status()
}

func (c *LoopController) eventLoop(ctx context.Context, events <-chan transport.Event) {
	defer c.eventsWG.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-events:
			if !ok {
				return
			}
			_ = c.HandleEvent(ctx, event)
		}
	}
}

func (c *LoopController) statusTick(ctx context.Context) error {
	if c.runtime == nil {
		return nil
	}
	c.runtime.mu.Lock()
	sess := c.runtime.session
	stats := c.runtime.stats
	status := c.runtime.status
	accountID := c.runtime.accountID
	c.runtime.mu.Unlock()
	if status == nil {
		return nil
	}
	if sess != nil {
		status.UpdateUser(sess.State())
	}
	if stats != nil && accountID != "" {
		operations, err := stats.GetOperations(ctx, accountID, time.Now().Format("2006-01-02"))
		if err == nil {
			for name, value := range operations {
				status.SetOperation(name, value)
			}
		}
	}
	c.markNextAction(LoopStatus)
	return nil
}

func (c *LoopController) markNextAction(current string) {
	if c.runtime == nil || c.runtime.status == nil {
		return
	}
	if provider, ok := c.scheduler.(interface{ Status() SchedulerStatus }); ok {
		status := provider.Status()
		var next TaskStatus
		found := false
		for _, task := range status.Tasks {
			if task.NextRunAt.IsZero() || (found && !task.NextRunAt.Before(next.NextRunAt)) {
				continue
			}
			next, found = task, true
		}
		if found {
			c.runtime.status.SetNextAction(next.Name, next.NextRunAt)
			return
		}
	}
	if current != "" {
		c.runtime.status.SetNextAction(current, time.Now())
	}
}

func stopNamedTask(scheduler LoopScheduler, name string) {
	if stopper, ok := scheduler.(interface{ Stop(string) bool }); ok {
		stopper.Stop(name)
		return
	}
	if stopper, ok := scheduler.(interface{ StopTask(string) bool }); ok {
		stopper.StopTask(name)
	}
}

func mergeLoopIntervals(value LoopIntervals) LoopIntervals {
	defaults := DefaultLoopIntervals()
	if value.Farm <= 0 {
		value.Farm = defaults.Farm
	}
	if value.Help <= 0 {
		value.Help = defaults.Help
	}
	if value.Steal <= 0 {
		value.Steal = defaults.Steal
	}
	if value.Daily <= 0 {
		value.Daily = defaults.Daily
	}
	if value.Status <= 0 {
		value.Status = defaults.Status
	}
	if value.FarmJitter < 0 {
		value.FarmJitter = 0
	}
	if value.HelpJitter < 0 {
		value.HelpJitter = 0
	}
	if value.StealJitter < 0 {
		value.StealJitter = 0
	}
	return value
}

func normalizeLoopHooks(hooks LoopHooks) (loopHooks, error) {
	var result loopHooks
	var err error
	if result.farm, err = normalizeOptionalTask(hooks.Farm, "farm"); err != nil {
		return loopHooks{}, err
	}
	if result.help, err = normalizeOptionalTask(hooks.Help, "help"); err != nil {
		return loopHooks{}, err
	}
	if result.steal, err = normalizeOptionalTask(hooks.Steal, "steal"); err != nil {
		return loopHooks{}, err
	}
	if result.daily, err = normalizeOptionalTask(hooks.Daily, "daily"); err != nil {
		return loopHooks{}, err
	}
	if result.onSell, err = normalizeOptionalEvent(hooks.OnSell, "sell"); err != nil {
		return loopHooks{}, err
	}
	if result.onFarmHarvested, err = normalizeOptionalEvent(hooks.OnFarmHarvested, "farmHarvested"); err != nil {
		return loopHooks{}, err
	}
	if result.onKickout, err = normalizeOptionalEvent(hooks.OnKickout, "kickout"); err != nil {
		return loopHooks{}, err
	}
	if result.onDisconnect, err = normalizeOptionalEvent(hooks.OnDisconnect, "disconnect"); err != nil {
		return loopHooks{}, err
	}
	if result.onTaskInfo, err = normalizeOptionalEvent(hooks.OnTaskInfo, "taskInfoNotify"); err != nil {
		return loopHooks{}, err
	}
	return result, nil
}

func normalizeOptionalTask(value any, name string) (taskFunc, error) {
	if value == nil {
		return nil, nil
	}
	fn, err := normalizeTaskFunc(value)
	if err != nil {
		return nil, fmt.Errorf("%s loop: %w", name, err)
	}
	return fn, nil
}

type EventHandler func(context.Context, transport.Event)

func normalizeOptionalEvent(value any, name string) (eventFunc, error) {
	if value == nil {
		return nil, nil
	}
	switch handler := value.(type) {
	case EventHandler:
		if handler == nil {
			return nil, fmt.Errorf("%s event handler is nil", name)
		}
		return func(ctx context.Context, event transport.Event) error {
			handler(ctx, event)
			return nil
		}, nil
	case func(context.Context, transport.Event):
		if handler == nil {
			return nil, fmt.Errorf("%s event handler is nil", name)
		}
		return func(ctx context.Context, event transport.Event) error {
			handler(ctx, event)
			return nil
		}, nil
	case func(transport.Event):
		if handler == nil {
			return nil, fmt.Errorf("%s event handler is nil", name)
		}
		return func(_ context.Context, event transport.Event) error {
			handler(event)
			return nil
		}, nil
	case func(context.Context, transport.Event) error:
		if handler == nil {
			return nil, fmt.Errorf("%s event handler is nil", name)
		}
		return handler, nil
	case func(transport.Event) error:
		if handler == nil {
			return nil, fmt.Errorf("%s event handler is nil", name)
		}
		return func(_ context.Context, event transport.Event) error { return handler(event) }, nil
	default:
		return nil, fmt.Errorf("%s event handler has unsupported type %T", name, value)
	}
}
