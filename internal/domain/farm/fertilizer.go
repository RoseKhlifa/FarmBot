package farm

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/RoseKhlifa/FarmBot/internal/game/pb"
)

type FertilizerMode string

const (
	FertilizerNone         FertilizerMode = "none"
	FertilizerNormal       FertilizerMode = "normal"
	FertilizerOrganic      FertilizerMode = "organic"
	FertilizerBoth         FertilizerMode = "both"
	FertilizerSmart        FertilizerMode = "smart"
	FertilizerSmartOnly    FertilizerMode = "smart_only"
	FertilizerSmartNormal  FertilizerMode = "smart_normal"
	FertilizerFinalNormal  FertilizerMode = "final_normal"
	FertilizerFinalOrganic FertilizerMode = "final_organic"
)

var ErrFertilizerAPIRequired = errors.New("fertilizer API is required")

var allFertilizerLandTypes = []LandType{
	LandTypePurple, LandTypeGold, LandTypeBlack, LandTypeRed, LandTypeNormal,
}

// FertilizerConfig is account-local policy. A nil LandTypes slice means all
// land qualities; a non-nil empty slice intentionally disables fertilizing.
type FertilizerConfig struct {
	Mode                   FertilizerMode
	LandTypes              []LandType
	SmartThreshold         time.Duration
	BatchDelay             time.Duration
	OrganicMaxApplications int
	OnApplied              func(FertilizerMode, int)
	// BuyCheck is an optional composition-root adapter for the mall domain.
	// Keeping it as a callback avoids a farm -> mall import cycle while still
	// allowing the old fertilizer purchase timer to live in this domain.
	BuyCheck    func(context.Context) error
	BuyInterval time.Duration
	BuyJitter   time.Duration
}

type FertilizerRunOptions struct {
	Reason     string
	SkipNormal bool
}

type FertilizerResult struct {
	Normal  int
	Organic int
}

type Fertilizer struct {
	api    API
	config FertilizerConfig
	now    func() time.Time
}

func NewFertilizer(api API, config FertilizerConfig) (*Fertilizer, error) {
	if api == nil {
		return nil, ErrFertilizerAPIRequired
	}
	if config.SmartThreshold <= 0 {
		config.SmartThreshold = time.Hour
	}
	if config.BatchDelay < 0 {
		config.BatchDelay = 0
	}
	if config.BuyInterval <= 0 {
		config.BuyInterval = 10 * time.Minute
	}
	if config.BuyJitter < 0 {
		config.BuyJitter = -config.BuyJitter
	}
	return &Fertilizer{api: api, config: config, now: time.Now}, nil
}

// CheckAndBuy runs the optional mall adapter once. A nil adapter is a valid
// configuration because fertilizer purchasing is account-policy dependent.
func (f *Fertilizer) CheckAndBuy(ctx context.Context) error {
	if f == nil || f.config.BuyCheck == nil {
		return nil
	}
	if err := contextError(ctx); err != nil {
		return err
	}
	return f.config.BuyCheck(ctx)
}

func (f *Fertilizer) BuyCheckConfigured() bool {
	return f != nil && f.config.BuyCheck != nil
}

func (f *Fertilizer) SetNow(now func() time.Time) {
	if f != nil && now != nil {
		f.now = now
	}
}

// Mode reports the normalized policy selected for this account.
func (f *Fertilizer) Mode() FertilizerMode {
	if f == nil {
		return FertilizerNone
	}
	return normalizeFertilizerMode(f.config.Mode)
}

// IsSmartMode reports whether the policy is intended for the periodic farm
// pass rather than an explicit fertilize command.
func (f *Fertilizer) IsSmartMode() bool {
	return f != nil && isSmartMode(f.config.Mode)
}

func AllFertilizerLandTypes() []LandType {
	return append([]LandType(nil), allFertilizerLandTypes...)
}

