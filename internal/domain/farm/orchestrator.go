package farm

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/RoseKhlifa/FarmBot/internal/game/pb"
)

const (
	FarmLoopTaskName      = "farm"
	FertilizerBuyTaskName = "farm_fertilizer_buy"
)

var (
	ErrOrchestratorAPIRequired = errors.New("farm orchestrator API is required")
	ErrFarmOperationRunning    = errors.New("farm operation is already running")
	ErrFarmLoopRunning         = errors.New("farm loop is already running")
)

// OperationType names the small set of actions exposed by the farm domain.
// Keeping this type local avoids the stringly-typed RPC switch used by the
// legacy worker while still allowing HTTP handlers to pass a path parameter.
type OperationType string

const (
	OperationAll     OperationType = "all"
	OperationHarvest OperationType = "harvest"
	OperationPlant   OperationType = "plant"
	OperationClear   OperationType = "clear"
	OperationUpgrade OperationType = "upgrade"

	FarmOperationAll     = OperationAll
	FarmOperationHarvest = OperationHarvest
	FarmOperationPlant   = OperationPlant
	FarmOperationClear   = OperationClear
	FarmOperationUpgrade = OperationUpgrade
)

func NormalizeOperation(value string) (OperationType, error) {
	operation := OperationType(value)
	switch operation {
	case OperationAll, OperationHarvest, OperationPlant, OperationClear, OperationUpgrade:
		return operation, nil
	default:
		return "", fmt.Errorf("unsupported farm operation %q", value)
	}
}

// FarmLoopScheduler is the narrow scheduling capability needed by the farm
// loop. account.Scheduler satisfies it without making this domain import the
// account package.
type FarmLoopScheduler interface {
	Every(name string, interval, jitter time.Duration, fn any) error
}

type farmLoopStopper interface {
	Stop(name string) bool
}

// OrchestratorConfig contains policy switches that used to be read from
// process-global Node state. The composition root can populate them from the
// account's configuration without adding an account dependency here.
type OrchestratorConfig struct {
	API                   API
	Analyzer              *LandAnalyzer
	Fertilizer            *Fertilizer
	Planter               *Planter
	Scheduler             FarmLoopScheduler
	Interval              time.Duration
	Jitter                time.Duration
	SkipOwnWeedBug        bool
	GoldenBugClear        bool
	LandUpgrade           bool
	MultiSeasonFertilizer bool
	SmartFertilizer       bool
}

// FarmOperationResult is a best-effort snapshot of one farm pass. RPC errors
// from an individual action are retained in Errors so a transient failure in
// one operation does not prevent harvesting or planting other plots.
type FarmOperationResult struct {
	Operation      OperationType
	HadWork        bool
	Actions        []string
	Errors         []error
	Analysis       LandAnalysis
	FarmingLandIDs []int64
	Harvested      []int64
	Removed        []int64
	MultiSeason    []int64
	Unlocked       []int64
	Upgraded       []int64
	Planting       PlantingResult
	Fertilizer     FertilizerResult
}

func (r FarmOperationResult) FirstError() error {
	if len(r.Errors) == 0 {
		return nil
	}
	return r.Errors[0]
}

// Orchestrator owns one account's farm pass and its optional repeating loop.
// It has no package-level state, so separate accounts cannot suppress each
// other's checks.
type Orchestrator struct {
	api        API
	analyzer   *LandAnalyzer
	fertilizer *Fertilizer
	planter    *Planter
	config     OrchestratorConfig

	operationMu sync.Mutex
	operationOn bool

	loopMu        sync.Mutex
	loopRunning   bool
	loopCancel    context.CancelFunc
	loopScheduler FarmLoopScheduler
	loopWG        sync.WaitGroup
}

var _ interface {
	Run(context.Context, OperationType) (FarmOperationResult, error)
	StartLoop(context.Context, ...FarmLoopScheduler) error
	StopLoop() error
} = (*Orchestrator)(nil)

