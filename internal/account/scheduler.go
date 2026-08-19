package account

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"
)

const schedulerMinimumDelay = time.Millisecond

// TaskFunc is the preferred callback shape for account-owned scheduled work.
// The scheduler also accepts func() and error-returning variants through Every
// so existing service callbacks can be migrated without adapters.
type TaskFunc func(context.Context)

// TaskStatus is an immutable snapshot of one registered task.
type TaskStatus struct {
	Name           string
	Kind           string
	Interval       time.Duration
	Jitter         time.Duration
	NextDelay      time.Duration
	CreatedAt      time.Time
	NextRunAt      time.Time
	LastRunAt      time.Time
	RunCount       uint64
	Running        bool
	Active         bool
	PreventOverlap bool
	LastError      string
}

// SchedulerStatus is a point-in-time view of one account's scheduler. Tasks
// are sorted by name to keep API responses and tests deterministic.
type SchedulerStatus struct {
	CreatedAt time.Time
	TaskCount int
	Tasks     []TaskStatus
}

type scheduledTask struct {
	name     string
	interval time.Duration
	jitter   time.Duration
	fn       taskFunc

	ctx    context.Context
	cancel context.CancelFunc

	createdAt time.Time
	nextRunAt time.Time
	lastRunAt time.Time
	nextDelay time.Duration
	runCount  uint64
	running   bool
	lastError string
}

// Scheduler owns all timers for one account Runtime. It deliberately has no
// package-level registry: stopping or discarding a Runtime stops only its own
// tasks.
type scheduler struct {
	mu sync.RWMutex

	parent    context.Context
	ctx       context.Context
	cancel    context.CancelFunc
	createdAt time.Time
	tasks     map[string]*scheduledTask
	autoStart bool

	randMu sync.Mutex
	rand   *rand.Rand
}

// NewScheduler creates an account-local scheduler. An optional parent context
// lets Runtime tie all scheduled work to its lifecycle.
func NewScheduler(parents ...context.Context) *scheduler {
	parent := context.Background()
	if len(parents) > 0 && parents[0] != nil {
		parent = parents[0]
	}
	s := &scheduler{
		parent:    parent,
		createdAt: time.Now(),
		tasks:     make(map[string]*scheduledTask),
		autoStart: true,
		rand:      rand.New(rand.NewSource(time.Now().UnixNano())),
	}
	return s
}

func (s *scheduler) ensureLocked() {
	if s.tasks == nil {
		s.tasks = make(map[string]*scheduledTask)
	}
	if s.parent == nil {
		s.parent = context.Background()
	}
	if s.createdAt.IsZero() {
		s.createdAt = time.Now()
	}
	if s.rand == nil {
		s.rand = rand.New(rand.NewSource(time.Now().UnixNano()))
	}
}

func (s *scheduler) startLocked(parent context.Context) []*scheduledTask {
	if s.ctx != nil {
		return nil
	}
	if parent == nil {
		parent = s.parent
	}
	if parent == nil {
		parent = context.Background()
	}
	s.parent = parent
	s.ctx, s.cancel = context.WithCancel(parent)
	tasks := make([]*scheduledTask, 0, len(s.tasks))
	for _, task := range s.tasks {
		task.ctx, task.cancel = context.WithCancel(s.ctx)
		tasks = append(tasks, task)
	}
	return tasks
}

// Start binds the scheduler to a lifecycle context and starts tasks that were
// registered before Start. Every also starts an idle scheduler lazily with its
// configured parent, which keeps the standalone API convenient in tests.
func (s *scheduler) Start(parent context.Context) error {
	s.mu.Lock()
	s.ensureLocked()
	tasks := s.startLocked(parent)
	s.mu.Unlock()
	for _, task := range tasks {
		go s.run(task)
	}
	return nil
}

// RuntimeScheduler adapts the named-task API to the lifecycle interface used
// by Runtime. StopTask retains explicit task control for callers that need it.
type RuntimeScheduler struct{ *scheduler }

// NewRuntimeScheduler constructs a scheduler suitable for Dependencies.Scheduler.
func NewRuntimeScheduler(parents ...context.Context) *RuntimeScheduler {
	instance := NewScheduler(parents...)
	instance.autoStart = false
	return &RuntimeScheduler{scheduler: instance}
}

func (s *RuntimeScheduler) Start(ctx context.Context) error {
	if s == nil || s.scheduler == nil {
		return errors.New("runtime scheduler is nil")
	}
	return s.scheduler.Start(ctx)
}

