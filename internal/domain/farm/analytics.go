package farm

import (
	"sort"
	"strconv"
	"strings"
)

const secondsPerHour = 3600.0

// PlantRecord is the small game-config projection consumed by farm policy.
// The assets package can provide this projection later without making the
// domain depend on a JSON representation or an embed implementation.
type PlantRecord struct {
	ID            int64
	SeedID        int64
	Name          string
	GrowPhases    string
	Exp           int64
	Seasons       int64
	FruitID       int64
	FruitCount    int64
	RequiredLevel int64
	Price         int64
	Image         string
	PlantSize     int64
}

// Catalog supplies static game data to analytics and planting strategies.
// Implementations may read embedded assets, a disk override, or test data.
type Catalog interface {
	AllPlants() []PlantRecord
	PlantByID(int64) (PlantRecord, bool)
	PlantBySeedID(int64) (PlantRecord, bool)
	FruitPrice(int64) int64
	SeedPrice(int64) int64
	SeedLevel(int64) int64
}

// Ranking is the comparable representation used by seed-selection policy.
type Ranking struct {
	ID                            int64
	SeedID                        int64
	Name                          string
	Seasons                       int64
	Level                         *int64
	GrowTime                      int64
	GrowTimeText                  string
	ReduceSeconds                 int64
	ReduceSecondsApplied          int64
	ExpPerHour                    float64
	NormalFertilizerExpPerHour    float64
	GoldPerHour                   float64
	ProfitPerHour                 float64
	NormalFertilizerProfitPerHour float64
	Income                        int64
	NetProfit                     int64
	FruitID                       int64
	FruitCount                    int64
	FruitPrice                    int64
	SeedPrice                     int64
	Image                         string
}

// Analytics is a stateless ranking service. It deliberately owns no cache so
// a later asset reload cannot leave stale rankings in an account runtime.
type Analytics struct {
	catalog Catalog
}

func NewAnalytics(catalog Catalog) *Analytics { return &Analytics{catalog: catalog} }

// GetPlantRankings calculates the same five policy rankings as the Node
// analytics service. Unknown sort values preserve catalog order.
func (a *Analytics) GetPlantRankings(sortBy string) []Ranking {
	if a == nil || a.catalog == nil {
		return nil
	}
	return RankPlants(a.catalog.AllPlants(), a.catalog, sortBy)
}

// PlantRankings is a descriptive alias.
func (a *Analytics) PlantRankings(sortBy string) []Ranking { return a.GetPlantRankings(sortBy) }

