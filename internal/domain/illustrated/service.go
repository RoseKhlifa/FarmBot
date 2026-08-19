// Package illustrated provides account-scoped crop illustrated RPCs.
package illustrated

import (
	"context"
	"errors"
	"fmt"

	"github.com/RoseKhlifa/FarmBot/internal/domain/warehouse"
	"github.com/RoseKhlifa/FarmBot/internal/game/pb"
	"github.com/RoseKhlifa/FarmBot/internal/game/transport"
	"google.golang.org/protobuf/proto"
)

const illustratedService = "gamepb.illustratedpb.IllustratedService"

var ErrTransportRequired = errors.New("illustrated transport is required")

// GameTransport reuses the account-local protocol boundary from warehouse.
type GameTransport = warehouse.GameTransport

type Config struct{ Transport GameTransport }

// List is the generated response plus its stable item slice. Raw is retained
// so callers can access newly added protocol fields without changing this API.
type List struct {
	Items            []*pb.IllustratedItem
	CurrentScore     int32
	Level            int32
	LevelReward      []byte
	UnlockedTiers    []int32
	CurrentTier      int32
	NextScore        int32
	HasLevelReward   bool
	LevelRewards     []*pb.Item
	NextLevelRewards []*pb.Item
	Raw              *pb.GetIllustratedListV2Reply
}

type Rewards struct {
	Items      []*pb.Item
	BonusItems []*pb.Item
	Raw        *pb.ClaimAllRewardsV2Reply
}

type Summary struct {
	Total     int
	Unlocked  int
	Planted   int
	Claimable int
}

type Service interface {
	GetIllustratedList(context.Context, bool, int32) (List, error)
	GetIllustratedListV2(context.Context, bool, int32) (List, error)
	ClaimAllRewards(context.Context, bool) (Rewards, error)
	ClaimAllRewardsV2(context.Context, bool) (Rewards, error)
}

type service struct{ transport GameTransport }

var _ Service = (*service)(nil)

func New(cfg Config) (Service, error) {
	if cfg.Transport == nil {
		return nil, ErrTransportRequired
	}
	return &service{transport: cfg.Transport}, nil
}

func NewService(cfg Config) (Service, error) { return New(cfg) }

func (s *service) GetIllustratedList(ctx context.Context, refresh bool, illustratedType int32) (List, error) {
	if err := checkContext(ctx); err != nil {
		return List{}, err
	}
	if illustratedType <= 0 {
		illustratedType = 1
	}
	reply := new(pb.GetIllustratedListV2Reply)
	response, err := s.transport.SendMsg(ctx, transport.Command{
		ServiceName: illustratedService, MethodName: "GetIllustratedListV2", Response: reply,
	}, &pb.GetIllustratedListV2Request{Refresh: refresh, IllustratedType: illustratedType})
	if err != nil {
		return List{}, fmt.Errorf("get illustrated list: %w", err)
	}
	if response != nil {
		var ok bool
		reply, ok = response.(*pb.GetIllustratedListV2Reply)
		if !ok {
			return List{}, fmt.Errorf("illustrated list returned %T, want *pb.GetIllustratedListV2Reply", response)
		}
	}
	return listFromReply(reply), nil
}

func (s *service) GetIllustratedListV2(ctx context.Context, refresh bool, illustratedType int32) (List, error) {
	return s.GetIllustratedList(ctx, refresh, illustratedType)
}

func (s *service) ClaimAllRewards(ctx context.Context, onlyClaimable bool) (Rewards, error) {
	if err := checkContext(ctx); err != nil {
		return Rewards{}, err
	}
	reply := new(pb.ClaimAllRewardsV2Reply)
	response, err := s.transport.SendMsg(ctx, transport.Command{
		ServiceName: illustratedService, MethodName: "ClaimAllRewardsV2", Response: reply,
	}, &pb.ClaimAllRewardsV2Request{OnlyClaimable: onlyClaimable})
	if err != nil {
		return Rewards{}, fmt.Errorf("claim illustrated rewards: %w", err)
	}
	if response != nil {
		var ok bool
		reply, ok = response.(*pb.ClaimAllRewardsV2Reply)
		if !ok {
			return Rewards{}, fmt.Errorf("illustrated rewards returned %T, want *pb.ClaimAllRewardsV2Reply", response)
		}
	}
	return Rewards{Items: cloneItems(reply.GetItems()), BonusItems: cloneItems(reply.GetBonusItems()), Raw: reply}, nil
}

func (s *service) ClaimAllRewardsV2(ctx context.Context, onlyClaimable bool) (Rewards, error) {
	return s.ClaimAllRewards(ctx, onlyClaimable)
}

func listFromReply(reply *pb.GetIllustratedListV2Reply) List {
	if reply == nil {
		return List{}
	}
	return List{
		Items: cloneIllustratedItems(reply.GetItems()), CurrentScore: reply.GetCurrentScore(), Level: reply.GetLevel(),
		LevelReward: append([]byte(nil), reply.GetLevelReward()...), UnlockedTiers: append([]int32(nil), reply.GetUnlockedTiers()...),
		CurrentTier: reply.GetCurrentTier(), NextScore: reply.GetNextScore(), HasLevelReward: reply.GetHasLevelReward(),
		LevelRewards: cloneItems(reply.GetLevelRewards()), NextLevelRewards: cloneItems(reply.GetNextLevelRewards()), Raw: reply,
	}
}

func Summarize(items []*pb.IllustratedItem) Summary {
	result := Summary{Total: len(items)}
	for _, item := range items {
		if item == nil {
			continue
		}
		if item.GetUnlocked() {
			result.Unlocked++
		}
		if item.GetUnlocked() { // the protocol has no separate planted bit
			result.Planted++
		}
		if item.GetHasReward() {
			result.Claimable++
		}
	}
	return result
}

func cloneIllustratedItems(items []*pb.IllustratedItem) []*pb.IllustratedItem {
	result := make([]*pb.IllustratedItem, 0, len(items))
	for _, item := range items {
		if item != nil {
			result = append(result, proto.Clone(item).(*pb.IllustratedItem))
		}
	}
	return result
}

func cloneItems(items []*pb.Item) []*pb.Item {
	result := make([]*pb.Item, 0, len(items))
	for _, item := range items {
		if item != nil {
			result = append(result, proto.Clone(item).(*pb.Item))
		}
	}
	return result
}

func checkContext(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	return ctx.Err()
}
