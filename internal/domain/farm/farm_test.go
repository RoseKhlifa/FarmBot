package farm

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/RoseKhlifa/FarmBot/internal/game/pb"
)

func phaseInfo(kind pb.PlantPhase, begin int64) *pb.PlantPhaseInfo {
	return &pb.PlantPhaseInfo{Phase: int32(kind), BeginTime: begin}
}

func TestCurrentPhaseAndAnalyzeLandPhase(t *testing.T) {
	now := time.Unix(10_000, 0)
	phases := []*pb.PlantPhaseInfo{
		phaseInfo(pb.PlantPhase_SEED, now.Add(-20*time.Second).Unix()),
		phaseInfo(pb.PlantPhase_MATURE, now.Add(10*time.Second).Unix()),
	}
	if got := CurrentPhase(phases, now); got.GetPhase() != int32(pb.PlantPhase_SEED) {
		t.Fatalf("CurrentPhase before maturity = %v, want seed", got.GetPhase())
	}
	if got := CurrentPhase(phases, now.Add(15*time.Second)); got.GetPhase() != int32(pb.PlantPhase_MATURE) {
		t.Fatalf("CurrentPhase after maturity = %v, want mature", got.GetPhase())
	}

	lands := []*pb.LandInfo{
		{Id: 1, Unlocked: true, Plant: &pb.PlantInfo{Id: 101, Phases: phases, DryNum: 1}},
		{Id: 2, Unlocked: true, MasterLandId: 1},
		{Id: 3, Unlocked: true},
		{Id: 4, Unlocked: false, CouldUnlock: true},
	}
	analysis := AnalyzeLands(lands, now)
	if !reflect.DeepEqual(analysis.Growing, []int64{1}) || !reflect.DeepEqual(analysis.NeedWater, []int64{1}) {
		t.Fatalf("analysis = %+v, want growing/water on master only", analysis)
	}
	if !reflect.DeepEqual(analysis.Empty, []int64{3}) || !reflect.DeepEqual(analysis.Unlockable, []int64{4}) {
		t.Fatalf("analysis empty/unlockable = %+v", analysis)
	}
}

func TestSelectMaximumNonOverlappingGroups(t *testing.T) {
	groups := []TwoByTwoGroup{
		{Key: "a", MasterLandID: 1, LandIDs: []int64{1, 2, 5, 6}},
		{Key: "b", MasterLandID: 2, LandIDs: []int64{2, 3, 6, 7}},
		{Key: "c", MasterLandID: 9, LandIDs: []int64{9, 10, 13, 14}},
	}
	selected := SelectMaximumNonOverlappingGroups(groups, 2)
	if len(selected) != 2 {
		t.Fatalf("selected %d groups, want 2", len(selected))
	}
	if selected[0].Key != "a" || selected[1].Key != "c" {
		t.Fatalf("selected = %#v, want a,c", selected)
	}
}

type rankingCatalog struct {
	plants []PlantRecord
}

func (c rankingCatalog) AllPlants() []PlantRecord { return append([]PlantRecord(nil), c.plants...) }
func (c rankingCatalog) PlantByID(id int64) (PlantRecord, bool) {
	for _, plant := range c.plants {
		if plant.ID == id {
			return plant, true
		}
	}
	return PlantRecord{}, false
}
func (c rankingCatalog) PlantBySeedID(id int64) (PlantRecord, bool) {
	for _, plant := range c.plants {
		if plant.SeedID == id {
			return plant, true
		}
	}
	return PlantRecord{}, false
}
func (rankingCatalog) FruitPrice(int64) int64 { return 10 }
func (rankingCatalog) SeedPrice(int64) int64  { return 1 }
func (rankingCatalog) SeedLevel(int64) int64  { return 1 }

