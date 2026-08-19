package illustrated

import (
	"context"
	"testing"

	"github.com/RoseKhlifa/FarmBot/internal/game/pb"
	"github.com/RoseKhlifa/FarmBot/internal/game/transport"
	"google.golang.org/protobuf/proto"
)

type illustratedTransportFake struct {
	commands []transport.Command
	requests []proto.Message
	results  []proto.Message
}

func (f *illustratedTransportFake) SendMsg(_ context.Context, command transport.Command, request proto.Message) (proto.Message, error) {
	f.commands = append(f.commands, command)
	f.requests = append(f.requests, request)
	result := command.Response
	if len(f.results) > 0 {
		result, f.results = f.results[0], f.results[1:]
	}
	return result, nil
}

func TestIllustratedRPCsAndSummary(t *testing.T) {
	fake := &illustratedTransportFake{results: []proto.Message{
		&pb.GetIllustratedListV2Reply{Items: []*pb.IllustratedItem{
			{SeedId: 1, Unlocked: true, HasReward: true}, {SeedId: 2},
		}},
		&pb.ClaimAllRewardsV2Reply{Items: []*pb.Item{{Id: 1001, Count: 3}}},
	}}
	service, err := New(Config{Transport: fake})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	list, err := service.GetIllustratedList(context.Background(), true, 0)
	if err != nil {
		t.Fatalf("GetIllustratedList() error = %v", err)
	}
	request := fake.requests[0].(*pb.GetIllustratedListV2Request)
	if !request.GetRefresh() || request.GetIllustratedType() != 1 {
		t.Fatalf("request = %v", request)
	}
	if got := Summarize(list.Items); got != (Summary{Total: 2, Unlocked: 1, Planted: 1, Claimable: 1}) {
		t.Fatalf("summary = %#v", got)
	}
	rewards, err := service.ClaimAllRewards(context.Background(), true)
	if err != nil || len(rewards.Items) != 1 || rewards.Items[0].GetId() != 1001 {
		t.Fatalf("rewards = %#v, err = %v", rewards, err)
	}
	if fake.commands[1].MethodName != "ClaimAllRewardsV2" {
		t.Fatalf("claim command = %+v", fake.commands[1])
	}
}
