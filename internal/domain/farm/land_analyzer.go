package farm

import (
	"context"
	"sort"
	"time"

	"github.com/RoseKhlifa/FarmBot/internal/game/pb"
)

const (
	GoldenBugItemID     int64 = 301101
	GoldenBugSocialType int64 = 2
)

// LandAnalysis is the action-oriented snapshot consumed by the orchestrator.
// IDs are de-duplicated and slave plots occupied by a planted master are not
// returned as independent actions.
type LandAnalysis struct {
	Harvestable     []int64
	NeedWater       []int64
	NeedWeed        []int64
	NeedBug         []int64
	NeedGoldenBug   []int64
	Growing         []int64
	Empty           []int64
	Dead            []int64
	Unlockable      []int64
	Upgradable      []int64
	HarvestableInfo []HarvestableInfo
}

type HarvestableInfo struct {
	LandID  int64
	PlantID int64
	Name    string
	Exp     int64
}

type LandSummary struct {
	Harvestable int
	Growing     int
	Empty       int
	Dead        int
	NeedWater   int
	NeedWeed    int
	NeedBug     int
}

type LandType string

const (
	LandTypeNormal LandType = "normal"
	LandTypeRed    LandType = "red"
	LandTypeBlack  LandType = "black"
	LandTypeGold   LandType = "gold"
	LandTypePurple LandType = "purple"
)

// LandDetail is the stable UI-facing projection of a protocol LandInfo.
type LandDetail struct {
	ID               int64
	Unlocked         bool
	Status           string
	PlantName        string
	PlantID          int64
	SeedID           int64
	Phase            pb.PlantPhase
	PhaseName        string
	CurrentSeason    int64
	TotalSeason      int64
	MatureIn         time.Duration
	NeedWater        bool
	NeedWeed         bool
	NeedBug          bool
	Stealable        bool
	Level            int64
	MaxLevel         int64
	LandsLevel       int64
	LandSize         int64
	LandType         LandType
	CouldUnlock      bool
	CouldUpgrade     bool
	OccupiedByMaster bool
	MasterLandID     int64
	OccupiedLandIDs  []int64
	PlantSize        int64
}

type DisplayLandContext struct {
	SourceLand       *pb.LandInfo
	OccupiedByMaster bool
	MasterLandID     int64
	OccupiedLandIDs  []int64
}

// CurrentPhase returns the last phase whose begin time is in the past. When
// all begin times are in the future or absent, it returns the first phase,
// matching the Node implementation's conservative fallback.
func CurrentPhase(phases []*pb.PlantPhaseInfo, now ...time.Time) *pb.PlantPhaseInfo {
	if len(phases) == 0 {
		return nil
	}
	serverSeconds := epochSeconds(firstNow(now))
	for i := len(phases) - 1; i >= 0; i-- {
		if phases[i] != nil && phaseEpochSeconds(phases[i].GetBeginTime()) > 0 && phaseEpochSeconds(phases[i].GetBeginTime()) <= serverSeconds {
			return phases[i]
		}
	}
	return phases[0]
}

// GetCurrentPhase is a compatibility alias for callers migrating from the
// JavaScript helper name.
func GetCurrentPhase(phases []*pb.PlantPhaseInfo, now ...time.Time) *pb.PlantPhaseInfo {
	return CurrentPhase(phases, now...)
}

func BuildLandMap(lands []*pb.LandInfo) map[int64]*pb.LandInfo {
	result := make(map[int64]*pb.LandInfo, len(lands))
	for _, land := range lands {
		if land != nil && land.GetId() > 0 {
			result[land.GetId()] = land
		}
	}
	return result
}

func SlaveLandIDs(land *pb.LandInfo) []int64 {
	if land == nil {
		return nil
	}
	result := uniquePositive(land.GetSlaveLandIds())
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}

func HasPlantData(land *pb.LandInfo) bool {
	return land != nil && land.GetPlant() != nil && len(land.GetPlant().GetPhases()) > 0
}