func NewOrchestrator(config OrchestratorConfig) (*Orchestrator, error) {
	if config.API == nil {
		return nil, ErrOrchestratorAPIRequired
	}
	if config.Analyzer == nil {
		config.Analyzer = NewLandAnalyzer(nil, config.API)
	}
	if config.Interval <= 0 {
		config.Interval = 4 * time.Second
	}
	if config.Jitter < 0 {
		config.Jitter = 0
	}
	return &Orchestrator{
		api:        config.API,
		analyzer:   config.Analyzer,
		fertilizer: config.Fertilizer,
		planter:    config.Planter,
		config:     config,
	}, nil
}

// Run executes one operation. An operation is intentionally serialized per
// orchestrator; callers may invoke this from HTTP and scheduler goroutines at
// the same time without duplicating RPCs.
func (o *Orchestrator) Run(ctx context.Context, operation OperationType) (FarmOperationResult, error) {
	if o == nil || o.api == nil {
		return FarmOperationResult{}, ErrOrchestratorAPIRequired
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return FarmOperationResult{Operation: operation}, err
	}
	if operation == "" {
		operation = OperationAll
	}
	if _, err := NormalizeOperation(string(operation)); err != nil {
		return FarmOperationResult{Operation: operation}, err
	}

	o.operationMu.Lock()
	if o.operationOn {
		o.operationMu.Unlock()
		return FarmOperationResult{Operation: operation}, ErrFarmOperationRunning
	}
	o.operationOn = true
	o.operationMu.Unlock()
	defer func() {
		o.operationMu.Lock()
		o.operationOn = false
		o.operationMu.Unlock()
	}()

	return o.runOnce(ctx, operation)
}

func (o *Orchestrator) Check(ctx context.Context) (FarmOperationResult, error) {
	return o.Run(ctx, OperationAll)
}

func (o *Orchestrator) RunFarmOperation(ctx context.Context, operation string) (FarmOperationResult, error) {
	normalized, err := NormalizeOperation(operation)
	if err != nil {
		return FarmOperationResult{}, err
	}
	return o.Run(ctx, normalized)
}

func (o *Orchestrator) RunOperation(ctx context.Context, operation string) (FarmOperationResult, error) {
	return o.RunFarmOperation(ctx, operation)
}

func (o *Orchestrator) runOnce(ctx context.Context, operation OperationType) (FarmOperationResult, error) {
	result := FarmOperationResult{Operation: operation}
	landsReply, err := o.api.GetAllLands(ctx)
	if err != nil {
		return result, err
	}
	if landsReply == nil || len(landsReply.GetLands()) == 0 {
		return result, nil
	}
	lands := landsReply.GetLands()
	result.Analysis = o.analyzer.Analyze(lands)

	if operation == OperationAll || operation == OperationClear {
		o.runFarming(ctx, &result)
	}

	var harvestReply *pb.HarvestReply
	if operation == OperationAll || operation == OperationHarvest {
		if len(result.Analysis.Harvestable) > 0 {
			harvestReply, err = o.api.Harvest(ctx, result.Analysis.Harvestable)
			if err != nil {
				result.Errors = append(result.Errors, fmt.Errorf("harvest: %w", err))
			} else {
				result.Harvested = append([]int64(nil), result.Analysis.Harvestable...)
				result.Actions = append(result.Actions, fmt.Sprintf("harvest:%d", len(result.Harvested)))
			}
		}
	}

	if operation == OperationAll || operation == OperationPlant {
		o.runPlanting(ctx, &result, lands, harvestReply)
	}

	if operation == OperationUpgrade || (operation == OperationAll && o.config.LandUpgrade) {
		o.runLandChanges(ctx, &result)
	}

	if operation == OperationAll {
		o.runSmartFertilizer(ctx, &result)
	}
	result.HadWork = len(result.Actions) > 0
	return result, nil
}

