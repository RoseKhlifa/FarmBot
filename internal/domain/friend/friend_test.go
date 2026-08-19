package friend

import (
	"context"
	"encoding/json"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/RoseKhlifa/FarmBot/internal/game/pb"
	"github.com/RoseKhlifa/FarmBot/internal/game/transport"
	"github.com/RoseKhlifa/FarmBot/internal/store"
	"google.golang.org/protobuf/proto"
)

func TestNormalizeAndQuietHours(t *testing.T) {
	if got := NormalizeGIDs([]int64{0, 3, 3, -1, 7}); !reflect.DeepEqual(got, []int64{3, 7}) {
		t.Fatalf("gids = %v", got)
	}
	quiet, err := ParseQuietHours("23:00-07:00,12:00-13:00")
	if err != nil {
		t.Fatal(err)
	}
	if !quiet.Contains(time.Date(2026, 1, 1, 23, 30, 0, 0, time.UTC)) || !quiet.Contains(time.Date(2026, 1, 1, 6, 59, 0, 0, time.UTC)) || quiet.Contains(time.Date(2026, 1, 1, 7, 0, 0, 0, time.UTC)) || quiet.Contains(time.Date(2026, 1, 1, 13, 0, 0, 0, time.UTC)) {
		t.Fatalf("quiet-hours boundary mismatch: %v %v %v %v", quiet.Contains(time.Date(2026, 1, 1, 23, 30, 0, 0, time.UTC)), quiet.Contains(time.Date(2026, 1, 1, 6, 59, 0, 0, time.UTC)), quiet.Contains(time.Date(2026, 1, 1, 7, 0, 0, 0, time.UTC)), quiet.Contains(time.Date(2026, 1, 1, 13, 0, 0, 0, time.UTC)))
	}
}

func TestLimitsResetAndEvents(t *testing.T) {
	now := time.Date(2026, 1, 1, 23, 59, 0, 0, time.UTC)
	clock := now
	limits := NewLimits(LimitsConfig{Now: func() time.Time { return clock }})
	limits.Update([]*pb.OperationLimit{{Id: OperationHelpWater, DayTimes: 2, DayTimesLt: 2, DayExpTimes: 1, DayExTimesLt: 2}})
	if limits.CanOperate(OperationHelpWater) || limits.Remaining(OperationHelpWater) != 0 || !limits.CanGetExp(OperationHelpWater) {
		t.Fatal("limit boundary not enforced")
	}
	limits.Update([]*pb.OperationLimit{{Id: OperationBadDaily, DayTimes: 4, DayTimesLt: 5}})
	if limits.BadRemaining() != 1 {
		t.Fatalf("bad remaining = %d", limits.BadRemaining())
	}
	clock = now.Add(2 * time.Minute)
	if !limits.CheckDailyReset() || !limits.CanOperate(OperationHelpWater) {
		t.Fatal("daily reset not applied")
	}
	select {
	case event := <-limits.Events():
		if event.Type != EventExpLimitReset {
			t.Fatalf("event = %+v", event)
		}
	default:
		t.Fatal("expected reset event")
	}
}

func TestAnalyzerActionsAndSlaveFiltering(t *testing.T) {
	now := time.Unix(2_000_000_000, 0)
	phase := func(value int32) *pb.PlantPhaseInfo { return &pb.PlantPhaseInfo{Phase: value, BeginTime: now.Unix()} }
	master := &pb.LandInfo{Id: 1, Unlocked: true, SlaveLandIds: []int64{2}, Plant: &pb.PlantInfo{Id: 10, Stealable: true, LeftFruitNum: 2, Phases: []*pb.PlantPhaseInfo{phase(int32(pb.PlantPhase_MATURE))}}}
	slave := &pb.LandInfo{Id: 2, Unlocked: true, MasterLandId: 1, Plant: master.Plant}
	needs := &pb.LandInfo{Id: 3, Unlocked: true, Plant: &pb.PlantInfo{Id: 11, DryNum: 1, WeedOwners: []int64{8}, InsectOwners: []int64{9}, Phases: []*pb.PlantPhaseInfo{phase(int32(pb.PlantPhase_GERMINATION))}}}
	analysis := NewLandAnalyzer(LandAnalyzerConfig{MyGID: 8, Now: func() time.Time { return now }}).Analyze([]*pb.LandInfo{master, slave, needs})
	if !reflect.DeepEqual(analysis.Stealable, []int64{1}) || !reflect.DeepEqual(analysis.NeedWater, []int64{3}) || !reflect.DeepEqual(analysis.NeedWeed, []int64{3}) || !reflect.DeepEqual(analysis.NeedBug, []int64{3}) || !reflect.DeepEqual(analysis.CanPutBug, []int64{1, 3}) || !reflect.DeepEqual(analysis.CanPutWeed, []int64{1}) {
		t.Fatalf("analysis = %+v", analysis)
	}
}

