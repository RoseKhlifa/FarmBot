// Package task implements account-local task and activity reward automation.
package task

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/RoseKhlifa/FarmBot/internal/domain/warehouse"
	"github.com/RoseKhlifa/FarmBot/internal/game/pb"
	"github.com/RoseKhlifa/FarmBot/internal/game/transport"
	"google.golang.org/protobuf/proto"
)

const (
	taskService         = "gamepb.taskpb.TaskService"
	TaskScheduleName    = "task"
	defaultTaskInterval = time.Minute
)

var (
	ErrTransportRequired = errors.New("task transport is required")
	ErrTaskIDRequired    = errors.New("task id must be positive")
	ErrPointIDsRequired  = errors.New("task point ids must not be empty")
	ErrSchedulerRequired = errors.New("task scheduler is required")
	ErrServiceStarted    = errors.New("task service is already started")
)

// GameTransport aliases the warehouse domain's shared authenticated RPC
// surface. session.Session and transport.Client satisfy it, while tests can
// use a fake without opening a WebSocket.
type GameTransport = warehouse.GameTransport

// Scheduler is the part of account's P4-02 scheduler needed by task. The
// account scheduler accepts this interface without making task own lifecycle.
type Scheduler interface {
	Every(name string, interval, jitter time.Duration, fn any) error
}

// Config contains account-local task collaborators. Events is optional when
// the service is wired through account.LoopHooks.OnTaskInfo instead.
type Config struct {
	Transport     GameTransport
	Warehouse     warehouse.Service
	Events        <-chan transport.Event
	Scheduler     Scheduler
	TaskInterval  time.Duration
	TaskJitter    time.Duration
	NotifyDelay   time.Duration
	AutomationOn  func(context.Context, string) (bool, error)
	OnRewardClaim func(RewardResult)
	OnError       func(error)
}

// Reward is a protocol-independent reward item.
type Reward struct {
	ID    int64
	Count int64
	UID   int64
}

// Category identifies the source list of a claimable task.
type Category string

const (
	CategoryMain   Category = "main"
	CategoryDaily  Category = "daily"
	CategoryGrowth Category = "growth"
)

// Task is the stable domain representation used by automation decisions.
type Task struct {
	ID            int64
	Description   string
	Category      Category
	Progress      int64
	TotalProgress int64
	Claimed       bool
	Unlocked      bool
	ShareMultiple int64
	Rewards       []Reward
}

// Claimable reports whether a task can be claimed now.
func (t Task) Claimable() bool {
	return t.ID > 0 && t.Unlocked && !t.Claimed && t.TotalProgress > 0 && t.Progress >= t.TotalProgress
}

// RewardResult is shared by task, batch-task and active-reward RPCs.
type RewardResult struct {
	Items            []warehouse.Item
	CompensatedItems []warehouse.Item
	TaskInfo         *pb.TaskInfo
	Raw              proto.Message
}

// ClaimResult is a descriptive alias retained for callers migrating from the
// Node task service.
type ClaimResult = RewardResult

// RunResult summarizes one scan-and-claim pass.
type RunResult struct {
	Scanned       int
	Claimed       int
	ActiveScanned int
	ActiveClaimed int
	Failed        int
	Errors        []error
}

// Service is the task domain contract consumed by account wiring and handlers.
type Service interface {
	GetTaskInfo(context.Context) (*pb.TaskInfo, error)
	FetchTaskInfo(context.Context) (*pb.TaskInfo, error)
	ClaimTaskReward(context.Context, int64, bool) (RewardResult, error)
	ClaimTask(context.Context, Task) (RewardResult, error)
	BatchClaimTaskRewards(context.Context, []int64, bool) (RewardResult, error)
	BatchClaimTaskReward(context.Context, []int64, bool) (RewardResult, error)
	ClaimDailyReward(context.Context, int32, []int64) (RewardResult, error)
	CheckAndClaimTasks(context.Context) (RunResult, error)
	HandleEvent(context.Context, transport.Event) error
	OnTaskInfoNotify(context.Context, *pb.TaskInfo) error
	RegisterScheduler(Scheduler) error
	Schedule(Scheduler) error
	Start(context.Context) error
	Stop() error
	Close() error
}

type service struct {
	transport    GameTransport
	warehouse    warehouse.Service
	events       <-chan transport.Event
	scheduler    Scheduler
	interval     time.Duration
	jitter       time.Duration
	notifyDelay  time.Duration
	automationOn func(context.Context, string) (bool, error)
	onReward     func(RewardResult)
	onError      func(error)

	mu          sync.Mutex
	ctx         context.Context
	cancel      context.CancelFunc
	started     bool
	checking    bool
	eventsWG    sync.WaitGroup
	notifyTimer *time.Timer
}

