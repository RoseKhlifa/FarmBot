package farm

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/RoseKhlifa/FarmBot/internal/domain/warehouse"
	"github.com/RoseKhlifa/FarmBot/internal/game/pb"
)

const (
	FarmColumns = 4
	FarmRows    = 6
)

type PlantingStrategy string

const (
	StrategyPreferred     PlantingStrategy = "preferred"
	StrategyLevel         PlantingStrategy = "level"
	StrategyMaxExp        PlantingStrategy = "max_exp"
	StrategyMaxFertilized PlantingStrategy = "max_fert_exp"
	StrategyMaxProfit     PlantingStrategy = "max_profit"
	StrategyMaxFertProfit PlantingStrategy = "max_fert_profit"
	StrategyBagPriority   PlantingStrategy = "bag_priority"
)

var ErrPlanterAPIRequired = errors.New("planting API is required")

type Seed struct {
	SeedID        int64
	Name          string
	RequiredLevel int64
	Price         int64
	Count         int64
	PlantSize     int64
	GrowTime      int64
	GoodsID       int64
	Unlocked      bool
	Bought        int64
	Limit         int64
}

type PlantingConfig struct {
	Strategy             PlantingStrategy
	PreferredSeedID      int64
	PrioritySeedIDs      []int64
	UserLevel            int64
	BagPriorityLandTypes []LandType
	FallbackStrategy     PlantingStrategy
	Prioritize2x2        bool
	BatchDelay           time.Duration
}

type PlantingResult struct {
	PlantedLandIDs  []int64
	OccupiedLandIDs []int64
	PlantedCount    int
	OccupiedCount   int
	RemovedCount    int
	ReservedLandIDs []int64
}

type TwoByTwoGroup struct {
	Key          string
	MasterLandID int64
	LandIDs      []int64
}

type Planter struct {
	api       API
	warehouse warehouse.Service
	catalog   Catalog
	analytics *Analytics
	config    PlantingConfig

	mu                sync.Mutex
	reservedGroupKeys []string
	failedRetries     map[string]time.Time
}

func NewPlanter(api API, bag warehouse.Service, catalog Catalog, config PlantingConfig) (*Planter, error) {
	if api == nil {
		return nil, ErrPlanterAPIRequired
	}
	if config.BatchDelay < 0 {
		config.BatchDelay = 0
	}
	return &Planter{
		api:           api,
		warehouse:     bag,
		catalog:       catalog,
		analytics:     NewAnalytics(catalog),
		config:        config,
		failedRetries: make(map[string]time.Time),
	}, nil
}

func EncodePlantRequest(seedID int64, landIDs []int64, autoSlave bool) (*pb.PlantRequest, error) {
	if seedID <= 0 {
		return nil, ErrInvalidSeedID
	}
	ids, err := cleanIDs(landIDs)
	if err != nil {
		return nil, err
	}
	return &pb.PlantRequest{Items: []*pb.PlantItem{{SeedId: seedID, LandIds: ids, AutoSlave: autoSlave}}}, nil
}

func PlantingStrategyLabel(strategy PlantingStrategy) string {
	labels := map[PlantingStrategy]string{
		StrategyPreferred:     "优先种植种子",
		StrategyLevel:         "最高等级作物",
		StrategyMaxExp:        "最大经验/时",
		StrategyMaxFertilized: "最大普通肥经验/时",
		StrategyMaxProfit:     "最大净利润/时",
		StrategyMaxFertProfit: "最大普通肥净利润/时",
		StrategyBagPriority:   "背包种子优先",
	}
	if label := labels[strategy]; label != "" {
		return label
	}
	return string(strategy)
}

func Build2x2LandGroups(lands []*pb.LandInfo) []TwoByTwoGroup {
	unlocked := make(map[int64]struct{})
	for _, land := range lands {
		if land != nil && land.GetUnlocked() && land.GetId() > 0 {
			unlocked[land.GetId()] = struct{}{}
		}
	}
	groups := make([]TwoByTwoGroup, 0)
	for row := 1; row < FarmRows; row++ {
		for column := 0; column < FarmColumns-1; column++ {
			master := int64(row*FarmColumns + column + 1)
			ids := []int64{master, master + 1, master - FarmColumns, master - FarmColumns + 1}
			if !allPresent(unlocked, ids) {
				continue
			}
			groups = append(groups, TwoByTwoGroup{Key: groupKey(ids), MasterLandID: master, LandIDs: ids})
		}
	}
	return groups
}

