package app

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/RoseKhlifa/FarmBot/assets"
	"github.com/RoseKhlifa/FarmBot/internal/account"
	"github.com/RoseKhlifa/FarmBot/internal/config"
	"github.com/RoseKhlifa/FarmBot/internal/httpapi"
	"github.com/RoseKhlifa/FarmBot/internal/httpapi/handlers"
	"github.com/RoseKhlifa/FarmBot/internal/httpapi/middleware"
	"github.com/RoseKhlifa/FarmBot/internal/httpapi/realtime"
	platformmetrics "github.com/RoseKhlifa/FarmBot/internal/platform/metrics"
	"github.com/RoseKhlifa/FarmBot/internal/store"
	"github.com/RoseKhlifa/FarmBot/internal/yyb"
	"github.com/gin-gonic/gin"
)

func newApplication(cfg config.Config) (*Application, error) {
	if cfg.AdminPort == 0 {
		loaded := config.Load()
		if cfg.DataDir == "" {
			cfg.DataDir = loaded.DataDir
		}
		if cfg.Paths.DataDir == "" {
			cfg.Paths = loaded.Paths
		}
		if cfg.AdminPort == 0 {
			cfg.AdminPort = loaded.AdminPort
		}
	}
	if cfg.Paths.DataDir == "" || cfg.Paths.Embedded == nil {
		cfg.Paths = config.NewPaths(cfg.DataDir, cfg.ResourceDir, assets.EmbeddedFS())
	}
	if cfg.DataDir == "" {
		cfg.DataDir = cfg.Paths.DataDir
	}

	db, err := store.Open(cfg)
	if err != nil {
		return nil, fmt.Errorf("open application store: %w", err)
	}
	cleanup := func() { _ = db.Close() }

	accounts := store.NewAccountRepo(db)
	cache := store.NewCacheRepo(db)
	configRepo := store.NewConfigRepo(db)
	stats := store.NewStatsRepo(db)
	users := store.NewUserRepo(db)
	cards := store.NewCardRepo(db)
	if err := users.InitializeDefaultAdmin(context.Background()); err != nil {
		cleanup()
		return nil, fmt.Errorf("initialize default admin: %w", err)
	}

	yybDB, err := yyb.NewDB(db)
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("initialize yyb store: %w", err)
	}
	yybService, err := yyb.NewService(yybDB, yyb.ServiceConfig{})
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("initialize yyb service: %w", err)
	}

	sessions, err := middleware.NewSessionManager(middleware.SessionManagerConfig{
		ConfigRepo: configRepo,
		UserRepo:   users,
	})
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("initialize admin sessions: %w", err)
	}
	sessions.Start(context.Background())

	manager := account.NewManager(account.ManagerConfig{
		Accounts: accounts,
		Config:   configRepo,
		Yyb:      yybService,
		AppID:    cfg.TSDK.AppKey,
		Context:  context.Background(),
		RuntimeDependencies: account.Dependencies{
			Stats: stats,
		},
	})

	metrics := platformmetrics.New(platformmetrics.Config{
		AccountSource: func(ctx context.Context) ([]platformmetrics.AccountSnapshot, error) {
			rows, err := accounts.List(ctx)
			if err != nil {
				return nil, err
			}
			result := make([]platformmetrics.AccountSnapshot, 0, len(rows))
			for _, row := range rows {
				runtime := manager.Get(row.ID)
				result = append(result, platformmetrics.AccountSnapshot{
					AccountID: row.ID,
					Online:    runtime != nil && runtime.Status().Running,
				})
			}
			return result, nil
		},
	})

	app := &Application{
		Config: cfg, DB: db, Accounts: accounts, Cache: cache, ConfigDB: configRepo,
		Stats: stats, Users: users, Cards: cards, YybDB: yybDB, Yyb: yybService,
		AccountManager: manager, Sessions: sessions, Metrics: metrics,
	}

	accountAccess := newAccountProvider(accounts, manager)
	handlerApp := &handlers.Application{
		Accounts: accountAccess,
		Cache:    cache,
		Config:   configRepo,
		Users:    users,
		Cards:    cards,
		Sessions: sessions,
		Auth:     authProvider{users: users},
		Yyb:      newYybProvider(yybService),
		QR:       newQRProvider(yybService),
		Public:   publicProvider{config: configRepo, users: users},
	}

	realtimeHub := realtime.NewHub(realtime.Config{
		Sessions: sessions,
		AccountAccess: middleware.AccountAccessConfig{
			Repo: accounts,
		},
		Snapshot: realtime.SnapshotProvider{},
	})
	app.Realtime = realtimeHub

	server, err := httpapi.NewServer(cfg, httpapi.ServerOptions{
		WebFS:        assets.WebDistFS(),
		GameConfigFS: assets.GameConfigFS(),
		RegisterRoutes: func(router *gin.Engine) {
			router.Use(middleware.CORS(), middleware.AuthGate(sessions), middleware.Timeout(0), handlers.MetricsHTTPMiddleware(metrics))
			handlers.RegisterRoutes(router, handlerApp)
			realtimeHub.RegisterRoutes(router)
			handlers.RegisterMetricsRoutes(router, handlers.MetricsRouteConfig{Registry: metrics, Sessions: sessions})
		},
	})
	if err != nil {
		_ = manager.Close()
		sessions.Close()
		cleanup()
		return nil, fmt.Errorf("initialize HTTP server: %w", err)
	}
	app.Server = server
	return app, nil
}