var _ Service = (*service)(nil)

// New creates an account-local task service.
func New(cfg Config) (Service, error) {
	if cfg.Transport == nil {
		return nil, ErrTransportRequired
	}
	interval := cfg.TaskInterval
	if interval <= 0 {
		interval = defaultTaskInterval
	}
	jitter := cfg.TaskJitter
	if jitter < 0 {
		jitter = -jitter
	}
	delay := cfg.NotifyDelay
	if delay < 0 {
		delay = 0
	}
	return &service{
		transport:    cfg.Transport,
		warehouse:    cfg.Warehouse,
		events:       cfg.Events,
		scheduler:    cfg.Scheduler,
		interval:     interval,
		jitter:       jitter,
		notifyDelay:  delay,
		automationOn: cfg.AutomationOn,
		onReward:     cfg.OnRewardClaim,
		onError:      cfg.OnError,
	}, nil
}

// NewService is a descriptive constructor alias.
func NewService(cfg Config) (Service, error) { return New(cfg) }

// GetTaskInfo requests the current task and activity state.
func (s *service) GetTaskInfo(ctx context.Context) (*pb.TaskInfo, error) {
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	reply := new(pb.TaskInfoReply)
	response, err := s.transport.SendMsg(ctx, transport.Command{
		ServiceName: taskService,
		MethodName:  "TaskInfo",
		Response:    reply,
	}, &pb.TaskInfoRequest{})
	if err != nil {
		return nil, fmt.Errorf("get task info: %w", err)
	}
	if response != nil {
		var ok bool
		reply, ok = response.(*pb.TaskInfoReply)
		if !ok {
			return nil, fmt.Errorf("task info returned %T, want *pb.TaskInfoReply", response)
		}
	}
	if reply.GetTaskInfo() == nil {
		return nil, nil
	}
	return proto.Clone(reply.GetTaskInfo()).(*pb.TaskInfo), nil
}

// FetchTaskInfo is an explicit alias for callers that prefer fetch wording.
func (s *service) FetchTaskInfo(ctx context.Context) (*pb.TaskInfo, error) {
	return s.GetTaskInfo(ctx)
}

// ClaimTaskReward claims one completed task.
func (s *service) ClaimTaskReward(ctx context.Context, id int64, doShared bool) (RewardResult, error) {
	if err := checkContext(ctx); err != nil {
		return RewardResult{}, err
	}
	if id <= 0 {
		return RewardResult{}, ErrTaskIDRequired
	}
	reply := new(pb.ClaimTaskRewardReply)
	response, err := s.transport.SendMsg(ctx, transport.Command{
		ServiceName: taskService,
		MethodName:  "ClaimTaskReward",
		Response:    reply,
	}, &pb.ClaimTaskRewardRequest{Id: id, DoShared: doShared})
	if err != nil {
		return RewardResult{}, fmt.Errorf("claim task %d: %w", id, err)
	}
	if response != nil {
		var ok bool
		reply, ok = response.(*pb.ClaimTaskRewardReply)
		if !ok {
			return RewardResult{}, fmt.Errorf("claim task returned %T, want *pb.ClaimTaskRewardReply", response)
		}
	}
	result := rewardResult(reply.GetItems(), reply.GetCompensatedItems(), reply.GetTaskInfo(), reply)
	s.emitReward(result)
	return result, nil
}

// ClaimTask is a convenience wrapper for a domain task value.
func (s *service) ClaimTask(ctx context.Context, item Task) (RewardResult, error) {
	return s.ClaimTaskReward(ctx, item.ID, item.ShareMultiple > 1)
}