func TestVisitEnterActionLeave(t *testing.T) {
	fake := &rpcFake{enter: &pb.EnterReply{Lands: []*pb.LandInfo{{Id: 1, Unlocked: true}}}, farming: &pb.FarmingReply{Results: []*pb.FarmingResult{{LandId: 1}}}}
	api, err := NewAPI(APIConfig{Transport: fake, AccountID: "a"})
	if err != nil {
		t.Fatal(err)
	}
	limits := NewLimits(LimitsConfig{Transport: fake, HostGID: 99})
	visit := NewVisitService(VisitConfig{API: api, Limits: limits, Now: time.Now})
	result, err := visit.VisitFriendForHelp(context.Background(), 42, []int64{1})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Entered || !result.Left || !reflect.DeepEqual(result.Acted, []int64{1}) {
		t.Fatalf("result = %+v", result)
	}
	if !reflect.DeepEqual(fake.methods, []string{"Enter", "Farming", "Leave"}) {
		t.Fatalf("methods = %v", fake.methods)
	}
}

func TestAPICacheAndBlacklistUseRepository(t *testing.T) {
	cache := &cacheFake{}
	fake := &rpcFake{friends: &pb.GetAllReply{GameFriends: []*pb.GameFriend{{Gid: 11}, {Gid: 11}, {Gid: 12}}}}
	api, err := NewAPI(APIConfig{Transport: fake, AccountID: "account", Cache: cache})
	if err != nil {
		t.Fatal(err)
	}
	friends, err := api.GetAllFriends(context.Background())
	if err != nil || len(friends) != 2 {
		t.Fatalf("friends = %+v, err = %v", friends, err)
	}
	var gids []int64
	if err := json.Unmarshal(cache.known.Payload, &gids); err != nil || !reflect.DeepEqual(gids, []int64{11, 12}) {
		t.Fatalf("known gids = %v, err = %v", gids, err)
	}
	if err := api.AddBlacklist(context.Background(), 12, "banned"); err != nil {
		t.Fatal(err)
	}
	entries, err := api.Blacklist(context.Background())
	if err != nil || len(entries) != 1 || entries[12].Reason != "banned" {
		t.Fatalf("blacklist = %+v, err = %v", entries, err)
	}
}

type rpcFake struct {
	mu      sync.Mutex
	enter   *pb.EnterReply
	farming *pb.FarmingReply
	friends *pb.GetAllReply
	methods []string
}

func (f *rpcFake) SendMsg(_ context.Context, command transport.Command, _ proto.Message) (proto.Message, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.methods = append(f.methods, command.MethodName)
	switch command.MethodName {
	case "Enter":
		return f.enter, nil
	case "GetAll":
		return f.friends, nil
	case "Farming":
		return f.farming, nil
	case "Leave":
		return &pb.LeaveReply{}, nil
	}
	return command.Response, nil
}

type cacheFake struct {
	known, list, dog store.CacheValue
	blacklist        []store.BlacklistEntry
}

func (c *cacheFake) GetKnownFriendGIDs(context.Context, string) (store.CacheValue, error) {
	return c.known, nil
}
func (c *cacheFake) PutKnownFriendGIDs(_ context.Context, _ string, v store.CacheValue) error {
	c.known = v
	return nil
}
func (c *cacheFake) InvalidateKnownFriendGIDs(context.Context, string) error { return nil }
func (c *cacheFake) GetFriendDogInfo(context.Context, string) (store.CacheValue, error) {
	return c.dog, nil
}
func (c *cacheFake) PutFriendDogInfo(_ context.Context, _ string, v store.CacheValue) error {
	c.dog = v
	return nil
}
func (c *cacheFake) InvalidateFriendDogInfo(context.Context, string) error { return nil }
func (c *cacheFake) GetFriendList(context.Context, string) (store.CacheValue, error) {
	return c.list, nil
}
func (c *cacheFake) PutFriendList(_ context.Context, _ string, v store.CacheValue) error {
	c.list = v
	return nil
}
func (c *cacheFake) InvalidateFriendList(context.Context, string) error          { return nil }
func (c *cacheFake) RemoveFriendFromCache(context.Context, string, string) error { return nil }
func (c *cacheFake) DeleteAccountCaches(context.Context, string) error           { return nil }
func (c *cacheFake) ListBlacklist(context.Context, string) ([]store.BlacklistEntry, error) {
	return c.blacklist, nil
}
func (c *cacheFake) UpsertBlacklist(_ context.Context, e store.BlacklistEntry) error {
	c.blacklist = append(c.blacklist, e)
	return nil
}
func (c *cacheFake) DeleteBlacklist(_ context.Context, _ string, gid string) error {
	for i := range c.blacklist {
		if c.blacklist[i].GID == gid {
			c.blacklist = append(c.blacklist[:i], c.blacklist[i+1:]...)
			break
		}
	}
	return nil
}

var _ store.CacheRepo = (*cacheFake)(nil)