func LinkedMasterLand(land *pb.LandInfo, landMap map[int64]*pb.LandInfo) *pb.LandInfo {
	if land == nil {
		return nil
	}
	landID, masterID := land.GetId(), land.GetMasterLandId()
	if masterID <= 0 || masterID == landID {
		return nil
	}
	master := landMap[masterID]
	if master == nil {
		return nil
	}
	slaves := SlaveLandIDs(master)
	if len(slaves) > 0 && !containsID(slaves, landID) {
		return nil
	}
	return master
}

func GetDisplayLandContext(land *pb.LandInfo, landMap map[int64]*pb.LandInfo) DisplayLandContext {
	if master := LinkedMasterLand(land, landMap); master != nil && HasPlantData(master) {
		ids := append([]int64{master.GetId()}, SlaveLandIDs(master)...)
		return DisplayLandContext{
			SourceLand:       master,
			OccupiedByMaster: true,
			MasterLandID:     master.GetId(),
			OccupiedLandIDs:  uniquePositive(ids),
		}
	}
	landID := int64(0)
	if land != nil {
		landID = land.GetId()
	}
	return DisplayLandContext{SourceLand: land, MasterLandID: landID, OccupiedLandIDs: uniquePositive([]int64{landID})}
}

func IsOccupiedSlaveLand(land *pb.LandInfo, landMap map[int64]*pb.LandInfo) bool {
	return GetDisplayLandContext(land, landMap).OccupiedByMaster
}

// AnalyzeLands is the pure default analyzer. A variadic time keeps production
// callers on server time while making phase-boundary tests deterministic.
func AnalyzeLands(lands []*pb.LandInfo, now ...time.Time) LandAnalysis {
	return analyzeLands(lands, firstNow(now), nil)
}

type LandAnalyzer struct {
	catalog Catalog
	api     API
	now     func() time.Time
}

func NewLandAnalyzer(catalog Catalog, api API) *LandAnalyzer {
	return &LandAnalyzer{catalog: catalog, api: api, now: time.Now}
}

func (a *LandAnalyzer) Analyze(lands []*pb.LandInfo, now ...time.Time) LandAnalysis {
	clock := firstNow(now)
	if len(now) == 0 && a != nil && a.now != nil {
		clock = a.now()
	}
	var catalog Catalog
	if a != nil {
		catalog = a.catalog
	}
	return analyzeLands(lands, clock, catalog)
}

func (a *LandAnalyzer) Summary(lands []*pb.LandInfo, now ...time.Time) LandSummary {
	return SummarizeAnalysis(a.Analyze(lands, now...))
}

