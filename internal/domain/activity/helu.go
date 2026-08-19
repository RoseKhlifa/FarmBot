package activity

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/RoseKhlifa/FarmBot/internal/game/pb"
)

func (s *service) GetHeluActivity(ctx context.Context) (HeluActivity, error) {
	reply, uid, err := s.GetActivityGroupWithUIDFallback(ctx, HeluActivityID, []string{HeluActivityUID, "SAIJI_DRAW", "SaiJi", "HeLu", "Helu", ""})
	if err != nil {
		return HeluActivity{}, err
	}
	activities := FlattenActivityChildren(reply)
	result := HeluActivity{UID: uid, Title: "荷风十里蝉初鸣", ActivityID: HeluActivityID, DrawActivityID: HeluDrawActivityID}
	if result.UID == "" {
		result.UID = HeluActivityUID
	}
	for _, activity := range activities {
		if activity == nil {
			continue
		}
		if root := replyRootActivity(reply); root == activity && activity.GetTitle() != "" {
			result.Title = activity.GetTitle()
		}
		if activity.GetDrawInfo() != nil && result.Draw.RewardPool == nil {
			result.DrawActivityID = activity.GetId()
			result.Draw = NormalizeDrawInfo(activity.GetDrawInfo(), s.itemNameFor)
		}
		if activity.GetExchangeShop() != nil && len(result.ExchangeShop) == 0 {
			result.ExchangeActivityID = activity.GetId()
			result.ExchangeShop = NormalizeExchangeShopInfo(activity.GetExchangeShop(), s.itemNameFor)
		}
	}
	if result.Draw.RewardPool == nil {
		result.Draw = NormalizeDrawInfo(nil, s.itemNameFor)
	}
	result.HeluBalance = s.heluBalance(ctx)
	result.ActivityCount = len(activities)
	return result, nil
}

func (s *service) heluBalance(ctx context.Context) int64 {
	if s.warehouse == nil {
		return 0
	}
	bag, err := s.warehouse.ListBag(ctx)
	if err != nil {
		return 0
	}
	var total int64
	for _, item := range bag.Items {
		if item.ID == HeluCurrencyItemID && item.Count > 0 {
			total += item.Count
		}
	}
	return total
}

func (s *service) DrawHeluGiftLotus(ctx context.Context, options HeluDrawOptions) (HeluDrawResult, error) {
	if err := s.ensureConnected("helu draw"); err != nil {
		return HeluDrawResult{}, err
	}
	before, err := s.GetHeluActivity(ctx)
	if err != nil {
		return HeluDrawResult{}, err
	}
	count, mode := resolveHeluDrawCount(before.Draw, options)
	if count <= 0 {
		return HeluDrawResult{}, fmt.Errorf("helu draw has no remaining attempts")
	}
	usingFree := before.Draw.FreeRemaining > 0
	if usingFree {
		count = minInt64(count, before.Draw.FreeRemaining)
	} else {
		count = minInt64(count, before.Draw.PaidRemaining)
	}
	if count <= 0 {
		return HeluDrawResult{}, fmt.Errorf("helu draw has no remaining attempts")
	}
	expectedCost := int64(0)
	if !usingFree {
		expectedCost = count * before.Draw.PaidPrice
		if expectedCost > before.HeluBalance {
			return HeluDrawResult{}, fmt.Errorf("helu currency is insufficient: need %d, have %d", expectedCost, before.HeluBalance)
		}
	}
	var rewards []DrawReward
	var items []ActivityItem
	var cost ActivityItem
	for index := int64(0); index < count; index++ {
		if index > 0 && s.heluGap > 0 {
			if err := waitContext(ctx, s.heluGap); err != nil {
				return HeluDrawResult{}, err
			}
		}
		var options OperateOptions
		if usingFree {
			options.Draw = &pb.DrawParams{Id: HeluDrawActivityID, Count: 1}
		} else {
			options.HeluPaidDraw = &pb.HeluPaidDrawParams{Type: 0, Count: 1}
		}
		reply, err := s.OperateActivity(ctx, HeluDrawActivityID, HeluDrawCommand, options)
		if err != nil {
			return HeluDrawResult{}, fmt.Errorf("helu draw: %w", err)
		}
		if reply.GetDrawResult() == nil {
			continue
		}
		decodedRewards, decodedItems, decodedCost := normalizeDrawResult(reply.GetDrawResult(), s.itemNameFor)
		rewards = append(rewards, decodedRewards...)
		items = append(items, decodedItems...)
		if cost.ItemID == 0 {
			cost = decodedCost
		}
	}
	if s.heluRefresh > 0 {
		if err := waitContext(ctx, s.heluRefresh); err != nil {
			return HeluDrawResult{}, err
		}
	}
	after, err := s.GetHeluActivity(ctx)
	if err != nil {
		return HeluDrawResult{}, err
	}
	if mode == "" {
		if usingFree {
			mode = "free"
		} else {
			mode = "paid"
		}
	}
	return HeluDrawResult{
		OK: true, Count: count, ExpectedCost: expectedCost, Mode: mode,
		CurrencyID: chooseCurrency(usingFree, before.Draw.PaidCurrencyID),
		Rewards:    rewards, Items: items, Cost: cost, Activity: after,
	}, nil
}