func TestSelectSeedUsesRankingThenLevelFallback(t *testing.T) {
	catalog := rankingCatalog{plants: []PlantRecord{
		{ID: 1, SeedID: 101, Name: "slow", GrowPhases: "1:3600", Exp: 10, FruitCount: 1},
		{ID: 2, SeedID: 102, Name: "fast", GrowPhases: "1:1800", Exp: 10, FruitCount: 1},
	}}
	analytics := NewAnalytics(catalog)
	candidates := []Seed{
		{SeedID: 101, RequiredLevel: 5, Unlocked: true, Count: 1},
		{SeedID: 102, RequiredLevel: 5, Unlocked: true, Count: 1},
	}
	seed, ok := SelectSeed(candidates, PlantingConfig{Strategy: StrategyMaxProfit, UserLevel: 5}, analytics)
	if !ok || seed.SeedID != 102 {
		t.Fatalf("ranked seed = %#v, %v; want 102", seed, ok)
	}
	seed, ok = SelectSeed(candidates, PlantingConfig{Strategy: StrategyLevel, UserLevel: 5}, nil)
	if !ok || seed.SeedID != 101 {
		t.Fatalf("level fallback seed = %#v, %v; want lower ID tie-break", seed, ok)
	}
}

func TestFertilizerTargetFilters(t *testing.T) {
	now := time.Unix(20_000, 0)
	lands := []*pb.LandInfo{
		{Id: 1, Unlocked: true, Level: 2, Plant: &pb.PlantInfo{Phases: []*pb.PlantPhaseInfo{phaseInfo(pb.PlantPhase_SEED, now.Add(-time.Minute).Unix()), phaseInfo(pb.PlantPhase_MATURE, now.Add(time.Hour).Unix())}, LeftInorcFertTimes: 2}},
		{Id: 2, Unlocked: true, Level: 3, Plant: &pb.PlantInfo{Phases: []*pb.PlantPhaseInfo{phaseInfo(pb.PlantPhase_DEAD, now.Add(-time.Minute).Unix())}, LeftInorcFertTimes: 2}},
		{Id: 3, Unlocked: true, Level: 4},
		{Id: 4, Unlocked: false, Level: 2, Plant: &pb.PlantInfo{Phases: []*pb.PlantPhaseInfo{phaseInfo(pb.PlantPhase_SEED, now.Add(-time.Minute).Unix())}, LeftInorcFertTimes: 2}},
	}
	if got := GetOrganicFertilizerTargetsFromLands(lands, now); !reflect.DeepEqual(got, []int64{1}) {
		t.Fatalf("organic targets = %#v, want [1]", got)
	}
	landTypes := map[int64]LandType{1: LandTypeRed, 2: LandTypeBlack, 3: LandTypeGold}
	if got := FilterLandIDsByTypes([]int64{1, 2, 3}, landTypes, []LandType{LandTypeRed, LandTypeGold}); !reflect.DeepEqual(got, []int64{1, 3}) {
		t.Fatalf("filtered IDs = %#v, want [1 3]", got)
	}
}

type routeAPI struct {
	API
	mu    sync.Mutex
	calls []string
	lands []*pb.LandInfo
	block chan struct{}
}

func (f *routeAPI) record(name string) {
	f.mu.Lock()
	f.calls = append(f.calls, name)
	f.mu.Unlock()
}
func (f *routeAPI) GetAllLands(context.Context) (*pb.AllLandsReply, error) {
	f.record("lands")
	return &pb.AllLandsReply{Lands: f.lands}, nil
}
func (f *routeAPI) Farming(context.Context, []int64) (*pb.FarmingReply, error) {
	f.record("farming")
	return &pb.FarmingReply{}, nil
}
func (f *routeAPI) Harvest(context.Context, []int64) (*pb.HarvestReply, error) {
	f.record("harvest")
	return &pb.HarvestReply{}, nil
}
func (f *routeAPI) UnlockLand(context.Context, int64, bool) (*pb.UnlockLandReply, error) {
	f.record("unlock")
	return &pb.UnlockLandReply{}, nil
}
func (f *routeAPI) UpgradeLand(context.Context, int64) (*pb.UpgradeLandReply, error) {
	f.record("upgrade")
	return &pb.UpgradeLandReply{}, nil
}