// BatchClaimTaskRewards claims several task IDs in one RPC.
func (s *service) BatchClaimTaskRewards(ctx context.Context, ids []int64, doShared bool) (RewardResult, error) {
	if err := checkContext(ctx); err != nil {
		return RewardResult{}, err
	}
	if len(ids) == 0 {
		return RewardResult{}, ErrTaskIDRequired
	}
	clean := append([]int64(nil), ids...)
	for _, id := range clean {
		if id <= 0 {
			return RewardResult{}, ErrTaskIDRequired
		}
	}
	reply := new(pb.BatchClaimTaskRewardReply)
	response, err := s.transport.SendMsg(ctx, transport.Command{
		ServiceName: taskService,
		MethodName:  "BatchClaimTaskReward",
		Response:    reply,
	}, &pb.BatchClaimTaskRewardRequest{Ids: clean, DoShared: doShared})
	if err != nil {
		return RewardResult{}, fmt.Errorf("claim %d tasks: %w", len(clean), err)
	}
	if response != nil {
		var ok bool
		reply, ok = response.(*pb.BatchClaimTaskRewardReply)
		if !ok {
			return RewardResult{}, fmt.Errorf("batch claim returned %T, want *pb.BatchClaimTaskRewardReply", response)
		}
	}
	result := rewardResult(reply.GetItems(), reply.GetCompensatedItems(), reply.GetTaskInfo(), reply)
	s.emitReward(result)
	return result, nil
}

// BatchClaimTaskReward is a singular compatibility alias.
func (s *service) BatchClaimTaskReward(ctx context.Context, ids []int64, doShared bool) (RewardResult, error) {
	return s.BatchClaimTaskRewards(ctx, ids, doShared)
}

// ClaimDailyReward claims completed daily or weekly active reward tiers.
func (s *service) ClaimDailyReward(ctx context.Context, activeType int32, pointIDs []int64) (RewardResult, error) {
	if err := checkContext(ctx); err != nil {
		return RewardResult{}, err
	}
	if len(pointIDs) == 0 {
		return RewardResult{}, ErrPointIDsRequired
	}
	clean := append([]int64(nil), pointIDs...)
	for _, id := range clean {
		if id <= 0 {
			return RewardResult{}, ErrPointIDsRequired
		}
	}
	reply := new(pb.ClaimDailyRewardReply)
	response, err := s.transport.SendMsg(ctx, transport.Command{
		ServiceName: taskService,
		MethodName:  "ClaimDailyReward",
		Response:    reply,
	}, &pb.ClaimDailyRewardRequest{Type: activeType, PointIds: clean})
	if err != nil {
		return RewardResult{}, fmt.Errorf("claim active rewards: %w", err)
	}
	if response != nil {
		var ok bool
		reply, ok = response.(*pb.ClaimDailyRewardReply)
		if !ok {
			return RewardResult{}, fmt.Errorf("claim active reward returned %T, want *pb.ClaimDailyRewardReply", response)
		}
	}
	result := rewardResult(reply.GetItems(), reply.GetCompensatedItems(), reply.GetTaskInfo(), reply)
	s.emitReward(result)
	return result, nil
}

// ClaimableTasks returns daily, growth and main tasks that are ready to claim.
// Explicit daily/growth lists take precedence; when absent, task_type 2/1
// fallback matches the Node implementation's compatibility behavior.
func ClaimableTasks(info *pb.TaskInfo) []Task {
	if info == nil {
		return nil
	}
	result := make([]Task, 0)
	seen := make(map[int64]struct{})
	appendTasks := func(tasks []*pb.Task, category Category) {
		for _, raw := range tasks {
			item := taskFromProto(raw, category)
			if !item.Claimable() {
				continue
			}
			if _, ok := seen[item.ID]; ok {
				continue
			}
			seen[item.ID] = struct{}{}
			result = append(result, item)
		}
	}

	daily := info.GetDailyTasks()
	if len(daily) == 0 {
		for _, raw := range info.GetTasks() {
			if raw != nil && raw.GetTaskType() == 2 {
				daily = append(daily, raw)
			}
		}
	}
	growth := info.GetGrowthTasks()
	if len(growth) == 0 {
		for _, raw := range info.GetTasks() {
			if raw != nil && raw.GetTaskType() == 1 {
				growth = append(growth, raw)
			}
		}
	}
	appendTasks(daily, CategoryDaily)
	appendTasks(growth, CategoryGrowth)
	appendTasks(info.GetTasks(), CategoryMain)
	return result
}

// AnalyzeTaskList adapts one protobuf task list to claimable domain tasks.
func AnalyzeTaskList(tasks []*pb.Task, category Category) []Task {
	result := make([]Task, 0, len(tasks))
	for _, raw := range tasks {
		item := taskFromProto(raw, category)
		if item.Claimable() {
			result = append(result, item)
		}
	}
	return result
}

