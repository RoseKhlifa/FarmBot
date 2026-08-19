package activity

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/RoseKhlifa/FarmBot/internal/domain/warehouse"
	"github.com/RoseKhlifa/FarmBot/internal/game/pb"
	"github.com/RoseKhlifa/FarmBot/internal/game/transport"
)

func (s *service) GetQingmeiActivity(ctx context.Context) (QingmeiActivity, error) {
	reply, _, err := s.GetActivityGroupWithUIDFallback(ctx, QingmeiActivityID, []string{QingmeiActivityUID, ""})
	if err != nil {
		return QingmeiActivity{}, err
	}
	activities := FlattenActivityChildren(reply)
	root := replyRootActivity(reply)
	var claim, wine *pb.ActivityInfo
	for _, activity := range activities {
		if activity == nil {
			continue
		}
		if activity.GetId() == QingmeiSeedClaimID {
			claim = activity
		}
		if activity.GetId() == QingmeiWineActivityID {
			wine = activity
		}
	}
	if root == nil {
		for _, activity := range activities {
			if activity.GetId() == QingmeiActivityID {
				root = activity
				break
			}
		}
	}
	result := QingmeiActivity{
		UID: QingmeiActivityUID, Title: "青酿换万金", ActivityID: QingmeiActivityID,
		ClaimActivityID: QingmeiSeedClaimID, WineActivityID: QingmeiWineActivityID,
		Reward:   ActivityItem{ItemID: QingmeiSeedItemID, ItemCount: 24, Count: 24, Name: s.itemNameFor(QingmeiSeedItemID)},
		Material: ActivityItem{ItemID: QingmeiFruitItemID, Name: s.itemNameFor(QingmeiFruitItemID)},
	}
	if root != nil {
		result.StartTime, result.EndTime = root.GetStartTime(), root.GetEndTime()
		if root.GetTitle() != "" {
			result.Title = root.GetTitle()
		}
	}
	if claim != nil {
		result.StartTime = chooseNonZero(result.StartTime, claim.GetStartTime())
		result.EndTime = chooseNonZero(result.EndTime, claim.GetEndTime())
		result.Claimed = claim.GetStatus() == 3 || s.qingmeiClaimedToday()
		result.Claimable = !result.Claimed && claim.GetEnabled()
	}
	if wine != nil {
		result.StartTime = chooseNonZero(result.StartTime, wine.GetStartTime())
		result.EndTime = chooseNonZero(result.EndTime, wine.GetEndTime())
	}
	result.Material.ItemCount = s.bagItemCount(ctx, QingmeiFruitItemID)
	result.Material.Count = result.Material.ItemCount
	return result, nil
}

func (s *service) ClaimQingmeiSeeds(ctx context.Context) (QingmeiClaimResult, error) {
	if err := s.ensureConnected("qingmei seed claim"); err != nil {
		return QingmeiClaimResult{}, err
	}
	before := s.bagItemCount(ctx, QingmeiSeedItemID)
	if s.qingmeiClaimedToday() {
		qingmei, err := s.GetQingmeiActivity(ctx)
		if err != nil {
			return QingmeiClaimResult{}, err
		}
		return QingmeiClaimResult{OK: true, AlreadyClaimed: true, BeforeCount: before, AfterCount: before, Qingmei: qingmei}, nil
	}
	reply, err := s.OperateActivity(ctx, QingmeiSeedClaimID, QingmeiSeedClaimCommand, OperateOptions{
		QingmeiClaimParams: &pb.QingmeiClaimParams{GrantId: QingmeiDailyGrantID},
	})
	if err != nil {
		if isAlreadyClaimedError(err) {
			s.markQingmeiClaimedToday()
			qingmei, stateErr := s.GetQingmeiActivity(ctx)
			if stateErr != nil {
				return QingmeiClaimResult{}, stateErr
			}
			return QingmeiClaimResult{OK: true, AlreadyClaimed: true, BeforeCount: before, AfterCount: before, Qingmei: qingmei}, nil
		}
		return QingmeiClaimResult{}, err
	}
	after := s.bagItemCount(ctx, QingmeiSeedItemID)
	rewards := make([]ActivityItem, 0)
	if reply.GetQingmeiClaim() != nil {
		rewards = itemSliceFromProto(reply.GetQingmeiClaim().GetItems(), s.itemNameFor)
	}
	claimedCount := after - before
	if claimedCount <= 0 {
		for _, item := range rewards {
			if item.ItemID == QingmeiSeedItemID {
				claimedCount += item.ItemCount
			}
		}
	}
	if claimedCount <= 0 {
		return QingmeiClaimResult{}, fmt.Errorf("qingmei seed claim returned no reward")
	}
	s.markQingmeiClaimedToday()
	qingmei, err := s.GetQingmeiActivity(ctx)
	if err != nil {
		return QingmeiClaimResult{}, err
	}
	return QingmeiClaimResult{OK: true, ClaimedCount: claimedCount, BeforeCount: before, AfterCount: after, Rewards: rewards, Qingmei: qingmei}, nil
}