// Details returns a deterministic UI projection while retaining the same
// master/slave occupancy and phase rules as Analyze.
func (a *LandAnalyzer) Details(lands []*pb.LandInfo, now ...time.Time) []LandDetail {
	clock := firstNow(now)
	if len(now) == 0 && a != nil && a.now != nil {
		clock = a.now()
	}
	var catalog Catalog
	if a != nil {
		catalog = a.catalog
	}
	landMap := BuildLandMap(lands)
	details := make([]LandDetail, 0, len(lands))
	for _, land := range lands {
		if land == nil || land.GetId() <= 0 {
			continue
		}
		ctx := GetDisplayLandContext(land, landMap)
		detail := LandDetail{
			ID:               land.GetId(),
			Unlocked:         land.GetUnlocked(),
			Level:            land.GetLevel(),
			MaxLevel:         land.GetMaxLevel(),
			LandsLevel:       land.GetLandsLevel(),
			LandSize:         land.GetLandSize(),
			LandType:         LandTypeByLevel(land.GetLevel()),
			CouldUnlock:      land.GetCouldUnlock(),
			CouldUpgrade:     land.GetCouldUpgrade(),
			OccupiedByMaster: ctx.OccupiedByMaster,
			MasterLandID:     ctx.MasterLandID,
			OccupiedLandIDs:  append([]int64(nil), ctx.OccupiedLandIDs...),
			PlantSize:        1,
		}
		if !detail.Unlocked {
			detail.Status = "locked"
			details = append(details, detail)
			continue
		}
		plant := ctx.SourceLand.GetPlant()
		if plant == nil || len(plant.GetPhases()) == 0 {
			detail.Status, detail.PhaseName = "empty", "空地"
			details = append(details, detail)
			continue
		}
		phase := CurrentPhase(plant.GetPhases(), clock)
		if phase == nil {
			detail.Status = "empty"
			details = append(details, detail)
			continue
		}
		detail.Phase = pb.PlantPhase(phase.GetPhase())
		detail.PhaseName = PhaseName(detail.Phase)
		detail.PlantID = plant.GetId()
		detail.PlantName = plant.GetName()
		if catalog != nil {
			if record, ok := catalog.PlantByID(detail.PlantID); ok {
				if record.Name != "" {
					detail.PlantName = record.Name
				}
				detail.SeedID = record.SeedID
				detail.TotalSeason = record.Seasons
				detail.PlantSize = maxInt64(1, record.PlantSize)
			}
		}
		if detail.TotalSeason <= 0 {
			detail.TotalSeason = 1
		}
		detail.CurrentSeason = maxInt64(1, plant.GetSeason())
		if detail.CurrentSeason > detail.TotalSeason {
			detail.CurrentSeason = detail.TotalSeason
		}
		mature := maturePhase(plant.GetPhases())
		if mature != nil {
			remaining := phaseEpochSeconds(mature.GetBeginTime()) - epochSeconds(clock)
			if remaining > 0 {
				detail.MatureIn = time.Duration(remaining) * time.Second
			}
		}
		detail.NeedWater = needsWater(plant, phase, clock)
		detail.NeedWeed = needsWeed(plant, phase, clock)
		detail.NeedBug = needsBug(plant, phase, clock)
		detail.Stealable = plant.GetStealable()
		switch detail.Phase {
		case pb.PlantPhase_MATURE:
			detail.Status = "harvestable"
		case pb.PlantPhase_DEAD:
			detail.Status = "dead"
		case pb.PlantPhase_PHASE_UNKNOWN:
			detail.Status = "empty"
		default:
			detail.Status = "growing"
		}
		details = append(details, detail)
	}
	return details
}

// ResolveRemovableHarvestedLands distinguishes multi-season crops from dead
// or empty plots. Unknown states are refreshed once and then conservatively
// treated as removable, matching the legacy safety fallback.
func (a *LandAnalyzer) ResolveRemovableHarvestedLands(ctx context.Context, harvested []int64, reply *pb.HarvestReply) HarvestResolution {
	ids := uniquePositive(harvested)
	if len(ids) == 0 {
		return HarvestResolution{}
	}
	removable, growing, unknown := classifyHarvestedLands(ids, BuildLandMap(reply.GetLand()))
	if len(unknown) > 0 && a != nil && a.api != nil {
		fresh, err := a.api.GetAllLands(ctx)
		if err == nil {
			var extra []int64
			removable, growing, extra = appendClassified(removable, growing, unknown, BuildLandMap(fresh.GetLands()))
			unknown = extra
		}
	}
	return HarvestResolution{
		Removable:       uniquePositive(append(removable, unknown...)),
		Growing:         uniquePositive(growing),
		FallbackRemoved: len(unknown),
	}
}

type HarvestResolution struct {
	Removable       []int64
	Growing         []int64
	FallbackRemoved int
}

func ClassifyHarvestedLands(landIDs []int64, landMap map[int64]*pb.LandInfo, now ...time.Time) (removable, growing, unknown []int64) {
	clock := firstNow(now)
	for _, id := range uniquePositive(landIDs) {
		land := landMap[id]
		state := LifecycleState(land, clock)
		switch state {
		case "dead", "empty":
			removable = append(removable, id)
		case "growing":
			growing = append(growing, id)
		default:
			unknown = append(unknown, id)
		}
	}
	return
}