// RankPlants is the pure ranking function used by deterministic tests and by
// callers that already have a catalog snapshot.
func RankPlants(plants []PlantRecord, prices Catalog, sortBy string) []Ranking {
	rankings := make([]Ranking, 0, len(plants))
	for _, plant := range plants {
		if plant.SeedID <= 0 || strings.TrimSpace(plant.GrowPhases) == "" {
			continue
		}
		growTime := ParseGrowTime(plant.GrowPhases)
		if growTime <= 0 {
			continue
		}
		seasons := plant.Seasons
		if seasons <= 0 {
			seasons = 1
		}
		multiSeason := seasons == 2
		effectiveGrow := float64(growTime)
		totalExp := float64(plant.Exp)
		if multiSeason {
			effectiveGrow *= 1.5
			totalExp *= 2
		}

		fruitPrice := int64(0)
		seedPrice := plant.Price
		level := plant.RequiredLevel
		if prices != nil {
			fruitPrice = prices.FruitPrice(plant.FruitID)
			if seedPrice == 0 {
				seedPrice = prices.SeedPrice(plant.SeedID)
			}
			if level == 0 {
				level = prices.SeedLevel(plant.SeedID)
			}
		}
		reduce := ParseNormalFertilizerReduceSeconds(plant.GrowPhases)
		reduceApplied := reduce
		if multiSeason {
			reduceApplied = int64(float64(reduce) * 1.5)
		}
		fertGrow := effectiveGrow - float64(reduceApplied)
		if fertGrow <= 0 {
			fertGrow = effectiveGrow
		}
		income := plant.FruitCount * fruitPrice
		if multiSeason {
			income *= 2
		}
		netProfit := income - seedPrice
		ranking := Ranking{
			ID:                            plant.ID,
			SeedID:                        plant.SeedID,
			Name:                          plant.Name,
			Seasons:                       seasons,
			GrowTime:                      int64(effectiveGrow),
			GrowTimeText:                  FormatGrowTime(int64(effectiveGrow)),
			ReduceSeconds:                 reduce,
			ReduceSecondsApplied:          reduceApplied,
			ExpPerHour:                    round2(totalExp / effectiveGrow * secondsPerHour),
			NormalFertilizerExpPerHour:    round2(totalExp / fertGrow * secondsPerHour),
			GoldPerHour:                   round2(float64(income) / effectiveGrow * secondsPerHour),
			ProfitPerHour:                 round2(float64(netProfit) / effectiveGrow * secondsPerHour),
			NormalFertilizerProfitPerHour: round2(float64(netProfit) / fertGrow * secondsPerHour),
			Income:                        income,
			NetProfit:                     netProfit,
			FruitID:                       plant.FruitID,
			FruitCount:                    plant.FruitCount,
			FruitPrice:                    fruitPrice,
			SeedPrice:                     seedPrice,
			Image:                         plant.Image,
		}
		if level > 0 {
			levelCopy := level
			ranking.Level = &levelCopy
		}
		rankings = append(rankings, ranking)
	}

	less := func(i, j int) bool { return rankings[i].SeedID < rankings[j].SeedID }
	switch strings.ToLower(strings.TrimSpace(sortBy)) {
	case "exp", "max_exp", "":
		less = func(i, j int) bool {
			return compareFloat(rankings[i].ExpPerHour, rankings[j].ExpPerHour, rankings[i].SeedID, rankings[j].SeedID)
		}
	case "fert", "max_fert_exp":
		less = func(i, j int) bool {
			return compareFloat(rankings[i].NormalFertilizerExpPerHour, rankings[j].NormalFertilizerExpPerHour, rankings[i].SeedID, rankings[j].SeedID)
		}
	case "gold", "max_gold":
		less = func(i, j int) bool {
			return compareFloat(rankings[i].GoldPerHour, rankings[j].GoldPerHour, rankings[i].SeedID, rankings[j].SeedID)
		}
	case "profit", "max_profit":
		less = func(i, j int) bool {
			return compareFloat(rankings[i].ProfitPerHour, rankings[j].ProfitPerHour, rankings[i].SeedID, rankings[j].SeedID)
		}
	case "fert_profit", "max_fert_profit":
		less = func(i, j int) bool {
			return compareFloat(rankings[i].NormalFertilizerProfitPerHour, rankings[j].NormalFertilizerProfitPerHour, rankings[i].SeedID, rankings[j].SeedID)
		}
	case "level":
		less = func(i, j int) bool {
			left, right := levelValue(rankings[i].Level), levelValue(rankings[j].Level)
			if left != right {
				return left > right
			}
			return rankings[i].SeedID < rankings[j].SeedID
		}
	}
	sort.SliceStable(rankings, less)
	return rankings
}

func ParseGrowTime(growPhases string) int64 {
	var total int64
	for _, phase := range strings.Split(growPhases, ";") {
		phase = strings.TrimSpace(phase)
		if phase == "" {
			continue
		}
		parts := strings.Split(phase, ":")
		seconds, err := strconv.ParseInt(strings.TrimSpace(parts[len(parts)-1]), 10, 64)
		if err == nil && seconds > 0 {
			total += seconds
		}
	}
	if total == 0 && strings.TrimSpace(growPhases) != "" {
		return -1
	}
	return total
}

func ParseNormalFertilizerReduceSeconds(growPhases string) int64 {
	parts := strings.Split(growPhases, ";")
	for _, phase := range parts {
		phase = strings.TrimSpace(phase)
		if phase == "" {
			continue
		}
		pieces := strings.Split(phase, ":")
		seconds, err := strconv.ParseInt(strings.TrimSpace(pieces[len(pieces)-1]), 10, 64)
		if err == nil && seconds > 0 {
			return seconds
		}
		return 0
	}
	return 0
}

func FormatGrowTime(seconds int64) string {
	if seconds < 60 {
		return strconv.FormatInt(seconds, 10) + "秒"
	}
	if seconds < 3600 {
		return strconv.FormatInt(seconds/60, 10) + "分" + strconv.FormatInt(seconds%60, 10) + "秒"
	}
	hours := seconds / 3600
	minutes := seconds % 3600 / 60
	if minutes > 0 {
		return strconv.FormatInt(hours, 10) + "时" + strconv.FormatInt(minutes, 10) + "分"
	}
	return strconv.FormatInt(hours, 10) + "时"
}

func round2(value float64) float64 {
	return float64(int64(value*100+0.5)) / 100
}

func compareFloat(left, right float64, leftID, rightID int64) bool {
	if left != right {
		return left > right
	}
	return leftID < rightID
}

func levelValue(level *int64) int64 {
	if level == nil {
		return -1
	}
	return *level
}