func (s *service) BrewAndSellQingmeiWine(ctx context.Context, options QingmeiWineOptions) (QingmeiWineResult, error) {
	if err := s.ensureConnected("qingmei wine"); err != nil {
		return QingmeiWineResult{}, err
	}
	materialItems := s.qingmeiWineMaterialItems(ctx)
	var beforeMaterial int64
	for _, item := range materialItems {
		beforeMaterial += item.Count
	}
	if beforeMaterial <= 0 {
		return QingmeiWineResult{}, ErrNoMaterial
	}
	brewSteps := options.BrewSteps
	if brewSteps <= 0 {
		brewSteps = 3
	}
	previewReply, previewErr := s.OperateActivity(ctx, QingmeiWineActivityID, QingmeiWinePreviewCommand, OperateOptions{
		QingmeiWineStart: &pb.QingmeiWineStartParams{Items: materialItems},
	})
	preview := int64(0)
	if previewReply.GetQingmeiPreview() != nil {
		preview = previewReply.GetQingmeiPreview().GetPrice()
	}
	if previewErr != nil {
		preview = 0
	}
	if err := waitContext(ctx, s.qingmeiDelay); err != nil {
		return QingmeiWineResult{}, err
	}
	brews := make([]QingmeiBrew, 0, brewSteps)
	for index := 0; index < brewSteps; index++ {
		reply, err := s.OperateActivity(ctx, QingmeiWineActivityID, QingmeiWineBrewCommand, OperateOptions{
			QingmeiWineBrew: &pb.QingmeiWineBrewParams{},
		})
		if err != nil {
			return QingmeiWineResult{}, fmt.Errorf("qingmei brew step %d: %w", index+1, err)
		}
		if brew := reply.GetQingmeiBrew(); brew != nil {
			brews = append(brews, QingmeiBrew{WineType: int64(brew.GetWineType()), Cost: brew.GetCost(), Price: brew.GetPrice(), CanDouble: brew.GetCanDouble()})
		}
		if err := waitContext(ctx, s.qingmeiDelay); err != nil {
			return QingmeiWineResult{}, err
		}
	}
	if len(brews) == 0 {
		return QingmeiWineResult{}, fmt.Errorf("qingmei brew returned no result")
	}
	last := brews[len(brews)-1]
	shared := false
	if options.Share && last.CanDouble {
		var shareErr error
		shared, shareErr = s.reportShare(ctx)
		if shareErr != nil {
			return QingmeiWineResult{}, fmt.Errorf("qingmei wine share: %w", shareErr)
		}
	}
	multiple := int32(1)
	if shared {
		multiple = 2
	}
	sellReply, err := s.OperateActivity(ctx, QingmeiWineActivityID, QingmeiWineSellCommand, OperateOptions{
		QingmeiWineSell: &pb.QingmeiWineSellParams{Multiple: multiple},
	})
	if err != nil {
		return QingmeiWineResult{}, fmt.Errorf("qingmei wine sell: %w", err)
	}
	sale := QingmeiWineSale{Multiple: int64(multiple)}
	if sell := sellReply.GetQingmeiSell(); sell != nil {
		sale.Gold = sell.GetGold()
		sale.Item = itemFromProto(sell.GetItem(), s.itemNameFor)
	}
	if sale.Gold <= 0 {
		return QingmeiWineResult{}, fmt.Errorf("qingmei wine sell returned no gold")
	}
	afterMaterial := s.bagItemCount(ctx, QingmeiFruitItemID)
	qingmei, err := s.GetQingmeiActivity(ctx)
	if err != nil {
		return QingmeiWineResult{}, err
	}
	return QingmeiWineResult{OK: true, BeforeMaterialCount: beforeMaterial, AfterMaterialCount: afterMaterial,
		ConsumedCount: maxInt64(0, beforeMaterial-afterMaterial), Preview: preview, Brews: brews, Brew: last,
		Shared: shared, Sell: sale, Activity: qingmei}, nil
}

