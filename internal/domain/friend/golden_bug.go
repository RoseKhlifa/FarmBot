package friend

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/RoseKhlifa/FarmBot/internal/domain/warehouse"
	"github.com/RoseKhlifa/FarmBot/internal/store"
)

type GoldenBugConfig struct {
	API          *API
	Analyzer     *LandAnalyzer
	Limits       *Limits
	Transport    GameTransport
	Warehouse    warehouse.Service
	Cache        store.CacheRepo
	AccountID    string
	MyGID        int64
	Now          func() time.Time
	QuietHours   string
	KeepCount    int64
	RoundLimit   int
	MinInterval  time.Duration
	AutomationOn func(context.Context) bool
}
type GoldenBugResult struct {
	Candidates int
	Placed     int
	Remaining  int64
	Failures   int
	Skipped    bool
}
type GoldenBugService struct {
	api          *API
	analyzer     *LandAnalyzer
	limits       *Limits
	warehouse    warehouse.Service
	accountID    string
	myGID        int64
	now          func() time.Time
	quiet        QuietHours
	keepCount    int64
	roundLimit   int
	minInterval  time.Duration
	automationOn func(context.Context) bool
	mu           sync.Mutex
	lastRun      time.Time
}

func NewGoldenBugService(cfg GoldenBugConfig) *GoldenBugService {
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.RoundLimit <= 0 {
		cfg.RoundLimit = 100
	}
	if cfg.MinInterval <= 0 {
		cfg.MinInterval = 10 * time.Minute
	}
	quiet, _ := ParseQuietHours(cfg.QuietHours)
	return &GoldenBugService{api: cfg.API, analyzer: cfg.Analyzer, limits: cfg.Limits, warehouse: cfg.Warehouse, accountID: cfg.AccountID, myGID: cfg.MyGID, now: cfg.Now, quiet: quiet, keepCount: cfg.KeepCount, roundLimit: cfg.RoundLimit, minInterval: cfg.MinInterval, automationOn: cfg.AutomationOn}
}

func (g *GoldenBugService) Run(ctx context.Context) (GoldenBugResult, error) {
	result := GoldenBugResult{}
	if g == nil || g.api == nil {
		return result, ErrAPIRequired
	}
	now := g.now()
	if g.quiet.Contains(now) {
		result.Skipped = true
		return result, nil
	}
	if g.automationOn != nil && !g.automationOn(ctx) {
		result.Skipped = true
		return result, nil
	}
	g.mu.Lock()
	if !g.lastRun.IsZero() && now.Sub(g.lastRun) < g.minInterval {
		g.mu.Unlock()
		result.Skipped = true
		return result, nil
	}
	g.lastRun = now
	g.mu.Unlock()
	available := int64(0)
	if g.warehouse != nil {
		bag, err := g.warehouse.ListBag(ctx)
		if err != nil {
			return result, err
		}
		for _, item := range bag.Items {
			if item.ID == GoldenBugItemID {
				available += item.Count
			}
		}
	}
	available -= g.keepCount
	if available <= 0 {
		result.Remaining = 0
		return result, nil
	}
	daily := int64(g.roundLimit)
	if g.limits != nil {
		left := g.limits.Remaining(OperationGoldenBug, int64(g.roundLimit))
		if left < daily {
			daily = left
		}
	}
	if available > daily {
		available = daily
	}
	if available <= 0 {
		return result, nil
	}
	friends, err := g.api.GetAllFriends(ctx)
	if err != nil {
		return result, err
	}
	blacklist, _ := g.api.Blacklist(ctx)
	sort.SliceStable(friends, func(i, j int) bool { return friends[i].Level > friends[j].Level })
	failures := 0
	for _, friend := range friends {
		if available <= 0 {
			break
		}
		if friend.GID == g.myGID {
			continue
		}
		if _, ok := blacklist[friend.GID]; ok {
			continue
		}
		visit, err := g.api.EnterFriendFarm(ctx, friend.GID)
		if err != nil {
			failures++
			if failures >= 3 {
				break
			}
			continue
		}
		lands := visit.GetLands()
		targets := []int64{}
		if g.analyzer != nil {
			analysis := g.analyzer.Analyze(lands, now)
			targets = analysis.CanPutGoldenBug
		}
		if len(targets) > int(available) {
			targets = targets[:available]
		}
		if len(targets) > 0 {
			placed, placeErr := g.put(ctx, friend.GID, targets)
			if placeErr == nil {
				result.Placed += len(placed)
				available -= int64(len(placed))
				failures = 0
			} else {
				failures++
			}
		}
		_ = g.api.LeaveFriendFarm(ctx, friend.GID)
		if failures >= 3 {
			break
		}
	}
	result.Candidates = len(friends)
	result.Remaining = available
	return result, nil
}
func (g *GoldenBugService) put(ctx context.Context, gid int64, ids []int64) ([]int64, error) {
	if g.api == nil {
		return nil, ErrAPIRequired
	}
	return (&VisitService{transport: g.api.transport, limits: g.limits, now: g.now}).golden(ctx, gid, ids)
}
