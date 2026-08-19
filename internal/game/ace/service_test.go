package ace

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/RoseKhlifa/FarmBot/internal/game/pb"
	"github.com/RoseKhlifa/FarmBot/internal/game/tsdk"
	"google.golang.org/protobuf/proto"
)

type mockRuntime struct {
	mu        sync.Mutex
	status    tsdk.Status
	process   int
	heartbeat int
	speed     int
	statusN   int
	function  int
	getData   int
	sentData  [][]byte
	data      []byte
}

func newMockRuntime() *mockRuntime {
	return &mockRuntime{status: tsdk.Status{Ready: true, AccountID: "test-account"}, data: []byte("anti-data")}
}

func (m *mockRuntime) ProcessReceivedData() error {
	m.mu.Lock()
	m.process++
	m.mu.Unlock()
	return nil
}

func (m *mockRuntime) HeartbeatTick() error {
	m.mu.Lock()
	m.heartbeat++
	m.mu.Unlock()
	return nil
}

func (m *mockRuntime) DetectSpeedHack(time.Duration) error {
	m.mu.Lock()
	m.speed++
	m.mu.Unlock()
	return nil
}

func (m *mockRuntime) SendStatus() error {
	m.mu.Lock()
	m.statusN++
	m.mu.Unlock()
	return nil
}

func (m *mockRuntime) CheckFunctionArray([]string, uint32) error {
	m.mu.Lock()
	m.function++
	m.mu.Unlock()
	return nil
}

func (m *mockRuntime) GetDataToServer() ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.getData++
	return append([]byte(nil), m.data...), nil
}

func (m *mockRuntime) SendDataFromServer(data []byte) error {
	m.mu.Lock()
	m.sentData = append(m.sentData, append([]byte(nil), data...))
	m.mu.Unlock()
	return nil
}

func (m *mockRuntime) GetStatus() tsdk.Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.status
}

func (m *mockRuntime) counts() (int, int, int, int, int, int, int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.process, m.heartbeat, m.speed, m.statusN, m.function, m.getData, len(m.sentData)
}

type mockSender struct {
	mu       sync.Mutex
	count    int
	requests [][]byte
	reply    []byte
	started  chan struct{}
	release  chan struct{}
}