func NormalizeFertilizerLandTypes(types []LandType) []LandType {
	if types == nil {
		return AllFertilizerLandTypes()
	}
	seen := make(map[LandType]struct{}, len(types))
	result := make([]LandType, 0, len(types))
	for _, value := range types {
		normalized := LandType(strings.ToLower(strings.TrimSpace(string(value))))
		if !containsLandType(normalized) {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	return result
}

func FilterLandIDsByTypes(landIDs []int64, landTypes map[int64]LandType, allowed []LandType) []int64 {
	types := NormalizeFertilizerLandTypes(allowed)
	if len(types) == 0 {
		return nil
	}
	if len(types) == len(allFertilizerLandTypes) {
		return uniquePositive(landIDs)
	}
	set := make(map[LandType]struct{}, len(types))
	for _, value := range types {
		set[value] = struct{}{}
	}
	result := make([]int64, 0, len(landIDs))
	for _, id := range uniquePositive(landIDs) {
		if _, ok := set[landTypes[id]]; ok {
			result = append(result, id)
		}
	}
	return result
}

func FertilizerLandTypeLabels(types []LandType) []string {
	labels := map[LandType]string{
		LandTypePurple: "紫土地",
		LandTypeGold:   "金土地",
		LandTypeBlack:  "黑土地",
		LandTypeRed:    "红土地",
		LandTypeNormal: "普通土地",
	}
	result := make([]string, 0)
	for _, value := range NormalizeFertilizerLandTypes(types) {
		if label := labels[value]; label != "" {
			result = append(result, label)
		}
	}
	return result
}

func GetOrganicFertilizerTargetsFromLands(lands []*pb.LandInfo, now ...time.Time) []int64 {
	result := make([]int64, 0)
	for _, land := range lands {
		if land == nil || !land.GetUnlocked() || land.GetId() <= 0 || !HasPlantData(land) {
			continue
		}
		if phase := CurrentPhase(land.GetPlant().GetPhases(), now...); phase == nil || pb.PlantPhase(phase.GetPhase()) == pb.PlantPhase_DEAD {
			continue
		}
		if land.GetPlant().GetLeftInorcFertTimes() <= 0 && hasExplicitFertilizerCount(land.GetPlant()) {
			continue
		}
		result = append(result, land.GetId())
	}
	return uniquePositive(result)
}

func GetFastMatureLands(lands []*pb.LandInfo, threshold time.Duration, now ...time.Time) []int64 {
	if threshold <= 0 {
		threshold = time.Hour
	}
	clock := firstNow(now)
	result := make([]int64, 0)
	for _, land := range lands {
		if land == nil || !land.GetUnlocked() || land.GetId() <= 0 || !HasPlantData(land) {
			continue
		}
		phase := CurrentPhase(land.GetPlant().GetPhases(), clock)
		if phase == nil || phase.GetPhase() == int32(pb.PlantPhase_DEAD) || phase.GetPhase() == int32(pb.PlantPhase_MATURE) {
			continue
		}
		mature := maturePhase(land.GetPlant().GetPhases())
		if mature == nil {
			continue
		}
		remaining := phaseEpochSeconds(mature.GetBeginTime()) - epochSeconds(clock)
		if remaining >= 0 && time.Duration(remaining)*time.Second <= threshold {
			if land.GetPlant().GetLeftInorcFertTimes() > 0 || !hasExplicitFertilizerCount(land.GetPlant()) {
				result = append(result, land.GetId())
			}
		}
	}
	return uniquePositive(result)
}

func GetFinalStageLands(lands []*pb.LandInfo, organicOnly bool, now ...time.Time) []int64 {
	result := make([]int64, 0)
	for _, land := range lands {
		if land == nil || !land.GetUnlocked() || land.GetId() <= 0 || !HasPlantData(land) {
			continue
		}
		phases := land.GetPlant().GetPhases()
		current := CurrentPhase(phases, now...)
		if current == nil || current.GetPhase() == int32(pb.PlantPhase_DEAD) || current.GetPhase() == int32(pb.PlantPhase_MATURE) {
			continue
		}
		ordered := make([]phaseWithTime, 0, len(phases))
		for index, phase := range phases {
			if phase == nil || phaseEpochSeconds(phase.GetBeginTime()) <= 0 {
				continue
			}
			ordered = append(ordered, phaseWithTime{phase: phase, index: index, begin: phaseEpochSeconds(phase.GetBeginTime())})
		}
		for i := 1; i < len(ordered); i++ {
			for j := i; j > 0 && (ordered[j].begin < ordered[j-1].begin || (ordered[j].begin == ordered[j-1].begin && ordered[j].index < ordered[j-1].index)); j-- {
				ordered[j], ordered[j-1] = ordered[j-1], ordered[j]
			}
		}
		matureIndex := -1
		currentIndex := -1
		for index, item := range ordered {
			if item.phase.GetPhase() == int32(pb.PlantPhase_MATURE) && matureIndex < 0 {
				matureIndex = index
			}
			if item.phase == current {
				currentIndex = index
			}
		}
		if matureIndex <= 0 || currentIndex != matureIndex-1 {
			continue
		}
		if organicOnly && land.GetPlant().GetLeftInorcFertTimes() <= 0 && hasExplicitFertilizerCount(land.GetPlant()) {
			continue
		}
		result = append(result, land.GetId())
	}
	return uniquePositive(result)
}

// Run applies the configured policy. Normal application stops on the first
// failed RPC; organic application treats a gateway rejection as exhaustion.
func (f *Fertilizer) Run(ctx context.Context, explicitLandIDs []int64, options FertilizerRunOptions) (FertilizerResult, error) {
	if f == nil || f.api == nil {
		return FertilizerResult{}, ErrFertilizerAPIRequired
	}
	mode := normalizeFertilizerMode(f.config.Mode)
	if mode == FertilizerNone {
		return FertilizerResult{}, nil
	}
	if options.Reason == "multi_season" && mode == FertilizerFinalNormal {
		return FertilizerResult{}, nil
	}
	explicit := uniquePositive(explicitLandIDs)
	allowed := NormalizeFertilizerLandTypes(f.config.LandTypes)
	if len(allowed) == 0 {
		return FertilizerResult{}, nil
	}
	clock := firstNow(nil)
	if f.now != nil {
		clock = f.now()
	}

	lands, landTypes, err := f.loadLands(ctx, explicit, allowed)
	if err != nil {
		return FertilizerResult{}, err
	}
	if len(landTypes) > 0 {
		explicit = FilterLandIDsByTypes(explicit, landTypes, allowed)
	}
	result := FertilizerResult{}
	if !options.SkipNormal && (mode == FertilizerNormal || mode == FertilizerBoth || mode == FertilizerSmart) && len(explicit) > 0 {
		result.Normal, err = f.applyNormal(ctx, explicit)
		if err != nil {
			return result, err
		}
	}

	if mode == FertilizerOrganic || mode == FertilizerBoth {
		targets := explicit
		if len(lands) > 0 {
			targets = GetOrganicFertilizerTargetsFromLands(lands, clock)
		}
		if len(landTypes) > 0 {
			targets = FilterLandIDsByTypes(targets, landTypes, allowed)
		}
		result.Organic, err = f.applyOrganic(ctx, targets)
		if err != nil {
			return result, err
		}
	}

	if mode == FertilizerSmartOnly || mode == FertilizerSmartNormal || mode == FertilizerFinalNormal || mode == FertilizerFinalOrganic {
		targets := []int64(nil)
		organic := mode == FertilizerSmartOnly || mode == FertilizerFinalOrganic
		if mode == FertilizerFinalNormal || mode == FertilizerFinalOrganic {
			targets = GetFinalStageLands(lands, organic, clock)
		} else {
			targets = GetFastMatureLands(lands, f.config.SmartThreshold, clock)
		}
		if len(explicit) > 0 {
			targets = intersectIDs(targets, explicit)
		}
		if len(landTypes) > 0 {
			targets = FilterLandIDsByTypes(targets, landTypes, allowed)
		}
		if organic {
			result.Organic, err = f.applyOrganic(ctx, targets)
		} else if !options.SkipNormal {
			result.Normal, err = f.applyNormal(ctx, targets)
		}
		if err != nil {
			return result, err
		}
	}
	return result, nil
}

func (f *Fertilizer) applyNormal(ctx context.Context, ids []int64) (int, error) {
	count := 0
	for _, id := range uniquePositive(ids) {
		if err := contextError(ctx); err != nil {
			return count, err
		}
		if _, err := f.api.Fertilize(ctx, []int64{id}, NormalFertilizerID); err != nil {
			return count, err
		}
		count++
		if err := f.wait(ctx); err != nil {
			return count, err
		}
	}
	if count > 0 && f.config.OnApplied != nil {
		f.config.OnApplied(FertilizerNormal, count)
	}
	return count, nil
}

func (f *Fertilizer) applyOrganic(ctx context.Context, ids []int64) (int, error) {
	ids = uniquePositive(ids)
	if len(ids) == 0 {
		return 0, nil
	}
	limit := f.config.OrganicMaxApplications
	if limit <= 0 {
		limit = len(ids) * 100
	}
	count := 0
	for index := 0; count < limit; index++ {
		if err := contextError(ctx); err != nil {
			return count, err
		}
		id := ids[index%len(ids)]
		if _, err := f.api.Fertilize(ctx, []int64{id}, OrganicFertilizerID); err != nil {
			break
		}
		count++
		if err := f.wait(ctx); err != nil {
			return count, err
		}
	}
	if count > 0 && f.config.OnApplied != nil {
		f.config.OnApplied(FertilizerOrganic, count)
	}
	return count, nil
}

func (f *Fertilizer) wait(ctx context.Context) error {
	if f.config.BatchDelay <= 0 {
		return nil
	}
	timer := time.NewTimer(f.config.BatchDelay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (f *Fertilizer) loadLands(ctx context.Context, explicit []int64, allowed []LandType) ([]*pb.LandInfo, map[int64]LandType, error) {
	needsLands := len(explicit) == 0 || len(allowed) != len(allFertilizerLandTypes) || isSmartMode(f.config.Mode)
	if !needsLands {
		return nil, nil, nil
	}
	reply, err := f.api.GetAllLands(ctx)
	if err != nil {
		if len(allowed) != len(allFertilizerLandTypes) || len(explicit) == 0 || isSmartMode(f.config.Mode) {
			return nil, nil, err
		}
		return nil, nil, nil
	}
	lands := reply.GetLands()
	types := make(map[int64]LandType, len(lands))
	for _, land := range lands {
		if land != nil && land.GetId() > 0 {
			types[land.GetId()] = LandTypeByLevel(land.GetLevel())
		}
	}
	return lands, types, nil
}

func normalizeFertilizerMode(mode FertilizerMode) FertilizerMode {
	switch FertilizerMode(strings.ToLower(strings.TrimSpace(string(mode)))) {
	case FertilizerNormal, FertilizerOrganic, FertilizerBoth, FertilizerSmart, FertilizerSmartOnly, FertilizerSmartNormal, FertilizerFinalNormal, FertilizerFinalOrganic:
		return FertilizerMode(strings.ToLower(strings.TrimSpace(string(mode))))
	default:
		return FertilizerNone
	}
}

func isSmartMode(mode FertilizerMode) bool {
	switch normalizeFertilizerMode(mode) {
	case FertilizerSmart, FertilizerSmartOnly, FertilizerSmartNormal, FertilizerFinalNormal, FertilizerFinalOrganic:
		return true
	default:
		return false
	}
}

func containsLandType(target LandType) bool {
	for _, value := range allFertilizerLandTypes {
		if target == value {
			return true
		}
	}
	return false
}

func hasExplicitFertilizerCount(plant *pb.PlantInfo) bool {
	return plant != nil && plant.GetLeftInorcFertTimes() != 0
}

type phaseWithTime struct {
	phase *pb.PlantPhaseInfo
	index int
	begin int64
}

func intersectIDs(left, right []int64) []int64 {
	set := make(map[int64]struct{}, len(right))
	for _, id := range right {
		set[id] = struct{}{}
	}
	result := make([]int64, 0, len(left))
	for _, id := range uniquePositive(left) {
		if _, ok := set[id]; ok {
			result = append(result, id)
		}
	}
	return result
}
