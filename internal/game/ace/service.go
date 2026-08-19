// Package ace drives the per-account anti-cheat lifecycle.
package ace

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/RoseKhlifa/FarmBot/internal/game/pb"
	"github.com/RoseKhlifa/FarmBot/internal/game/tsdk"
	"google.golang.org/protobuf/proto"
)

const (
	ServiceName = "gamepb.acepb.AceService"
	MethodName  = "AntiData"

	ProcessInterval       = 5 * time.Second
	PollInterval          = 5 * time.Second
	ACEHeartbeatInterval  = 25 * time.Second
	UserHeartbeatInterval = 25 * time.Second
	SpeedCheckInterval    = 30 * time.Second
	StatusDelay           = 150 * time.Second
	FunctionCheckDelay    = 180 * time.Second
	RequestTimeout        = 10 * time.Second
	MinBackoff            = 2 * time.Second
	MaxBackoff            = 30 * time.Second
)

var defaultFunctionChecks = []string{
	"processReceivedData",
	"heartbeatTick",
	"getDataToServer",
	"sendDataFromServer",
}

// Runtime is the subset of the P2-02 TSDK runtime used by the ACE lifecycle.
type Runtime interface {
	ProcessReceivedData() error
	HeartbeatTick() error
	DetectSpeedHack(time.Duration) error
	SendStatus() error
	CheckFunctionArray([]string, uint32) error
	GetDataToServer() ([]byte, error)
	SendDataFromServer([]byte) error
	GetStatus() tsdk.Status
}

// Sender sends one encoded game RPC and returns the encoded response body.
// The P2-04 transport can satisfy this interface without ACE importing it.
type Sender interface {
	Send(context.Context, string, string, []byte, time.Duration) ([]byte, error)
}

// SendFunc adapts a function to Sender.
type SendFunc func(context.Context, string, string, []byte, time.Duration) ([]byte, error)

func (fn SendFunc) Send(ctx context.Context, service, method string, body []byte, timeout time.Duration) ([]byte, error) {
	return fn(ctx, service, method, body, timeout)
}

// Logger receives lifecycle messages without coupling ACE to a logger package.
type Logger func(level, message string)

// Intervals defines every ACE and user-heartbeat cadence. DefaultIntervals is
// the production contract; custom values exist only for deterministic tests.
type Intervals struct {
	Process       time.Duration
	Poll          time.Duration
	ACEHeartbeat  time.Duration
	UserHeartbeat time.Duration
	SpeedCheck    time.Duration
	Status        time.Duration
	FunctionCheck time.Duration
	Request       time.Duration
	BackoffMin    time.Duration
	BackoffMax    time.Duration
}

// DefaultIntervals returns the fixed cadence documented by the Node runtime.
func DefaultIntervals() Intervals {
	return Intervals{
		Process:       ProcessInterval,
		Poll:          PollInterval,
		ACEHeartbeat:  ACEHeartbeatInterval,
		UserHeartbeat: UserHeartbeatInterval,
		SpeedCheck:    SpeedCheckInterval,
		Status:        StatusDelay,
		FunctionCheck: FunctionCheckDelay,
		Request:       RequestTimeout,
		BackoffMin:    MinBackoff,
		BackoffMax:    MaxBackoff,
	}
}

// Options configures one account-scoped ACE service.
type Options struct {
	Runtime        Runtime
	Sender         Sender
	IsConnected    func() bool
	UserHeartbeat  func(context.Context) error
	Logger         Logger
	Intervals      Intervals
	FunctionChecks []string
}

// Status is a race-free snapshot of the service and its TSDK runtime.
type Status struct {
	Running            bool
	InFlight           bool
	Failures           uint32
	UploadCount        uint64
	LastUploadAt       time.Time
	LastError          string
	UserHeartbeatTicks uint64
	Runtime            tsdk.Status
}

// Service owns all ACE timers for one account. A Service must not be shared by
// accounts; each account Runtime creates and stops its own instance.
type Service struct {
	runtime        Runtime
	sender         Sender
	isConnected    func() bool
	userHeartbeat  func(context.Context) error
	logger         Logger
	intervals      Intervals
	functionChecks []string

	mu                 sync.Mutex
	runtimeMu          sync.Mutex
	running            bool
	inFlight           bool
	failures           uint32
	uploadCount        uint64
	lastUploadAt       time.Time
	lastError          string
	userHeartbeatTicks uint64
	cancel             context.CancelFunc
	done               chan struct{}
	wg                 sync.WaitGroup
}

