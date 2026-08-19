package activity

import (
	"context"
	"fmt"

	"github.com/RoseKhlifa/FarmBot/internal/game/pb"
)

func (s *service) GetSolarTermsInfo(ctx context.Context) (SolarTermsInfo, error) {
	body, err := s.sendRaw(ctx, SolarServiceName, "GetSolarTerms", &pb.GetGroupRequest{})
	if err != nil {
		return SolarTermsInfo{}, err
	}
	return NormalizeSolarTermsInfo(body, s.itemNameFor), nil
}

func (s *service) ClaimSolarTermsReward(ctx context.Context, termID int64) (SolarClaimResult, error) {
	if err := s.ensureConnected("solar terms claim"); err != nil {
		return SolarClaimResult{}, err
	}
	before, err := s.GetSolarTermsInfo(ctx)
	if err != nil {
		return SolarClaimResult{}, err
	}
	if termID <= 0 && before.CurrentTerm != nil {
		termID = before.CurrentTerm.ID
	}
	if termID <= 0 {
		return SolarClaimResult{}, fmt.Errorf("no claimable solar term")
	}
	for _, term := range before.Terms {
		if term.ID == termID && !term.Claimable {
			return SolarClaimResult{}, fmt.Errorf("solar term %s is not claimable", term.Title)
		}
	}
	request := &pb.GetGroupRequest{Id: termID}
	body, err := s.sendRaw(ctx, SolarServiceName, "ClaimSolarTerms", request)
	if err != nil {
		return SolarClaimResult{}, err
	}
	rewards, term := NormalizeSolarTermsClaimResult(body, s.itemNameFor)
	after, err := s.GetSolarTermsInfo(ctx)
	if err != nil {
		return SolarClaimResult{}, err
	}
	return SolarClaimResult{OK: true, TermID: termID, Rewards: rewards, Term: term, SolarTerms: after}, nil
}
