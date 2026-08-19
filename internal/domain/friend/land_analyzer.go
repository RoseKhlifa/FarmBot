package friend

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"github.com/RoseKhlifa/FarmBot/internal/domain/farm"
	"github.com/RoseKhlifa/FarmBot/internal/game/pb"
	"github.com/RoseKhlifa/FarmBot/internal/store"
	"google.golang.org/protobuf/proto"
)

type StealableInfo struct{ LandID, PlantID, FruitID, Count int64 }
type Analysis struct {
	Stealable       []int64
	StealableInfo   []StealableInfo
	NeedWater       []int64
	NeedWeed        []int64
	NeedBug         []int64
	CanPutWeed      []int64
	CanPutBug       []int64
	CanPutGoldenBug []int64
}
type DogInfo struct{ DogID, Status, LeftSec int64 }
type LandAnalyzerConfig struct {
	API            *API
	Cache          store.CacheRepo
	AccountID      string
	MyGID          int64
	Now            func() time.Time
	PlantBlacklist func(int64) bool
}
type LandAnalyzer struct {
	api            *API
	cache          store.CacheRepo
	accountID      string
	myGID          int64
	now            func() time.Time
	plantBlacklist func(int64) bool
}

func NewLandAnalyzer(cfg LandAnalyzerConfig) *LandAnalyzer {
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return &LandAnalyzer{api: cfg.API, cache: cfg.Cache, accountID: accountText(cfg.AccountID), myGID: cfg.MyGID, now: cfg.Now, plantBlacklist: cfg.PlantBlacklist}
}

func (a *LandAnalyzer) Analyze(lands []*pb.LandInfo, now ...time.Time) Analysis {
	clock := a.now()
	if len(now) > 0 {
		clock = now[0]
	}
	result := Analysis{}
	landMap := farm.BuildLandMap(lands)
	for _, land := range lands {
		if land == nil || land.GetId() <= 0 || !land.GetUnlocked() || farm.IsOccupiedSlaveLand(land, landMap) {
			continue
		}
		plant := land.GetPlant()
		if plant == nil || len(plant.GetPhases()) == 0 {
			continue
		}
		phase := farm.CurrentPhase(plant.GetPhases(), clock)
		if phase == nil {
			continue
		}
		phaseID := pb.PlantPhase(phase.GetPhase())
		if phaseID == pb.PlantPhase_DEAD {
			continue
		}
		if phaseID == pb.PlantPhase_MATURE && plant.GetStealable() && (a.plantBlacklist == nil || !a.plantBlacklist(plant.GetId())) {
			result.Stealable = append(result.Stealable, land.GetId())
			result.StealableInfo = append(result.StealableInfo, StealableInfo{LandID: land.GetId(), PlantID: plant.GetId(), FruitID: plant.GetFruitId(), Count: plant.GetLeftFruitNum()})
		}
		if phaseID != pb.PlantPhase_MATURE {
			if plant.GetDryNum() > 0 || due(phase.GetDryTime(), clock) {
				result.NeedWater = append(result.NeedWater, land.GetId())
			}
			if len(plant.GetWeedOwners()) > 0 || due(phase.GetWeedsTime(), clock) {
				result.NeedWeed = append(result.NeedWeed, land.GetId())
			}
			if len(plant.GetInsectOwners()) > 0 || due(phase.GetInsectTime(), clock) {
				result.NeedBug = append(result.NeedBug, land.GetId())
			}
		}
		if len(plant.GetWeedOwners()) < 2 && !contains(plant.GetWeedOwners(), a.myGID) {
			result.CanPutWeed = append(result.CanPutWeed, land.GetId())
		}
		if len(plant.GetInsectOwners()) < 2 && !contains(plant.GetInsectOwners(), a.myGID) {
			result.CanPutBug = append(result.CanPutBug, land.GetId())
		}
		if phaseID != pb.PlantPhase_MATURE && !hasSocial(plant, GoldenBugItemID, GoldenBugSocialType) {
			result.CanPutGoldenBug = append(result.CanPutGoldenBug, land.GetId())
		}
	}
	return result
}

func due(value int64, now time.Time) bool {
	if value > 1_000_000_000_000_000 {
		value /= 1_000_000
	} else if value > 1_000_000_000_000 {
		value /= 1000
	}
	return value > 0 && value <= now.Unix()
}
func contains(values []int64, target int64) bool {
	for _, v := range values {
		if v == target {
			return true
		}
	}
	return false
}
func hasSocial(plant *pb.PlantInfo, itemID, socialType int64) bool {
	for _, item := range plant.GetSocialItems() {
		if item != nil && item.GetItemId() == itemID && item.GetType() == socialType {
			return true
		}
	}
	return false
}

func (a *LandAnalyzer) GetFriendDogInfo(ctx context.Context, gid int64) (DogInfo, error) {
	if gid <= 0 {
		return DogInfo{}, ErrInvalidGID
	}
	if a.cache != nil && a.accountID != "" {
		if value, err := a.cache.GetFriendDogInfo(ctx, a.accountID); err == nil {
			var values map[string]DogInfo
			if json.Unmarshal(value.Payload, &values) == nil {
				if dog, ok := values[strconv.FormatInt(gid, 10)]; ok {
					return dog, nil
				}
			}
		}
	}
	if a.api == nil {
		return DogInfo{}, ErrAPIRequired
	}
	reply, err := a.api.EnterFriendFarm(ctx, gid)
	if err != nil {
		return DogInfo{}, err
	}
	dog := DogInfo{}
	if len(reply.GetBriefDogInfo()) > 0 {
		info := new(pb.BriefDogInfo)
		if proto.Unmarshal(reply.GetBriefDogInfo(), info) == nil {
			dog = DogInfo{DogID: info.GetDogId(), Status: info.GetStatus(), LeftSec: info.GetLeftSec()}
		}
	}
	_ = a.api.LeaveFriendFarm(ctx, gid)
	if a.cache != nil && a.accountID != "" {
		var values map[string]DogInfo
		if value, err := a.cache.GetFriendDogInfo(ctx, a.accountID); err == nil {
			_ = json.Unmarshal(value.Payload, &values)
		}
		if values == nil {
			values = map[string]DogInfo{}
		}
		values[strconv.FormatInt(gid, 10)] = dog
		raw, _ := json.Marshal(values)
		_ = a.cache.PutFriendDogInfo(ctx, a.accountID, store.CacheValue{Payload: raw, UpdatedAt: a.now().UnixMilli()})
	}
	return dog, nil
}
func (a *LandAnalyzer) BatchGetFriendDogInfo(ctx context.Context, gids []int64) (map[int64]DogInfo, error) {
	out := make(map[int64]DogInfo)
	for _, gid := range NormalizeGIDs(gids) {
		dog, err := a.GetFriendDogInfo(ctx, gid)
		if err != nil {
			return out, err
		}
		out[gid] = dog
	}
	return out, nil
}
func (d DogInfo) IsGuardDog() bool { return d.DogID == GuardDogID }
