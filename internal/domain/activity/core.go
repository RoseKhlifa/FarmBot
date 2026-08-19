package activity

import (
	"context"
	"errors"
	"fmt"

	"github.com/RoseKhlifa/FarmBot/internal/game/pb"
	"github.com/RoseKhlifa/FarmBot/internal/game/transport"
	"google.golang.org/protobuf/proto"
)

// OperateOptions contains the generated oneof-like payloads supported by the
// current ActivityService schema. Activity-specific helpers construct these
// values so callers rarely need to touch protobuf directly.
type OperateOptions struct {
	Draw                *pb.DrawParams
	RandomShopOperate   *pb.RandomShopOperateParams
	ExchangeShopOperate *pb.ExchangeShopOperateParams
	QingmeiClaimParams  *pb.QingmeiClaimParams
	HeluPaidDraw        *pb.HeluPaidDrawParams
	QingmeiWineStart    *pb.QingmeiWineStartParams
	QingmeiWineBrew     *pb.QingmeiWineBrewParams
	QingmeiWineSell     *pb.QingmeiWineSellParams
}

func (s *service) GetActivityGroup(ctx context.Context, activityID int64, uid string) (*pb.GetGroupReply, error) {
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	if activityID <= 0 {
		return nil, ErrActivityIDRequired
	}
	reply := new(pb.GetGroupReply)
	response, err := s.transport.SendMsg(ctx, transport.Command{
		ServiceName: ActivityServiceName,
		MethodName:  "GetGroup",
		Response:    reply,
	}, &pb.GetGroupRequest{Id: activityID, Uid: uid})
	if err != nil {
		return nil, fmt.Errorf("get activity group %d: %w", activityID, err)
	}
	if response != nil {
		var ok bool
		reply, ok = response.(*pb.GetGroupReply)
		if !ok {
			return nil, fmt.Errorf("get activity group returned %T, want *pb.GetGroupReply", response)
		}
	}
	return reply, nil
}

// GetActivityGroupWithUIDFallback retries the common UID aliases used by
// server deployments that expose the same activity under different names.
func (s *service) GetActivityGroupWithUIDFallback(ctx context.Context, activityID int64, uids []string) (*pb.GetGroupReply, string, error) {
	if len(uids) == 0 {
		uids = []string{""}
	}
	var lastErr error
	for _, uid := range uids {
		reply, err := s.GetActivityGroup(ctx, activityID, uid)
		if err != nil {
			lastErr = err
			continue
		}
		if len(FlattenActivityChildren(reply)) > 0 || uid == uids[len(uids)-1] {
			return reply, uid, nil
		}
	}
	if lastErr != nil {
		return nil, "", lastErr
	}
	return nil, "", errors.New("activity group lookup returned no reply")
}

func (s *service) ListActivities(ctx context.Context) ([]*pb.ActivityInfo, error) {
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	reply := new(pb.ListReply)
	response, err := s.transport.SendMsg(ctx, transport.Command{
		ServiceName: ActivityServiceName,
		MethodName:  "List",
		Response:    reply,
	}, &pb.ListRequest{})
	if err != nil {
		return nil, fmt.Errorf("list activities: %w", err)
	}
	if response != nil {
		var ok bool
		reply, ok = response.(*pb.ListReply)
		if !ok {
			return nil, fmt.Errorf("list activities returned %T, want *pb.ListReply", response)
		}
	}
	activities := make([]*pb.ActivityInfo, 0, len(reply.GetActivities()))
	for _, item := range reply.GetActivities() {
		if item != nil {
			activities = append(activities, proto.Clone(item).(*pb.ActivityInfo))
		}
	}
	return activities, nil
}

