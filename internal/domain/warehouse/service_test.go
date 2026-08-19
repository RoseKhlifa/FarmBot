package warehouse

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/RoseKhlifa/FarmBot/internal/game/pb"
	"github.com/RoseKhlifa/FarmBot/internal/game/transport"
	"google.golang.org/protobuf/proto"
)

type warehouseTransportFake struct {
	handle func(transport.Command, proto.Message) (proto.Message, error)
	calls  []warehouseCall
}

type warehouseCall struct {
	command transport.Command
	request proto.Message
}

func (f *warehouseTransportFake) SendMsg(_ context.Context, command transport.Command, request proto.Message) (proto.Message, error) {
	f.calls = append(f.calls, warehouseCall{
		command: command,
		request: proto.Clone(request),
	})
	if f.handle == nil {
		return command.Response, nil
	}
	return f.handle(command, request)
}

func TestListBagCopiesProtocolItems(t *testing.T) {
	fake := &warehouseTransportFake{
		handle: func(command transport.Command, request proto.Message) (proto.Message, error) {
			if command.ServiceName != itemService || command.MethodName != "Bag" {
				t.Fatalf("unexpected command: %+v", command)
			}
			if _, ok := request.(*pb.BagRequest); !ok {
				t.Fatalf("unexpected request type %T", request)
			}
			return &pb.BagReply{ItemBag: &pb.ItemBag{Items: []*pb.Item{{Id: 40001, Count: 3, Uid: 7}}}}, nil
		},
	}
	service, err := New(Config{Transport: fake})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	bag, err := service.ListBag(context.Background())
	if err != nil {
		t.Fatalf("ListBag() error = %v", err)
	}
	if want := []Item{{ID: 40001, Count: 3, UID: 7}}; !reflect.DeepEqual(bag.Items, want) {
		t.Fatalf("ListBag() = %#v, want %#v", bag.Items, want)
	}
	bag.Items[0].Count = 99
	if len(fake.calls) != 1 {
		t.Fatalf("transport calls = %d, want 1", len(fake.calls))
	}
}