func (s *mockSender) Send(ctx context.Context, service, method string, body []byte, timeout time.Duration) ([]byte, error) {
	if service != ServiceName || method != MethodName || timeout <= 0 {
		return nil, errors.New("unexpected AntiData request metadata")
	}
	s.mu.Lock()
	s.count++
	s.requests = append(s.requests, append([]byte(nil), body...))
	count := s.count
	reply := append([]byte(nil), s.reply...)
	s.mu.Unlock()
	if count == 1 && s.started != nil {
		close(s.started)
	}
	if s.release != nil {
		select {
		case <-s.release:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return reply, nil
}

func (s *mockSender) countNow() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.count
}

func testIntervals() Intervals {
	return Intervals{
		Process:       8 * time.Millisecond,
		Poll:          8 * time.Millisecond,
		ACEHeartbeat:  8 * time.Millisecond,
		UserHeartbeat: 8 * time.Millisecond,
		SpeedCheck:    8 * time.Millisecond,
		Status:        24 * time.Millisecond,
		FunctionCheck: 24 * time.Millisecond,
		Request:       100 * time.Millisecond,
		BackoffMin:    2 * time.Millisecond,
		BackoffMax:    16 * time.Millisecond,
	}
}

func TestDefaultIntervalsMatchNodeContract(t *testing.T) {
	intervals := DefaultIntervals()
	wants := map[string]time.Duration{
		"process":        intervals.Process,
		"poll":           intervals.Poll,
		"ACE heartbeat":  intervals.ACEHeartbeat,
		"user heartbeat": intervals.UserHeartbeat,
		"speed":          intervals.SpeedCheck,
		"status":         intervals.Status,
		"function":       intervals.FunctionCheck,
	}
	for name, got := range wants {
		if got <= 0 {
			t.Errorf("%s interval is not positive: %s", name, got)
		}
	}
	if intervals.Process != 5*time.Second || intervals.Poll != 5*time.Second || intervals.ACEHeartbeat != 25*time.Second || intervals.UserHeartbeat != 25*time.Second || intervals.SpeedCheck != 30*time.Second || intervals.Status != 150*time.Second || intervals.FunctionCheck != 180*time.Second {
		t.Fatalf("production ACE cadence changed: %#v", intervals)
	}
	if intervals.BackoffMin != 2*time.Second || intervals.BackoffMax != 30*time.Second {
		t.Fatalf("production backoff changed: %#v", intervals)
	}
}

func TestServiceRunsAllLifecyclesAndFeedsReply(t *testing.T) {
	runtime := newMockRuntime()
	reply, err := proto.Marshal(&pb.AntiDataReply{Data: []byte("server-data")})
	if err != nil {
		t.Fatal(err)
	}
	sender := &mockSender{reply: reply}
	userHeartbeats := 0
	service, err := New(Options{
		Runtime: runtime,
		Sender:  sender,
		UserHeartbeat: func(context.Context) error {
			userHeartbeats++
			return nil
		},
		Intervals: testIntervals(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	time.Sleep(75 * time.Millisecond)
	service.Stop()

	process, heartbeat, speed, statusN, function, getData, sentData := runtime.counts()
	if process == 0 || heartbeat == 0 || speed == 0 || statusN == 0 || function == 0 || getData == 0 || sentData == 0 {
		t.Fatalf("not all ACE lifecycle actions ran: process=%d heartbeat=%d speed=%d status=%d function=%d get=%d sent=%d", process, heartbeat, speed, statusN, function, getData, sentData)
	}
	if userHeartbeats == 0 {
		t.Fatal("ordinary user heartbeat did not run independently")
	}
	status := service.Status()
	if status.Running || status.InFlight || status.UploadCount == 0 {
		t.Fatalf("service did not stop cleanly or upload: %+v", status)
	}
	if status.UserHeartbeatTicks == 0 {
		t.Fatalf("user heartbeat counter was not updated: %+v", status)
	}
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	if string(runtime.sentData[0]) != "server-data" {
		t.Fatalf("reply was not fed back unchanged: %q", runtime.sentData[0])
	}
	var request pb.AntiDataRequest
	if err := proto.Unmarshal(sender.requests[0], &request); err != nil {
		t.Fatal(err)
	}
	if string(request.Data) != "anti-data" {
		t.Fatalf("request payload changed: %q", request.Data)
	}
}

func TestServiceDeduplicatesInFlightAntiData(t *testing.T) {
	runtime := newMockRuntime()
	reply, err := proto.Marshal(&pb.AntiDataReply{Data: []byte("reply")})
	if err != nil {
		t.Fatal(err)
	}
	sender := &mockSender{reply: reply, started: make(chan struct{}), release: make(chan struct{})}
	intervals := testIntervals()
	intervals.Process = time.Hour
	intervals.ACEHeartbeat = time.Hour
	intervals.UserHeartbeat = time.Hour
	intervals.SpeedCheck = time.Hour
	intervals.Status = time.Hour
	intervals.FunctionCheck = time.Hour
	service, err := New(Options{Runtime: runtime, Sender: sender, Intervals: intervals})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	select {
	case <-sender.started:
	case <-time.After(200 * time.Millisecond):
		service.Stop()
		t.Fatal("AntiData request did not start")
	}
	time.Sleep(35 * time.Millisecond)
	if got := sender.countNow(); got != 1 {
		close(sender.release)
		service.Stop()
		t.Fatalf("in-flight AntiData was duplicated: %d requests", got)
	}
	close(sender.release)
	deadline := time.Now().Add(200 * time.Millisecond)
	for sender.countNow() == 1 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	service.Stop()
}

func TestServiceCancellationDestroysLifecycle(t *testing.T) {
	runtime := newMockRuntime()
	sender := &mockSender{reply: nil}
	intervals := testIntervals()
	intervals.Process = time.Hour
	intervals.Poll = time.Hour
	intervals.ACEHeartbeat = time.Hour
	intervals.UserHeartbeat = time.Hour
	intervals.SpeedCheck = time.Hour
	intervals.Status = time.Hour
	intervals.FunctionCheck = time.Hour
	service, err := New(Options{Runtime: runtime, Sender: sender, Intervals: intervals})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	if err := service.Start(ctx); err != nil {
		t.Fatal(err)
	}
	cancel()
	service.Stop()
	if status := service.Status(); status.Running || status.InFlight {
		t.Fatalf("context cancellation left service active: %+v", status)
	}
	service.Stop()
}

func TestBackoffDelayIsBounded(t *testing.T) {
	for failures, want := range map[uint32]time.Duration{0: 2 * time.Second, 1: 2 * time.Second, 2: 4 * time.Second, 3: 8 * time.Second, 4: 16 * time.Second, 5: 30 * time.Second, 20: 30 * time.Second} {
		if got := backoffDelay(failures, 2*time.Second, 30*time.Second); got != want {
			t.Errorf("backoffDelay(%d) = %s, want %s", failures, got, want)
		}
	}
}

func TestNewRejectsMissingDependenciesAndInvalidIntervals(t *testing.T) {
	if _, err := New(Options{}); err == nil {
		t.Fatal("missing runtime was accepted")
	}
	runtime := newMockRuntime()
	if _, err := New(Options{Runtime: runtime}); err == nil {
		t.Fatal("missing sender was accepted")
	}
	if _, err := New(Options{Runtime: runtime, Sender: SendFunc(func(context.Context, string, string, []byte, time.Duration) ([]byte, error) { return nil, nil }), Intervals: Intervals{}}); err != nil {
		t.Fatalf("zero intervals should select defaults: %v", err)
	}
	bad := testIntervals()
	bad.BackoffMax = bad.BackoffMin / 2
	if _, err := New(Options{Runtime: runtime, Sender: SendFunc(func(context.Context, string, string, []byte, time.Duration) ([]byte, error) { return nil, nil }), Intervals: bad}); err == nil {
		t.Fatal("invalid backoff range was accepted")
	}
}
