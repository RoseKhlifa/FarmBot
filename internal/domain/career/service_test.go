package career

import (
	"context"
	"reflect"
	"testing"

	"github.com/RoseKhlifa/FarmBot/internal/game/pb"
	"github.com/RoseKhlifa/FarmBot/internal/game/transport"
	"google.golang.org/protobuf/proto"
)

type careerTransportFake struct {
	command transport.Command
	request proto.Message
	result  proto.Message
}

func (f *careerTransportFake) SendMsg(_ context.Context, command transport.Command, request proto.Message) (proto.Message, error) {
	f.command = command
	f.request = request
	return f.result, nil
}

func TestGetCareerInfoMapsAndSortsStats(t *testing.T) {
	fake := &careerTransportFake{result: &pb.CareerInfoGetReply{
		Items:      []*pb.CareerStatItem{{FruitId: 11, Count: 2}, {FruitId: 12, Count: 8}},
		LevelStats: []*pb.CareerLevelStat{{FruitId: 11, Count: 3, Level: 4}},
		Name:       "tester", Gid: 99, Level: 7, Exp: 123,
	}}
	service, err := New(Config{
		Transport: fake,
		LookupItem: func(id int64) (ItemInfo, bool) {
			return ItemInfo{Name: "seed", Image: "image", Rarity: 2}, id == 11
		},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	got, err := service.GetCareerInfo(context.Background())
	if err != nil {
		t.Fatalf("GetCareerInfo() error = %v", err)
	}
	if fake.command.ServiceName != careerService || fake.command.MethodName != "CareerInfoGet" {
		t.Fatalf("command = %+v", fake.command)
	}
	if _, ok := fake.request.(*pb.CareerInfoGetRequest); !ok {
		t.Fatalf("request type = %T", fake.request)
	}
	if got.Items[0].ID != 12 || got.Items[1].ID != 11 {
		t.Fatalf("items were not sorted by count: %#v", got.Items)
	}
	if got.Items[1].Name != "seed" || got.Items[1].Rarity != 2 || got.Player.GID != 99 {
		t.Fatalf("decorated result = %#v", got)
	}
	if !reflect.DeepEqual(got.LevelStats, []LevelStat{{ID: 11, Count: 3, Level: 4, Name: "seed", Image: "image"}}) {
		t.Fatalf("level stats = %#v", got.LevelStats)
	}
}

func TestNewRequiresTransport(t *testing.T) {
	if _, err := New(Config{}); err != ErrTransportRequired {
		t.Fatalf("New() error = %v, want %v", err, ErrTransportRequired)
	}
}
