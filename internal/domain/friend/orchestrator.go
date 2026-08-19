package friend

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/RoseKhlifa/FarmBot/internal/domain/farm"
	"github.com/RoseKhlifa/FarmBot/internal/domain/warehouse"
	"github.com/RoseKhlifa/FarmBot/internal/store"
)

type RunMode string

const (
	ModeHelp      RunMode = "help"
	ModeSteal     RunMode = "steal"
	ModeBad       RunMode = "bad"
	ModeOnlyHelp  RunMode = "onlyHelp"
	ModeOnlySteal RunMode = "onlySteal"
	ModeOnlyBad   RunMode = "onlyBad"
	ModeTurbo     RunMode = "turbo"
)

type RunOptions struct {
	Mode                         RunMode
	Help, Steal, Bad             bool
	OnlyHelp, OnlySteal, OnlyBad bool
	Turbo                        bool
	GuardDog                     bool
	ExpLimit                     bool
	GIDs                         []int64
	Action                       VisitAction
}
type RunResult struct {
	Friends   int
	Visited   int
	Acted     int
	Helped    int
	Stole     int
	Bad       int
	GoldenBug GoldenBugResult
	Errors    []error
}
type OrchestratorConfig struct {
	API       *API
	Limits    *Limits
	Analyzer  *LandAnalyzer
	Visit     *VisitService
	GoldenBug *GoldenBugService
	Farm      farm.Service
	Warehouse warehouse.Service
	Cache     store.CacheRepo
	AccountID string
	MyGID     int64
	Scheduler Scheduler
	Now       func() time.Time
}
type Orchestrator struct {
	api       *API
	limits    *Limits
	analyzer  *LandAnalyzer
	visit     *VisitService
	goldenBug *GoldenBugService
	farm      farm.Service
	warehouse warehouse.Service
	cache     store.CacheRepo
	accountID string
	myGID     int64
	scheduler Scheduler
	now       func() time.Time
	mu        sync.Mutex
	stop      func()
}

func NewOrchestrator(cfg OrchestratorConfig) *Orchestrator {
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return &Orchestrator{api: cfg.API, limits: cfg.Limits, analyzer: cfg.Analyzer, visit: cfg.Visit, goldenBug: cfg.GoldenBug, farm: cfg.Farm, warehouse: cfg.Warehouse, cache: cfg.Cache, accountID: cfg.AccountID, myGID: cfg.MyGID, scheduler: cfg.Scheduler, now: cfg.Now}
}
func (o *Orchestrator) RunOnce(ctx context.Context, options RunOptions) (RunResult, error) {
	result := RunResult{}
	if o == nil || o.api == nil {
		return result, ErrAPIRequired
	}
	friends, err := o.api.GetAllFriends(ctx)
	if err != nil {
		return result, err
	}
	blacklist, _ := o.api.Blacklist(ctx)
	filtered := make([]Friend, 0, len(friends))
	wanted := NormalizeGIDs(options.GIDs)
	for _, friend := range friends {
		if friend.GID == o.myGID {
			continue
		}
		if _, ok := blacklist[friend.GID]; ok {
			continue
		}
		if len(wanted) > 0 && !contains(wanted, friend.GID) {
			continue
		}
		filtered = append(filtered, friend)
	}
	result.Friends = len(filtered)
	if options.Mode == ModeTurbo || options.Turbo || options.Action == ActionGoldenBug {
		if o.goldenBug != nil {
			golden, ge := o.goldenBug.Run(ctx)
			result.GoldenBug = golden
			if ge != nil {
				result.Errors = append(result.Errors, ge)
			}
		}
	}
	actions := o.actions(options)
	for _, action := range actions {
		sort.SliceStable(filtered, func(i, j int) bool { return filtered[i].Level > filtered[j].Level })
		for _, friend := range filtered {
			if o.visit == nil {
				continue
			}
			visit, ve := o.visit.VisitFriend(ctx, friend.GID, action, nil)
			result.Visited++
			if ve != nil {
				result.Errors = append(result.Errors, ve)
				continue
			}
			result.Acted += len(visit.Acted)
			switch action {
			case ActionHelp:
				result.Helped += len(visit.Acted)
			case ActionSteal:
				result.Stole += len(visit.Acted)
			case ActionBad:
				result.Bad += len(visit.Acted)
			}
		}
	}
	return result, nil
}
func (o *Orchestrator) actions(options RunOptions) []VisitAction {
	if options.Mode == ModeTurbo || options.Turbo {
		return []VisitAction{ActionSteal, ActionHelp, ActionBad}
	}
	if options.Action != "" && options.Action != ActionGoldenBug {
		return []VisitAction{options.Action}
	}
	if options.OnlyHelp || options.Mode == ModeOnlyHelp {
		return []VisitAction{ActionHelp}
	}
	if options.OnlySteal || options.Mode == ModeOnlySteal {
		return []VisitAction{ActionSteal}
	}
	if options.OnlyBad || options.Mode == ModeOnlyBad {
		return []VisitAction{ActionBad}
	}
	actions := make([]VisitAction, 0, 3)
	if options.Help || options.Mode == ModeHelp || options.Mode == "" {
		actions = append(actions, ActionHelp)
	}
	if options.Steal || options.Mode == ModeSteal {
		actions = append(actions, ActionSteal)
	}
	if options.Bad || options.Mode == ModeBad {
		actions = append(actions, ActionBad)
	}
	return actions
}
func (o *Orchestrator) Start(ctx context.Context, interval time.Duration) error {
	if o == nil {
		return ErrAPIRequired
	}
	if o.scheduler == nil {
		return nil
	}
	if interval <= 0 {
		interval = time.Minute
	}
	stop, err := o.scheduler.Register(ctx, "friend", interval, func(runCtx context.Context) { _, _ = o.RunOnce(runCtx, RunOptions{Mode: ModeHelp}) })
	if err == nil {
		o.mu.Lock()
		o.stop = stop
		o.mu.Unlock()
	}
	return err
}
func (o *Orchestrator) Stop() error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.stop != nil {
		o.stop()
		o.stop = nil
	}
	return nil
}