// OnTaskInfoNotify handles a decoded taskInfoNotify payload. With a zero
// NotifyDelay it claims synchronously, which is convenient for direct hooks
// and deterministic tests; a positive delay debounces bursts of notifications.
func (s *service) OnTaskInfoNotify(ctx context.Context, info *pb.TaskInfo) error {
	if info == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	copyInfo, ok := proto.Clone(info).(*pb.TaskInfo)
	if !ok {
		return errors.New("clone task info failed")
	}

	s.mu.Lock()
	delay := s.notifyDelay
	serviceCtx := s.ctx
	if serviceCtx == nil {
		serviceCtx = ctx
	}
	if s.notifyTimer != nil {
		s.notifyTimer.Stop()
		s.notifyTimer = nil
	}
	s.mu.Unlock()

	if delay <= 0 {
		_, err := s.claimInfo(serviceCtx, copyInfo)
		return err
	}
	timer := time.AfterFunc(delay, func() {
		_, err := s.claimInfo(serviceCtx, copyInfo)
		if err != nil {
			s.reportError(err)
		}
	})
	s.mu.Lock()
	s.notifyTimer = timer
	s.mu.Unlock()
	return nil
}

// HandleEvent is compatible with account.LoopHooks.OnTaskInfo and also makes
// the transport event subscription directly testable.
func (s *service) HandleEvent(ctx context.Context, event transport.Event) error {
	if event.Type != transport.EventTaskInfoNotify {
		return nil
	}
	switch payload := event.Payload.(type) {
	case *pb.TaskInfoNotify:
		return s.OnTaskInfoNotify(ctx, payload.GetTaskInfo())
	case *pb.TaskInfo:
		return s.OnTaskInfoNotify(ctx, payload)
	default:
		return nil
	}
}

// CheckAndClaimTasks performs one fresh scan and claims all completed tasks
// and active reward tiers. Concurrent scans are coalesced into one pass.
func (s *service) CheckAndClaimTasks(ctx context.Context) (RunResult, error) {
	if err := checkContext(ctx); err != nil {
		return RunResult{}, err
	}
	enabled, err := s.automationEnabled(ctx)
	if err != nil {
		return RunResult{}, err
	}
	if !enabled {
		return RunResult{}, nil
	}
	if !s.beginCheck() {
		return RunResult{}, nil
	}
	defer s.endCheck()
	info, err := s.GetTaskInfo(ctx)
	if err != nil {
		return RunResult{}, err
	}
	return s.claimInfoActive(ctx, info)
}

func (s *service) claimInfo(ctx context.Context, info *pb.TaskInfo) (RunResult, error) {
	if err := checkContext(ctx); err != nil {
		return RunResult{}, err
	}
	enabled, err := s.automationEnabled(ctx)
	if err != nil || !enabled {
		return RunResult{}, err
	}
	if !s.beginCheck() {
		return RunResult{}, nil
	}
	defer s.endCheck()
	return s.claimInfoActive(ctx, info)
}

func (s *service) claimInfoActive(ctx context.Context, info *pb.TaskInfo) (RunResult, error) {
	if err := checkContext(ctx); err != nil {
		return RunResult{}, err
	}
	var result RunResult
	for _, item := range ClaimableTasks(info) {
		result.Scanned++
		if _, err := s.ClaimTask(ctx, item); err != nil {
			result.Failed++
			result.Errors = append(result.Errors, err)
			continue
		}
		result.Claimed++
	}

	activeIDs := make(map[int32][]int64)
	if info != nil {
		for _, active := range info.GetActives() {
			if active == nil {
				continue
			}
			for _, reward := range active.GetRewards() {
				if reward != nil && reward.GetStatus() == int32(pb.ActiveStatus_DONE) && reward.GetPointId() > 0 {
					result.ActiveScanned++
					activeIDs[active.GetType()] = append(activeIDs[active.GetType()], reward.GetPointId())
				}
			}
		}
	}
	for activeType, pointIDs := range activeIDs {
		if _, err := s.ClaimDailyReward(ctx, activeType, pointIDs); err != nil {
			result.Failed++
			result.Errors = append(result.Errors, err)
			continue
		}
		result.ActiveClaimed += len(pointIDs)
	}
	if len(result.Errors) > 0 {
		return result, errors.Join(result.Errors...)
	}
	return result, nil
}

// RegisterScheduler installs the periodic scan into the P4-02 account
// scheduler. Re-registering replaces the named task, matching Every's API.
func (s *service) RegisterScheduler(scheduler Scheduler) error {
	if scheduler == nil {
		return ErrSchedulerRequired
	}
	s.mu.Lock()
	s.scheduler = scheduler
	interval, jitter := s.interval, s.jitter
	s.mu.Unlock()
	if err := scheduler.Every(TaskScheduleName, interval, jitter, func(ctx context.Context) error {
		_, err := s.CheckAndClaimTasks(ctx)
		return err
	}); err != nil {
		return fmt.Errorf("register task scheduler: %w", err)
	}
	return nil
}