// New validates dependencies and creates an idle account-scoped service.
func New(options Options) (*Service, error) {
	if options.Runtime == nil {
		return nil, errors.New("ACE runtime is required")
	}
	if options.Sender == nil {
		return nil, errors.New("ACE sender is required")
	}
	intervals := options.Intervals
	if intervals == (Intervals{}) {
		intervals = DefaultIntervals()
	}
	if err := intervals.validate(); err != nil {
		return nil, err
	}
	isConnected := options.IsConnected
	if isConnected == nil {
		isConnected = func() bool { return true }
	}
	checks := append([]string(nil), options.FunctionChecks...)
	if len(checks) == 0 {
		checks = append(checks, defaultFunctionChecks...)
	}
	return &Service{
		runtime:        options.Runtime,
		sender:         options.Sender,
		isConnected:    isConnected,
		userHeartbeat:  options.UserHeartbeat,
		logger:         options.Logger,
		intervals:      intervals,
		functionChecks: checks,
	}, nil
}

// Start attaches all timers to ctx. Canceling ctx has the same lifecycle
// effect as Stop and ensures reconnects cannot retain old ACE goroutines.
func (s *Service) Start(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if !s.runtimeStatus().Ready {
		return errors.New("cannot start ACE before TSDK is ready")
	}

	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return nil
	}
	runCtx, cancel := context.WithCancel(ctx)
	s.running = true
	s.inFlight = false
	s.cancel = cancel
	s.done = make(chan struct{})
	done := s.done
	s.mu.Unlock()

	s.run(runCtx, func(ctx context.Context) { s.runProcess(ctx) })
	s.run(runCtx, func(ctx context.Context) { s.runACEHeartbeat(ctx) })
	s.run(runCtx, func(ctx context.Context) { s.runSpeedCheck(ctx) })
	s.run(runCtx, func(ctx context.Context) { s.runOnce(ctx, s.intervals.Status, "status report", s.runtime.SendStatus) })
	s.run(runCtx, func(ctx context.Context) {
		s.runOnce(ctx, s.intervals.FunctionCheck, "function check", func() error {
			return s.runtime.CheckFunctionArray(s.functionChecks, 0)
		})
	})
	s.run(runCtx, func(ctx context.Context) { s.runPoll(ctx) })
	if s.userHeartbeat != nil {
		s.run(runCtx, func(ctx context.Context) { s.runUserHeartbeat(ctx) })
	}

	go func() {
		s.wg.Wait()
		s.mu.Lock()
		s.running = false
		s.inFlight = false
		s.cancel = nil
		s.mu.Unlock()
		close(done)
	}()
	s.log("info", "ACE started")
	return nil
}

// Stop cancels every timer and waits for any context-aware request to return.
func (s *Service) Stop() {
	s.mu.Lock()
	cancel := s.cancel
	done := s.done
	s.mu.Unlock()
	if cancel == nil || done == nil {
		return
	}
	cancel()
	<-done
	s.log("info", "ACE stopped")
}

// Close allows Service to be used as an account-runtime lifecycle resource.
func (s *Service) Close() error {
	s.Stop()
	return nil
}

// Status returns current counters and the underlying P2-02 runtime status.
func (s *Service) Status() Status {
	s.mu.Lock()
	status := Status{
		Running:            s.running,
		InFlight:           s.inFlight,
		Failures:           s.failures,
		UploadCount:        s.uploadCount,
		LastUploadAt:       s.lastUploadAt,
		LastError:          s.lastError,
		UserHeartbeatTicks: s.userHeartbeatTicks,
	}
	s.mu.Unlock()
	status.Runtime = s.runtimeStatus()
	return status
}

func (s *Service) run(ctx context.Context, fn func(context.Context)) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		fn(ctx)
	}()
}

func (s *Service) runProcess(ctx context.Context) {
	s.runTicker(ctx, s.intervals.Process, "received-data processing", func() error {
		return s.withRuntime(s.runtime.ProcessReceivedData)
	})
}

func (s *Service) runACEHeartbeat(ctx context.Context) {
	s.runTicker(ctx, s.intervals.ACEHeartbeat, "ACE heartbeat", func() error {
		return s.withRuntime(s.runtime.HeartbeatTick)
	})
}

func (s *Service) runSpeedCheck(ctx context.Context) {
	last := time.Now()
	ticker := time.NewTicker(s.intervals.SpeedCheck)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			elapsed := now.Sub(last)
			last = now
			if err := s.withRuntime(func() error { return s.runtime.DetectSpeedHack(elapsed) }); err != nil {
				s.recordError("speed check", err)
			}
		}
	}
}

func (s *Service) runOnce(ctx context.Context, delay time.Duration, operation string, fn func() error) {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return
	case <-timer.C:
		if err := s.withRuntime(fn); err != nil {
			s.recordError(operation, err)
		}
	}
}

func (s *Service) runUserHeartbeat(ctx context.Context) {
	ticker := time.NewTicker(s.intervals.UserHeartbeat)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.userHeartbeat(ctx); err != nil {
				s.recordError("user heartbeat", err)
				continue
			}
			s.mu.Lock()
			s.userHeartbeatTicks++
			s.mu.Unlock()
		}
	}
}

