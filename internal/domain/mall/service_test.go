package mall

import (
	"context"
	"encoding/binary"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/RoseKhlifa/FarmBot/internal/domain/warehouse"
	"github.com/RoseKhlifa/FarmBot/internal/game/pb"
	"github.com/RoseKhlifa/FarmBot/internal/game/transport"
	"google.golang.org/protobuf/proto"
)

type mallTransportFake struct {
	handle func(transport.Command, proto.Message) (proto.Message, error)
	calls  []transport.Command
}

func (f *mallTransportFake) SendMsg(_ context.Context, command transport.Command, request proto.Message) (proto.Message, error) {
	f.calls = append(f.calls, command)
	if f.handle == nil {
		return command.Response, nil
	}
	return f.handle(command, request)
}

func TestMallGoodsParsingAndFertilizerPurchase(t *testing.T) {
	price := encodeVarintFields(map[uint64]uint64{1: 2})
	goodsBytes, err := proto.Marshal(&pb.MallGoods{GoodsId: int32(OrganicFertilizerGoodsID), Name: "organic", Price: price})
	if err != nil {
		t.Fatal(err)
	}
	fake := &mallTransportFake{handle: func(command transport.Command, request proto.Message) (proto.Message, error) {
		switch command.MethodName {
		case "GetMallListBySlotType":
			if request.(*pb.GetMallListBySlotTypeRequest).GetSlotType() != MallSlotType {
				t.Fatalf("slot type = %d", request.(*pb.GetMallListBySlotTypeRequest).GetSlotType())
			}
			return &pb.GetMallListBySlotTypeResponse{GoodsList: [][]byte{goodsBytes}}, nil
		case "Purchase":
			purchase := request.(*pb.PurchaseRequest)
			if purchase.GetGoodsId() != int32(OrganicFertilizerGoodsID) || purchase.GetCount() != 6 {
				t.Fatalf("purchase request = %v", purchase)
			}
			return &pb.PurchaseResponse{GoodsId: purchase.GetGoodsId(), Count: purchase.GetCount()}, nil
		default:
			return nil, errors.New("unexpected mall method")
		}
	}}

	service, err := NewService(Config{Transport: fake, Balance: func(context.Context, int64) (int64, error) { return 20, nil }})
	if err != nil {
		t.Fatal(err)
	}
	goods, err := service.GetMallGoodsList(context.Background(), MallSlotType)
	if err != nil || len(goods) != 1 || goods[0].Price != 2 {
		t.Fatalf("goods = %#v, err = %v", goods, err)
	}
	bought, err := service.BuyFertilizer(context.Background(), FertilizerOrganic, 6, true)
	if err != nil || bought != 6 {
		t.Fatalf("bought = %d, err = %v", bought, err)
	}
}

func TestMallSkipsMalformedGoodsAndStopsOnKnownEmptyBalance(t *testing.T) {
	good, err := proto.Marshal(&pb.MallGoods{GoodsId: int32(OrganicFertilizerGoodsID), Price: encodeVarintFields(map[uint64]uint64{1: 2})})
	if err != nil {
		t.Fatal(err)
	}
	var purchases int
	fake := &mallTransportFake{handle: func(command transport.Command, request proto.Message) (proto.Message, error) {
		switch command.MethodName {
		case "GetMallListBySlotType":
			return &pb.GetMallListBySlotTypeResponse{GoodsList: [][]byte{{0xff}, good}}, nil
		case "Purchase":
			purchases++
			return &pb.PurchaseResponse{GoodsId: request.(*pb.PurchaseRequest).GetGoodsId(), Count: request.(*pb.PurchaseRequest).GetCount()}, nil
		default:
			return nil, errors.New("unexpected mall method")
		}
	}}
	service, err := NewService(Config{Transport: fake, Balance: func(context.Context, int64) (int64, error) { return 0, nil }})
	if err != nil {
		t.Fatal(err)
	}
	goods, err := service.GetMallGoodsList(context.Background(), MallSlotType)
	if err != nil || len(goods) != 1 {
		t.Fatalf("goods = %#v, err = %v", goods, err)
	}
	bought, err := service.BuyFertilizer(context.Background(), FertilizerOrganic, 0, true)
	if err != nil || bought != 0 || purchases != 0 {
		t.Fatalf("empty balance purchase = %d, calls = %d, err = %v", bought, purchases, err)
	}
}

func TestMysteryAutoBuyAndSchedulerRegistration(t *testing.T) {
	fake := &mallTransportFake{handle: func(command transport.Command, request proto.Message) (proto.Message, error) {
		switch command.MethodName {
		case "GetActiveNPC":
			return &pb.GetActiveNPCReply{Active: true, Npc: &pb.MysteryShopNPC{NpcId: 7, ItemId: 88, CurrencyId: 1001}}, nil
		case "Buy":
			return &pb.BuyReply{Npc: &pb.MysteryShopNPC{NpcId: request.(*pb.BuyRequest).GetNpcId(), Purchased: true}, Reward: &pb.MysteryShopReward{ItemId: 88, Count: 2}}, nil
		default:
			return nil, errors.New("unexpected mystery method")
		}
	}}
	service := NewMysteryService(Config{Transport: fake, MysteryAutoBuyCurrencies: func(context.Context) ([]int64, error) { return []int64{1001}, nil }})
	bought, err := service.RunAutoBuyOnce(context.Background())
	if err != nil || !bought || service.DailyState().Bought != 1 {
		t.Fatalf("mystery result = %v, err = %v, state = %+v", bought, err, service.DailyState())
	}
	scheduler := &schedulerFake{}
	if err := service.RegisterAutoBuy(scheduler); err != nil {
		t.Fatal(err)
	}
	if scheduler.name != "mall:mystery-auto-buy" || scheduler.interval != time.Hour {
		t.Fatalf("scheduler registration = %+v", scheduler)
	}
}

