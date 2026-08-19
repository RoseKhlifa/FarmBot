package friend

import (
	"context"
	"fmt"
	"hash/fnv"
	"sync"
	"time"

	"github.com/RoseKhlifa/FarmBot/internal/domain/farm"
	"github.com/RoseKhlifa/FarmBot/internal/domain/warehouse"
	"github.com/RoseKhlifa/FarmBot/internal/game/pb"
	"github.com/RoseKhlifa/FarmBot/internal/game/transport"
)

type VisitAction string

const (
	ActionSteal     VisitAction = "steal"
	ActionHelp      VisitAction = "help"
	ActionBad       VisitAction = "bad"
	ActionGoldenBug VisitAction = "golden_bug"
)

type VisitConfig struct {
	Transport GameTransport
	AccountID string
	HostGID   int64
	API       *API
	Limits    *Limits
	Analyzer  *LandAnalyzer
	Warehouse warehouse.Service
	Farm      farm.Service
	Now       func() time.Time
	RecentTTL time.Duration
}
type VisitResult struct {
	HostGID                                         int64
	Entered, Left                                   bool
	Acted                                           []int64
	StealCount, HelpCount, BadCount, GoldenBugCount int
	Lands                                           []*pb.LandInfo
	Analysis                                        Analysis
	Err                                             error
}
type visitKey struct {
	host, land int64
	action     VisitAction
}
type recentHelp struct {
	snapshot int64
	expires  time.Time
	inflight bool
}
type VisitService struct {
	transport GameTransport
	accountID string
	hostGID   int64
	api       *API
	limits    *Limits
	analyzer  *LandAnalyzer
	warehouse warehouse.Service
	now       func() time.Time
	recentTTL time.Duration
	mu        sync.Mutex
	recent    map[visitKey]recentHelp
}

func NewVisitService(cfg VisitConfig) *VisitService {
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.RecentTTL <= 0 {
		cfg.RecentTTL = 30 * time.Second
	}
	return &VisitService{transport: cfg.Transport, accountID: cfg.AccountID, hostGID: cfg.HostGID, api: cfg.API, limits: cfg.Limits, analyzer: cfg.Analyzer, warehouse: cfg.Warehouse, now: cfg.Now, recentTTL: cfg.RecentTTL, recent: make(map[visitKey]recentHelp)}
}

func (v *VisitService) VisitFriend(ctx context.Context, gid int64, action VisitAction, landIDs []int64) (result VisitResult, err error) {
	if gid <= 0 {
		return result, ErrInvalidGID
	}
	if v.api == nil {
		return result, ErrAPIRequired
	}
	entered, err := v.api.EnterFriendFarm(ctx, gid)
	if err != nil {
		return result, err
	}
	result.HostGID = gid
	result.Entered = true
	defer func() {
		leaveErr := v.api.LeaveFriendFarm(ctx, gid)
		result.Left = leaveErr == nil
		if err == nil && leaveErr != nil {
			err = leaveErr
		}
		result.Err = err
	}()
	result.Lands = entered.GetLands()
	if v.analyzer != nil {
		result.Analysis = v.analyzer.Analyze(result.Lands, v.now())
	}
	ids := NormalizeGIDs(landIDs)
	if len(ids) == 0 {
		switch action {
		case ActionSteal:
			ids = result.Analysis.Stealable
		case ActionHelp:
			ids = uniqueAppend(result.Analysis.NeedWater, result.Analysis.NeedWeed, result.Analysis.NeedBug)
		case ActionBad:
			ids = uniqueAppend(result.Analysis.CanPutBug, result.Analysis.CanPutWeed)
		case ActionGoldenBug:
			ids = result.Analysis.CanPutGoldenBug
		}
	}
	acted, actionErr := v.doAction(ctx, gid, action, ids, result.Lands)
	result.Acted = acted
	err = actionErr
	switch action {
	case ActionSteal:
		result.StealCount = len(acted)
	case ActionHelp:
		result.HelpCount = len(acted)
	case ActionBad:
		result.BadCount = len(acted)
	case ActionGoldenBug:
		result.GoldenBugCount = len(acted)
	}
	return result, err
}

