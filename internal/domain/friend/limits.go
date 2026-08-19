package friend

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/RoseKhlifa/FarmBot/internal/game/pb"
	"github.com/RoseKhlifa/FarmBot/internal/game/transport"
)

type LimitEventType string

const (
	EventExpLimitReached LimitEventType = "exp_limit_reached"
	EventExpLimitReset   LimitEventType = "exp_limit_reset"
)

type LimitEvent struct {
	Type        LimitEventType
	OperationID int64
	At          time.Time
}
type OperationLimitState struct{ ID, DayTimes, DayTimesLimit, DayExpTimes, DayExpTimesLimit int64 }
type OperationLimitSnapshot struct {
	Date             string
	Limits           map[int64]OperationLimitState
	BadCount         int64
	HelpExpAvailable bool
}
type LimitsConfig struct {
	Transport   GameTransport
	HostGID     int64
	Now         func() time.Time
	EventBuffer int
}
type Limits struct {
	mu               sync.Mutex
	transport        GameTransport
	hostGID          int64
	now              func() time.Time
	date             string
	limits           map[int64]OperationLimitState
	badCount         int64
	helpExpAvailable bool
	events           chan LimitEvent
}

func NewLimits(cfg LimitsConfig) *Limits {
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.EventBuffer <= 0 {
		cfg.EventBuffer = 16
	}
	now := cfg.Now()
	return &Limits{transport: cfg.Transport, hostGID: cfg.HostGID, now: cfg.Now, date: now.Format("2006-01-02"), limits: make(map[int64]OperationLimitState), helpExpAvailable: true, events: make(chan LimitEvent, cfg.EventBuffer)}
}
func NewOperationLimits(cfg LimitsConfig) *Limits { return NewLimits(cfg) }
func (l *Limits) Events() <-chan LimitEvent {
	if l == nil {
		return nil
	}
	return l.events
}

