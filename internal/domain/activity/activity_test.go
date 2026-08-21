package activity

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/RoseKhlifa/FarmBot/internal/domain/warehouse"
	"github.com/RoseKhlifa/FarmBot/internal/game/pb"
	"github.com/RoseKhlifa/FarmBot/internal/game/transport"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
)

type activityCall struct {
	Command transport.Command
	Request proto.Message
}

type activityTransportFake struct {
	mu      sync.Mutex
	Calls   []activityCall
	Replies map[string][]proto.Message
	Raw     map[string][][]byte
}

func (f *activityTransportFake) SendMsg(_ context.Context, command transport.Command, request proto.Message) (proto.Message, error) {
	f.mu.Lock()
	f.Calls = append(f.Calls, activityCall{Command: command, Request: proto.Clone(request)})
	queue := f.Replies[command.MethodName]
	var reply proto.Message
	if len(queue) > 0 {
		reply = queue[0]
		f.Replies[command.MethodName] = queue[1:]
	}
	f.mu.Unlock()
	if reply != nil {
		return reply, nil
	}
	return command.Response, nil
}

func (f *activityTransportFake) SendMsgRaw(_ context.Context, command transport.Command, request proto.Message) ([]byte, *pb.Meta, error) {
	f.mu.Lock()
	f.Calls = append(f.Calls, activityCall{Command: command, Request: proto.Clone(request)})
	queue := f.Raw[command.MethodName]
	var body []byte
	if len(queue) > 0 {
		body = queue[0]
		f.Raw[command.MethodName] = queue[1:]
	}
	f.mu.Unlock()
	return append([]byte(nil), body...), &pb.Meta{}, nil
}

type activityWarehouseFake struct {
	mu   sync.Mutex
	Bags []warehouse.Bag
	idx  int
}

func (f *activityWarehouseFake) ListBag(context.Context) (warehouse.Bag, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.Bags) == 0 {
		return warehouse.Bag{}, nil
	}
	index := f.idx
	if index >= len(f.Bags) {
		index = len(f.Bags) - 1
	}
	f.idx++
	return f.Bags[index], nil
}
func (*activityWarehouseFake) SellItem(context.Context, warehouse.Item) (warehouse.SellResult, error) {
	return warehouse.SellResult{}, nil
}
func (*activityWarehouseFake) SellItems(context.Context, []warehouse.Item) (warehouse.SellResult, error) {
	return warehouse.SellResult{}, nil
}
func (*activityWarehouseFake) SellAll(context.Context) (warehouse.SellAllResult, error) {
	return warehouse.SellAllResult{}, nil
}
func (*activityWarehouseFake) UseItem(context.Context, int64, int64, []int64) (warehouse.UseResult, error) {
	return warehouse.UseResult{}, nil
}
func (*activityWarehouseFake) BatchUse(context.Context, []warehouse.UseEntry) (warehouse.BatchUseResult, error) {
	return warehouse.BatchUseResult{}, nil
}