func LifecycleState(land *pb.LandInfo, now ...time.Time) string {
	if land == nil || land.GetPlant() == nil || len(land.GetPlant().GetPhases()) == 0 {
		return "empty"
	}
	phase := CurrentPhase(land.GetPlant().GetPhases(), now...)
	if phase == nil || pb.PlantPhase(phase.GetPhase()) == pb.PlantPhase_PHASE_UNKNOWN {
		return "empty"
	}
	switch pb.PlantPhase(phase.GetPhase()) {
	case pb.PlantPhase_DEAD:
		return "dead"
	case pb.PlantPhase_SEED, pb.PlantPhase_GERMINATION, pb.PlantPhase_SMALL_LEAVES, pb.PlantPhase_LARGE_LEAVES, pb.PlantPhase_BLOOMING, pb.PlantPhase_MATURE:
		return "growing"
	default:
		return "unknown"
	}
}

func SummarizeLandDetails(details []LandDetail) LandSummary {
	var result LandSummary
	for _, detail := range details {
		if !detail.Unlocked {
			continue
		}
		switch detail.Status {
		case "harvestable":
			result.Harvestable++
		case "growing", "stealable", "harvested":
			result.Growing++
		case "empty":
			result.Empty++
		case "dead":
			result.Dead++
		}
		if detail.NeedWater {
			result.NeedWater++
		}
		if detail.NeedWeed {
			result.NeedWeed++
		}
		if detail.NeedBug {
			result.NeedBug++
		}
	}
	return result
}

func SummarizeAnalysis(analysis LandAnalysis) LandSummary {
	return LandSummary{
		Harvestable: len(analysis.Harvestable),
		Growing:     len(analysis.Growing),
		Empty:       len(analysis.Empty),
		Dead:        len(analysis.Dead),
		NeedWater:   len(analysis.NeedWater),
		NeedWeed:    len(analysis.NeedWeed),
		NeedBug:     len(analysis.NeedBug),
	}
}

func LandTypeByLevel(level int64) LandType {
	switch level {
	case 5:
		return LandTypePurple
	case 4:
		return LandTypeGold
	case 3:
		return LandTypeBlack
	case 2:
		return LandTypeRed
	default:
		return LandTypeNormal
	}
}

func PhaseName(phase pb.PlantPhase) string {
	switch phase {
	case pb.PlantPhase_SEED:
		return "种子"
	case pb.PlantPhase_GERMINATION:
		return "发芽"
	case pb.PlantPhase_SMALL_LEAVES:
		return "小叶"
	case pb.PlantPhase_LARGE_LEAVES:
		return "大叶"
	case pb.PlantPhase_BLOOMING:
		return "开花"
	case pb.PlantPhase_MATURE:
		return "成熟"
	case pb.PlantPhase_DEAD:
		return "枯死"
	default:
		return "未知"
	}
}

func hasGoldenBug(plant *pb.PlantInfo) bool {
	if plant == nil {
		return false
	}
	for _, item := range plant.GetSocialItems() {
		if item != nil && item.GetItemId() == GoldenBugItemID && item.GetType() == GoldenBugSocialType {
			return true
		}
	}
	return false
}