func (o *Orchestrator) runFarming(ctx context.Context, result *FarmOperationResult) {
	ids := append([]int64(nil), result.Analysis.NeedWater...)
	if !o.config.SkipOwnWeedBug || result.Operation != OperationAll {
		ids = append(ids, result.Analysis.NeedWeed...)
		ids = append(ids, result.Analysis.NeedBug...)
	}
	if o.config.GoldenBugClear {
		ids = append(ids, result.Analysis.NeedGoldenBug...)
	}
	ids = uniquePositive(ids)
	if len(ids) == 0 {
		return
	}
	if _, err := o.api.Farming(ctx, ids); err != nil {
		result.Errors = append(result.Errors, fmt.Errorf("farming: %w", err))
		return
	}
	result.FarmingLandIDs = ids
	result.Actions = append(result.Actions, fmt.Sprintf("farming:%d", len(ids)))
}

func (o *Orchestrator) runPlanting(ctx context.Context, result *FarmOperationResult, lands []*pb.LandInfo, harvestReply *pb.HarvestReply) {
	dead := append([]int64(nil), result.Analysis.Dead...)
	empty := append([]int64(nil), result.Analysis.Empty...)
	if result.Operation == OperationAll && len(result.Harvested) > 0 {
		resolution := o.analyzer.ResolveRemovableHarvestedLands(ctx, result.Harvested, harvestReply)
		dead = append(dead, resolution.Removable...)
		result.MultiSeason = append([]int64(nil), resolution.Growing...)
	}
	dead = uniquePositive(dead)
	empty = uniquePositive(empty)
	if o.planter != nil && (len(dead) > 0 || len(empty) > 0) {
		planted, err := o.planter.AutoPlantEmptyLands(ctx, dead, empty, lands)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("plant: %w", err))
		} else {
			result.Planting = planted
			result.Removed = append([]int64(nil), dead...)
			if planted.RemovedCount > 0 {
				result.Actions = append(result.Actions, fmt.Sprintf("remove:%d", planted.RemovedCount))
			}
			if planted.OccupiedCount > 0 || planted.PlantedCount > 0 {
				result.Actions = append(result.Actions, fmt.Sprintf("plant:%d", planted.OccupiedCount))
			}
		}
	}
	if o.config.MultiSeasonFertilizer && len(result.MultiSeason) > 0 && o.fertilizer != nil {
		fertilized, err := o.fertilizer.Run(ctx, result.MultiSeason, FertilizerRunOptions{Reason: "multi_season"})
		result.Fertilizer.Normal += fertilized.Normal
		result.Fertilizer.Organic += fertilized.Organic
		if err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("multi-season fertilizer: %w", err))
		} else if fertilized.Normal+fertilized.Organic > 0 {
			result.Actions = append(result.Actions, fmt.Sprintf("fertilizer:%d", fertilized.Normal+fertilized.Organic))
		}
	}
}

func (o *Orchestrator) runLandChanges(ctx context.Context, result *FarmOperationResult) {
	for _, id := range result.Analysis.Unlockable {
		if _, err := o.api.UnlockLand(ctx, id, false); err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("unlock land %d: %w", id, err))
			continue
		}
		result.Unlocked = append(result.Unlocked, id)
	}
	if len(result.Unlocked) > 0 {
		result.Actions = append(result.Actions, fmt.Sprintf("unlock:%d", len(result.Unlocked)))
	}
	for _, id := range result.Analysis.Upgradable {
		if _, err := o.api.UpgradeLand(ctx, id); err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("upgrade land %d: %w", id, err))
			continue
		}
		result.Upgraded = append(result.Upgraded, id)
	}
	if len(result.Upgraded) > 0 {
		result.Actions = append(result.Actions, fmt.Sprintf("upgrade:%d", len(result.Upgraded)))
	}
}