func newActivityService(t *testing.T, fake *activityTransportFake, warehouseService warehouse.Service) Service {
	t.Helper()
	service, err := New(Config{
		Transport: fake, RawTransport: fake, Warehouse: warehouseService,
		Now:            func() time.Time { return time.Date(2026, time.August, 19, 10, 0, 0, 0, time.FixedZone("CST", 8*60*60)) },
		HeluRequestGap: 0, HeluRefreshDelay: 0, QingmeiStepDelay: 0,
		ItemName: func(id int64) string { return "item" + intToString(id) },
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return service
}

func TestCoreGetAndOperateUseGeneratedActivityPB(t *testing.T) {
	fake := &activityTransportFake{Replies: map[string][]proto.Message{
		"GetGroup": {&pb.GetGroupReply{Group: &pb.ActivityNode{Activity: &pb.ActivityInfo{Id: 10}}}},
		"Operate":  {&pb.OperateReply{Id: 10, Cmd: 2}},
	}}
	service := newActivityService(t, fake, nil)
	group, err := service.GetActivityGroup(context.Background(), 10, "uid")
	if err != nil || group.GetGroup().GetActivity().GetId() != 10 {
		t.Fatalf("GetActivityGroup() = %#v, err=%v", group, err)
	}
	if _, err := service.OperateActivity(context.Background(), 10, 2, OperateOptions{RandomShopOperate: &pb.RandomShopOperateParams{Id: 3, Count: 1}}); err != nil {
		t.Fatalf("OperateActivity() error = %v", err)
	}
	if len(fake.Calls) != 2 || fake.Calls[1].Request.(*pb.OperateRequest).GetRandomShopOperate().GetId() != 3 {
		t.Fatalf("calls = %#v", fake.Calls)
	}
}

func TestNanguaBuyUsesSlotAndOperateCommand(t *testing.T) {
	group := &pb.GetGroupReply{Group: &pb.ActivityNode{Activity: &pb.ActivityInfo{Title: "南瓜乐翻天"}, Children: []*pb.ActivityNode{{Activity: &pb.ActivityInfo{Id: NanguaRandomActivityID, RandomShop: &pb.RandomShopInfo{Items: []*pb.RandomShopItem{{Id: 7, Item: &pb.Item{Id: 500, Count: 1}, Cost: &pb.Item{Id: 1001, Count: 2}, StockCount: 3, BoughtCount: 1, Special: true}}}}}}}}
	fake := &activityTransportFake{Replies: map[string][]proto.Message{"GetGroup": {group, group}, "Operate": {&pb.OperateReply{}}}}
	service := newActivityService(t, fake, nil)
	shop, err := service.BuyNanguaShopItem(context.Background(), 7, 1)
	if err != nil {
		t.Fatalf("BuyNanguaShopItem() error = %v", err)
	}
	if len(shop.RandomShop.Items) != 1 || shop.RandomShop.Items[0].RemainingCount != 2 {
		t.Fatalf("shop = %#v", shop)
	}
	request := fake.Calls[1].Request.(*pb.OperateRequest)
	if request.GetId() != NanguaRandomActivityID || request.GetCmd() != NanguaShopBuyCommand || request.GetRandomShopOperate().GetCount() != 2 {
		t.Fatalf("operate request = %v", request)
	}
}

func TestHeluDrawGetAndOperate(t *testing.T) {
	group := &pb.GetGroupReply{Group: &pb.ActivityNode{Children: []*pb.ActivityNode{{Activity: &pb.ActivityInfo{Id: HeluDrawActivityID, DrawInfo: &pb.DrawInfo{FreeRemainingCount: 1, MaxFreeCount: 1, PaidRemainingCount: 0, MaxPaidCount: 1, Rewards: []*pb.DrawPoolItem{{Id: 1, Item: &pb.Item{Id: 600, Count: 1}}}}}}}}}
	fake := &activityTransportFake{Replies: map[string][]proto.Message{
		"GetGroup": {group, group, group},
		"Operate":  {&pb.OperateReply{DrawResult: &pb.DrawResult{Items: []*pb.Item{{Id: 600, Count: 1}}}}},
	}}
	service := newActivityService(t, fake, nil)
	activity, err := service.GetHeluActivity(context.Background())
	if err != nil || activity.Draw.FreeRemaining != 1 {
		t.Fatalf("GetHeluActivity() = %#v, err=%v", activity, err)
	}
	result, err := service.DrawHeluGiftLotus(context.Background(), HeluDrawOptions{Mode: "one"})
	if err != nil || len(result.Items) != 1 {
		t.Fatalf("DrawHeluGiftLotus() = %#v, err=%v", result, err)
	}
	request := fake.Calls[2].Request.(*pb.OperateRequest)
	if request.GetDraw().GetId() != HeluDrawActivityID || request.GetDraw().GetCount() != 1 {
		t.Fatalf("draw request = %v", request)
	}
}

func TestQingmeiGetAndClaimUsesInstanceTodayState(t *testing.T) {
	group := &pb.GetGroupReply{Group: &pb.ActivityNode{Children: []*pb.ActivityNode{{Activity: &pb.ActivityInfo{Id: QingmeiSeedClaimID, Enabled: true, Status: 1}}}}}
	fake := &activityTransportFake{Replies: map[string][]proto.Message{
		"GetGroup": {group, group, group},
		"Operate":  {&pb.OperateReply{QingmeiClaim: &pb.QingmeiClaimResult{Items: []*pb.Item{{Id: QingmeiSeedItemID, Count: 24}}}}},
	}}
	bag := &activityWarehouseFake{Bags: []warehouse.Bag{{Items: []warehouse.Item{{ID: QingmeiSeedItemID, Count: 0}}}, {Items: []warehouse.Item{{ID: QingmeiSeedItemID, Count: 24}}}}}
	service := newActivityService(t, fake, bag)
	activity, err := service.GetQingmeiActivity(context.Background())
	if err != nil || !activity.Claimable {
		t.Fatalf("GetQingmeiActivity() = %#v, err=%v", activity, err)
	}
	result, err := service.ClaimQingmeiSeeds(context.Background())
	if err != nil || result.ClaimedCount != 24 {
		t.Fatalf("ClaimQingmeiSeeds() = %#v, err=%v", result, err)
	}
	if fake.Calls[1].Request.(*pb.OperateRequest).GetQingmeiClaimParams().GetGrantId() != QingmeiDailyGrantID {
		t.Fatalf("qingmei request = %v", fake.Calls[1].Request)
	}
	second, err := service.GetQingmeiActivity(context.Background())
	if err != nil || !second.Claimed {
		t.Fatalf("instance claim state = %#v, err=%v", second, err)
	}
}

func TestOpaqueActivityDecodersMatchNodeFieldLayout(t *testing.T) {
	item := protowire.AppendTag(nil, 1, protowire.VarintType)
	item = protowire.AppendVarint(item, 21221)
	item = protowire.AppendTag(item, 2, protowire.VarintType)
	item = protowire.AppendVarint(item, 24)
	tier := protowire.AppendTag(nil, 1, protowire.VarintType)
	tier = protowire.AppendVarint(tier, 3)
	tier = protowire.AppendTag(tier, 2, protowire.BytesType)
	tier = protowire.AppendBytes(tier, item)
	passport := protowire.AppendTag(nil, 2, protowire.VarintType)
	passport = protowire.AppendVarint(passport, 4)
	passport = protowire.AppendTag(passport, 9, protowire.VarintType)
	passport = protowire.AppendVarint(passport, 2)
	passport = protowire.AppendTag(passport, 8, protowire.BytesType)
	passport = protowire.AppendBytes(passport, tier)
	season := protowire.AppendTag(nil, 10, protowire.BytesType)
	season = protowire.AppendBytes(season, passport)
	season = protowire.AppendTag(season, 5, protowire.VarintType)
	season = protowire.AppendVarint(season, 100)
	seasonReply := protowire.AppendTag(nil, 1, protowire.BytesType)
	seasonReply = protowire.AppendBytes(seasonReply, season)
	gotSeason := NormalizeSeasonInfo(seasonReply, func(id int64) string { return "seed" })
	if gotSeason.CurrentLevel != 4 || gotSeason.FreeClaimedLevel != 2 || len(gotSeason.RewardTiers) != 1 || gotSeason.RewardTiers[0].FreeRewards[0].ItemID != QingmeiSeedItemID {
		t.Fatalf("season decode = %#v", gotSeason)
	}

	term := protowire.AppendTag(nil, 1, protowire.VarintType)
	term = protowire.AppendVarint(term, 9)
	term = protowire.AppendTag(term, 2, protowire.VarintType)
	term = protowire.AppendVarint(term, 2)
	term = protowire.AppendTag(term, 6, protowire.BytesType)
	term = protowire.AppendString(term, "白露")
	solar := protowire.AppendTag(nil, 1, protowire.BytesType)
	solar = protowire.AppendBytes(solar, term)
	gotSolar := NormalizeSolarTermsInfo(solar, nil)
	if gotSolar.ClaimableCount != 1 || gotSolar.CurrentTerm == nil || gotSolar.CurrentTerm.ID != 9 {
		t.Fatalf("solar decode = %#v", gotSolar)
	}

	node := protowire.AppendTag(nil, 1, protowire.VarintType)
	node = protowire.AppendVarint(node, 1)
	node = protowire.AppendTag(node, 2, protowire.VarintType)
	node = protowire.AppendVarint(node, 1)
	node = protowire.AppendTag(node, 4, protowire.VarintType)
	node = protowire.AppendVarint(node, 1)
	constellation := protowire.AppendTag(nil, 1, protowire.VarintType)
	constellation = protowire.AppendVarint(constellation, 1)
	constellation = protowire.AppendTag(constellation, 4, protowire.BytesType)
	constellation = protowire.AppendBytes(constellation, node)
	guanxing := protowire.AppendTag(nil, 110, protowire.BytesType)
	guanxing = protowire.AppendBytes(guanxing, constellation)
	gotGuanxing := NormalizeGuanxingActivity(guanxing, time.Unix(100, 0), nil)
	if gotGuanxing.TotalDays != 1 || gotGuanxing.ClaimableCount != 1 || !gotGuanxing.Nodes[0].Claimable {
		t.Fatalf("guanxing decode = %#v", gotGuanxing)
	}
}

func TestStarsandUsesHeluExchangeData(t *testing.T) {
	service := newActivityService(t, &activityTransportFake{}, nil)
	starsand, err := service.GetStarsandActivity(context.Background())
	if err != nil || starsand.ItemID != StarsandItemID {
		t.Fatalf("starsand = %#v, err=%v", starsand, err)
	}
}

func TestRawActivityServicesGetAndOperate(t *testing.T) {
	seasonBefore := seasonFixture(4, 2)
	seasonAfter := seasonFixture(5, 3)
	seasonClaim := protowire.AppendTag(nil, 1, protowire.BytesType)
	seasonClaim = protowire.AppendBytes(seasonClaim, itemFixture(QingmeiSeedItemID, 24))
	fake := &activityTransportFake{Raw: map[string][][]byte{
		"GetSeasonInfo":          {seasonBefore, seasonBefore, seasonAfter},
		"ClaimBattlePassRewards": {seasonClaim},
		"GetSolarTerms":          {solarFixture(2), solarFixture(2), solarFixture(3)},
		"ClaimSolarTerms":        {solarClaimFixture()},
		"GetGroup":               {guanxingFixture(false), guanxingFixture(false)},
		"Operate":                {guanxingFixture(true)},
	}}
	service := newActivityService(t, fake, nil)
	passport, err := service.GetSeasonPassport(context.Background())
	if err != nil || passport.CurrentLevel != 4 {
		t.Fatalf("GetSeasonPassport() = %#v, err=%v", passport, err)
	}
	claim, err := service.ClaimSeasonPassportRewards(context.Background())
	if err != nil || claim.ClaimedLevels != 1 || len(claim.Rewards) != 1 {
		t.Fatalf("ClaimSeasonPassportRewards() = %#v, err=%v", claim, err)
	}
	solar, err := service.GetSolarTermsInfo(context.Background())
	if err != nil || solar.ClaimableCount != 1 {
		t.Fatalf("GetSolarTermsInfo() = %#v, err=%v", solar, err)
	}
	solarClaim, err := service.ClaimSolarTermsReward(context.Background(), 9)
	if err != nil || solarClaim.TermID != 9 || len(solarClaim.Rewards) != 1 {
		t.Fatalf("ClaimSolarTermsReward() = %#v, err=%v", solarClaim, err)
	}
	guanxing, err := service.GetGuanxingActivity(context.Background())
	if err != nil || guanxing.ClaimableCount != 1 {
		t.Fatalf("GetGuanxingActivity() = %#v, err=%v", guanxing, err)
	}
	guanxingClaim, err := service.ClaimGuanxingRewards(context.Background())
	if err != nil || !guanxingClaim.Claimed {
		t.Fatalf("ClaimGuanxingRewards() = %#v, err=%v", guanxingClaim, err)
	}
	var extensionPresent bool
	for _, call := range fake.Calls {
		if call.Command.MethodName != "Operate" {
			continue
		}
		request := call.Request.(*pb.OperateRequest)
		unknown := request.ProtoReflect().GetUnknown()
		for len(unknown) > 0 {
			number, wireType, n := protowire.ConsumeTag(unknown)
			if n < 0 {
				break
			}
			if number == 119 {
				extensionPresent = true
			}
			valueSize := protowire.ConsumeFieldValue(number, wireType, unknown[n:])
			if valueSize < 0 || n+valueSize > len(unknown) {
				break
			}
			unknown = unknown[n+valueSize:]
		}
	}
	if !extensionPresent {
		t.Fatal("guanxing operate extension field 119 missing")
	}
}

func itemFixture(id, count int64) []byte {
	item := protowire.AppendTag(nil, 1, protowire.VarintType)
	item = protowire.AppendVarint(item, uint64(id))
	item = protowire.AppendTag(item, 2, protowire.VarintType)
	return protowire.AppendVarint(item, uint64(count))
}

func seasonFixture(level, claimed int64) []byte {
	tier := protowire.AppendTag(nil, 1, protowire.VarintType)
	tier = protowire.AppendVarint(tier, 3)
	tier = protowire.AppendTag(tier, 2, protowire.BytesType)
	tier = protowire.AppendBytes(tier, itemFixture(QingmeiSeedItemID, 24))
	passport := protowire.AppendTag(nil, 2, protowire.VarintType)
	passport = protowire.AppendVarint(passport, uint64(level))
	passport = protowire.AppendTag(passport, 9, protowire.VarintType)
	passport = protowire.AppendVarint(passport, uint64(claimed))
	passport = protowire.AppendTag(passport, 8, protowire.BytesType)
	passport = protowire.AppendBytes(passport, tier)
	season := protowire.AppendTag(nil, 10, protowire.BytesType)
	season = protowire.AppendBytes(season, passport)
	season = protowire.AppendTag(season, 5, protowire.VarintType)
	season = protowire.AppendVarint(season, 100)
	reply := protowire.AppendTag(nil, 1, protowire.BytesType)
	return protowire.AppendBytes(reply, season)
}

func solarFixture(status int64) []byte {
	term := protowire.AppendTag(nil, 1, protowire.VarintType)
	term = protowire.AppendVarint(term, 9)
	term = protowire.AppendTag(term, 2, protowire.VarintType)
	term = protowire.AppendVarint(term, uint64(status))
	term = protowire.AppendTag(term, 6, protowire.BytesType)
	term = protowire.AppendString(term, "白露")
	reply := protowire.AppendTag(nil, 1, protowire.BytesType)
	return protowire.AppendBytes(reply, term)
}

func solarClaimFixture() []byte {
	reply := protowire.AppendTag(nil, 1, protowire.BytesType)
	reply = protowire.AppendBytes(reply, itemFixture(200, 1))
	term := protowire.AppendTag(nil, 1, protowire.VarintType)
	term = protowire.AppendVarint(term, 9)
	term = protowire.AppendTag(term, 2, protowire.VarintType)
	term = protowire.AppendVarint(term, 3)
	reply = protowire.AppendTag(reply, 2, protowire.BytesType)
	return protowire.AppendBytes(reply, term)
}

func guanxingFixture(claimed bool) []byte {
	node := protowire.AppendTag(nil, 1, protowire.VarintType)
	node = protowire.AppendVarint(node, 1)
	node = protowire.AppendTag(node, 2, protowire.VarintType)
	node = protowire.AppendVarint(node, 1)
	node = protowire.AppendTag(node, 3, protowire.VarintType)
	if claimed {
		node = protowire.AppendVarint(node, 1)
	} else {
		node = protowire.AppendVarint(node, 0)
	}
	node = protowire.AppendTag(node, 4, protowire.VarintType)
	if claimed {
		node = protowire.AppendVarint(node, 0)
	} else {
		node = protowire.AppendVarint(node, 1)
	}
	constellation := protowire.AppendTag(nil, 1, protowire.VarintType)
	constellation = protowire.AppendVarint(constellation, 1)
	constellation = protowire.AppendTag(constellation, 4, protowire.BytesType)
	constellation = protowire.AppendBytes(constellation, node)
	reply := protowire.AppendTag(nil, 110, protowire.BytesType)
	return protowire.AppendBytes(reply, constellation)
}