func TestOrchestratorAllRoutesAndPreventsConcurrentPasses(t *testing.T) {
	now := time.Unix(30_000, 0)
	api := &routeAPI{lands: []*pb.LandInfo{
		{Id: 1, Unlocked: true, Plant: &pb.PlantInfo{Id: 11, Phases: []*pb.PlantPhaseInfo{phaseInfo(pb.PlantPhase_MATURE, now.Add(-time.Minute).Unix())}}},
		{Id: 2, Unlocked: true, Plant: &pb.PlantInfo{Id: 12, DryNum: 1, Phases: []*pb.PlantPhaseInfo{phaseInfo(pb.PlantPhase_SEED, now.Add(-time.Minute).Unix()), phaseInfo(pb.PlantPhase_MATURE, now.Add(time.Hour).Unix())}}},
		{Id: 3, Unlocked: true},
		{Id: 4, Unlocked: false, CouldUnlock: true},
		{Id: 5, Unlocked: true, CouldUpgrade: true},
	}}
	analyzer := NewLandAnalyzer(nil, api)
	analyzer.now = func() time.Time { return now }
	orchestrator, err := NewOrchestrator(OrchestratorConfig{API: api, Analyzer: analyzer, LandUpgrade: true})
	if err != nil {
		t.Fatalf("NewOrchestrator() error = %v", err)
	}
	result, err := orchestrator.Run(context.Background(), OperationAll)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !result.HadWork || len(result.Harvested) != 1 || len(result.FarmingLandIDs) != 1 || len(result.Unlocked) != 1 || len(result.Upgraded) != 1 {
		t.Fatalf("result = %+v", result)
	}
	if !reflect.DeepEqual(api.calls, []string{"lands", "farming", "harvest", "unlock", "upgrade"}) {
		t.Fatalf("calls = %#v", api.calls)
	}

	// A direct concurrent call while the first pass is in progress is tested by
	// the operation guard with a deterministic blocking API below.
	blocking := &blockingAPI{routeAPI: routeAPI{lands: api.lands}, entered: make(chan struct{}), release: make(chan struct{})}
	second, err := NewOrchestrator(OrchestratorConfig{API: blocking})
	if err != nil {
		t.Fatal(err)
	}
	firstDone := make(chan struct{})
	go func() { _, _ = second.Run(context.Background(), OperationHarvest); close(firstDone) }()
	<-blocking.entered
	if _, err := second.Run(context.Background(), OperationHarvest); !errors.Is(err, ErrFarmOperationRunning) {
		t.Fatalf("concurrent Run() error = %v, want ErrFarmOperationRunning", err)
	}
	close(blocking.release)
	<-firstDone
}

type blockingAPI struct {
	routeAPI
	entered chan struct{}
	release chan struct{}
}

func (f *blockingAPI) GetAllLands(ctx context.Context) (*pb.AllLandsReply, error) {
	f.record("lands")
	select {
	case <-f.entered:
	default:
		close(f.entered)
	}
	select {
	case <-f.release:
		return &pb.AllLandsReply{Lands: f.lands}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

type testLoopScheduler struct {
	mu       sync.Mutex
	name     string
	callback any
	stopped  bool
}

func (s *testLoopScheduler) Every(name string, _ time.Duration, _ time.Duration, callback any) error {
	s.mu.Lock()
	s.name, s.callback = name, callback
	s.mu.Unlock()
	return nil
}

func (s *testLoopScheduler) Stop(name string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if name == s.name {
		s.stopped = true
	}
	return name == s.name
}

func TestOrchestratorLoopRegistersAndStops(t *testing.T) {
	api := &routeAPI{lands: []*pb.LandInfo{{Id: 1, Unlocked: true}}}
	scheduler := &testLoopScheduler{}
	o, err := NewOrchestrator(OrchestratorConfig{API: api, Scheduler: scheduler, Interval: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if err := o.StartLoop(context.Background()); err != nil {
		t.Fatalf("StartLoop() error = %v", err)
	}
	if !o.LoopRunning() || scheduler.name != FarmLoopTaskName {
		t.Fatalf("loop state/name = %v/%q", o.LoopRunning(), scheduler.name)
	}
	callback, ok := scheduler.callback.(func(context.Context))
	if !ok {
		t.Fatalf("scheduler callback type = %T", scheduler.callback)
	}
	callback(context.Background())
	if err := o.StopLoop(); err != nil {
		t.Fatalf("StopLoop() error = %v", err)
	}
	if o.LoopRunning() || !scheduler.stopped {
		t.Fatalf("loop did not stop: running=%v stopped=%v", o.LoopRunning(), scheduler.stopped)
	}
}