func (s *Service) runTicker(ctx context.Context, interval time.Duration, operation string, fn func() error) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := fn(); err != nil {
				s.recordError(operation, err)
			}
		}
	}
}

func (s *Service) runPoll(ctx context.Context) {
	delay := s.intervals.Poll
	timer := time.NewTimer(delay)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			delay = s.poll(ctx)
			timer.Reset(delay)
		}
	}
}

func (s *Service) poll(ctx context.Context) time.Duration {
	if !s.isConnected() {
		return s.intervals.Poll
	}

	data, err := s.getDataToServer()
	if err != nil {
		return s.fail("get ACE upload data", err)
	}
	if len(data) == 0 {
		return s.intervals.Poll
	}
	request, err := proto.Marshal(&pb.AntiDataRequest{Data: data})
	if err != nil {
		return s.fail("encode AntiData request", err)
	}
	if !s.beginRequest() {
		return s.intervals.Poll
	}
	defer s.endRequest()

	requestCtx, cancel := context.WithTimeout(ctx, s.intervals.Request)
	response, err := s.sender.Send(requestCtx, ServiceName, MethodName, request, s.intervals.Request)
	cancel()
	if err != nil {
		return s.fail("send AntiData", err)
	}
	var reply pb.AntiDataReply
	if err := proto.Unmarshal(response, &reply); err != nil {
		return s.fail("decode AntiData reply", err)
	}
	if len(reply.Data) > 0 {
		if err := s.sendDataFromServer(reply.Data); err != nil {
			return s.fail("feed ACE server data", err)
		}
	}

	s.mu.Lock()
	s.failures = 0
	s.uploadCount++
	s.lastUploadAt = time.Now()
	s.mu.Unlock()
	s.log("info", fmt.Sprintf("ACE upload succeeded: sent %d bytes, received %d bytes", len(data), len(reply.Data)))
	return s.intervals.Poll
}

func (s *Service) beginRequest() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.running || s.inFlight {
		return false
	}
	s.inFlight = true
	return true
}

func (s *Service) endRequest() {
	s.mu.Lock()
	s.inFlight = false
	s.mu.Unlock()
}

func (s *Service) fail(operation string, err error) time.Duration {
	s.mu.Lock()
	s.failures++
	failures := s.failures
	s.lastError = err.Error()
	s.mu.Unlock()
	delay := backoffDelay(failures, s.intervals.BackoffMin, s.intervals.BackoffMax)
	s.log("warn", fmt.Sprintf("ACE %s failed: %v; retrying in %s", operation, err, delay))
	return delay
}

func (s *Service) recordError(operation string, err error) {
	s.mu.Lock()
	s.lastError = err.Error()
	s.mu.Unlock()
	s.log("warn", fmt.Sprintf("ACE %s failed: %v", operation, err))
}

func (s *Service) getDataToServer() ([]byte, error) {
	s.runtimeMu.Lock()
	defer s.runtimeMu.Unlock()
	return s.runtime.GetDataToServer()
}

func (s *Service) sendDataFromServer(data []byte) error {
	copyOfData := append([]byte(nil), data...)
	return s.withRuntime(func() error { return s.runtime.SendDataFromServer(copyOfData) })
}

func (s *Service) runtimeStatus() tsdk.Status {
	s.runtimeMu.Lock()
	defer s.runtimeMu.Unlock()
	return s.runtime.GetStatus()
}

func (s *Service) withRuntime(fn func() error) error {
	s.runtimeMu.Lock()
	defer s.runtimeMu.Unlock()
	return fn()
}

func (s *Service) log(level, message string) {
	if s.logger != nil {
		s.logger(level, message)
	}
}

func (i Intervals) validate() error {
	values := map[string]time.Duration{
		"process": i.Process, "poll": i.Poll, "ACE heartbeat": i.ACEHeartbeat,
		"user heartbeat": i.UserHeartbeat, "speed check": i.SpeedCheck,
		"status": i.Status, "function check": i.FunctionCheck,
		"request": i.Request, "minimum backoff": i.BackoffMin, "maximum backoff": i.BackoffMax,
	}
	for name, value := range values {
		if value <= 0 {
			return fmt.Errorf("ACE %s interval must be positive", name)
		}
	}
	if i.BackoffMin > i.BackoffMax {
		return errors.New("ACE minimum backoff must not exceed maximum backoff")
	}
	return nil
}

func backoffDelay(failures uint32, minimum, maximum time.Duration) time.Duration {
	if failures == 0 {
		return minimum
	}
	delay := minimum
	for n := uint32(1); n < failures; n++ {
		if delay >= maximum/2 {
			return maximum
		}
		delay *= 2
	}
	if delay > maximum {
		return maximum
	}
	return delay
}
