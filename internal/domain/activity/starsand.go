package activity

import "context"

// Starsand is currently represented by the shared 荷露 exchange shop. The
// dedicated legacy endpoint was a stub, so this method intentionally returns a
// stable descriptor rather than inventing a new RPC.
func (s *service) GetStarsandActivity(context.Context) (StarsandActivity, error) {
	return StarsandActivity{ItemID: StarsandItemID, Title: "星砂", Note: "星砂商店复用荷露兑换商店数据源"}, nil
}
