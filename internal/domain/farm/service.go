package farm

import (
	"context"
	"errors"

	"github.com/RoseKhlifa/FarmBot/internal/domain/warehouse"
	"github.com/RoseKhlifa/FarmBot/internal/game/pb"
)

var ErrServiceAPIRequired = errors.New("farm service API is required")

// Service is the single public aggregate for the farm domain. HTTP handlers
// and account composition code depend on this contract instead of reaching
// through seven legacy service modules.
type Service interface {
	API() API
	Analytics() *Analytics
	Analyzer() *LandAnalyzer
	Fertilizer() *Fertilizer
	Planter() *Planter
	Orchestrator() *Orchestrator
	Run(context.Context, OperationType) (FarmOperationResult, error)
	RunFarmOperation(context.Context, string) (FarmOperationResult, error)
	StartLoop(context.Context) error
	StopLoop() error
	Start(context.Context) error
	Stop() error
	Close() error
}

// ServiceConfig wires the aggregate. Supplying pre-built components is useful
// for tests and for composition roots that share an account-local catalog.
// Nil components are constructed from the other fields where possible.
type ServiceConfig struct {
	API               API
	Transport         GameTransport
	HostGID           int64
	OnOperationLimits func([]*pb.OperationLimit)
	Catalog           Catalog
	Warehouse         warehouse.Service
	Analytics         *Analytics

	Analyzer         *LandAnalyzer
	Fertilizer       *Fertilizer
	FertilizerConfig FertilizerConfig
	Planter          *Planter
	PlantingConfig   PlantingConfig

	Orchestrator       *Orchestrator
	OrchestratorConfig OrchestratorConfig
}

// Config is the conventional constructor name used by the other domain
// packages. It remains an alias so callers can choose the more descriptive
// ServiceConfig without creating a second configuration shape.
type Config = ServiceConfig

// FarmService is the concrete aggregate implementation.
type FarmService struct {
	api          API
	analytics    *Analytics
	analyzer     *LandAnalyzer
	fertilizer   *Fertilizer
	planter      *Planter
	orchestrator *Orchestrator
}

var _ Service = (*FarmService)(nil)

func NewService(config ServiceConfig) (*FarmService, error) {
	if config.API == nil && config.Transport != nil {
		api, err := NewAPI(APIConfig{Transport: config.Transport, HostGID: config.HostGID, OnOperationLimits: config.OnOperationLimits})
		if err != nil {
			return nil, err
		}
		config.API = api
	}
	if config.API == nil && config.Orchestrator != nil {
		config.API = config.Orchestrator.api
	}
	if config.API == nil {
		return nil, ErrServiceAPIRequired
	}
	if config.Analyzer == nil {
		config.Analyzer = NewLandAnalyzer(config.Catalog, config.API)
	}
	if config.Analytics == nil {
		config.Analytics = NewAnalytics(config.Catalog)
	}
	if config.Fertilizer == nil {
		fertilizer, err := NewFertilizer(config.API, config.FertilizerConfig)
		if err != nil {
			return nil, err
		}
		config.Fertilizer = fertilizer
	}
	if config.Planter == nil {
		planter, err := NewPlanter(config.API, config.Warehouse, config.Catalog, config.PlantingConfig)
		if err != nil {
			return nil, err
		}
		config.Planter = planter
	}
	if config.Planter != nil && config.Planter.analytics == nil {
		config.Planter.analytics = config.Analytics
	}
	if config.Orchestrator == nil {
		orchestratorConfig := config.OrchestratorConfig
		orchestratorConfig.API = config.API
		orchestratorConfig.Analyzer = config.Analyzer
		orchestratorConfig.Fertilizer = config.Fertilizer
		orchestratorConfig.Planter = config.Planter
		orchestrator, err := NewOrchestrator(orchestratorConfig)
		if err != nil {
			return nil, err
		}
		config.Orchestrator = orchestrator
	}
	return &FarmService{
		api:          config.API,
		analytics:    config.Analytics,
		analyzer:     config.Analyzer,
		fertilizer:   config.Fertilizer,
		planter:      config.Planter,
		orchestrator: config.Orchestrator,
	}, nil
}

// New is kept as the concise constructor used by other domain packages.
func New(config ServiceConfig) (*FarmService, error) { return NewService(config) }

func (s *FarmService) API() API {
	if s == nil {
		return nil
	}
	return s.api
}

func (s *FarmService) Analytics() *Analytics {
	if s == nil {
		return nil
	}
	return s.analytics
}

func (s *FarmService) Analyzer() *LandAnalyzer {
	if s == nil {
		return nil
	}
	return s.analyzer
}

func (s *FarmService) Fertilizer() *Fertilizer {
	if s == nil {
		return nil
	}
	return s.fertilizer
}

func (s *FarmService) Planter() *Planter {
	if s == nil {
		return nil
	}
	return s.planter
}

func (s *FarmService) Orchestrator() *Orchestrator {
	if s == nil {
		return nil
	}
	return s.orchestrator
}

func (s *FarmService) Run(ctx context.Context, operation OperationType) (FarmOperationResult, error) {
	if s == nil || s.orchestrator == nil {
		return FarmOperationResult{}, ErrServiceAPIRequired
	}
	return s.orchestrator.Run(ctx, operation)
}

func (s *FarmService) RunFarmOperation(ctx context.Context, operation string) (FarmOperationResult, error) {
	if s == nil || s.orchestrator == nil {
		return FarmOperationResult{}, ErrServiceAPIRequired
	}
	return s.orchestrator.RunFarmOperation(ctx, operation)
}

func (s *FarmService) RunOperation(ctx context.Context, operation string) (FarmOperationResult, error) {
	return s.RunFarmOperation(ctx, operation)
}

func (s *FarmService) StartLoop(ctx context.Context) error {
	if s == nil || s.orchestrator == nil {
		return ErrServiceAPIRequired
	}
	return s.orchestrator.StartLoop(ctx)
}

func (s *FarmService) StopLoop() error {
	if s == nil || s.orchestrator == nil {
		return nil
	}
	return s.orchestrator.StopLoop()
}

func (s *FarmService) Start(ctx context.Context) error { return s.StartLoop(ctx) }

func (s *FarmService) Stop() error { return s.StopLoop() }

func (s *FarmService) Close() error { return s.StopLoop() }