// Schedule is a compatibility alias for RegisterScheduler.
func (s *service) Schedule(scheduler Scheduler) error { return s.RegisterScheduler(scheduler) }

// Start starts the optional event bridge and registers the optional scheduler.
func (s *service) Start(parent context.Context) error {
	if parent == nil {
		parent = context.Background()
	}
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return ErrServiceStarted
	}
	ctx, cancel := context.WithCancel(parent)
	s.ctx, s.cancel, s.started = ctx, cancel, true
	scheduler := s.scheduler
	events := s.events
	s.mu.Unlock()

	if scheduler != nil {
		if err := s.RegisterScheduler(scheduler); err != nil {
			_ = s.Stop()
			return err
		}
	}
	if events != nil {
		s.eventsWG.Add(1)
		go s.eventLoop(ctx, events)
	}
	return nil
}

// Stop cancels the task event bridge and any pending notification debounce.
func (s *service) Stop() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	if !s.started {
		if s.notifyTimer != nil {
			s.notifyTimer.Stop()
			s.notifyTimer = nil
		}
		s.mu.Unlock()
		return nil
	}
	cancel := s.cancel
	s.cancel, s.ctx, s.started = nil, nil, false
	if s.notifyTimer != nil {
		s.notifyTimer.Stop()
		s.notifyTimer = nil
	}
	scheduler := s.scheduler
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	s.eventsWG.Wait()
	if stopper, ok := scheduler.(interface{ StopTask(string) bool }); ok {
		stopper.StopTask(TaskScheduleName)
	} else if stopper, ok := scheduler.(interface{ Stop(string) bool }); ok {
		stopper.Stop(TaskScheduleName)
	}
	return nil
}

// Close is an alias for Stop.
func (s *service) Close() error { return s.Stop() }

func (s *service) eventLoop(ctx context.Context, events <-chan transport.Event) {
	defer s.eventsWG.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-events:
			if !ok {
				return
			}
			if err := s.HandleEvent(ctx, event); err != nil {
				s.reportError(err)
			}
		}
	}
}

func (s *service) beginCheck() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.checking {
		return false
	}
	s.checking = true
	return true
}

func (s *service) endCheck() {
	s.mu.Lock()
	s.checking = false
	s.mu.Unlock()
}

func (s *service) automationEnabled(ctx context.Context) (bool, error) {
	if s.automationOn == nil {
		return true, nil
	}
	return s.automationOn(ctx, "task")
}

func (s *service) emitReward(result RewardResult) {
	if s.onReward != nil {
		s.onReward(result)
	}
}

func (s *service) reportError(err error) {
	if err != nil && s.onError != nil {
		s.onError(err)
	}
}

func rewardResult(items, compensated []*pb.Item, info *pb.TaskInfo, raw proto.Message) RewardResult {
	result := RewardResult{
		Items:            rewardsFromProto(items),
		CompensatedItems: rewardsFromProto(compensated),
		Raw:              raw,
	}
	if info != nil {
		result.TaskInfo = proto.Clone(info).(*pb.TaskInfo)
	}
	return result
}

func rewardsFromProto(items []*pb.Item) []warehouse.Item {
	result := make([]warehouse.Item, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		result = append(result, warehouse.Item{ID: item.GetId(), Count: item.GetCount(), UID: item.GetUid()})
	}
	return result
}

func taskFromProto(raw *pb.Task, category Category) Task {
	if raw == nil {
		return Task{}
	}
	description := strings.TrimSpace(raw.GetDesc())
	if description == "" {
		description = fmt.Sprintf("任务#%d", raw.GetId())
	}
	return Task{
		ID:            raw.GetId(),
		Description:   description,
		Category:      category,
		Progress:      raw.GetProgress(),
		TotalProgress: raw.GetTotalProgress(),
		Claimed:       raw.GetIsClaimed(),
		Unlocked:      raw.GetIsUnlocked(),
		ShareMultiple: raw.GetShareMultiple(),
		Rewards:       domainRewardsFromProto(raw.GetRewards()),
	}
}

func domainRewardsFromProto(items []*pb.Item) []Reward {
	result := make([]Reward, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		result = append(result, Reward{ID: item.GetId(), Count: item.GetCount(), UID: item.GetUid()})
	}
	return result
}

func checkContext(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	return ctx.Err()
}