func TestSellAllUsesClassifierAndRetriesFailedBatch(t *testing.T) {
	var soldCalls [][]Item
	fake := &warehouseTransportFake{
		handle: func(command transport.Command, request proto.Message) (proto.Message, error) {
			switch command.MethodName {
			case "Bag":
				return &pb.BagReply{ItemBag: &pb.ItemBag{Items: []*pb.Item{
					{Id: 40001, Count: 3},
					{Id: 40002, Count: 4},
					{Id: 41221, Count: 8},
					{Id: 1001, Count: 20},
				}}}, nil
			case "Sell":
				sellRequest := request.(*pb.SellRequest)
				items := cloneItemsFromProto(sellRequest.GetItems())
				soldCalls = append(soldCalls, items)
				if len(items) > 1 {
					return nil, errors.New("batch rejected")
				}
				return &pb.SellReply{
					SellItems: sellRequest.GetItems(),
					GetItems:  []*pb.Item{{Id: 1001, Count: items[0].Count * 10}},
				}, nil
			default:
				t.Fatalf("unexpected method %q", command.MethodName)
				return nil, nil
			}
		},
	}
	var gold int64 = 5
	var callbackGold int64
	var operationName string
	var operationCount float64
	service, err := New(Config{
		Transport:     fake,
		IsFruit:       func(id int64) bool { return id == 40001 || id == 40002 || id == 41221 },
		SellBatchSize: 2,
		Status: StatusCallbacks{
			CurrentGold: func() int64 { return gold },
			OnGold:      func(value int64) { callbackGold = value },
			OnOperation: func(name string, count float64) {
				operationName, operationCount = name, count
			},
		},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	result, err := service.SellAll(context.Background())
	if err != nil {
		t.Fatalf("SellAll() error = %v", err)
	}
	if !result.Automation {
		t.Fatal("SellAll() reported automation disabled")
	}
	wantSold := []Item{{ID: 40001, Count: 3}, {ID: 40002, Count: 4}}
	if !reflect.DeepEqual(result.Sold, wantSold) {
		t.Fatalf("sold = %#v, want %#v", result.Sold, wantSold)
	}
	if result.GoldGain != 70 {
		t.Fatalf("gold gain = %d, want 70", result.GoldGain)
	}
	if len(result.Skipped) != 0 {
		t.Fatalf("skipped = %#v, want empty", result.Skipped)
	}
	if !reflect.DeepEqual(soldCalls, [][]Item{
		{{ID: 40001, Count: 3}, {ID: 40002, Count: 4}},
		{{ID: 40001, Count: 3}},
		{{ID: 40002, Count: 4}},
	}) {
		t.Fatalf("sell calls = %#v", soldCalls)
	}
	if callbackGold != 75 {
		t.Fatalf("gold callback = %d, want 75", callbackGold)
	}
	if operationName != "sell" || operationCount != 7 {
		t.Fatalf("operation callback = %q, %v; want sell, 7", operationName, operationCount)
	}
}

func TestSellAllHonorsAutomationCallback(t *testing.T) {
	fake := &warehouseTransportFake{}
	service, err := New(Config{
		Transport:    fake,
		AutomationOn: func(context.Context, string) (bool, error) { return false, nil },
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	result, err := service.SellAll(context.Background())
	if err != nil {
		t.Fatalf("SellAll() error = %v", err)
	}
	if result.Automation {
		t.Fatal("SellAll() reported automation enabled")
	}
	if len(fake.calls) != 0 {
		t.Fatalf("transport calls = %d, want 0", len(fake.calls))
	}
}

func TestUseAndBatchUseMapRequests(t *testing.T) {
	fake := &warehouseTransportFake{
		handle: func(command transport.Command, request proto.Message) (proto.Message, error) {
			switch command.MethodName {
			case "Use":
				got := request.(*pb.UseRequest)
				if got.GetItemId() != 80003 || got.GetCount() != 2 || !reflect.DeepEqual(got.GetLandIds(), []int64{11, 12}) {
					t.Fatalf("use request = %v", got)
				}
				return &pb.UseReply{Items: []*pb.Item{{Id: 1011, Count: 3600}}}, nil
			case "BatchUse":
				got := request.(*pb.BatchUseRequest).GetItems()
				want := []*pb.Item{{Id: 80011, Count: 2, Uid: 4}}
				if !proto.Equal(&pb.BatchUseRequest{Items: got}, &pb.BatchUseRequest{Items: want}) {
					t.Fatalf("batch request = %v, want %v", got, want)
				}
				return &pb.BatchUseReply{UsedItems: want}, nil
			default:
				t.Fatalf("unexpected method %q", command.MethodName)
				return nil, nil
			}
		},
	}
	service, err := New(Config{Transport: fake})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	used, err := service.UseItem(context.Background(), 80003, 2, []int64{11, 12})
	if err != nil {
		t.Fatalf("UseItem() error = %v", err)
	}
	if !reflect.DeepEqual(used.Items, []Item{{ID: 1011, Count: 3600}}) {
		t.Fatalf("use result = %#v", used.Items)
	}
	batch, err := service.BatchUse(context.Background(), []UseEntry{{ItemID: 80011, Count: 2, UID: 4}})
	if err != nil {
		t.Fatalf("BatchUse() error = %v", err)
	}
	if !reflect.DeepEqual(batch.UsedItems, []Item{{ID: 80011, Count: 2, UID: 4}}) {
		t.Fatalf("batch result = %#v", batch.UsedItems)
	}
}

func TestContainerHoursAndMergeItems(t *testing.T) {
	hours := ContainerHours([]Item{{ID: normalContainerID, Count: 7200}, {ID: organicContainerID, Count: 3600}})
	if hours.Normal != 2 || hours.Organic != 1 {
		t.Fatalf("container hours = %+v", hours)
	}
	merged := MergeItems([]Item{
		{ID: 2, Count: 1, UID: 3},
		{ID: 1, Count: 2},
		{ID: 2, Count: 4, UID: 4},
	})
	want := []Item{{ID: 1, Count: 2}, {ID: 2, Count: 5}}
	if !reflect.DeepEqual(merged, want) {
		t.Fatalf("merged = %#v, want %#v", merged, want)
	}
}