func GetActive2x2Footprints(lands []*pb.LandInfo) []map[int64]struct{} {
	result := make([]map[int64]struct{}, 0)
	for _, land := range lands {
		if land == nil || len(land.GetSlaveLandIds()) != 3 {
			continue
		}
		ids := append([]int64{land.GetId()}, land.GetSlaveLandIds()...)
		set := make(map[int64]struct{}, len(ids))
		for _, id := range ids {
			if id > 0 {
				set[id] = struct{}{}
			}
		}
		if len(set) == 4 {
			result = append(result, set)
		}
	}
	return result
}

func SelectMaximumNonOverlappingGroups(groups []TwoByTwoGroup, limit int) []TwoByTwoGroup {
	if limit <= 0 || len(groups) == 0 {
		return nil
	}
	candidates := append([]TwoByTwoGroup(nil), groups...)
	sort.SliceStable(candidates, func(i, j int) bool { return candidates[i].MasterLandID < candidates[j].MasterLandID })
	best := make([]TwoByTwoGroup, 0)
	var search func(int, []TwoByTwoGroup, map[int64]struct{})
	search = func(index int, selected []TwoByTwoGroup, occupied map[int64]struct{}) {
		if len(selected) > len(best) {
			best = append([]TwoByTwoGroup(nil), selected...)
		}
		if len(selected) >= limit || index >= len(candidates) || len(selected)+len(candidates)-index <= len(best) {
			return
		}
		candidate := candidates[index]
		if !overlaps(candidate.LandIDs, occupied) {
			next := cloneSet(occupied)
			for _, id := range candidate.LandIDs {
				next[id] = struct{}{}
			}
			search(index+1, append(selected, candidate), next)
		}
		search(index+1, selected, occupied)
	}
	search(0, nil, make(map[int64]struct{}))
	return best
}

func Select2x2Reservations(groups []TwoByTwoGroup, emptyLandIDs []int64, desired int, lands []*pb.LandInfo, now ...time.Time) []TwoByTwoGroup {
	empty := make(map[int64]struct{}, len(emptyLandIDs))
	for _, id := range uniquePositive(emptyLandIDs) {
		empty[id] = struct{}{}
	}
	active := GetActive2x2Footprints(lands)
	candidates := make([]TwoByTwoGroup, 0, len(groups))
	for _, group := range groups {
		blocked := false
		for _, footprint := range active {
			if overlapsSet(group.LandIDs, footprint) {
				blocked = true
				break
			}
		}
		if !blocked {
			candidates = append(candidates, group)
		}
	}
	ready := make([]TwoByTwoGroup, 0)
	waiting := make([]TwoByTwoGroup, 0)
	for _, group := range candidates {
		if allEmpty(group.LandIDs, empty) {
			ready = append(ready, group)
		} else {
			waiting = append(waiting, group)
		}
	}
	selected := SelectMaximumNonOverlappingGroups(ready, desired)
	occupied := make(map[int64]struct{})
	for _, group := range selected {
		for _, id := range group.LandIDs {
			occupied[id] = struct{}{}
		}
	}
	if len(selected) < desired {
		landMap := BuildLandMap(lands)
		sort.SliceStable(waiting, func(i, j int) bool {
			left, right := clearAt(waiting[i], landMap, empty, now...), clearAt(waiting[j], landMap, empty, now...)
			if left != right {
				return left < right
			}
			return waiting[i].MasterLandID < waiting[j].MasterLandID
		})
		for _, group := range waiting {
			if overlaps(group.LandIDs, occupied) {
				continue
			}
			selected = append(selected, group)
			for _, id := range group.LandIDs {
				occupied[id] = struct{}{}
			}
			break // retain at most one not-yet-empty reservation
		}
	}
	return selected
}