// OnExpLimit exposes the explicit event stream used by orchestrators. It
// replaces the legacy hidden callback registration.
func (l *Limits) OnExpLimit() <-chan LimitEvent { return l.Events() }
func (l *Limits) Snapshot() OperationLimitSnapshot {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.checkResetLocked(l.now())
	out := make(map[int64]OperationLimitState, len(l.limits))
	for id, v := range l.limits {
		out[id] = v
	}
	return OperationLimitSnapshot{Date: l.date, Limits: out, BadCount: l.badCount, HelpExpAvailable: l.helpExpAvailable}
}
func (l *Limits) Update(values []*pb.OperationLimit) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.checkResetLocked(l.now())
	for _, v := range values {
		if v == nil || v.GetId() <= 0 {
			continue
		}
		l.limits[v.GetId()] = OperationLimitState{ID: v.GetId(), DayTimes: v.GetDayTimes(), DayTimesLimit: v.GetDayTimesLt(), DayExpTimes: v.GetDayExpTimes(), DayExpTimesLimit: v.GetDayExTimesLt()}
		if v.GetDayExTimesLt() > 0 && v.GetDayExpTimes() >= v.GetDayExTimesLt() {
			l.helpExpAvailable = false
			l.emitLocked(LimitEvent{Type: EventExpLimitReached, OperationID: v.GetId(), At: l.now()})
		}
	}
}
func (l *Limits) Reset(now ...time.Time) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	clock := l.now()
	if len(now) > 0 {
		clock = now[0]
	}
	l.date = clock.Format("2006-01-02")
	l.limits = make(map[int64]OperationLimitState)
	l.badCount = 0
	l.helpExpAvailable = true
	l.emitLocked(LimitEvent{Type: EventExpLimitReset, At: clock})
}
func (l *Limits) CheckDailyReset(now ...time.Time) bool {
	if l == nil {
		return false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	clock := l.now()
	if len(now) > 0 {
		clock = now[0]
	}
	return l.checkResetLocked(clock)
}
func (l *Limits) checkResetLocked(clock time.Time) bool {
	date := clock.Format("2006-01-02")
	if date == l.date {
		return false
	}
	l.date = date
	l.limits = make(map[int64]OperationLimitState)
	l.badCount = 0
	l.helpExpAvailable = true
	l.emitLocked(LimitEvent{Type: EventExpLimitReset, At: clock})
	return true
}
func (l *Limits) emitLocked(event LimitEvent) {
	select {
	case l.events <- event:
	default:
	}
}
func (l *Limits) CanGetExp(id int64) bool {
	if l == nil {
		return true
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.checkResetLocked(l.now())
	v, ok := l.limits[id]
	if !ok || v.DayExpTimesLimit <= 0 {
		return true
	}
	return v.DayExpTimes < v.DayExpTimesLimit
}
func (l *Limits) CanGetExpByCandidates(ids []int64) bool {
	for _, id := range ids {
		if l.CanGetExp(id) {
			return true
		}
	}
	return false
}
func (l *Limits) CanOperate(id int64, fallback ...bool) bool {
	if l == nil {
		return true
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.checkResetLocked(l.now())
	v, ok := l.limits[id]
	if !ok || v.DayTimesLimit <= 0 {
		if len(fallback) > 0 {
			return fallback[0]
		}
		return true
	}
	return v.DayTimes < v.DayTimesLimit
}
func (l *Limits) Remaining(id int64, fallback ...int64) int64 {
	if l == nil {
		return firstInt(fallback, 999)
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.checkResetLocked(l.now())
	v, ok := l.limits[id]
	if !ok || v.DayTimesLimit <= 0 {
		return firstInt(fallback, 999)
	}
	left := v.DayTimesLimit - v.DayTimes
	if left < 0 {
		return 0
	}
	return left
}
func (l *Limits) GetRemainingTimes(id int64, fallback ...int64) int64 {
	return l.Remaining(id, fallback...)
}
func (l *Limits) BadRemaining() int64 {
	if l == nil {
		return 100
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.checkResetLocked(l.now())
	server := int64(0)
	capLimit := int64(100)
	if v, ok := l.limits[OperationBadDaily]; ok {
		if v.DayTimes > server {
			server = v.DayTimes
		}
		if v.DayTimesLimit > 0 && v.DayTimesLimit < capLimit {
			capLimit = v.DayTimesLimit
		}
	}
	for _, id := range []int64{OperationPutBug, OperationPutWeed} {
		if v, ok := l.limits[id]; ok && v.DayTimes > server {
			server = v.DayTimes
		}
		if v, ok := l.limits[id]; ok && v.DayTimesLimit > 0 && v.DayTimesLimit < capLimit {
			capLimit = v.DayTimesLimit
		}
	}
	used := server
	if l.badCount > used {
		used = l.badCount
	}
	if used >= capLimit {
		return 0
	}
	return capLimit - used
}
func (l *Limits) RecordBadSuccess(count int64) {
	if l == nil {
		return
	}
	if count < 0 {
		count = 0
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.checkResetLocked(l.now())
	l.badCount += count
	if l.badCount > 100 {
		l.badCount = 100
	}
}

type HelpResult struct {
	Requested  []int64
	Confirmed  []int64
	Noop       bool
	ExpLimited bool
	Raw        *pb.FarmingReply
}

func (l *Limits) HelpFarming(ctx context.Context, landIDs []int64, hostGID ...int64) (HelpResult, error) {
	ids := NormalizeGIDs(landIDs)
	result := HelpResult{Requested: ids}
	if len(ids) == 0 {
		return result, nil
	}
	if l == nil || l.transport == nil {
		return result, ErrTransportRequired
	}
	gid := l.hostGID
	if len(hostGID) > 0 {
		gid = hostGID[0]
	}
	reply := new(pb.FarmingReply)
	response, err := l.transport.SendMsg(ctx, transport.Command{ServiceName: PlantServiceName, MethodName: "Farming", Response: reply}, &pb.FarmingRequest{LandIds: ids, HostGid: gid, Field_3: 0, Field_4: 2})
	if err != nil {
		if IsErrorCode(err, 1001057) {
			result.Noop = true
			return result, nil
		}
		return result, err
	}
	if response != nil {
		var ok bool
		reply, ok = response.(*pb.FarmingReply)
		if !ok {
			return result, fmt.Errorf("farming returned %T", response)
		}
	}
	result.Raw = reply
	for _, item := range reply.GetResults() {
		if item != nil && item.GetLandId() > 0 {
			result.Confirmed = append(result.Confirmed, item.GetLandId())
		}
	}
	l.Update(reply.GetOperationLimits())
	for _, id := range ids {
		if !l.CanGetExp(id) {
			result.ExpLimited = true
			break
		}
	}
	return result, nil
}
func (l *Limits) HelpWater(ctx context.Context, ids []int64, host ...int64) (HelpResult, error) {
	return l.HelpFarming(ctx, ids, host...)
}
func (l *Limits) HelpWeed(ctx context.Context, ids []int64, host ...int64) (HelpResult, error) {
	return l.HelpFarming(ctx, ids, host...)
}
func (l *Limits) HelpInsecticide(ctx context.Context, ids []int64, host ...int64) (HelpResult, error) {
	return l.HelpFarming(ctx, ids, host...)
}
func (l *Limits) StealHarvest(ctx context.Context, ids []int64, host ...int64) (*pb.HarvestReply, error) {
	if l == nil || l.transport == nil {
		return nil, ErrTransportRequired
	}
	clean := NormalizeGIDs(ids)
	if len(clean) == 0 {
		return &pb.HarvestReply{}, nil
	}
	gid := l.hostGID
	if len(host) > 0 {
		gid = host[0]
	}
	reply := new(pb.HarvestReply)
	response, err := l.transport.SendMsg(ctx, transport.Command{ServiceName: PlantServiceName, MethodName: "Harvest", Response: reply}, &pb.HarvestRequest{LandIds: clean, HostGid: gid, IsAll: true})
	if err != nil {
		return nil, err
	}
	if response != nil {
		var ok bool
		reply, ok = response.(*pb.HarvestReply)
		if !ok {
			return nil, fmt.Errorf("harvest returned %T", response)
		}
	}
	l.Update(reply.GetOperationLimits())
	return reply, nil
}
func (l *Limits) PutInsects(ctx context.Context, ids []int64, host ...int64) (*pb.PutInsectsReply, error) {
	return l.putBad(ctx, "PutInsects", ids, host...)
}
func (l *Limits) PutWeeds(ctx context.Context, ids []int64, host ...int64) (*pb.PutWeedsReply, error) {
	if l == nil || l.transport == nil {
		return nil, ErrTransportRequired
	}
	clean := NormalizeGIDs(ids)
	if len(clean) == 0 {
		return &pb.PutWeedsReply{}, nil
	}
	gid := l.hostGID
	if len(host) > 0 {
		gid = host[0]
	}
	reply := new(pb.PutWeedsReply)
	response, err := l.transport.SendMsg(ctx, transport.Command{ServiceName: PlantServiceName, MethodName: "PutWeeds", Response: reply}, &pb.PutWeedsRequest{LandIds: clean, HostGid: gid})
	if err != nil {
		return nil, err
	}
	if response != nil {
		var ok bool
		reply, ok = response.(*pb.PutWeedsReply)
		if !ok {
			return nil, fmt.Errorf("put weeds returned %T", response)
		}
	}
	l.Update(reply.GetOperationLimits())
	l.RecordBadSuccess(int64(len(clean)))
	return reply, nil
}
func (l *Limits) putBad(ctx context.Context, method string, ids []int64, host ...int64) (*pb.PutInsectsReply, error) {
	if l == nil || l.transport == nil {
		return nil, ErrTransportRequired
	}
	clean := NormalizeGIDs(ids)
	if len(clean) == 0 {
		return &pb.PutInsectsReply{}, nil
	}
	gid := l.hostGID
	if len(host) > 0 {
		gid = host[0]
	}
	reply := new(pb.PutInsectsReply)
	response, err := l.transport.SendMsg(ctx, transport.Command{ServiceName: PlantServiceName, MethodName: method, Response: reply}, &pb.PutInsectsRequest{LandIds: clean, HostGid: gid})
	if err != nil {
		return nil, err
	}
	if response != nil {
		var ok bool
		reply, ok = response.(*pb.PutInsectsReply)
		if !ok {
			return nil, fmt.Errorf("put insects returned %T", response)
		}
	}
	l.Update(reply.GetOperationLimits())
	l.RecordBadSuccess(int64(len(clean)))
	return reply, nil
}
func firstInt(values []int64, fallback int64) int64 {
	if len(values) > 0 {
		return values[0]
	}
	return fallback
}