func TestMysteryStartSkipsDisabledAutomation(t *testing.T) {
	service := NewMysteryService(Config{
		Transport:             &mallTransportFake{},
		MysteryAutoBuyEnabled: func(context.Context) (bool, error) { return false, nil },
	})
	scheduler := &schedulerFake{}
	if err := service.StartAutoBuy(context.Background(), scheduler); err != nil {
		t.Fatal(err)
	}
	if scheduler.name != "" {
		t.Fatalf("disabled automation registered task %q", scheduler.name)
	}
}

func TestMonthCardAndVIPDailyClaims(t *testing.T) {
	fake := &mallTransportFake{handle: func(command transport.Command, request proto.Message) (proto.Message, error) {
		switch command.MethodName {
		case "GetMonthCardInfos":
			return &pb.GetMonthCardInfosReply{Infos: []*pb.MonthCardInfo{{GoodsId: 9, CanClaim: true, Reward: &pb.Item{Id: 1001, Count: 10}}}}, nil
		case "ClaimMonthCardReward":
			return &pb.ClaimMonthCardRewardReply{Items: []*pb.Item{{Id: 1001, Count: 10}}}, nil
		case "GetQQVipRewardsStatus":
			return &pb.GetQQVipRewardsStatusReply{HasGift: true, CanClaim: true}, nil
		case "ClaimQQVipRewards":
			if !reflect.DeepEqual(request.(*pb.ClaimQQVipRewardsRequest).GetVipTypes(), []int32{1, 2}) {
				t.Fatalf("VIP types = %v", request.(*pb.ClaimQQVipRewardsRequest).GetVipTypes())
			}
			return &pb.ClaimQQVipRewardsReply{Items: []*pb.Item{{Id: 500, Count: 3}}}, nil
		default:
			return nil, errors.New("unexpected daily method")
		}
	}}
	cfg := Config{Transport: fake}
	month := NewMonthCardService(cfg)
	claimed, err := month.PerformDailyMonthCardGift(context.Background(), true)
	if err != nil || !claimed || month.DailyState().Result != "ok" {
		t.Fatalf("month card = %v, err = %v, state = %+v", claimed, err, month.DailyState())
	}
	vip := NewQQVIPService(cfg)
	claimed, err = vip.PerformDailyVipGift(context.Background(), true)
	if err != nil || !claimed || vip.DailyState().Result != "ok" {
		t.Fatalf("VIP = %v, err = %v, state = %+v", claimed, err, vip.DailyState())
	}
	if got := RewardSummary([]RewardItem{{ID: 1001, Count: 10}, {ID: 500, Count: 3}, {ID: 77, Count: 2, Name: "种子"}}); got != "金币10/点券3/种子x2" {
		t.Fatalf("RewardSummary() = %q", got)
	}
}

type schedulerFake struct {
	name     string
	interval time.Duration
}

func (f *schedulerFake) Every(name string, interval, _ time.Duration, _ any) error {
	f.name, f.interval = name, interval
	return nil
}

type warehouseFake struct{}

func (warehouseFake) ListBag(context.Context) (warehouse.Bag, error) {
	return warehouse.Bag{Items: []warehouse.Item{{ID: 1011, Count: 0}, {ID: 1012, Count: 0}}}, nil
}
func (warehouseFake) SellItem(context.Context, warehouse.Item) (warehouse.SellResult, error) {
	return warehouse.SellResult{}, nil
}
func (warehouseFake) SellItems(context.Context, []warehouse.Item) (warehouse.SellResult, error) {
	return warehouse.SellResult{}, nil
}
func (warehouseFake) SellAll(context.Context) (warehouse.SellAllResult, error) {
	return warehouse.SellAllResult{}, nil
}
func (warehouseFake) UseItem(context.Context, int64, int64, []int64) (warehouse.UseResult, error) {
	return warehouse.UseResult{}, nil
}
func (warehouseFake) BatchUse(context.Context, []warehouse.UseEntry) (warehouse.BatchUseResult, error) {
	return warehouse.BatchUseResult{}, nil
}

func encodeVarintFields(fields map[uint64]uint64) []byte {
	result := make([]byte, 0)
	for field, value := range fields {
		var buffer [binary.MaxVarintLen64]byte
		n := binary.PutUvarint(buffer[:], field<<3)
		result = append(result, buffer[:n]...)
		n = binary.PutUvarint(buffer[:], value)
		result = append(result, buffer[:n]...)
	}
	return result
}