func (v *VisitService) VisitFriendForSteal(ctx context.Context, gid int64, ids []int64) (VisitResult, error) {
	return v.VisitFriend(ctx, gid, ActionSteal, ids)
}
func (v *VisitService) VisitFriendForHelp(ctx context.Context, gid int64, ids []int64) (VisitResult, error) {
	return v.VisitFriend(ctx, gid, ActionHelp, ids)
}
func (v *VisitService) DoFriendOperation(ctx context.Context, gid int64, action VisitAction, ids []int64) (VisitResult, error) {
	return v.VisitFriend(ctx, gid, action, ids)
}

func (v *VisitService) doAction(ctx context.Context, gid int64, action VisitAction, ids []int64, lands []*pb.LandInfo) ([]int64, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	if v.limits == nil && (action == ActionSteal || action == ActionHelp || action == ActionBad) {
		return nil, ErrAPIRequired
	}
	switch action {
	case ActionSteal:
		reply, err := v.limits.StealHarvest(ctx, ids, gid)
		if err != nil {
			return nil, err
		}
		acted := make([]int64, 0, len(reply.GetLand()))
		for _, land := range reply.GetLand() {
			if land != nil {
				acted = append(acted, land.GetId())
			}
		}
		if v.warehouse != nil {
			_, _ = v.warehouse.SellAll(ctx)
		}
		return acted, nil
	case ActionHelp:
		return v.help(ctx, gid, ids, lands)
	case ActionBad:
		return v.bad(ctx, gid, ids, lands)
	case ActionGoldenBug:
		return v.golden(ctx, gid, ids)
	default:
		return nil, fmt.Errorf("unknown friend action %q", action)
	}
}
func (v *VisitService) help(ctx context.Context, gid int64, ids []int64, lands []*pb.LandInfo) ([]int64, error) {
	ids = NormalizeGIDs(ids)
	snapshot := landSnapshot(lands)
	fresh := ids[:0]
	for _, id := range ids {
		key := visitKey{host: gid, land: id, action: ActionHelp}
		if v.seenHelp(key, snapshot) {
			continue
		}
		v.markHelpInFlight(key, snapshot)
		fresh = append(fresh, id)
	}
	if len(fresh) == 0 {
		return nil, nil
	}
	result, err := v.limits.HelpFarming(ctx, fresh, gid)
	if err != nil {
		for _, id := range fresh {
			v.clearHelp(visitKey{host: gid, land: id, action: ActionHelp})
		}
		return nil, err
	}
	for _, id := range result.Confirmed {
		v.markHelp(visitKey{host: gid, land: id, action: ActionHelp}, snapshot)
	}
	return result.Confirmed, nil
}
func (v *VisitService) bad(ctx context.Context, gid int64, ids []int64, lands []*pb.LandInfo) ([]int64, error) {
	limit := int(v.limits.BadRemaining())
	if len(ids) > limit {
		ids = ids[:limit]
	}
	if len(ids) == 0 {
		return nil, nil
	}
	bugs := make([]int64, 0)
	weeds := make([]int64, 0)
	if v.analyzer != nil {
		analysis := v.analyzer.Analyze(lands, v.now())
		for _, id := range ids {
			if contains(analysis.CanPutBug, id) {
				bugs = append(bugs, id)
			} else if contains(analysis.CanPutWeed, id) {
				weeds = append(weeds, id)
			}
		}
	}
	acted := make([]int64, 0)
	if len(bugs) > 0 {
		reply, err := v.limits.PutInsects(ctx, bugs, gid)
		if err != nil {
			return acted, err
		}
		for _, land := range reply.GetLand() {
			if land != nil {
				acted = append(acted, land.GetId())
			}
		}
	}
	if len(weeds) > 0 {
		reply, err := v.limits.PutWeeds(ctx, weeds, gid)
		if err != nil {
			return acted, err
		}
		for _, land := range reply.GetLand() {
			if land != nil {
				acted = append(acted, land.GetId())
			}
		}
	}
	return NormalizeGIDs(acted), nil
}
func (v *VisitService) golden(ctx context.Context, gid int64, ids []int64) ([]int64, error) {
	if v.transport == nil {
		return nil, ErrTransportRequired
	}
	reply := new(pb.PutSocialItemReply)
	response, err := v.transport.SendMsg(ctx, transport.Command{ServiceName: PlantServiceName, MethodName: "PutSocialItem", Response: reply}, &pb.PutSocialItemRequest{HostGid: gid, LandIds: NormalizeGIDs(ids), ItemId: GoldenBugItemID, SocialType: GoldenBugSocialType})
	if err != nil {
		return nil, err
	}
	if response != nil {
		var ok bool
		reply, ok = response.(*pb.PutSocialItemReply)
		if !ok {
			return nil, fmt.Errorf("put social item returned %T", response)
		}
	}
	if v.limits != nil {
		v.limits.Update(reply.GetOperationLimits())
	}
	acted := make([]int64, 0, len(reply.GetRewards()))
	for _, item := range reply.GetRewards() {
		if item != nil {
			acted = append(acted, item.GetLandId())
		}
	}
	return acted, nil
}