func ExpandRemoved2x2Lands(emptyLandIDs, removedLandIDs []int64, lands []*pb.LandInfo) []int64 {
	result := make(map[int64]struct{})
	for _, id := range emptyLandIDs {
		if id > 0 {
			result[id] = struct{}{}
		}
	}
	landMap := BuildLandMap(lands)
	for _, id := range uniquePositive(removedLandIDs) {
		result[id] = struct{}{}
		for _, slave := range landMap[id].GetSlaveLandIds() {
			if slave > 0 {
				result[slave] = struct{}{}
			}
		}
	}
	ids := make([]int64, 0, len(result))
	for id := range result {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func SortBagSeedsForPlanting(seeds []Seed, priority []int64) []Seed {
	priorityMap := make(map[int64]int, len(priority))
	for index, id := range priority {
		if id > 0 {
			priorityMap[id] = index
		}
	}
	result := append([]Seed(nil), seeds...)
	sort.SliceStable(result, func(i, j int) bool {
		left, leftOK := priorityMap[result[i].SeedID]
		right, rightOK := priorityMap[result[j].SeedID]
		if leftOK != rightOK {
			return leftOK
		}
		if leftOK && left != right {
			return left < right
		}
		if result[i].RequiredLevel != result[j].RequiredLevel {
			return result[i].RequiredLevel > result[j].RequiredLevel
		}
		return result[i].SeedID < result[j].SeedID
	})
	return result
}

func SelectSeed(candidates []Seed, config PlantingConfig, analytics *Analytics) (Seed, bool) {
	available := make([]Seed, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.SeedID <= 0 || candidate.Count < 0 || (candidate.RequiredLevel > 0 && candidate.RequiredLevel > config.UserLevel) || candidate.Unlocked == false && candidate.GoodsID > 0 {
			continue
		}
		if candidate.PlantSize <= 0 {
			candidate.PlantSize = 1
		}
		available = append(available, candidate)
	}
	if len(available) == 0 {
		return Seed{}, false
	}
	strategy := config.Strategy
	if strategy == "" {
		strategy = StrategyLevel
	}
	if strategy == StrategyPreferred && config.PreferredSeedID > 0 {
		for _, candidate := range available {
			if candidate.SeedID == config.PreferredSeedID {
				return candidate, true
			}
		}
	}
	if analytics != nil {
		sortBy := map[PlantingStrategy]string{
			StrategyMaxExp:        "exp",
			StrategyMaxFertilized: "fert",
			StrategyMaxProfit:     "profit",
			StrategyMaxFertProfit: "fert_profit",
		}[strategy]
		if sortBy != "" {
			rankings := analytics.GetPlantRankings(sortBy)
			bySeed := make(map[int64]Seed, len(available))
			for _, candidate := range available {
				bySeed[candidate.SeedID] = candidate
			}
			for _, ranking := range rankings {
				if candidate, ok := bySeed[ranking.SeedID]; ok {
					return candidate, true
				}
			}
		}
	}
	sort.SliceStable(available, func(i, j int) bool {
		if available[i].RequiredLevel != available[j].RequiredLevel {
			return available[i].RequiredLevel > available[j].RequiredLevel
		}
		return available[i].SeedID < available[j].SeedID
	})
	return available[0], true
}

func (p *Planter) PlantSeeds(ctx context.Context, seedID int64, landIDs []int64, options ...bool) (PlantingResult, error) {
	if p == nil || p.api == nil {
		return PlantingResult{}, ErrPlanterAPIRequired
	}
	autoSlave := false
	if len(options) > 0 {
		autoSlave = options[0]
	}
	ids := uniquePositive(landIDs)
	if seedID <= 0 || len(ids) == 0 {
		return PlantingResult{}, fmt.Errorf("plant requires a seed and at least one land")
	}
	result := PlantingResult{}
	for _, landID := range ids {
		if err := contextError(ctx); err != nil {
			return result, err
		}
		reply, err := p.api.Plant(ctx, seedID, []int64{landID}, autoSlave)
		if err != nil {
			continue
		}
		result.PlantedCount++
		landMap := BuildLandMap(reply.GetLand())
		land := landMap[landID]
		occupied := []int64{landID}
		master := landID
		if land != nil {
			ctx := GetDisplayLandContext(land, landMap)
			if len(ctx.OccupiedLandIDs) > 0 {
				occupied = ctx.OccupiedLandIDs
			}
			if ctx.MasterLandID > 0 {
				master = ctx.MasterLandID
			}
		}
		result.PlantedLandIDs = append(result.PlantedLandIDs, master)
		result.OccupiedLandIDs = appendUnique(result.OccupiedLandIDs, occupied...)
		if err := p.wait(ctx); err != nil {
			return result, err
		}
	}
	result.OccupiedCount = len(uniquePositive(result.OccupiedLandIDs))
	return result, nil
}

func (p *Planter) Plant2x2Seed(ctx context.Context, seedID int64, group TwoByTwoGroup) (PlantingResult, error) {
	if p == nil || p.api == nil {
		return PlantingResult{}, ErrPlanterAPIRequired
	}
	reply, err := p.api.Plant(ctx, seedID, group.LandIDs, true)
	if err != nil {
		return PlantingResult{}, err
	}
	landMap := BuildLandMap(reply.GetLand())
	master := landMap[group.MasterLandID]
	if master == nil || master.GetLandSize() != 2 || len(master.GetSlaveLandIds()) != 3 {
		return PlantingResult{}, fmt.Errorf("server did not confirm 2x2 land group %s", group.Key)
	}
	return PlantingResult{PlantedLandIDs: []int64{group.MasterLandID}, OccupiedLandIDs: append([]int64(nil), group.LandIDs...), PlantedCount: 1, OccupiedCount: len(group.LandIDs)}, nil
}

func (p *Planter) AutoPlantEmptyLands(ctx context.Context, deadLandIDs, emptyLandIDs []int64, lands []*pb.LandInfo) (PlantingResult, error) {
	if p == nil || p.api == nil {
		return PlantingResult{}, ErrPlanterAPIRequired
	}
	result := PlantingResult{}
	dead := uniquePositive(deadLandIDs)
	if len(dead) > 0 {
		if _, err := p.api.RemovePlant(ctx, dead); err != nil {
			return result, err
		}
		result.RemovedCount = len(dead)
	}
	available := ExpandRemoved2x2Lands(emptyLandIDs, dead, lands)
	if p.config.Prioritize2x2 {
		groups := Build2x2LandGroups(lands)
		reservations := Select2x2Reservations(groups, available, len(available)/4, lands)
		p.mu.Lock()
		p.reservedGroupKeys = make([]string, 0)
		for _, group := range reservations {
			if !allEmpty(group.LandIDs, toSet(available)) {
				p.reservedGroupKeys = append(p.reservedGroupKeys, group.Key)
				result.ReservedLandIDs = append(result.ReservedLandIDs, group.LandIDs...)
				continue
			}
			p.mu.Unlock()
			seed, ok := p.bestBagSeed(2)
			if ok {
				planted, err := p.Plant2x2Seed(ctx, seed.SeedID, group)
				if err == nil {
					result = mergePlantingResults(result, planted)
				}
			}
			p.mu.Lock()
		}
		p.mu.Unlock()
	}
	occupied := toSet(result.OccupiedLandIDs)
	remaining := make([]int64, 0, len(available))
	for _, id := range available {
		if _, ok := occupied[id]; !ok {
			remaining = append(remaining, id)
		}
	}
	if len(remaining) == 0 {
		return result, nil
	}
	seed, ok, err := p.findBestSeed(ctx)
	if err != nil || !ok {
		return result, err
	}
	if seed.GoodsID > 0 {
		count := int64(len(remaining))
		if seed.PlantSize > 1 {
			count /= seed.PlantSize * seed.PlantSize
		}
		if count <= 0 {
			return result, nil
		}
		if _, err := p.api.BuyGoods(ctx, seed.GoodsID, count, seed.Price); err != nil {
			return result, err
		}
	}
	planted, err := p.PlantSeeds(ctx, seed.SeedID, remaining)
	if err != nil {
		return result, err
	}
	return mergePlantingResults(result, planted), nil
}

func (p *Planter) findBestSeed(ctx context.Context) (Seed, bool, error) {
	if p.api != nil {
		if info, err := p.api.GetShopInfo(ctx, SeedShopID); err == nil {
			candidates := p.shopCandidates(info.GetGoodsList())
			if seed, ok := SelectSeed(candidates, p.config, p.analytics); ok {
				return seed, true, nil
			}
		}
	}
	seed, ok := p.bestBagSeed(1)
	return seed, ok, nil
}

func (p *Planter) bestBagSeed(size int64) (Seed, bool) {
	if p == nil || p.warehouse == nil || p.catalog == nil {
		return Seed{}, false
	}
	bag, err := p.warehouse.ListBag(context.Background())
	if err != nil {
		return Seed{}, false
	}
	seeds := make([]Seed, 0)
	for _, item := range bag.Items {
		if item.ID <= 0 || item.Count <= 0 {
			continue
		}
		record, ok := p.catalog.PlantBySeedID(item.ID)
		if !ok || maxInt64(1, record.PlantSize) != size {
			continue
		}
		seeds = append(seeds, Seed{SeedID: item.ID, Name: record.Name, Count: item.Count, RequiredLevel: record.RequiredLevel, Price: record.Price, PlantSize: maxInt64(1, record.PlantSize), Unlocked: true})
	}
	seeds = SortBagSeedsForPlanting(seeds, p.config.PrioritySeedIDs)
	return SelectSeed(seeds, p.config, p.analytics)
}

func (p *Planter) shopCandidates(goods []*pb.GoodsInfo) []Seed {
	result := make([]Seed, 0, len(goods))
	for _, item := range goods {
		if item == nil || item.GetItemId() <= 0 || !item.GetUnlocked() || (item.GetLimitCount() > 0 && item.GetBoughtNum() >= item.GetLimitCount()) {
			continue
		}
		seed := Seed{SeedID: item.GetItemId(), GoodsID: item.GetId(), Price: item.GetPrice(), Bought: item.GetBoughtNum(), Limit: item.GetLimitCount(), Unlocked: true, PlantSize: 1}
		if p.catalog != nil {
			if record, ok := p.catalog.PlantBySeedID(seed.SeedID); ok {
				seed.Name, seed.RequiredLevel, seed.PlantSize = record.Name, record.RequiredLevel, maxInt64(1, record.PlantSize)
			}
		}
		if seed.RequiredLevel <= p.config.UserLevel && seed.PlantSize == 1 {
			result = append(result, seed)
		}
	}
	return result
}

func (p *Planter) wait(ctx context.Context) error {
	if p.config.BatchDelay <= 0 {
		return nil
	}
	timer := time.NewTimer(p.config.BatchDelay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func mergePlantingResults(left, right PlantingResult) PlantingResult {
	left.PlantedLandIDs = appendUnique(left.PlantedLandIDs, right.PlantedLandIDs...)
	left.OccupiedLandIDs = appendUnique(left.OccupiedLandIDs, right.OccupiedLandIDs...)
	left.ReservedLandIDs = appendUnique(left.ReservedLandIDs, right.ReservedLandIDs...)
	left.PlantedCount += right.PlantedCount
	left.RemovedCount += right.RemovedCount
	left.OccupiedCount = len(uniquePositive(left.OccupiedLandIDs))
	return left
}

func groupKey(ids []int64) string {
	values := append([]int64(nil), ids...)
	sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
	parts := make([]string, len(values))
	for i, value := range values {
		parts[i] = fmt.Sprint(value)
	}
	return strings.Join(parts, "-")
}

func allPresent(set map[int64]struct{}, values []int64) bool {
	for _, value := range values {
		if _, ok := set[value]; !ok {
			return false
		}
	}
	return true
}

func allEmpty(values []int64, set map[int64]struct{}) bool { return allPresent(set, values) }

func overlaps(values []int64, set map[int64]struct{}) bool {
	for _, value := range values {
		if _, ok := set[value]; ok {
			return true
		}
	}
	return false
}

func overlapsSet(values []int64, set map[int64]struct{}) bool { return overlaps(values, set) }

func cloneSet(input map[int64]struct{}) map[int64]struct{} {
	result := make(map[int64]struct{}, len(input))
	for key := range input {
		result[key] = struct{}{}
	}
	return result
}

func clearAt(group TwoByTwoGroup, landMap map[int64]*pb.LandInfo, empty map[int64]struct{}, now ...time.Time) int64 {
	latest := int64(0)
	clock := firstNow(now)
	for _, id := range group.LandIDs {
		if _, ok := empty[id]; ok {
			continue
		}
		land := landMap[id]
		if land == nil || land.GetPlant() == nil {
			continue
		}
		mature := maturePhase(land.GetPlant().GetPhases())
		at := int64(0)
		if mature != nil {
			at = phaseEpochSeconds(mature.GetBeginTime())
		}
		if at == 0 {
			at = int64(^uint64(0) >> 1)
		}
		if at < epochSeconds(clock) {
			at = epochSeconds(clock)
		}
		if at > latest {
			latest = at
		}
	}
	return latest
}

func toSet(ids []int64) map[int64]struct{} {
	result := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if id > 0 {
			result[id] = struct{}{}
		}
	}
	return result
}

func appendUnique(target []int64, values ...int64) []int64 {
	seen := toSet(target)
	for _, value := range values {
		if value > 0 {
			if _, ok := seen[value]; !ok {
				seen[value] = struct{}{}
				target = append(target, value)
			}
		}
	}
	return target
}
