package activity

import (
	"context"
	"strings"

	"github.com/RoseKhlifa/FarmBot/internal/game/pb"
	"google.golang.org/protobuf/encoding/protowire"
)

func (s *service) GetGuanxingActivity(ctx context.Context) (GuanxingActivity, error) {
	body, err := s.sendRaw(ctx, ActivityServiceName, "GetGroup", &pb.GetGroupRequest{Id: GuanxingActivityID, Uid: ""})
	if err != nil {
		return GuanxingActivity{}, err
	}
	return NormalizeGuanxingActivity(body, s.now(), s.itemNameFor), nil
}

func (s *service) ClaimGuanxingRewards(ctx context.Context) (GuanxingClaimResult, error) {
	if err := s.ensureConnected("guanxing claim"); err != nil {
		return GuanxingClaimResult{}, err
	}
	before, err := s.GetGuanxingActivity(ctx)
	if err != nil {
		return GuanxingClaimResult{}, err
	}
	request := &pb.OperateRequest{Id: GuanxingActivityID, Cmd: GuanxingClaimCommand}
	unknown := protowire.AppendTag(nil, 119, protowire.BytesType)
	unknown = protowire.AppendBytes(unknown, nil)
	request.ProtoReflect().SetUnknown(unknown)
	body, err := s.sendRaw(ctx, ActivityServiceName, "Operate", request)
	if err != nil {
		if !isGuanxingNoRewardError(err) {
			return GuanxingClaimResult{}, err
		}
		return GuanxingClaimResult{OK: true, AlreadyClaimed: true, Activity: before}, nil
	}
	after := NormalizeGuanxingActivity(body, s.now(), s.itemNameFor)
	claimedBefore := make(map[int64]bool)
	for _, node := range before.Nodes {
		claimedBefore[node.ID] = node.Claimed
	}
	claimedNodes := make([]GuanxingNode, 0)
	var rewards []ActivityItem
	for _, node := range after.Nodes {
		if node.Claimed && !claimedBefore[node.ID] {
			claimedNodes = append(claimedNodes, node)
			rewards = mergeActivityItems(rewards, node.Rewards)
		}
	}
	return GuanxingClaimResult{OK: true, Claimed: len(claimedNodes) > 0, ClaimedNodes: claimedNodes, Rewards: rewards, Activity: after}, nil
}

func isGuanxingNoRewardError(err error) bool {
	message := errString(err)
	return containsAny(message, "1034038", "无可领取")
}

func containsAny(value string, parts ...string) bool {
	for _, part := range parts {
		if strings.Contains(value, part) {
			return true
		}
	}
	return false
}
