package activity

import (
	"context"
	"fmt"
	"strings"

	"github.com/RoseKhlifa/FarmBot/internal/game/pb"
)

func (s *service) GetNanguaShop(ctx context.Context) (NanguaShop, error) {
	reply, uid, err := s.GetActivityGroupWithUIDFallback(ctx, NanguaShopActivityID, []string{NanguaActivityUID, ""})
	if err != nil {
		return NanguaShop{}, err
	}
	return s.normalizeNanguaShop(reply, uid), nil
}

func (s *service) normalizeNanguaShop(reply *pb.GetGroupReply, uid string) NanguaShop {
	activities := FlattenActivityChildren(reply)
	var randomActivity, exchangeActivity *pb.ActivityInfo
	for _, activity := range activities {
		if activity == nil {
			continue
		}
		if randomActivity == nil && activity.GetRandomShop() != nil {
			randomActivity = activity
		}
		if exchangeActivity == nil && activity.GetExchangeShop() != nil {
			exchangeActivity = activity
		}
	}
	result := NanguaShop{UID: uid, Title: "南瓜乐翻天", ActivityCount: len(activities)}
	if result.UID == "" {
		result.UID = NanguaActivityUID
	}
	if root := replyRootActivity(reply); root != nil {
		if root.GetTitle() != "" {
			result.Title = root.GetTitle()
		}
	}
	if randomActivity != nil {
		result.RandomActivityID = randomActivity.GetId()
		result.RandomShop = NormalizeRandomShopInfo(randomActivity.GetRandomShop(), s.itemNameFor)
	}
	if exchangeActivity != nil {
		result.ExchangeActivityID = exchangeActivity.GetId()
		result.ExchangeShop = NormalizeExchangeShopInfo(exchangeActivity.GetExchangeShop(), s.itemNameFor)
	}
	for _, activity := range activities {
		if activity.GetId() > 0 {
			result.ActivityIDs = append(result.ActivityIDs, activity.GetId())
		}
	}
	return result
}

func (s *service) BuyNanguaShopItem(ctx context.Context, slotID, defaultCount int64) (NanguaShop, error) {
	if slotID <= 0 {
		return NanguaShop{}, ErrInvalidSlot
	}
	shop, err := s.GetNanguaShop(ctx)
	if err != nil {
		return NanguaShop{}, err
	}
	var slot *RandomShopItem
	for index := range shop.RandomShop.Items {
		if shop.RandomShop.Items[index].ID == slotID {
			slot = &shop.RandomShop.Items[index]
			break
		}
	}
	if slot == nil {
		return NanguaShop{}, fmt.Errorf("nangua shop slot %d was not found", slotID)
	}
	if !slot.Purchasable {
		return NanguaShop{}, fmt.Errorf("nangua shop slot %d is not purchasable: %s", slotID, slot.StatusLabel)
	}
	count := slot.RemainingCount
	if count <= 0 {
		count = defaultCount
	}
	if count <= 0 {
		return NanguaShop{}, ErrInvalidCount
	}
	_, err = s.OperateActivity(ctx, NanguaRandomActivityID, NanguaShopBuyCommand, OperateOptions{
		RandomShopOperate: &pb.RandomShopOperateParams{Id: slotID, Count: count},
	})
	if err != nil {
		return NanguaShop{}, fmt.Errorf("buy nangua slot %d: %w", slotID, err)
	}
	return s.GetNanguaShop(ctx)
}

func (s *service) RefreshNanguaShop(ctx context.Context) (NanguaShop, error) {
	before, err := s.GetNanguaShop(ctx)
	if err != nil {
		return NanguaShop{}, err
	}
	if _, err := s.OperateActivity(ctx, NanguaRandomActivityID, NanguaShopRefreshCommand, OperateOptions{}); err != nil {
		return NanguaShop{}, err
	}
	after, err := s.GetNanguaShop(ctx)
	if err != nil {
		return NanguaShop{}, err
	}
	if nanguaSignature(before) == nanguaSignature(after) {
		return NanguaShop{}, fmt.Errorf("nangua shop refresh returned unchanged state")
	}
	return after, nil
}

func nanguaSignature(shop NanguaShop) string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "%d/%d/%d/%d/", shop.RandomShop.NextRefreshTime, shop.RandomShop.ManualRefreshUsedCount, shop.RandomShop.ManualRefreshCost, shop.ActivityCount)
	for _, item := range shop.RandomShop.Items {
		fmt.Fprintf(&builder, "%d:%d:%d:%d:%t;", item.ID, item.Item.ItemID, item.StockCount, item.BoughtCount, item.Special)
	}
	return builder.String()
}

func replyRootActivity(reply *pb.GetGroupReply) *pb.ActivityInfo {
	if reply == nil || reply.GetGroup() == nil {
		return nil
	}
	return reply.GetGroup().GetActivity()
}