func analyzeLands(lands []*pb.LandInfo, now time.Time, catalog Catalog) LandAnalysis {
	result := LandAnalysis{}
	landMap := BuildLandMap(lands)
	for _, land := range lands {
		if land == nil || land.GetId() <= 0 {
			continue
		}
		id := land.GetId()
		if !land.GetUnlocked() {
			if land.GetCouldUnlock() {
				result.Unlockable = append(result.Unlockable, id)
			}
			continue
		}
		if land.GetCouldUpgrade() {
			result.Upgradable = append(result.Upgradable, id)
		}
		if IsOccupiedSlaveLand(land, landMap) {
			continue
		}
		plant := land.GetPlant()
		if plant == nil || len(plant.GetPhases()) == 0 {
			result.Empty = append(result.Empty, id)
			continue
		}
		if hasGoldenBug(plant) {
			result.NeedGoldenBug = append(result.NeedGoldenBug, id)
		}
		phase := CurrentPhase(plant.GetPhases(), now)
		if phase == nil {
			result.Empty = append(result.Empty, id)
			continue
		}
		switch pb.PlantPhase(phase.GetPhase()) {
		case pb.PlantPhase_DEAD:
			result.Dead = append(result.Dead, id)
		case pb.PlantPhase_MATURE:
			result.Harvestable = append(result.Harvestable, id)
			name := plant.GetName()
			exp, plantID := int64(0), plant.GetId()
			if catalog != nil {
				if record, ok := catalog.PlantByID(plantID); ok {
					if record.Name != "" {
						name = record.Name
					}
					exp = record.Exp
				}
			}
			result.HarvestableInfo = append(result.HarvestableInfo, HarvestableInfo{LandID: id, PlantID: plantID, Name: name, Exp: exp})
		default:
			if needsWater(plant, phase, now) {
				result.NeedWater = append(result.NeedWater, id)
			}
			if needsWeed(plant, phase, now) {
				result.NeedWeed = append(result.NeedWeed, id)
			}
			if needsBug(plant, phase, now) {
				result.NeedBug = append(result.NeedBug, id)
			}
			result.Growing = append(result.Growing, id)
		}
	}
	return result
}

func needsWater(plant *pb.PlantInfo, phase *pb.PlantPhaseInfo, now time.Time) bool {
	return plant != nil && (plant.GetDryNum() > 0 || dueAt(phase.GetDryTime(), now))
}

func needsWeed(plant *pb.PlantInfo, phase *pb.PlantPhaseInfo, now time.Time) bool {
	return plant != nil && (len(plant.GetWeedOwners()) > 0 || dueAt(phase.GetWeedsTime(), now))
}

func needsBug(plant *pb.PlantInfo, phase *pb.PlantPhaseInfo, now time.Time) bool {
	return plant != nil && (len(plant.GetInsectOwners()) > 0 || dueAt(phase.GetInsectTime(), now))
}

func dueAt(value int64, now time.Time) bool {
	when := phaseEpochSeconds(value)
	return when > 0 && when <= epochSeconds(now)
}

func maturePhase(phases []*pb.PlantPhaseInfo) *pb.PlantPhaseInfo {
	for _, phase := range phases {
		if phase != nil && pb.PlantPhase(phase.GetPhase()) == pb.PlantPhase_MATURE {
			return phase
		}
	}
	return nil
}

func appendClassified(removable, growing, unknown []int64, landMap map[int64]*pb.LandInfo) ([]int64, []int64, []int64) {
	moreRemovable, moreGrowing, stillUnknown := ClassifyHarvestedLands(unknown, landMap)
	return append(removable, moreRemovable...), append(growing, moreGrowing...), stillUnknown
}

func classifyHarvestedLands(ids []int64, landMap map[int64]*pb.LandInfo) ([]int64, []int64, []int64) {
	return ClassifyHarvestedLands(ids, landMap)
}

func firstNow(now []time.Time) time.Time {
	if len(now) > 0 && !now[0].IsZero() {
		return now[0]
	}
	return time.Now()
}

func epochSeconds(value time.Time) int64 { return value.Unix() }

func phaseEpochSeconds(value int64) int64 {
	if value > 1_000_000_000_000_000 {
		return value / 1_000_000
	}
	if value > 1_000_000_000_000 {
		return value / 1_000
	}
	return value
}

func uniquePositive(values []int64) []int64 {
	result := make([]int64, 0, len(values))
	seen := make(map[int64]struct{}, len(values))
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func containsID(values []int64, target int64) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func maxInt64(left, right int64) int64 {
	if left > right {
		return left
	}
	return right
}
