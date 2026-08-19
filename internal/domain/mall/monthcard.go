package mall

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/RoseKhlifa/FarmBot/internal/game/pb"
	"github.com/RoseKhlifa/FarmBot/internal/game/transport"
)

const monthCardDailyKey = "month_card_gift"

type MonthCardInfo struct {
	GoodsID  int64
	Reward   RewardItem
	CanClaim bool
	Raw      *pb.MonthCardInfo
}

type RewardItem struct {
	ID    int64
	Count int64
	Name  string
}

type MonthCardDailyState struct {
	Key          string
	DoneToday    bool
	LastCheckAt  time.Time
	LastClaimAt  time.Time
	Result       string
	HasCard      *bool
	HasClaimable *bool
}

type MonthCardService struct {
	transport GameTransport
	config    Config

	mu          sync.Mutex
	doneDateKey string
	lastCheckAt time.Time
	lastClaimAt time.Time
	lastResult  string
	hasCard     *bool
	claimable   *bool
}

func NewMonthCardService(cfg Config) *MonthCardService {
	return &MonthCardService{transport: cfg.Transport, config: cfg}
}

func (s *MonthCardService) GetMonthCardInfos(ctx context.Context) ([]MonthCardInfo, error) {
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	if err := ensureTransport(s.transport); err != nil {
		return nil, err
	}
	reply := new(pb.GetMonthCardInfosReply)
	response, err := s.transport.SendMsg(ctx, transport.Command{ServiceName: mallServiceName, MethodName: "GetMonthCardInfos", Response: reply}, &pb.GetMonthCardInfosRequest{})
	if err != nil {
		return nil, fmt.Errorf("get month card infos: %w", err)
	}
	if response != nil {
		var ok bool
		reply, ok = response.(*pb.GetMonthCardInfosReply)
		if !ok {
			return nil, fmt.Errorf("get month card infos returned %T, want *pb.GetMonthCardInfosReply", response)
		}
	}
	result := make([]MonthCardInfo, 0, len(reply.GetInfos()))
	for _, info := range reply.GetInfos() {
		if info == nil {
			continue
		}
		reward := info.GetReward()
		id, count := int64(0), int64(0)
		if reward != nil {
			id, count = int64(reward.GetId()), int64(reward.GetCount())
		}
		result = append(result, MonthCardInfo{GoodsID: int64(info.GetGoodsId()), Reward: RewardItem{ID: id, Count: count, Name: s.config.itemName(id)}, CanClaim: info.GetCanClaim(), Raw: info})
	}
	return result, nil
}

func (s *MonthCardService) ClaimMonthCardReward(ctx context.Context, goodsID int64) ([]RewardItem, error) {
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	if err := ensureTransport(s.transport); err != nil {
		return nil, err
	}
	if goodsID <= 0 {
		return nil, ErrInvalidGoods
	}
	reply := new(pb.ClaimMonthCardRewardReply)
	response, err := s.transport.SendMsg(ctx, transport.Command{ServiceName: mallServiceName, MethodName: "ClaimMonthCardReward", Response: reply}, &pb.ClaimMonthCardRewardRequest{GoodsId: int32(goodsID)})
	if err != nil {
		return nil, fmt.Errorf("claim month card reward %d: %w", goodsID, err)
	}
	if response != nil {
		var ok bool
		reply, ok = response.(*pb.ClaimMonthCardRewardReply)
		if !ok {
			return nil, fmt.Errorf("claim month card reward returned %T, want *pb.ClaimMonthCardRewardReply", response)
		}
	}
	items := make([]RewardItem, 0, len(reply.GetItems()))
	for _, item := range reply.GetItems() {
		if item == nil {
			continue
		}
		id := int64(item.GetId())
		items = append(items, RewardItem{ID: id, Count: int64(item.GetCount()), Name: s.config.itemName(id)})
	}
	return items, nil
}

func (s *MonthCardService) PerformDailyMonthCardGift(ctx context.Context, force bool) (bool, error) {
	if err := checkContext(ctx); err != nil {
		return false, err
	}
	now := s.config.now()
	s.mu.Lock()
	if !force && s.doneDateKey == dateKey(now) {
		s.mu.Unlock()
		return false, nil
	}
	if !force && !s.lastCheckAt.IsZero() && now.Sub(s.lastCheckAt) < mallBuyCooldown {
		s.mu.Unlock()
		return false, nil
	}
	s.lastCheckAt = now
	s.mu.Unlock()

	infos, err := s.GetMonthCardInfos(ctx)
	if err != nil {
		s.setResult("error")
		return false, err
	}
	hasCard := len(infos) > 0
	claimable := false
	for _, info := range infos {
		if info.CanClaim && info.GoodsID > 0 {
			claimable = true
			break
		}
	}
	s.mu.Lock()
	s.hasCard, s.claimable = boolPtr(hasCard), boolPtr(claimable)
	s.mu.Unlock()
	if !hasCard || !claimable {
		s.markDone("none")
		return false, nil
	}
	claimed := 0
	var firstErr error
	for _, info := range infos {
		if !info.CanClaim || info.GoodsID <= 0 {
			continue
		}
		if _, claimErr := s.ClaimMonthCardReward(ctx, info.GoodsID); claimErr != nil {
			if firstErr == nil {
				firstErr = claimErr
			}
			continue
		}
		claimed++
	}
	if claimed > 0 {
		s.mu.Lock()
		s.doneDateKey = dateKey(now)
		s.lastClaimAt = s.config.now()
		s.lastResult = "ok"
		s.mu.Unlock()
		return true, nil
	}
	s.setResult("none")
	return false, firstErr
}

func (s *MonthCardService) PerformDailyGift(ctx context.Context, force bool) (bool, error) {
	return s.PerformDailyMonthCardGift(ctx, force)
}

func (s *MonthCardService) DailyState() MonthCardDailyState {
	now := s.config.now()
	s.mu.Lock()
	defer s.mu.Unlock()
	return MonthCardDailyState{Key: monthCardDailyKey, DoneToday: s.doneDateKey == dateKey(now), LastCheckAt: s.lastCheckAt, LastClaimAt: s.lastClaimAt, Result: s.lastResult, HasCard: cloneBoolPtr(s.hasCard), HasClaimable: cloneBoolPtr(s.claimable)}
}

func (s *MonthCardService) markDone(result string) {
	s.mu.Lock()
	s.doneDateKey, s.lastResult = dateKey(s.config.now()), result
	s.mu.Unlock()
}

func (s *MonthCardService) setResult(result string) {
	s.mu.Lock()
	s.lastResult = result
	s.mu.Unlock()
}

func boolPtr(value bool) *bool { return &value }

func cloneBoolPtr(value *bool) *bool {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func RewardSummary(items []RewardItem) string {
	parts := make([]string, 0, len(items))
	for _, item := range items {
		if item.Count <= 0 {
			continue
		}
		name := item.Name
		currency := true
		switch item.ID {
		case 1001, 500001:
			name = "金币"
		case 1002, 500002:
			name = "经验"
		case 200, 500:
			name = "点券"
		default:
			currency = false
			if name == "" {
				name = fmt.Sprintf("物品#%d", item.ID)
			}
		}
		if currency {
			parts = append(parts, fmt.Sprintf("%s%d", name, item.Count))
		} else {
			parts = append(parts, fmt.Sprintf("%sx%d", name, item.Count))
		}
	}
	return strings.Join(parts, "/")
}