func (v *VisitService) RunBatchWithFallback(ctx context.Context, gid int64, action VisitAction, ids []int64) ([]int64, error) {
	ids = NormalizeGIDs(ids)
	if len(ids) == 0 {
		return nil, nil
	}
	result, err := v.VisitFriend(ctx, gid, action, ids)
	if err == nil {
		return result.Acted, nil
	}
	acted := make([]int64, 0, len(ids))
	for _, id := range ids {
		one, oneErr := v.VisitFriend(ctx, gid, action, []int64{id})
		if oneErr != nil {
			continue
		}
		acted = append(acted, one.Acted...)
	}
	if len(acted) > 0 {
		return acted, nil
	}
	return nil, err
}
func (v *VisitService) seenHelp(key visitKey, snapshot int64) bool {
	v.mu.Lock()
	defer v.mu.Unlock()
	entry, ok := v.recent[key]
	return ok && entry.snapshot == snapshot && v.now().Before(entry.expires)
}
func (v *VisitService) markHelp(key visitKey, snapshot int64) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.recent[key] = recentHelp{snapshot: snapshot, expires: v.now().Add(v.recentTTL)}
}

func (v *VisitService) markHelpInFlight(key visitKey, snapshot int64) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.recent[key] = recentHelp{snapshot: snapshot, expires: v.now().Add(15 * time.Second), inflight: true}
}

func (v *VisitService) clearHelp(key visitKey) {
	v.mu.Lock()
	defer v.mu.Unlock()
	delete(v.recent, key)
}
func uniqueAppend(values ...[]int64) []int64 {
	out := make([]int64, 0)
	for _, list := range values {
		out = append(out, list...)
	}
	return NormalizeGIDs(out)
}

func landSnapshot(lands []*pb.LandInfo) int64 {
	h := fnv.New64a()
	for _, land := range lands {
		if land == nil {
			continue
		}
		_, _ = fmt.Fprintf(h, "%d:%d:", land.GetId(), land.GetMasterLandId())
		if plant := land.GetPlant(); plant != nil {
			_, _ = fmt.Fprintf(h, "%d:%d:%d:%d:%d", plant.GetId(), plant.GetDryNum(), len(plant.GetWeedOwners()), len(plant.GetInsectOwners()), len(plant.GetPhases()))
		}
	}
	return int64(h.Sum64())
}
