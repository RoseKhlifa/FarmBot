package social

import (
	"context"
	"encoding/hex"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/RoseKhlifa/FarmBot/internal/game/pb"
	"github.com/RoseKhlifa/FarmBot/internal/game/transport"
	"google.golang.org/protobuf/proto"
)

type socialCall struct {
	command transport.Command
	request proto.Message
}

type socialTransportFake struct {
	calls   []socialCall
	results map[string]proto.Message
	errors  map[string]error
}

func (f *socialTransportFake) SendMsg(_ context.Context, command transport.Command, request proto.Message) (proto.Message, error) {
	f.calls = append(f.calls, socialCall{command: command, request: proto.Clone(request)})
	key := command.ServiceName + "." + command.MethodName
	if err := f.errors[key]; err != nil {
		return nil, err
	}
	if result, ok := f.results[key]; ok {
		return result, nil
	}
	return command.Response, nil
}

func TestDailyShareUsesThreeRPCsAndKeepsInstanceState(t *testing.T) {
	clock := time.Date(2026, 8, 19, 9, 0, 0, 0, time.Local)
	fake := &socialTransportFake{results: map[string]proto.Message{
		shareService + ".CheckCanShare":    &pb.CheckCanShareReply{CanShare: true},
		shareService + ".ReportShare":      &pb.ReportShareReply{Success: true},
		shareService + ".ClaimShareReward": &pb.ClaimShareRewardReply{Success: true},
	}}
	service, err := New(Config{Transport: fake, Now: func() time.Time { return clock }, ShareCheckCooldown: time.Hour})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	claimed, err := service.PerformDailyShare(context.Background(), false)
	if err != nil || !claimed {
		t.Fatalf("PerformDailyShare() = %v, err = %v", claimed, err)
	}
	if again, err := service.PerformDailyShare(context.Background(), false); err != nil || again {
		t.Fatalf("second PerformDailyShare() = %v, err = %v", again, err)
	}
	if state := service.ShareDailyState(); !state.DoneToday || state.Key != shareDailyKey {
		t.Fatalf("state = %#v", state)
	}
	if len(fake.calls) != 3 {
		t.Fatalf("RPC calls = %d, want 3", len(fake.calls))
	}
}

func TestInvitePayloadCarriesShareKeyAsUnknownField(t *testing.T) {
	fake := &socialTransportFake{}
	service, err := New(Config{Transport: fake})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	code := InviteCode{UID: 42, OpenID: "openid", ShareKey: "abcdef"}
	result, err := service.SendReportArkClick(context.Background(), code)
	if err != nil || !result.OK {
		t.Fatalf("SendReportArkClick() = %#v, err = %v", result, err)
	}
	request := fake.calls[0].request.(*pb.ReportArkClickRequest)
	raw, err := proto.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	if got := hex.EncodeToString(raw); got != "082a12066f70656e69641a043130303820072a06616263646566" {
		t.Fatalf("payload = %s", got)
	}
}

func TestInteractFallbackNormalizesAndSorts(t *testing.T) {
	fake := &socialTransportFake{
		errors: map[string]error{"gamepb.interactpb.InteractService.InteractRecords": errors.New("route missing")},
		results: map[string]proto.Message{"gamepb.interactpb.InteractService.GetInteractRecords": &pb.InteractRecordsReply{Records: []*pb.InteractRecord{
			{ServerTime: 100, ActionType: 1, VisitorGid: 2, CropId: 7, CropCount: 3, Extra: &pb.InteractRecord_Extra{LandId: 9}},
			{ServerTime: 200, ActionType: 2, VisitorGid: 1, Times: 2},
		}}},
	}
	service, err := New(Config{Transport: fake, CropName: func(int64) string { return "玉米" }})
	if err != nil {
		t.Fatal(err)
	}
	records, err := service.GetInteractRecords(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 || records[0].ServerTimeSec != 200 || records[1].CropName != "玉米" || records[1].LandID != 9 {
		t.Fatalf("records = %#v", records)
	}
}

func TestDogGiftRawVarintsAndShareLink(t *testing.T) {
	if value, ok := ExtractVarintField([]byte{0x38, 0x42}, 7); !ok || value != 66 {
		t.Fatalf("ExtractVarintField() = %d, %v", value, ok)
	}
	link := ParseShareLink("?uid=42&openid=o&share_key=k&share_source=s&doc_id=d")
	if !reflect.DeepEqual(link, InviteCode{UID: 42, OpenID: "o", ShareKey: "k", ShareSource: "s", DocID: "d"}) {
		t.Fatalf("link = %#v", link)
	}
	fake := &socialTransportFake{results: map[string]proto.Message{
		dogService + ".GetDogInfo": func() proto.Message {
			reply := &pb.GetDogInfoReply{}
			reply.ProtoReflect().SetUnknown([]byte{0x38, 0x42})
			return reply
		}(),
		dogService + ".ClaimSkillGifts": &pb.GetDogInfoReply{Coin: 5},
	}}
	service, err := New(Config{Transport: fake})
	if err != nil {
		t.Fatal(err)
	}
	status, err := service.GetDogGiftStatus(context.Background())
	if err != nil || status.Claimable != 66 {
		t.Fatalf("status = %#v, err = %v", status, err)
	}
	claimed, err := service.ClaimDogGifts(context.Background())
	if err != nil || claimed.Claimed != 5 {
		t.Fatalf("claimed = %#v, err = %v", claimed, err)
	}
}