type authProvider struct{ users store.UserRepo }

func (p authProvider) Login(ctx context.Context, username, password string) (store.User, error) {
	if p.users == nil {
		return store.User{}, errors.New("user repository is not configured")
	}
	user, err := p.users.Authenticate(ctx, username, password, "")
	if err != nil || user == nil {
		return store.User{}, err
	}
	return *user, nil
}

func (p authProvider) Register(ctx context.Context, username, password string) (store.User, error) {
	user := store.User{Username: strings.TrimSpace(username), Password: password}
	if err := p.users.Create(ctx, user); err != nil {
		return store.User{}, err
	}
	created, err := p.users.Get(ctx, user.Username)
	if err != nil || created == nil {
		return store.User{}, err
	}
	return *created, nil
}

type accountProvider struct {
	repo    store.AccountRepo
	manager *account.Manager
}

func newAccountProvider(repo store.AccountRepo, manager *account.Manager) *accountProvider {
	return &accountProvider{repo: repo, manager: manager}
}

func (p *accountProvider) List(ctx context.Context) ([]store.Account, error) { return p.repo.List(ctx) }
func (p *accountProvider) Start(_ context.Context, id string) error          { return p.manager.Start(id) }
func (p *accountProvider) Stop(_ context.Context, id string) error           { return p.manager.Stop(id) }
func (p *accountProvider) Delete(ctx context.Context, id string) error {
	if p.manager != nil {
		_ = p.manager.Stop(id)
	}
	return p.repo.Delete(ctx, id)
}
func (p *accountProvider) SetRemark(ctx context.Context, id, remark string) error {
	row, err := p.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	row.Remark = remark
	return p.repo.Upsert(ctx, *row)
}

type publicProvider struct {
	config store.ConfigRepo
	users  store.UserRepo
}

func (p publicProvider) Value(ctx context.Context, key string) (any, error) {
	switch key {
	case "user-count":
		users, err := p.users.List(ctx)
		return map[string]any{"count": len(users)}, err
	case "anti-resale-config":
		return p.config.GetAntiResaleConfig(ctx)
	case "changelog":
		return map[string]any{"version": "go"}, nil
	default:
		return map[string]any{}, nil
	}
}

type yybProvider struct{ service yyb.Service }

func newYybProvider(service yyb.Service) handlers.YybProvider { return yybProvider{service: service} }

func (p yybProvider) Handle(ctx context.Context, route string, body map[string]any) (any, error) {
	switch route {
	case "/api/yyb/accounts":
		return p.service.ListAccounts(ctx)
	case "/api/yyb/getcode", "/api/yyb/thirdparty-code":
		openid, _ := body["openid"].(string)
		appID, _ := body["appId"].(string)
		return p.service.GetCode(ctx, openid, appID)
	case "/api/yyb/qr/create":
		return p.service.QRCreate(ctx)
	case "/api/yyb/qr/poll":
		id, _ := body["sessionId"].(string)
		return p.service.QRPoll(ctx, id)
	case "/api/yyb/qr/confirm":
		id, _ := body["sessionId"].(string)
		return p.service.QRConfirm(ctx, id)
	default:
		return nil, fmt.Errorf("unsupported yyb route %q", route)
	}
}

type qrProvider struct{ service yyb.Service }

func newQRProvider(service yyb.Service) handlers.QRProvider { return qrProvider{service: service} }
func (p qrProvider) Handle(ctx context.Context, route string, body map[string]any) (any, error) {
	if route == "/api/qr/create" {
		return p.service.QRCreate(ctx)
	}
	id, _ := body["sessionId"].(string)
	return p.service.QRPoll(ctx, id)
}