func (o *Orchestrator) runSmartFertilizer(ctx context.Context, result *FarmOperationResult) {
	if o.fertilizer == nil || (!o.config.SmartFertilizer && !isSmartMode(o.fertilizer.config.Mode)) {
		return
	}
	fertilized, err := o.fertilizer.Run(ctx, nil, FertilizerRunOptions{SkipNormal: true})
	result.Fertilizer.Normal += fertilized.Normal
	result.Fertilizer.Organic += fertilized.Organic
	if err != nil {
		result.Errors = append(result.Errors, fmt.Errorf("smart fertilizer: %w", err))
		return
	}
	if total := fertilized.Normal + fertilized.Organic; total > 0 {
		result.Actions = append(result.Actions, fmt.Sprintf("fertilizer:%d", total))
	}
}

func (o *Orchestrator) StartLoop(ctx context.Context, scheduler ...FarmLoopScheduler) error {
	if o == nil {
		return ErrOrchestratorAPIRequired
	}
	if ctx == nil {
		ctx = context.Background()
	}
	o.loopMu.Lock()
	if o.loopRunning {
		o.loopMu.Unlock()
		return ErrFarmLoopRunning
	}
	loopCtx, cancel := context.WithCancel(ctx)
	o.loopRunning = true
	o.loopCancel = cancel
	selected := o.config.Scheduler
	if len(scheduler) > 0 && scheduler[0] != nil {
		selected = scheduler[0]
	}
	o.loopScheduler = selected
	o.loopMu.Unlock()

	if selected != nil {
		if err := selected.Every(FarmLoopTaskName, o.config.Interval, o.config.Jitter, func(runCtx context.Context) {
			if _, runErr := o.Run(runCtx, OperationAll); runErr != nil && !errors.Is(runErr, ErrFarmOperationRunning) {
				return
			}
		}); err != nil {
			cancel()
			o.loopMu.Lock()
			o.loopRunning = false
			o.loopCancel = nil
			o.loopScheduler = nil
			o.loopMu.Unlock()
			return fmt.Errorf("start farm loop: %w", err)
		}
		if o.fertilizer != nil && o.fertilizer.BuyCheckConfigured() {
			if err := selected.Every(FertilizerBuyTaskName, o.fertilizer.config.BuyInterval, o.fertilizer.config.BuyJitter, func(buyCtx context.Context) {
				_ = o.fertilizer.CheckAndBuy(buyCtx)
			}); err != nil {
				if stopper, ok := selected.(farmLoopStopper); ok {
					stopper.Stop(FarmLoopTaskName)
				}
				cancel()
				o.loopMu.Lock()
				o.loopRunning = false
				o.loopCancel = nil
				o.loopScheduler = nil
				o.loopMu.Unlock()
				return fmt.Errorf("start fertilizer buy loop: %w", err)
			}
		}
		return nil
	}

	o.loopWG.Add(1)
	go o.runTicker(loopCtx)
	return nil
}

func (o *Orchestrator) runTicker(ctx context.Context) {
	defer o.loopWG.Done()
	ticker := time.NewTicker(o.config.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_, _ = o.Run(ctx, OperationAll)
		}
	}
}

func (o *Orchestrator) StopLoop() error {
	if o == nil {
		return nil
	}
	o.loopMu.Lock()
	if !o.loopRunning {
		o.loopMu.Unlock()
		return nil
	}
	o.loopRunning = false
	cancel := o.loopCancel
	o.loopCancel = nil
	scheduler := o.loopScheduler
	o.loopScheduler = nil
	o.loopMu.Unlock()
	if cancel != nil {
		cancel()
	}
	if stopper, ok := scheduler.(farmLoopStopper); ok {
		stopper.Stop(FarmLoopTaskName)
		stopper.Stop(FertilizerBuyTaskName)
	}
	o.loopWG.Wait()
	return nil
}

func (o *Orchestrator) Start(ctx context.Context, scheduler ...FarmLoopScheduler) error {
	return o.StartLoop(ctx, scheduler...)
}

func (o *Orchestrator) Stop() error { return o.StopLoop() }

func (o *Orchestrator) LoopRunning() bool {
	if o == nil {
		return false
	}
	o.loopMu.Lock()
	defer o.loopMu.Unlock()
	return o.loopRunning
}