func (s *RuntimeScheduler) Stop() {
	if s == nil || s.scheduler == nil {
		return
	}
	s.scheduler.StopAll()
	_ = s.scheduler.Close()
}

func (s *RuntimeScheduler) StopTask(name string) bool {
	if s == nil || s.scheduler == nil {
		return false
	}
	return s.scheduler.Stop(name)
}

var _ Scheduler = (*RuntimeScheduler)(nil)

// Every replaces any existing task with the same name and runs fn at a
// randomly jittered interval until Stop, StopAll, Close, or the parent context
// cancels. The next delay is sampled from [interval-jitter, interval+jitter],
// clamped to at least one millisecond. A zero jitter gives a fixed interval.
//
// fn may be TaskFunc, func(context.Context), func(), or either of those forms
// returning error. Errors are recorded in Status and do not terminate a task.
func (s *scheduler) Every(name string, interval, jitter time.Duration, fn any) error {
	key := strings.TrimSpace(name)
	if key == "" {
		return errors.New("scheduler task name is required")
	}
	callback, err := normalizeTaskFunc(fn)
	if err != nil {
		return fmt.Errorf("scheduler task %q: %w", key, err)
	}
	interval = normalizeInterval(interval)
	if jitter < 0 {
		if jitter == time.Duration(-1<<63) {
			jitter = time.Duration(1<<63 - 1)
		} else {
			jitter = -jitter
		}
	}

	s.mu.Lock()
	s.ensureLocked()
	var startTasks []*scheduledTask
	if s.ctx == nil && s.autoStart {
		startTasks = s.startLocked(nil)
	}
	if old := s.tasks[key]; old != nil {
		if old.cancel != nil {
			old.cancel()
		}
	}
	var taskCtx context.Context
	var cancel context.CancelFunc
	if s.ctx != nil {
		taskCtx, cancel = context.WithCancel(s.ctx)
	}
	task := &scheduledTask{
		name:      key,
		interval:  interval,
		jitter:    jitter,
		fn:        callback,
		ctx:       taskCtx,
		cancel:    cancel,
		createdAt: time.Now(),
	}
	s.tasks[key] = task
	s.mu.Unlock()

	for _, existing := range startTasks {
		go s.run(existing)
	}
	if taskCtx != nil {
		go s.run(task)
	}
	return nil
}

// Stop cancels and removes a named task. It is safe to call more than once.
func (s *scheduler) Stop(name string) bool {
	key := strings.TrimSpace(name)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.tasks == nil {
		return false
	}
	task, ok := s.tasks[key]
	if !ok {
		return false
	}
	delete(s.tasks, key)
	if task.cancel != nil {
		task.cancel()
	}
	return true
}

// StopAll cancels every task owned by this scheduler and leaves the scheduler
// reusable for another Runtime lifecycle.
func (s *scheduler) StopAll() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for name, task := range s.tasks {
		delete(s.tasks, name)
		if task.cancel != nil {
			task.cancel()
		}
	}
}

// Close cancels the current scheduler context and all of its tasks. A Runtime
// can create another lifecycle with the same value after Close if needed.
func (s *scheduler) Close() error {
	s.mu.Lock()
	for name, task := range s.tasks {
		delete(s.tasks, name)
		if task.cancel != nil {
			task.cancel()
		}
	}
	cancel := s.cancel
	s.cancel = nil
	s.ctx = nil
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	return nil
}

// Has reports whether a task is currently registered.
func (s *scheduler) Has(name string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.tasks[strings.TrimSpace(name)]
	return ok
}

// Status returns a deterministic snapshot of all currently registered tasks.
func (s *scheduler) Status() SchedulerStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	status := SchedulerStatus{CreatedAt: s.createdAt, TaskCount: len(s.tasks)}
	status.Tasks = make([]TaskStatus, 0, len(s.tasks))
	for _, task := range s.tasks {
		status.Tasks = append(status.Tasks, task.status())
	}
	for i := 1; i < len(status.Tasks); i++ {
		for j := i; j > 0 && status.Tasks[j].Name < status.Tasks[j-1].Name; j-- {
			status.Tasks[j], status.Tasks[j-1] = status.Tasks[j-1], status.Tasks[j]
		}
	}
	return status
}

// Snapshot is an alias useful to callers that use snapshot terminology.
func (s *scheduler) Snapshot() SchedulerStatus { return s.Status() }

