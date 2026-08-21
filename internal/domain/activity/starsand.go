package activity

import "context"

// GetStarsandActivity exposes the live exchange entries whose currency is the
// legacy 星砂 item. The game removed the standalone activity endpoint, but its
// items remain part of the shared 荷露 exchange payload.
func (s *service) GetStarsandActivity(ctx context.Context) (StarsandActivity, error) {
	helu, err := s.GetHeluActivity(ctx)
	if err != nil {
		return StarsandActivity{}, err
	}
	shop := make([]ExchangeShopItem, 0)
	for _, item := range helu.ExchangeShop {
		if item.Cost.ItemID == StarsandItemID {
			shop = append(shop, item)
		}
	}
	return StarsandActivity{
		ItemID: StarsandItemID, Title: "星砂", Note: "星砂商店复用荷露兑换商店数据源",
		Balance: helu.HeluBalance, ExchangeShop: shop, Activity: helu, Available: len(shop) > 0,
	}, nil
}

func (s *service) ExchangeStarsandItem(ctx context.Context, slotID, count int64) (HeluExchangeResult, error) {
	activity, err := s.GetStarsandActivity(ctx)
	if err != nil {
		return HeluExchangeResult{}, err
	}
	for _, item := range activity.ExchangeShop {
		if item.ID == slotID {
			return s.ExchangeHeluShopItem(ctx, slotID, count)
		}
	}
	return HeluExchangeResult{}, ErrInvalidSlot
}
