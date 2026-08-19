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

const vipDailyKey = "vip_daily_gift"

type QQVIPDailyState struct {
	Key         string
	DoneToday   bool
	LastCheckAt time.Time
	LastClaimAt time.Time
	Result      string
	HasGift     *bool
	CanClaim    *bool
}

type QQVIPService struct {
	transport GameTransport
	config    Config

	mu          sync.Mutex
	doneDateKey string
	lastCheckAt time.Time
	lastClaimAt time.Time
	lastResult  string
	hasGift     *bool
	canClaim    *bool
}

func NewQQVIPService(cfg Config) *QQVIPService {
	return &QQVIPService{transport: cfg.Transport, config: cfg}
}

func (s *QQVIPService) GetDailyGiftStatus(ctx context.Context) (hasGift, canClaim bool, err error) {
	if err = checkContext(ctx); err != nil {
		return false, false, err
	}
	if err = ensureTransport(s.transport); err != nil {
		return false, false, err
	}
	reply := new(pb.GetQQVipRewardsStatusReply)
	response, err := s.transport.SendMsg(ctx, transport.Command{ServiceName: "gamepb.qqvippb.QQVipService", MethodName: "GetQQVipRewardsStatus", Response: reply}, &pb.GetQQVipRewardsStatusRequest{})
	if err != nil {
		return false, false, fmt.Errorf("get QQ VIP reward status: %w", err)
	}
	if response != nil {
		var ok bool
		reply, ok = response.(*pb.GetQQVipRewardsStatusReply)
		if !ok {
			return false, false, fmt.Errorf("get QQ VIP reward status returned %T, want *pb.GetQQVipRewardsStatusReply", response)
		}
	}
	return reply.GetHasGift(), reply.GetCanClaim(), nil
}

func (s *QQVIPService) ClaimDailyGift(ctx context.Context) ([]RewardItem, error) {
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	if err := ensureTransport(s.transport); err != nil {
		return nil, err
	}
	reply := new(pb.ClaimQQVipRewardsReply)
	response, err := s.transport.SendMsg(ctx, transport.Command{ServiceName: "gamepb.qqvippb.QQVipService", MethodName: "ClaimQQVipRewards", Response: reply}, &pb.ClaimQQVipRewardsRequest{VipTypes: []int32{1, 2}})
	if err != nil {
		return nil, fmt.Errorf("claim QQ VIP reward: %w", err)
	}
	if response != nil {
		var ok bool
		reply, ok = response.(*pb.ClaimQQVipRewardsReply)
		if !ok {
			return nil, fmt.Errorf("claim QQ VIP reward returned %T, want *pb.ClaimQQVipRewardsReply", response)
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

func (s *QQVIPService) PerformDailyVipGift(ctx context.Context, force bool) (bool, error) {
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

	hasGift, canClaim, err := s.GetDailyGiftStatus(ctx)
	if err != nil {
		s.setResult("error")
		return false, err
	}
	s.mu.Lock()
	s.hasGift, s.canClaim = boolPtr(hasGift), boolPtr(canClaim)
	s.mu.Unlock()
	if !canClaim {
		s.markDone("none")
		return false, nil
	}
	_, err = s.ClaimDailyGift(ctx)
	if err != nil {
		if isAlreadyClaimedError(err) {
			s.mu.Lock()
			s.doneDateKey, s.lastClaimAt, s.lastResult = dateKey(now), s.config.now(), "ok"
			s.mu.Unlock()
			return false, nil
		}
		s.setResult("error")
		return false, err
	}
	s.mu.Lock()
	s.doneDateKey, s.lastClaimAt, s.lastResult = dateKey(now), s.config.now(), "ok"
	s.mu.Unlock()
	return true, nil
}

func (s *QQVIPService) PerformDailyGift(ctx context.Context, force bool) (bool, error) {
	return s.PerformDailyVipGift(ctx, force)
}

func (s *QQVIPService) DailyState() QQVIPDailyState {
	now := s.config.now()
	s.mu.Lock()
	defer s.mu.Unlock()
	return QQVIPDailyState{Key: vipDailyKey, DoneToday: s.doneDateKey == dateKey(now), LastCheckAt: s.lastCheckAt, LastClaimAt: s.lastClaimAt, Result: s.lastResult, HasGift: cloneBoolPtr(s.hasGift), CanClaim: cloneBoolPtr(s.canClaim)}
}

func (s *QQVIPService) markDone(result string) {
	s.mu.Lock()
	s.doneDateKey, s.lastResult = dateKey(s.config.now()), result
	s.mu.Unlock()
}

func (s *QQVIPService) setResult(result string) {
	s.mu.Lock()
	s.lastResult = result
	s.mu.Unlock()
}

func isAlreadyClaimedError(err error) bool {
	message := strings.ToLower(fmt.Sprint(err))
	return strings.Contains(message, "code=1021002") || strings.Contains(message, "今日已领取") || strings.Contains(message, "已领取")
}