func (s *service) OperateActivity(ctx context.Context, activityID, command int64, options OperateOptions) (*pb.OperateReply, error) {
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	if err := s.ensureConnected("activity operation"); err != nil {
		return nil, err
	}
	if activityID <= 0 {
		return nil, ErrActivityIDRequired
	}
	if command < 0 {
		return nil, ErrActivityCommand
	}
	request := &pb.OperateRequest{Id: activityID, Cmd: command}
	if options.Draw != nil {
		request.Draw = proto.Clone(options.Draw).(*pb.DrawParams)
	}
	if options.RandomShopOperate != nil {
		request.RandomShopOperate = proto.Clone(options.RandomShopOperate).(*pb.RandomShopOperateParams)
	}
	if options.ExchangeShopOperate != nil {
		request.ExchangeShopOperate = proto.Clone(options.ExchangeShopOperate).(*pb.ExchangeShopOperateParams)
	}
	if options.QingmeiClaimParams != nil {
		request.QingmeiClaimParams = proto.Clone(options.QingmeiClaimParams).(*pb.QingmeiClaimParams)
	}
	if options.HeluPaidDraw != nil {
		request.HeluPaidDraw = proto.Clone(options.HeluPaidDraw).(*pb.HeluPaidDrawParams)
	}
	if options.QingmeiWineStart != nil {
		request.QingmeiWineStart = proto.Clone(options.QingmeiWineStart).(*pb.QingmeiWineStartParams)
	}
	if options.QingmeiWineBrew != nil {
		request.QingmeiWineBrew = proto.Clone(options.QingmeiWineBrew).(*pb.QingmeiWineBrewParams)
	}
	if options.QingmeiWineSell != nil {
		request.QingmeiWineSell = proto.Clone(options.QingmeiWineSell).(*pb.QingmeiWineSellParams)
	}
	reply := new(pb.OperateReply)
	response, err := s.transport.SendMsg(ctx, transport.Command{
		ServiceName: ActivityServiceName,
		MethodName:  "Operate",
		Response:    reply,
	}, request)
	if err != nil {
		return nil, fmt.Errorf("operate activity %d command %d: %w", activityID, command, err)
	}
	if response != nil {
		var ok bool
		reply, ok = response.(*pb.OperateReply)
		if !ok {
			return nil, fmt.Errorf("operate activity returned %T, want *pb.OperateReply", response)
		}
	}
	return reply, nil
}

func (s *service) sendRaw(ctx context.Context, serviceName, method string, request proto.Message) ([]byte, error) {
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	if s.raw == nil {
		return nil, ErrRawTransportNeeded
	}
	body, _, err := s.raw.SendMsgRaw(ctx, transport.Command{ServiceName: serviceName, MethodName: method}, request)
	if err != nil {
		return nil, fmt.Errorf("raw activity %s.%s: %w", serviceName, method, err)
	}
	return append([]byte(nil), body...), nil
}

func (s *service) ensureConnected(action string) error {
	if s.connected != nil && !s.connected() {
		return fmt.Errorf("%s: game connection is offline", action)
	}
	return nil
}

func checkContext(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	return ctx.Err()
}

func (s *service) itemNameFor(id int64) string {
	if s.itemName != nil {
		if value := s.itemName(id); value != "" {
			return value
		}
	}
	return fmt.Sprintf("物品#%d", id)
}

func itemFromProto(item *pb.Item, name func(int64) string) ActivityItem {
	if item == nil {
		return ActivityItem{}
	}
	itemID, count := item.GetId(), item.GetCount()
	if count < 0 {
		count = 0
	}
	itemName := ""
	if name != nil {
		itemName = name(itemID)
	}
	return ActivityItem{ItemID: itemID, ItemCount: count, Count: count, UID: item.GetUid(), Name: itemName}
}

func itemSliceFromProto(items []*pb.Item, name func(int64) string) []ActivityItem {
	result := make([]ActivityItem, 0, len(items))
	for _, item := range items {
		converted := itemFromProto(item, name)
		if converted.ItemID > 0 {
			result = append(result, converted)
		}
	}
	return result
}