// Task returns one task snapshot without exposing mutable scheduler state.
func (s *scheduler) Task(name string) (TaskStatus, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	task, ok := s.tasks[strings.TrimSpace(name)]
	if !ok {
		return TaskStatus{}, false
	}
	return task.status(), true
}

func (s *scheduler) run(task *scheduledTask) {
	for {
		delay := s.nextDelay(task.interval, task.jitter)
		if !s.setNext(task, delay) {
			return
		}

		// A fresh ticker per cycle lets each iteration receive an independent
		// jitter while retaining ticker/context cancellation semantics.
		ticker := time.NewTicker(delay)
		select {
		case <-task.ctx.Done():
			ticker.Stop()
			s.removeIfCurrent(task)
			return
		case <-ticker.C:
			ticker.Stop()
		}

		if !s.begin(task) {
			return
		}
		err := callTask(task.ctx, task.fn)
		s.end(task, err)
		if task.ctx.Err() != nil {
			s.removeIfCurrent(task)
			return
		}
	}
}

func (s *scheduler) nextDelay(interval, jitter time.Duration) time.Duration {
	if jitter <= 0 {
		return interval
	}
	min := interval - jitter
	if min < schedulerMinimumDelay {
		min = schedulerMinimumDelay
	}
	max := interval + jitter
	if jitter > time.Duration(1<<63-1)-interval {
		max = time.Duration(1<<63 - 1)
	}
	if max < min {
		max = min
	}
	span := max - min
	if span <= 0 {
		return min
	}
	s.randMu.Lock()
	n := s.rand.Int63n(int64(span) + 1)
	s.randMu.Unlock()
	return min + time.Duration(n)
}

func (s *scheduler) setNext(task *scheduledTask, delay time.Duration) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	current, ok := s.tasks[task.name]
	if !ok || current != task || task.ctx.Err() != nil {
		return false
	}
	task.nextDelay = delay
	task.nextRunAt = time.Now().Add(delay)
	return true
}

func (s *scheduler) begin(task *scheduledTask) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	current, ok := s.tasks[task.name]
	if !ok || current != task || task.ctx.Err() != nil {
		return false
	}
	task.running = true
	task.lastRunAt = time.Now()
	task.runCount++
	task.nextRunAt = time.Time{}
	return true
}

func (s *scheduler) end(task *scheduledTask, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if current, ok := s.tasks[task.name]; !ok || current != task {
		return
	}
	task.running = false
	if err != nil {
		task.lastError = err.Error()
	}
}

func (s *scheduler) removeIfCurrent(task *scheduledTask) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if current, ok := s.tasks[task.name]; ok && current == task {
		delete(s.tasks, task.name)
	}
}

func (t *scheduledTask) status() TaskStatus {
	return TaskStatus{
		Name:           t.name,
		Kind:           "interval",
		Interval:       t.interval,
		Jitter:         t.jitter,
		NextDelay:      t.nextDelay,
		CreatedAt:      t.createdAt,
		NextRunAt:      t.nextRunAt,
		LastRunAt:      t.lastRunAt,
		RunCount:       t.runCount,
		Running:        t.running,
		Active:         true,
		PreventOverlap: true,
		LastError:      t.lastError,
	}
}

func normalizeInterval(interval time.Duration) time.Duration {
	if interval < schedulerMinimumDelay {
		return schedulerMinimumDelay
	}
	return interval
}

type taskFunc func(context.Context) error

func normalizeTaskFunc(fn any) (taskFunc, error) {
	switch callback := fn.(type) {
	case TaskFunc:
		if callback == nil {
			return nil, errors.New("callback is required")
		}
		return func(ctx context.Context) error {
			callback(ctx)
			return nil
		}, nil
	case func(context.Context):
		if callback == nil {
			return nil, errors.New("callback is required")
		}
		return func(ctx context.Context) error {
			callback(ctx)
			return nil
		}, nil
	case func():
		if callback == nil {
			return nil, errors.New("callback is required")
		}
		return func(context.Context) error {
			callback()
			return nil
		}, nil
	case func(context.Context) error:
		if callback == nil {
			return nil, errors.New("callback is required")
		}
		return callback, nil
	case func() error:
		if callback == nil {
			return nil, errors.New("callback is required")
		}
		return func(context.Context) error { return callback() }, nil
	default:
		return nil, errors.New("callback must be func(), func(context.Context), or an error-returning variant")
	}
}

func callTask(ctx context.Context, fn taskFunc) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("panic: %v", recovered)
		}
	}()
	return fn(ctx)
}
