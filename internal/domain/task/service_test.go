package task

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/RoseKhlifa/FarmBot/internal/domain/warehouse"
	"github.com/RoseKhlifa/FarmBot/internal/game/pb"
	"github.com/RoseKhlifa/FarmBot/internal/game/transport"
	"google.golang.org/protobuf/proto"
)

type taskCall struct {
	command transport.Command
	request proto.Message
}

type taskTransportFake struct {
	handle func(transport.Command, proto.Message) (proto.Message, error)
	calls  []taskCall
	mu     sync.Mutex
}

func (f *taskTransportFake) SendMsg(_ context.Context, command transport.Command, request proto.Message) (proto.Message, error) {
	f.mu.Lock()
	f.calls = append(f.calls, taskCall{command: command, request: proto.Clone(request)})
	f.mu.Unlock()
	if f.handle == nil {
		return command.Response, nil
	}
	return f.handle(command, request)
}

func (f *taskTransportFake) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

func (f *taskTransportFake) callAt(index int) taskCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls[index]
}

type taskSchedulerFake struct {
	name     string
	interval time.Duration
	jitter   time.Duration
	callback any
}

func (f *taskSchedulerFake) Every(name string, interval, jitter time.Duration, callback any) error {
	f.name, f.interval, f.jitter, f.callback = name, interval, jitter, callback
	return nil
}

func TestClaimTaskRewardMapsRPCAndRewards(t *testing.T) {
	fake := &taskTransportFake{
		handle: func(command transport.Command, request proto.Message) (proto.Message, error) {
			if command.ServiceName != taskService || command.MethodName != "ClaimTaskReward" {
				t.Fatalf("unexpected command: %+v", command)
			}
			got := request.(*pb.ClaimTaskRewardRequest)
			if got.GetId() != 42 || !got.GetDoShared() {
				t.Fatalf("request = %v", got)
			}
			return &pb.ClaimTaskRewardReply{
				Items:            []*pb.Item{{Id: 1001, Count: 20}},
				CompensatedItems: []*pb.Item{{Id: 200, Count: 1}},
			}, nil
		},
	}
	service, err := New(Config{Transport: fake})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	result, err := service.ClaimTaskReward(context.Background(), 42, true)
	if err != nil {
		t.Fatalf("ClaimTaskReward() error = %v", err)
	}
	if !reflect.DeepEqual(result.Items, []warehouse.Item{{ID: 1001, Count: 20}}) {
		t.Fatalf("items = %#v", result.Items)
	}
	if len(result.CompensatedItems) != 1 || result.CompensatedItems[0].ID != 200 {
		t.Fatalf("compensated items = %#v", result.CompensatedItems)
	}
}

func TestClaimableTasksUsesDailyGrowthFallbackAndSkipsDuplicates(t *testing.T) {
	info := &pb.TaskInfo{Tasks: []*pb.Task{
		{Id: 1, TaskType: 2, Progress: 1, TotalProgress: 1, IsUnlocked: true},
		{Id: 2, TaskType: 1, Progress: 2, TotalProgress: 1, IsUnlocked: true},
		{Id: 3, Progress: 1, TotalProgress: 2, IsUnlocked: true},
	}}
	got := ClaimableTasks(info)
	if len(got) != 2 {
		t.Fatalf("claimable count = %d, want 2: %#v", len(got), got)
	}
	if got[0].Category != CategoryDaily || got[1].Category != CategoryGrowth {
		t.Fatalf("categories = %q, %q", got[0].Category, got[1].Category)
	}
}

func TestCheckAndClaimTasksClaimsTasksAndActiveRewards(t *testing.T) {
	fake := &taskTransportFake{
		handle: func(command transport.Command, request proto.Message) (proto.Message, error) {
			switch command.MethodName {
			case "TaskInfo":
				return &pb.TaskInfoReply{TaskInfo: &pb.TaskInfo{
					DailyTasks: []*pb.Task{{Id: 7, Progress: 3, TotalProgress: 3, IsUnlocked: true, ShareMultiple: 2}},
					Actives:    []*pb.Active{{Type: int32(pb.ActiveType_DAILYACTIVE), Rewards: []*pb.ActiveReward{{PointId: 9, Status: int32(pb.ActiveStatus_DONE)}}}},
				}}, nil
			case "ClaimTaskReward":
				return &pb.ClaimTaskRewardReply{Items: []*pb.Item{{Id: 1001, Count: 1}}}, nil
			case "ClaimDailyReward":
				return &pb.ClaimDailyRewardReply{Items: []*pb.Item{{Id: 200, Count: 2}}}, nil
			default:
				t.Fatalf("unexpected method %q", command.MethodName)
				return nil, nil
			}
		},
	}
	service, err := New(Config{Transport: fake})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	result, err := service.CheckAndClaimTasks(context.Background())
	if err != nil {
		t.Fatalf("CheckAndClaimTasks() error = %v", err)
	}
	if result.Scanned != 1 || result.Claimed != 1 || result.ActiveScanned != 1 || result.ActiveClaimed != 1 {
		t.Fatalf("result = %+v", result)
	}
	if len(fake.calls) != 3 {
		t.Fatalf("RPC calls = %d, want 3", len(fake.calls))
	}
}

func TestHandleEventClaimsTaskInfoNotify(t *testing.T) {
	events := make(chan transport.Event, 1)
	fake := &taskTransportFake{
		handle: func(command transport.Command, _ proto.Message) (proto.Message, error) {
			if command.MethodName != "ClaimTaskReward" {
				return nil, errors.New("unexpected scan RPC")
			}
			return &pb.ClaimTaskRewardReply{}, nil
		},
	}
	service, err := New(Config{Transport: fake, Events: events, NotifyDelay: 0})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if err := service.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	events <- transport.Event{Type: transport.EventTaskInfoNotify, Payload: &pb.TaskInfoNotify{TaskInfo: &pb.TaskInfo{
		DailyTasks: []*pb.Task{{Id: 8, Progress: 1, TotalProgress: 1, IsUnlocked: true}},
	}}}
	deadline := time.After(time.Second)
	for fake.callCount() == 0 {
		select {
		case <-deadline:
			t.Fatal("task notification did not trigger claim")
		default:
			time.Sleep(time.Millisecond)
		}
	}
	if fake.callAt(0).command.MethodName != "ClaimTaskReward" {
		t.Fatalf("first RPC = %+v", fake.callAt(0).command)
	}
	if err := service.Stop(); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
}

func TestRegisterSchedulerUsesP402Contract(t *testing.T) {
	fakeTransport := &taskTransportFake{}
	scheduler := new(taskSchedulerFake)
	service, err := New(Config{Transport: fakeTransport, TaskInterval: 3 * time.Second, TaskJitter: time.Second})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if err := service.RegisterScheduler(scheduler); err != nil {
		t.Fatalf("RegisterScheduler() error = %v", err)
	}
	if scheduler.name != TaskScheduleName || scheduler.interval != 3*time.Second || scheduler.jitter != time.Second || scheduler.callback == nil {
		t.Fatalf("scheduler registration = %+v", scheduler)
	}
}
