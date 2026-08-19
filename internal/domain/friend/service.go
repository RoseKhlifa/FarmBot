package friend

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/RoseKhlifa/FarmBot/internal/domain/farm"
	"github.com/RoseKhlifa/FarmBot/internal/domain/warehouse"
	"github.com/RoseKhlifa/FarmBot/internal/store"
)

const (
	FriendServiceName = "gamepb.friendpb.FriendService"
	VisitServiceName  = "gamepb.visitpb.VisitService"
	PlantServiceName  = "gamepb.plantpb.PlantService"
)

const (
	OperationHelpWater int64 = 10001
	OperationHelpWeed  int64 = 10002
	OperationHelpBug   int64 = 10003
	OperationSteal     int64 = 10004
	OperationPutBug    int64 = 10005
	OperationPutWeed   int64 = 10006
	OperationGoldenBug int64 = 10015
	OperationBadDaily  int64 = 100
)

const (
	GoldenBugItemID     int64 = 301101
	GoldenBugSocialType int64 = 2
	GuardDogID          int64 = 90021
)

var (
	ErrTransportRequired = errors.New("friend transport is required")
	ErrCacheRequired     = errors.New("friend cache repository is required")
	ErrAPIRequired       = errors.New("friend API is required")
	ErrInvalidGID        = errors.New("friend gid must be positive")
	ErrQuietHours        = errors.New("invalid quiet-hours window")
)

// GameTransport is the narrow RPC boundary shared by the friend services.
// warehouse.GameTransport is intentionally structurally identical; the alias
// keeps composition roots from having to wrap one transport implementation.
type GameTransport = warehouse.GameTransport

// Scheduler is an optional account-local loop owner. The domain never stores
// a package-global scheduler or callback.
type Scheduler interface {
	Register(context.Context, string, time.Duration, func(context.Context)) (func(), error)
}

// Config wires all friend collaborators. All state created from this config is
// account-local, which is important when multiple QQ/WeChat accounts run in
// one process.
type Config struct {
	Transport  GameTransport
	AccountID  string
	MyGID      int64
	Platform   Platform
	Cache      store.CacheRepo
	Farm       farm.Service
	Warehouse  warehouse.Service
	Now        func() time.Time
	QuietHours string

	Limits       *Limits
	API          *API
	Analyzer     *LandAnalyzer
	Visit        *VisitService
	GoldenBug    *GoldenBugService
	Orchestrator *Orchestrator
	Scheduler    Scheduler
}

// Service is the aggregate contract exposed by the friend domain.
type Service interface {
	API() *API
	Limits() *Limits
	Analyzer() *LandAnalyzer
	Visit() *VisitService
	GoldenBug() *GoldenBugService
	Orchestrator() *Orchestrator
	Run(context.Context, RunOptions) (RunResult, error)
	Start(context.Context, time.Duration) error
	Stop() error
}

// FriendService is the concrete aggregate. Its members are intentionally
// exported only through accessors so the dependency graph stays explicit.
type FriendService struct {
	api          *API
	limits       *Limits
	analyzer     *LandAnalyzer
	visit        *VisitService
	goldenBug    *GoldenBugService
	orchestrator *Orchestrator
}

var _ Service = (*FriendService)(nil)

func New(cfg Config) (*FriendService, error) {
	if cfg.Transport == nil && cfg.API == nil {
		return nil, ErrTransportRequired
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.API == nil {
		api, err := NewAPI(APIConfig{
			Transport: cfg.Transport, AccountID: cfg.AccountID, MyGID: cfg.MyGID,
			Platform: cfg.Platform, Cache: cfg.Cache, Now: cfg.Now,
			QuietHours: cfg.QuietHours,
		})
		if err != nil {
			return nil, err
		}
		cfg.API = api
	}
	if cfg.Limits == nil {
		cfg.Limits = NewLimits(LimitsConfig{Transport: cfg.Transport, HostGID: cfg.MyGID, Now: cfg.Now})
	}
	if cfg.Analyzer == nil {
		cfg.Analyzer = NewLandAnalyzer(LandAnalyzerConfig{
			API: cfg.API, Cache: cfg.Cache, AccountID: cfg.AccountID,
			MyGID: cfg.MyGID, Now: cfg.Now,
		})
	}
	if cfg.Visit == nil {
		cfg.Visit = NewVisitService(VisitConfig{
			Transport: cfg.Transport, AccountID: cfg.AccountID, HostGID: cfg.MyGID,
			API: cfg.API, Limits: cfg.Limits, Analyzer: cfg.Analyzer,
			Warehouse: cfg.Warehouse, Farm: cfg.Farm, Now: cfg.Now,
		})
	}
	if cfg.GoldenBug == nil {
		cfg.GoldenBug = NewGoldenBugService(GoldenBugConfig{
			API: cfg.API, Analyzer: cfg.Analyzer, Limits: cfg.Limits,
			Transport: cfg.Transport, Warehouse: cfg.Warehouse,
			AccountID: cfg.AccountID, MyGID: cfg.MyGID, Cache: cfg.Cache,
			Now: cfg.Now, QuietHours: cfg.QuietHours,
		})
	}
	if cfg.Orchestrator == nil {
		cfg.Orchestrator = NewOrchestrator(OrchestratorConfig{
			API: cfg.API, Limits: cfg.Limits, Analyzer: cfg.Analyzer,
			Visit: cfg.Visit, GoldenBug: cfg.GoldenBug, Farm: cfg.Farm,
			Warehouse: cfg.Warehouse, Cache: cfg.Cache, AccountID: cfg.AccountID,
			MyGID: cfg.MyGID, Scheduler: cfg.Scheduler, Now: cfg.Now,
		})
	}
	return &FriendService{api: cfg.API, limits: cfg.Limits, analyzer: cfg.Analyzer,
		visit: cfg.Visit, goldenBug: cfg.GoldenBug, orchestrator: cfg.Orchestrator}, nil
}

func NewService(cfg Config) (*FriendService, error) { return New(cfg) }

func (s *FriendService) API() *API {
	if s == nil {
		return nil
	}
	return s.api
}
func (s *FriendService) Limits() *Limits {
	if s == nil {
		return nil
	}
	return s.limits
}
func (s *FriendService) Analyzer() *LandAnalyzer {
	if s == nil {
		return nil
	}
	return s.analyzer
}
func (s *FriendService) Visit() *VisitService {
	if s == nil {
		return nil
	}
	return s.visit
}
func (s *FriendService) GoldenBug() *GoldenBugService {
	if s == nil {
		return nil
	}
	return s.goldenBug
}
func (s *FriendService) Orchestrator() *Orchestrator {
	if s == nil {
		return nil
	}
	return s.orchestrator
}

func (s *FriendService) Run(ctx context.Context, options RunOptions) (RunResult, error) {
	if s == nil || s.orchestrator == nil {
		return RunResult{}, ErrAPIRequired
	}
	return s.orchestrator.RunOnce(ctx, options)
}
func (s *FriendService) Start(ctx context.Context, interval time.Duration) error {
	if s == nil || s.orchestrator == nil {
		return ErrAPIRequired
	}
	return s.orchestrator.Start(ctx, interval)
}
func (s *FriendService) Stop() error {
	if s == nil || s.orchestrator == nil {
		return nil
	}
	return s.orchestrator.Stop()
}

func accountText(value string) string { return strings.TrimSpace(value) }
