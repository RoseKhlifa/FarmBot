package activity

import (
	"context"

	"google.golang.org/protobuf/types/known/emptypb"
)

func (s *service) GetSeasonPassport(ctx context.Context) (SeasonPassport, error) {
	body, err := s.sendRaw(ctx, SeasonServiceName, "GetSeasonInfo", &emptypb.Empty{})
	if err != nil {
		return SeasonPassport{}, err
	}
	return NormalizeSeasonInfo(body, s.itemNameFor), nil
}

func (s *service) ClaimSeasonPassportRewards(ctx context.Context) (SeasonClaimResult, error) {
	if err := s.ensureConnected("season passport claim"); err != nil {
		return SeasonClaimResult{}, err
	}
	before, err := s.GetSeasonPassport(ctx)
	if err != nil {
		return SeasonClaimResult{}, err
	}
	body, err := s.sendRaw(ctx, SeasonServiceName, "ClaimBattlePassRewards", &emptypb.Empty{})
	if err != nil {
		return SeasonClaimResult{}, err
	}
	rewards, claimedPassport := NormalizeSeasonClaimResult(body, s.itemNameFor)
	after, err := s.GetSeasonPassport(ctx)
	if err != nil {
		return SeasonClaimResult{}, err
	}
	if claimedPassport != nil {
		after.FreeClaimedLevel = maxInt64(after.FreeClaimedLevel, claimedPassport.FreeClaimedLevel)
	}
	return SeasonClaimResult{OK: true, Rewards: rewards, Passport: after, ClaimedLevels: maxInt64(0, after.FreeClaimedLevel-before.FreeClaimedLevel)}, nil
}