func resolveHeluDrawCount(draw DrawInfo, options HeluDrawOptions) (int64, string) {
	mode := strings.ToLower(strings.TrimSpace(options.Mode))
	if mode == "batch" || mode == "four" || mode == "max" {
		return draw.Actions.Batch.Count, ""
	}
	if mode == "one" {
		return draw.Actions.One.Count, ""
	}
	if options.Count > 0 {
		if draw.FreeRemaining > 0 {
			return minInt64(options.Count, draw.FreeRemaining), ""
		}
		return minInt64(options.Count, draw.PaidRemaining), ""
	}
	return draw.Actions.One.Count, ""
}

func (s *service) ExchangeHeluShopItem(ctx context.Context, slotID, count int64) (HeluExchangeResult, error) {
	if err := s.ensureConnected("helu exchange"); err != nil {
		return HeluExchangeResult{}, err
	}
	if slotID <= 0 {
		return HeluExchangeResult{}, ErrInvalidSlot
	}
	if count <= 0 {
		return HeluExchangeResult{}, ErrInvalidCount
	}
	before, err := s.GetHeluActivity(ctx)
	if err != nil {
		return HeluExchangeResult{}, err
	}
	var slot *ExchangeShopItem
	for index := range before.ExchangeShop {
		if before.ExchangeShop[index].ID == slotID {
			slot = &before.ExchangeShop[index]
			break
		}
	}
	if slot == nil {
		return HeluExchangeResult{}, fmt.Errorf("helu exchange slot %d was not found", slotID)
	}
	if slot.Cost.ItemID != HeluCurrencyItemID {
		return HeluExchangeResult{}, fmt.Errorf("unsupported helu exchange currency %d", slot.Cost.ItemID)
	}
	if slot.OwnedBlocksExchange {
		return HeluExchangeResult{}, fmt.Errorf("helu exchange slot %d is already owned", slotID)
	}
	if !slot.IsRepeatable && count > 1 {
		return HeluExchangeResult{}, fmt.Errorf("helu exchange slot %d only accepts one item", slotID)
	}
	if slot.ExchangeLimit > 0 && count > slot.ExchangeLimit {
		return HeluExchangeResult{}, fmt.Errorf("helu exchange count exceeds limit %d", slot.ExchangeLimit)
	}
	total := slot.Cost.ItemCount * count
	if total > before.HeluBalance {
		return HeluExchangeResult{}, fmt.Errorf("helu currency is insufficient: need %d, have %d", total, before.HeluBalance)
	}
	if _, err := s.OperateActivity(ctx, HeluExchangeActivityID, HeluExchangeCommand, OperateOptions{
		ExchangeShopOperate: &pb.ExchangeShopOperateParams{Id: slotID, Count: count},
	}); err != nil {
		return HeluExchangeResult{}, fmt.Errorf("helu exchange slot %d: %w", slotID, err)
	}
	after, err := s.GetHeluActivity(ctx)
	if err != nil {
		return HeluExchangeResult{}, err
	}
	return HeluExchangeResult{OK: true, SlotID: slotID, Price: slot.Cost.ItemCount, Count: count, TotalPrice: total, CurrencyID: HeluCurrencyItemID, Item: *slot, Activity: after}, nil
}

func normalizeDrawResult(raw *pb.DrawResult, itemName func(int64) string) ([]DrawReward, []ActivityItem, ActivityItem) {
	if raw == nil {
		return nil, nil, ActivityItem{}
	}
	rewards := make([]DrawReward, 0, len(raw.GetRewards()))
	for _, reward := range raw.GetRewards() {
		if reward == nil || reward.GetItem() == nil {
			continue
		}
		rewards = append(rewards, DrawReward{SlotID: int64(reward.GetSlotId()), Item: itemFromProto(reward.GetItem(), itemName), Flag: int64(reward.GetFlag())})
	}
	return rewards, itemSliceFromProto(raw.GetItems(), itemName), itemFromProto(raw.GetCost(), itemName)
}

func chooseCurrency(free bool, currency int64) int64 {
	if free {
		return 0
	}
	if currency <= 0 {
		return 1002
	}
	return currency
}

func waitContext(ctx context.Context, duration time.Duration) error {
	if duration <= 0 {
		return nil
	}
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