func (s *service) reportShare(ctx context.Context) (bool, error) {
	check := new(pb.CheckCanShareReply)
	response, err := s.transport.SendMsg(ctx, transport.Command{ServiceName: "gamepb.sharepb.ShareService", MethodName: "CheckCanShare", Response: check}, &pb.CheckCanShareRequest{})
	if err != nil {
		return false, err
	}
	if response != nil {
		check = response.(*pb.CheckCanShareReply)
	}
	if !check.GetCanShare() {
		return false, fmt.Errorf("share is unavailable")
	}
	report := new(pb.ReportShareReply)
	response, err = s.transport.SendMsg(ctx, transport.Command{ServiceName: "gamepb.sharepb.ShareService", MethodName: "ReportShare", Response: report}, &pb.ReportShareRequest{Shared: true})
	if err != nil {
		return false, err
	}
	if response != nil {
		report = response.(*pb.ReportShareReply)
	}
	if !report.GetSuccess() {
		return false, fmt.Errorf("share report failed")
	}
	return true, nil
}

func (s *service) bagItemCount(ctx context.Context, itemID int64) int64 {
	if s.warehouse == nil {
		return 0
	}
	bag, err := s.warehouse.ListBag(ctx)
	if err != nil {
		return 0
	}
	var total int64
	for _, item := range bag.Items {
		if item.ID == itemID && item.Count > 0 {
			total += item.Count
		}
	}
	return total
}

func (s *service) qingmeiWineMaterialItems(ctx context.Context) []*pb.Item {
	if s.warehouse == nil {
		return nil
	}
	bag, err := s.warehouse.ListBag(ctx)
	if err != nil {
		return nil
	}
	items := make([]warehouse.Item, 0)
	for _, item := range bag.Items {
		if item.ID == QingmeiFruitItemID && item.UID > 0 && item.Count > 0 {
			items = append(items, item)
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].UID < items[j].UID })
	result := make([]*pb.Item, 0, len(items))
	for _, item := range items {
		result = append(result, &pb.Item{Id: item.UID, Count: item.Count, Uid: item.UID})
	}
	return result
}

func (s *service) qingmeiClaimedToday() bool {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	return s.qingmeiClaimedDate == s.now().Local().Format("2006-01-02")
}

func (s *service) markQingmeiClaimedToday() {
	s.stateMu.Lock()
	s.qingmeiClaimedDate = s.now().Local().Format("2006-01-02")
	s.stateMu.Unlock()
}

func isAlreadyClaimedError(err error) bool {
	message := strings.ToLower(errString(err))
	return strings.Contains(message, "已领取") || strings.Contains(message, "已经领取") || strings.Contains(message, "重复领取") || strings.Contains(message, "already") || strings.Contains(message, "1009001")
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func chooseNonZero(first, second int64) int64 {
	if first != 0 {
		return first
	}
	return second
}
