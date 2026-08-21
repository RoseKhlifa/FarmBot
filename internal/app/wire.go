package app

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/RoseKhlifa/FarmBot/assets"
	"github.com/RoseKhlifa/FarmBot/internal/account"
	"github.com/RoseKhlifa/FarmBot/internal/audit"
	"github.com/RoseKhlifa/FarmBot/internal/backup"
	"github.com/RoseKhlifa/FarmBot/internal/config"
	secretcrypto "github.com/RoseKhlifa/FarmBot/internal/crypto"
	"github.com/RoseKhlifa/FarmBot/internal/domain/activity"
	"github.com/RoseKhlifa/FarmBot/internal/domain/career"
	"github.com/RoseKhlifa/FarmBot/internal/domain/farm"
	"github.com/RoseKhlifa/FarmBot/internal/domain/friend"
	"github.com/RoseKhlifa/FarmBot/internal/domain/illustrated"
	"github.com/RoseKhlifa/FarmBot/internal/domain/mall"
	"github.com/RoseKhlifa/FarmBot/internal/domain/social"
	"github.com/RoseKhlifa/FarmBot/internal/domain/task"
	"github.com/RoseKhlifa/FarmBot/internal/domain/warehouse"
	"github.com/RoseKhlifa/FarmBot/internal/game/session"
	"github.com/RoseKhlifa/FarmBot/internal/game/transport"
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
	secretBox, err := secretcrypto.NewSecretBoxFromValue(cfg.MasterKey)
	if err != nil {
		return nil, fmt.Errorf("configure credential encryption: %w", err)
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
	if err := users.InitializeDefaultAdmin(context.Background(), cfg.AdminPassword); err != nil {
		cleanup()
		return nil, fmt.Errorf("initialize default admin: %w", err)
	}

	yybDB, err := yyb.NewDBWithSecretBox(db, secretBox)
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

	var manager *account.Manager
	var realtimeHub *realtime.Hub
	logs := newRuntimeLogStore()
	realtimeHub = realtime.NewHub(realtime.Config{
		Sessions:      sessions,
		AccountAccess: middleware.AccountAccessConfig{Repo: accounts},
		Snapshot: realtime.SnapshotProvider{
			Status: func(_ context.Context, id string) (any, error) {
				if manager == nil {
					return map[string]any{"accountId": id, "running": false}, nil
				}
				runtime := manager.Get(id)
				if runtime == nil {
					return map[string]any{"accountId": id, "running": false}, nil
				}
				return runtime.Status(), nil
			},
			Logs:        logs.Logs,
			AccountLogs: func(ctx context.Context, limit int) (any, error) { return logs.AccountLogs(ctx, "all", limit) },
		},
	})
	manager = account.NewManager(account.ManagerConfig{
		Accounts: accounts,
		Config:   configRepo,
		Yyb:      yybService,
		// The YYB service owns the WeChat mini-program AppID. Do not pass the
		// game TSDK app key here; its default value ("0") is not a valid YYB ID.
		AppID:        "",
		CodeProvider: accountCodeProvider(yybService),
		Context:      context.Background(),
		RuntimeDependencies: account.Dependencies{
			Stats: stats,
		},
		RuntimeFactory: func(spec account.RuntimeSpec) *account.Runtime {
			deps := spec.Dependencies
			deps.StatusChanged = func(snapshot account.StatusSnapshot) {
				logs.Append(snapshot.AccountID, map[string]any{"accountId": snapshot.AccountID, "status": snapshot})
				if realtimeHub != nil {
					realtimeHub.PublishStatus(snapshot.AccountID, snapshot)
				}
			}
			deps.Event = func(accountID string, event transport.Event) {
				entry := map[string]any{"accountId": accountID, "type": event.Type, "name": event.Name, "reason": event.Reason, "reasonCode": event.ReasonCode}
				logs.Append(accountID, entry)
				if realtimeHub != nil {
					realtimeHub.PublishLog(entry)
					realtimeHub.PublishAccountLog(entry)
				}
			}
			deps.Initialize = func(ctx context.Context, runtime *account.Runtime, gameSession *session.Session) error {
				return initializeRuntimeDomains(ctx, runtime, gameSession, spec.Account, spec.AccountConfig, accounts, cache, configRepo, stats)
			}
			return account.NewRuntime(spec.RuntimeConfig, deps)
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

	capture := newCaptureProvider(configRepo, accounts, cache, users, manager)
	app := &Application{
		Config: cfg, DB: db, Accounts: accounts, Cache: cache, ConfigDB: configRepo,
		Stats: stats, Users: users, Cards: cards, YybDB: yybDB, Yyb: yybService,
		AccountManager: manager, Sessions: sessions, Metrics: metrics,
		Capture: capture,
	}
	auditRepo := &audit.Repository{DB: db}

	accountAccess := newAccountProvider(accounts, manager)
	handlerApp := &handlers.Application{
		Accounts:      accountAccess,
		Cache:         cache,
		Config:        configRepo,
		Users:         users,
		Cards:         cards,
		Sessions:      sessions,
		Auth:          authProvider{users: users},
		Yyb:           newYybProvider(yybService),
		QR:            newQRProvider(yybService),
		Public:        publicProvider{config: configRepo, users: users},
		Runtime:       runtimeProvider{manager: manager, accounts: accounts},
		Logs:          logs,
		Domains:       runtimeDomains(manager),
		Capture:       capture,
		Proxy:         newProxyProvider(cfg),
		Audit:         auditRepo,
		ExportAccount: func(ctx context.Context, id string) ([]byte, error) { return backup.ExportAccount(ctx, db, id) },
	}
	app.Realtime = realtimeHub

	server, err := httpapi.NewServer(cfg, httpapi.ServerOptions{
		WebFS:        assets.WebDistFS(),
		GameConfigFS: assets.GameConfigFS(),
		Ready: func(ctx context.Context) error {
			if err := db.PingContext(ctx); err != nil {
				return err
			}
			return nil
		},
		RegisterRoutes: func(router *gin.Engine) {
			router.Use(middleware.CORS(), middleware.AuthGate(sessions), middleware.RequireAdminAPI(), middleware.RequireSuperAdminAPI(), middleware.AccountAccess(middleware.AccountAccessConfig{Repo: accounts}), middleware.Timeout(0), middleware.AuditLog(auditRepo), handlers.MetricsHTTPMiddleware(metrics))
			handlers.RegisterRoutes(router, handlerApp)
			realtimeHub.RegisterRoutes(router)
			handlers.RegisterMetricsRoutes(router, handlers.MetricsRouteConfig{Registry: metrics, Sessions: sessions})
		},
	})
	if err != nil {
		_ = capture.Close()
		_ = manager.Close()
		sessions.Close()
		cleanup()
		return nil, fmt.Errorf("initialize HTTP server: %w", err)
	}
	app.Server = server
	return app, nil
}

type authProvider struct{ users store.UserRepo }

func (p authProvider) Login(ctx context.Context, username, password, ip string) (store.User, error) {
	if p.users == nil {
		return store.User{}, errors.New("user repository is not configured")
	}
	user, err := p.users.Authenticate(ctx, username, password, ip)
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
func (p *accountProvider) Restart(_ context.Context, id string) error        { return p.manager.Restart(id) }
func (p *accountProvider) IsRunning(id string) bool {
	return p.manager != nil && p.manager.Get(id) != nil
}
func (p *accountProvider) GetConfig(ctx context.Context, id string) (*store.AccountConfig, error) {
	return p.repo.GetConfig(ctx, id)
}
func (p *accountProvider) ApplyConfigSnapshot(ctx context.Context, id string, config store.AccountConfig) error {
	return p.repo.ApplyConfigSnapshot(ctx, id, config)
}
func (p *accountProvider) Create(ctx context.Context, row store.Account) (store.Account, error) {
	if strings.TrimSpace(row.ID) == "" {
		return store.Account{}, errors.New("account ID is required")
	}
	// The web client uses the same endpoint for create and partial edits. Merge
	// an existing row before Upsert so a name/remark-only edit cannot erase the
	// login code or provider fields that were not included in the request.
	if existing, err := p.repo.Get(ctx, row.ID); err == nil && existing != nil {
		merged := *existing
		if row.Name != "" {
			merged.Name = row.Name
		}
		if row.Code != "" {
			merged.Code = row.Code
		}
		if row.Platform != "" {
			merged.Platform = row.Platform
		}
		if row.LoginType != "" {
			merged.LoginType = row.LoginType
		}
		if row.Provider != "" {
			merged.Provider = row.Provider
		}
		if row.WXID != "" {
			merged.WXID = row.WXID
		}
		if row.UIN != "" {
			merged.UIN = row.UIN
		}
		if row.QQ != "" {
			merged.QQ = row.QQ
		}
		if row.GID != "" {
			merged.GID = row.GID
		}
		if row.OpenID != "" {
			merged.OpenID = row.OpenID
		}
		if row.Avatar != "" {
			merged.Avatar = row.Avatar
		}
		if row.OwnerUser != "" {
			merged.OwnerUser = row.OwnerUser
		}
		if row.YYBOpenID != "" {
			merged.YYBOpenID = row.YYBOpenID
		}
		if row.TenantID != "" {
			merged.TenantID = row.TenantID
		}
		if row.Remark != "" {
			merged.Remark = row.Remark
		}
		if len(row.ThirdPartyJSON) > 0 && string(row.ThirdPartyJSON) != "{}" {
			merged.ThirdPartyJSON = mergeThirdPartyJSON(merged.ThirdPartyJSON, row.ThirdPartyJSON)
		}
		// Running is runtime state, not an edit payload field. Preserve it here;
		// explicit start/stop endpoints own that transition.
		row = merged
	}
	if err := p.repo.Upsert(ctx, row); err != nil {
		return store.Account{}, err
	}
	if _, err := p.repo.GetConfig(ctx, row.ID); err != nil {
		if err := p.repo.ApplyConfigSnapshot(ctx, row.ID, store.AccountConfig{AccountID: row.ID}); err != nil {
			return store.Account{}, err
		}
	}
	created, err := p.repo.Get(ctx, row.ID)
	if err != nil || created == nil {
		return store.Account{}, err
	}
	return *created, nil
}

func mergeThirdPartyJSON(existing, incoming json.RawMessage) json.RawMessage {
	current := objectJSON(existing)
	patch := objectJSON(incoming)
	for key, value := range patch {
		// The edit form deliberately sends an empty apiToken when the user does
		// not want to rotate credentials. Preserve the stored secret in that
		// case while still allowing boolean/numeric options to be changed.
		if text, ok := value.(string); ok && strings.TrimSpace(text) == "" {
			if previous, exists := current[key].(string); exists && strings.TrimSpace(previous) != "" {
				continue
			}
		}
		current[key] = value
	}
	raw, err := json.Marshal(current)
	if err != nil {
		return incoming
	}
	return raw
}
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
		if p.config == nil {
			return map[string]any{}, nil
		}
		return p.config.GetAntiResaleConfig(ctx)
	case "changelog":
		return map[string]any{"version": "go"}, nil
	case "login-links", "announcement":
		if p.config == nil {
			return map[string]any{}, nil
		}
		raw, err := p.getGlobal(ctx, key, legacyPublicKey(key))
		if errors.Is(err, sql.ErrNoRows) {
			return map[string]any{}, nil
		}
		if err != nil {
			return nil, err
		}
		var value any
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, fmt.Errorf("decode public config %q: %w", key, err)
		}
		return value, nil
	case "super-admin-announcement":
		if p.config == nil {
			return map[string]any{"content": "", "updatedAt": int64(0)}, nil
		}
		raw, err := p.getGlobal(ctx, key, legacyPublicKey(key))
		if errors.Is(err, sql.ErrNoRows) {
			return map[string]any{"content": "", "updatedAt": int64(0)}, nil
		}
		if err != nil {
			return nil, err
		}
		var value struct {
			Content   string `json:"content"`
			UpdatedAt int64  `json:"updatedAt"`
		}
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, fmt.Errorf("decode super-admin announcement: %w", err)
		}
		return value, nil
	case "system-info":
		return map[string]any{"version": "go"}, nil
	default:
		return map[string]any{}, nil
	}
}

func (p publicProvider) getGlobal(ctx context.Context, key, legacy string) (json.RawMessage, error) {
	raw, err := p.config.GetGlobal(ctx, key)
	if errors.Is(err, sql.ErrNoRows) && legacy != "" {
		return p.config.GetGlobal(ctx, legacy)
	}
	return raw, err
}

func legacyPublicKey(key string) string {
	switch key {
	case "login-links":
		return "legacy:loginLinks"
	case "super-admin-announcement":
		return "legacy:superAdminAnnouncement"
	case "announcement":
		return "legacy:announcement"
	default:
		return ""
	}
}

type yybProvider struct {
	service yyb.Service
	client  *http.Client
}

func newYybProvider(service yyb.Service) handlers.YybProvider {
	return yybProvider{service: service, client: &http.Client{Timeout: 30 * time.Second}}
}

func accountCodeProvider(service yyb.Service) func(context.Context, store.Account, *store.AccountConfig, string) (string, error) {
	external := yybProvider{service: service, client: &http.Client{Timeout: 30 * time.Second}}
	return func(ctx context.Context, account store.Account, _ *store.AccountConfig, appID string) (string, error) {
		thirdParty := objectJSON(account.ThirdPartyJSON)
		if !strings.EqualFold(strings.TrimSpace(account.Provider), "thirdparty") {
			if service == nil {
				return "", errors.New("yyb service is not configured")
			}
			ref := strings.TrimSpace(account.YYBOpenID)
			if ref == "" {
				ref = strings.TrimSpace(account.OpenID)
			}
			return service.GetCode(ctx, ref, appID)
		}
		ref := strings.TrimSpace(account.YYBOpenID)
		if ref == "" {
			ref = strings.TrimSpace(account.OpenID)
		}
		thirdParty["openid"] = ref
		result, err := external.externalThirdParty(ctx, stringValue(thirdParty, "apiBase"), thirdParty)
		if err != nil {
			return "", err
		}
		code := stringValue(objectFromJSONValue(result), "code")
		if code == "" {
			return "", errors.New("第三方接口未返回 code")
		}
		return code, nil
	}
}

func (p yybProvider) Handle(ctx context.Context, route string, body map[string]any) (any, error) {
	// Keep compatibility with installations that still point the UI at a
	// standalone yyb-go or third-party service. The normal path remains the
	// in-process service when no per-request endpoint is supplied.
	if strings.TrimSpace(stringValue(body, "apiBase")) != "" || strings.TrimSpace(stringValue(body, "apiKey")) != "" || route == "/api/yyb/thirdparty-code" {
		result, err := p.external(ctx, route, body)
		if err != nil {
			return nil, yybExternalError(err)
		}
		return result, nil
	}
	switch route {
	case "/api/yyb/accounts":
		return p.service.ListAccounts(ctx)
	case "/api/yyb/getcode", "/api/yyb/thirdparty-code":
		openid, _ := body["openid"].(string)
		appID, _ := body["appId"].(string)
		code, err := p.service.GetCode(ctx, openid, appID)
		if err != nil {
			return nil, err
		}
		// Keep the in-process endpoint contract identical to the external YYB
		// adapter: callers receive {data: {code: "..."}}, not a bare string.
		return map[string]any{"code": code, "openid": openid}, nil
	case "/api/yyb/qr/create":
		return p.service.QRCreate(ctx)
	case "/api/yyb/qr/poll":
		id, _ := body["sessionId"].(string)
		return p.service.QRPoll(ctx, id)
	case "/api/yyb/qr/confirm":
		id, _ := body["sessionId"].(string)
		result, err := p.service.QRConfirm(ctx, id)
		if err != nil {
			return nil, err
		}
		if result.Account == nil {
			return nil, errors.New("应用宝扫码未返回账号")
		}
		// Never expose login_buffer or protocol credentials to the browser. The
		// frontend only needs the public identity to create the FarmBot account
		// link after the YYB identity has been persisted.
		return map[string]any{"account": result.Account.Public()}, nil
	default:
		return nil, fmt.Errorf("unsupported yyb route %q", route)
	}
}

func yybExternalError(err error) error {
	if err == nil {
		return nil
	}
	var httpErr *handlers.HTTPError
	if errors.As(err, &httpErr) && httpErr != nil {
		return err
	}
	return &handlers.HTTPError{Status: http.StatusBadRequest, Err: err}
}

func (p yybProvider) external(ctx context.Context, route string, body map[string]any) (any, error) {
	base := strings.TrimRight(strings.TrimSpace(stringValue(body, "apiBase")), "/")
	if route == "/api/yyb/thirdparty-code" {
		return p.externalThirdParty(ctx, base, body)
	}
	if base == "" {
		return nil, errors.New("应用宝接口地址未配置")
	}
	parsed, err := url.Parse(base)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.User != nil {
		return nil, errors.New("应用宝接口地址格式无效")
	}
	base = normalizeYybBase(parsed, false)
	token := strings.TrimSpace(stringValue(body, "apiKey"))
	if token == "" {
		return nil, errors.New("应用宝 API Token 未配置")
	}
	path := "/accounts"
	method := http.MethodGet
	var payload any
	switch route {
	case "/api/yyb/accounts":
		// The accounts endpoint uses the default GET /accounts path.
	case "/api/yyb/getcode":
		openid := strings.TrimSpace(stringValue(body, "openid"))
		if openid == "" {
			return nil, errors.New("缺少 openid")
		}
		appID := strings.TrimSpace(stringValue(body, "appId"))
		if appID == "" {
			appID = "wx5306c5978fdb76e4"
		}
		path, method, payload = "/wxapp/getCode", http.MethodPost, map[string]any{"ref": openid, "app_id": appID}
	case "/api/yyb/qr/create":
		path, method = "/qr?as_base64=true", http.MethodPost
	case "/api/yyb/qr/poll":
		sessionID := strings.TrimSpace(stringValue(body, "sessionId"))
		if sessionID == "" {
			return nil, errors.New("缺少 sessionId")
		}
		path = "/qr/" + url.PathEscape(sessionID) + "/poll"
	case "/api/yyb/qr/confirm":
		sessionID := strings.TrimSpace(stringValue(body, "sessionId"))
		if sessionID == "" {
			return nil, errors.New("缺少 sessionId")
		}
		path, method = "/qr/"+url.PathEscape(sessionID)+"/confirm", http.MethodPost
	default:
		return nil, fmt.Errorf("unsupported yyb route %q", route)
	}
	result, err := p.externalRequest(ctx, base+path, method, token, payload)
	if err != nil {
		return nil, err
	}
	if route == "/api/yyb/getcode" {
		data := objectFromJSONValue(result)
		code := nestedString(data, "result", "code")
		if code == "" {
			code = stringValue(data, "code")
		}
		if code == "" {
			return nil, errors.New("应用宝接口未返回 code")
		}
		return map[string]any{"code": code, "openid": stringValue(data, "openid")}, nil
	}
	return result, nil
}

func (p yybProvider) externalThirdParty(ctx context.Context, base string, body map[string]any) (any, error) {
	if base == "" {
		return nil, errors.New("第三方接口地址未配置")
	}
	parsed, err := url.Parse(base)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.User != nil {
		return nil, errors.New("第三方接口地址格式无效")
	}
	base = normalizeYybBase(parsed, true)
	token := strings.TrimSpace(stringValue(body, "apiToken"))
	if token == "" {
		return nil, errors.New("第三方 API Token 未配置")
	}
	openid := strings.TrimSpace(stringValue(body, "openid"))
	if openid == "" {
		return nil, errors.New("缺少 openid")
	}
	result, err := p.externalRequest(ctx, base+"/api/open/v1/farm/code", http.MethodPost, token, map[string]any{"openid": openid, "forceRefresh": body["forceRefresh"] == true})
	if err != nil {
		return nil, err
	}
	data := objectFromJSONValue(result)
	code := stringValue(data, "code")
	if code == "" {
		code = nestedString(data, "result", "code")
	}
	if code == "" {
		return nil, errors.New("第三方接口未返回 code")
	}
	return map[string]any{"code": code, "openid": openid}, nil
}

func (p yybProvider) externalRequest(ctx context.Context, endpoint, method, token string, payload any) (any, error) {
	var reader io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	client := p.client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	var value any
	if len(raw) == 0 || json.Unmarshal(raw, &value) != nil {
		err := fmt.Errorf("应用宝接口返回非 JSON（HTTP %d）", resp.StatusCode)
		if resp.StatusCode >= 400 {
			return nil, &handlers.HTTPError{Status: resp.StatusCode, Err: err}
		}
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, &handlers.HTTPError{
			Status: resp.StatusCode,
			Err:    fmt.Errorf("应用宝接口请求失败（HTTP %d）：%s", resp.StatusCode, externalErrorMessage(value)),
		}
	}
	if object, ok := value.(map[string]any); ok {
		if code, present := object["code"].(float64); present && code != 0 {
			return nil, &handlers.HTTPError{
				Status: http.StatusBadRequest,
				Code:   strconv.FormatInt(int64(code), 10),
				Err:    fmt.Errorf("应用宝接口错误 code=%d：%s", int64(code), stringValue(object, "msg")),
			}
		}
		if data, present := object["data"]; present {
			return data, nil
		}
		if success, present := object["success"].(bool); present && !success {
			return nil, &handlers.HTTPError{Status: http.StatusBadRequest, Err: errors.New(externalErrorMessage(value))}
		}
	}
	return value, nil
}

func objectFromJSONValue(value any) map[string]any {
	if object, ok := value.(map[string]any); ok {
		return object
	}
	return map[string]any{}
}

func nestedString(value map[string]any, parent, key string) string {
	nested, _ := value[parent].(map[string]any)
	return stringValue(nested, key)
}

func externalErrorMessage(value any) string {
	if object, ok := value.(map[string]any); ok {
		for _, key := range []string{"error", "msg", "message"} {
			if message := stringValue(object, key); message != "" {
				return message
			}
		}
	}
	return "未知错误"
}

func normalizeYybBase(parsed *url.URL, thirdParty bool) string {
	path := strings.TrimRight(parsed.Path, "/")
	suffixes := []string{"/wxapp/getCode", "/wxapp", "/accounts", "/qr"}
	if thirdParty {
		suffixes = []string{"/api/open/v1/farm/code", "/api/open/v1", "/api/open"}
	}
	for _, suffix := range suffixes {
		if strings.HasSuffix(strings.ToLower(path), strings.ToLower(suffix)) {
			path = strings.TrimSuffix(path, path[len(path)-len(suffix):])
			break
		}
	}
	return strings.TrimRight(parsed.Scheme+"://"+parsed.Host+path, "/")
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

type captureFlow struct {
	ID                 string         `json:"id"`
	Token              string         `json:"token"`
	Owner              string         `json:"-"`
	RemoteSessionID    string         `json:"-"`
	CertificateToken   string         `json:"-"`
	AccountID          string         `json:"accountId,omitempty"`
	Platform           string         `json:"platform"`
	Code               string         `json:"-"`
	AccountGID         string         `json:"accountGid,omitempty"`
	OpenID             string         `json:"-"`
	FriendGIDs         []int64        `json:"-"`
	FriendSource       string         `json:"-"`
	FriendListComplete bool           `json:"-"`
	PublicInfo         map[string]any `json:"-"`
	Proxy              map[string]any `json:"-"`
	CaptureStatus      string         `json:"captureStatus"`
	Completed          bool           `json:"completed"`
	Completing         bool           `json:"-"`
	Cancelled          bool           `json:"-"`
	Result             any            `json:"result,omitempty"`
	CreatedAt          int64          `json:"createdAt"`
	UpdatedAt          int64          `json:"updatedAt"`
	Payload            map[string]any `json:"payload,omitempty"`
}

type captureProvider struct {
	config      store.ConfigRepo
	accounts    store.AccountRepo
	cache       store.CacheRepo
	users       store.UserRepo
	manager     *account.Manager
	client      *http.Client
	mu          sync.RWMutex
	flows       map[string]*captureFlow
	janitorStop chan struct{}
	janitorDone chan struct{}
	closeOnce   sync.Once
}

const captureFlowTTL = 15 * time.Minute

func newCaptureProvider(config store.ConfigRepo, accounts store.AccountRepo, cache store.CacheRepo, users store.UserRepo, manager *account.Manager) *captureProvider {
	p := &captureProvider{config: config, accounts: accounts, cache: cache, users: users, manager: manager, client: &http.Client{Timeout: 15 * time.Second}, flows: make(map[string]*captureFlow), janitorStop: make(chan struct{}), janitorDone: make(chan struct{})}
	go p.runJanitor()
	return p
}

func (p *captureProvider) runJanitor() {
	ticker := time.NewTicker(time.Minute)
	defer func() {
		ticker.Stop()
		close(p.janitorDone)
	}()
	for {
		select {
		case <-ticker.C:
			p.cleanupExpired(context.Background())
		case <-p.janitorStop:
			return
		}
	}
}

func (p *captureProvider) Close() error {
	if p == nil {
		return nil
	}
	p.closeOnce.Do(func() {
		close(p.janitorStop)
		<-p.janitorDone
		p.cleanupExpired(context.Background())
		p.mu.Lock()
		remaining := make([]*captureFlow, 0, len(p.flows))
		for id, flow := range p.flows {
			delete(p.flows, id)
			if flow != nil && !flow.Completed {
				remaining = append(remaining, flow)
			}
		}
		p.mu.Unlock()
		for _, flow := range remaining {
			_ = p.stopRemoteFlow(context.Background(), flow)
		}
	})
	return nil
}

func (p *captureProvider) Handle(ctx context.Context, route string, body map[string]any) (any, error) {
	if p == nil {
		return nil, errors.New("capture provider is not configured")
	}
	switch {
	case route == "/api/admin/capture-config" && strings.EqualFold(stringValue(body, "_method"), http.MethodGet):
		return p.getConfig(ctx)
	case route == "/api/admin/capture-config":
		return p.saveConfig(ctx, body)
	case route == "/api/admin/capture-config/test":
		return p.testConfig(ctx, body)
	case route == "/api/capture/config":
		return p.getConfig(ctx)
	case route == "/api/capture/sessions":
		return p.createFlow(ctx, body)
	case strings.HasPrefix(route, "/api/capture/sessions/") && strings.HasSuffix(route, "/complete"):
		return p.completeFlow(ctx, stringValue(body, "flowId"), body)
	case strings.HasPrefix(route, "/api/capture/sessions/") && strings.EqualFold(stringValue(body, "_method"), http.MethodGet):
		if id := stringValue(body, "flowId"); id != "" {
			return p.getFlow(ctx, id, stringValue(body, "_username"))
		}
	case strings.HasPrefix(route, "/api/public/capture-certificate/"):
		return p.certificate(ctx, stringValue(body, "flowId"), stringValue(body, "token"))
	case strings.HasPrefix(route, "/api/capture/sessions/") && strings.EqualFold(stringValue(body, "_method"), http.MethodDelete):
		id := stringValue(body, "flowId")
		if id == "" {
			return nil, errors.New("capture flow id is required")
		}
		if err := p.deleteFlow(ctx, id, stringValue(body, "_username")); err != nil {
			return nil, err
		}
		return map[string]any{"deleted": true, "id": id}, nil
	}
	return nil, fmt.Errorf("unsupported capture route %q", route)
}

func captureError(status int, message string) error {
	return &handlers.HTTPError{Status: status, Err: errors.New(message)}
}

func captureErrorCode(status int, code, message string) error {
	return &handlers.HTTPError{Status: status, Code: code, Err: errors.New(message)}
}

// cleanupExpired removes stale in-memory flows and releases their remote
// sessions. The map entry is removed before the network call so a slow remote
// service cannot block a new flow from being created.
func (p *captureProvider) cleanupExpired(ctx context.Context) {
	cutoff := time.Now().Add(-captureFlowTTL).UnixMilli()
	p.mu.Lock()
	expired := make([]*captureFlow, 0)
	for id, flow := range p.flows {
		if flow == nil || flow.Completing || flow.UpdatedAt >= cutoff {
			continue
		}
		flow.Cancelled = true
		delete(p.flows, id)
		expired = append(expired, flow)
	}
	p.mu.Unlock()
	for _, flow := range expired {
		_ = p.stopRemoteFlow(ctx, flow)
	}
}

func (p *captureProvider) removeOwnerFlows(ctx context.Context, owner string) {
	owner = strings.TrimSpace(owner)
	if owner == "" {
		return
	}
	p.mu.Lock()
	removed := make([]*captureFlow, 0)
	for id, flow := range p.flows {
		if flow == nil || flow.Owner != owner || flow.Completed || flow.Completing {
			continue
		}
		flow.Cancelled = true
		delete(p.flows, id)
		removed = append(removed, flow)
	}
	p.mu.Unlock()
	for _, flow := range removed {
		_ = p.stopRemoteFlow(ctx, flow)
	}
}

func (p *captureProvider) stopRemoteFlow(ctx context.Context, flow *captureFlow) error {
	if flow == nil || strings.TrimSpace(flow.RemoteSessionID) == "" {
		return nil
	}
	cfg, err := p.loadCaptureConfig(ctx, nil)
	if err != nil {
		return err
	}
	path := "/api/sessions/" + url.PathEscape(flow.RemoteSessionID)
	if _, err := p.remote(ctx, cfg, http.MethodDelete, path, flow.RemoteSessionID, nil); err == nil {
		return nil
	} else {
		// Older capture-service versions expose stop separately and may reject
		// DELETE once the proxy has already transitioned state.
		if _, stopErr := p.remote(ctx, cfg, http.MethodPost, "/api/capture/stop", flow.RemoteSessionID, map[string]any{}); stopErr == nil {
			return nil
		} else {
			return err
		}
	}
}

func (p *captureProvider) getConfig(ctx context.Context) (any, error) {
	cfg, err := p.loadCaptureConfig(ctx, nil)
	if err != nil {
		return nil, err
	}
	return map[string]any{"enabled": cfg.Enabled && cfg.APIBase != "" && cfg.APIToken != "", "apiBase": cfg.APIBase, "apiToken": "", "tokenConfigured": cfg.APIToken != "", "autoImportQqGids": cfg.AutoImportQQGIDs}, nil
}
func (p *captureProvider) saveConfig(ctx context.Context, body map[string]any) (any, error) {
	if p.config == nil {
		return nil, errors.New("capture config store is not configured")
	}
	cfg, err := p.loadCaptureConfig(ctx, body)
	if err != nil {
		return nil, err
	}
	if cfg.Enabled && (cfg.APIBase == "" || cfg.APIToken == "") {
		return nil, errors.New("启用前请填写抓包服务地址和 API Token")
	}
	raw, err := json.Marshal(map[string]any{"enabled": cfg.Enabled, "apiBase": cfg.APIBase, "apiToken": cfg.APIToken, "autoImportQqGids": cfg.AutoImportQQGIDs})
	if err != nil {
		return nil, err
	}
	if err := p.config.SetGlobal(ctx, "captureConfig", raw); err != nil {
		return nil, err
	}
	return map[string]any{"enabled": cfg.Enabled, "apiBase": cfg.APIBase, "apiToken": "", "tokenConfigured": cfg.APIToken != "", "autoImportQqGids": cfg.AutoImportQQGIDs}, nil
}

type captureConfig struct {
	Enabled          bool   `json:"enabled"`
	APIBase          string `json:"apiBase"`
	APIToken         string `json:"apiToken"`
	AutoImportQQGIDs bool   `json:"autoImportQqGids"`
}

func (p *captureProvider) loadCaptureConfig(ctx context.Context, override map[string]any) (captureConfig, error) {
	cfg := captureConfig{AutoImportQQGIDs: true}
	if p.config != nil {
		raw, err := p.config.GetGlobal(ctx, "captureConfig")
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return cfg, err
		}
		if len(raw) > 0 {
			_ = json.Unmarshal(raw, &cfg)
		}
	}
	if override != nil {
		if value, ok := override["enabled"].(bool); ok {
			cfg.Enabled = value
		}
		if value := stringValue(override, "apiBase"); value != "" {
			cfg.APIBase = value
		}
		if value := stringValue(override, "apiToken"); value != "" {
			cfg.APIToken = value
		}
		if value, ok := override["autoImportQqGids"].(bool); ok {
			cfg.AutoImportQQGIDs = value
		}
	}
	cfg.APIBase = strings.TrimRight(strings.TrimSpace(cfg.APIBase), "/")
	if cfg.APIBase != "" {
		parsed, err := url.Parse(cfg.APIBase)
		if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.User != nil {
			return cfg, errors.New("抓包服务地址格式无效")
		}
		parsed.RawQuery, parsed.Fragment = "", ""
		cfg.APIBase = strings.TrimRight(parsed.String(), "/")
	}
	cfg.APIToken = strings.TrimSpace(cfg.APIToken)
	return cfg, nil
}

func (p *captureProvider) testConfig(ctx context.Context, body map[string]any) (any, error) {
	cfg, err := p.loadCaptureConfig(ctx, body)
	if err != nil {
		return nil, err
	}
	result, err := p.remote(ctx, cfg, http.MethodGet, "/api/health", "", nil)
	if err != nil {
		return nil, err
	}
	return map[string]any{"uptime": numberValue(result, "uptime"), "sessions": numberValue(result, "sessions"), "portPoolSize": arrayLength(result, "portPool")}, nil
}

func (p *captureProvider) remote(ctx context.Context, cfg captureConfig, method, path, sessionID string, payload any) (map[string]any, error) {
	if cfg.APIBase == "" || cfg.APIToken == "" {
		return nil, errors.New("抓包服务未配置")
	}
	var reader io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, cfg.APIBase+path, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.APIToken)
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if sessionID != "" {
		req.Header.Set("x-capture-session-id", sessionID)
	}
	client := p.client
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = response.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(response.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	var result map[string]any
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &result); err != nil {
			return nil, fmt.Errorf("抓包服务返回了无效响应（HTTP %d）", response.StatusCode)
		}
	}
	if result == nil {
		result = map[string]any{}
	}
	if response.StatusCode >= 400 {
		if message := stringValue(result, "error"); message != "" {
			return nil, fmt.Errorf("抓包服务请求失败（HTTP %d）：%s", response.StatusCode, message)
		}
		return nil, fmt.Errorf("抓包服务请求失败（HTTP %d）", response.StatusCode)
	}
	if ok, exists := result["ok"].(bool); exists && !ok {
		message := stringValue(result, "error")
		if message == "" {
			message = "抓包服务拒绝了请求"
		}
		return nil, errors.New(message)
	}
	return result, nil
}

func captureData(result map[string]any) map[string]any {
	if nested, ok := result["data"].(map[string]any); ok {
		return nested
	}
	if nested, ok := result["state"].(map[string]any); ok {
		return nested
	}
	return result
}

func (p *captureProvider) addCapturedValues(flow *captureFlow, response map[string]any) {
	data := captureData(response)
	if value, ok := data["publicInfo"].(map[string]any); ok {
		flow.PublicInfo = value
	}
	if value, ok := data["proxy"].(map[string]any); ok {
		flow.Proxy = value
	}
	if channels, ok := data["channels"].(map[string]any); ok {
		if channel, ok := channels[flow.Platform].(map[string]any); ok {
			if status := stringValue(channel, "status"); status != "" {
				flow.CaptureStatus = status
			}
			if codes, ok := channel["codes"].([]any); ok {
				for _, item := range codes {
					entry, _ := item.(map[string]any)
					if flow.Code == "" {
						flow.Code = stringValue(entry, "code")
					}
					if flow.AccountGID == "" {
						flow.AccountGID = stringValue(entry, "gid")
					}
					if flow.OpenID == "" {
						flow.OpenID = stringValue(entry, "openid")
						if flow.OpenID == "" {
							flow.OpenID = stringValue(entry, "open_id")
						}
					}
				}
			}
		}
	}
	if friends, ok := data["friends"].(map[string]any); ok {
		if source := stringValue(friends, "source"); source != "" {
			flow.FriendSource = source
		}
		if items, ok := friends["items"].([]any); ok {
			seen := make(map[int64]bool, len(flow.FriendGIDs))
			for _, value := range flow.FriendGIDs {
				seen[value] = true
			}
			for _, item := range items {
				entry, _ := item.(map[string]any)
				gid, _ := strconv.ParseInt(stringValue(entry, "gid"), 10, 64)
				if gid > 0 && !seen[gid] {
					flow.FriendGIDs = append(flow.FriendGIDs, gid)
					seen[gid] = true
				}
			}
		}
	}
	flow.UpdatedAt = time.Now().UnixMilli()
}

func (p *captureProvider) createFlow(ctx context.Context, payload map[string]any) (any, error) {
	owner := stringValue(payload, "_username")
	if owner == "" {
		return nil, captureError(http.StatusUnauthorized, "未登录")
	}
	cfg, err := p.loadCaptureConfig(ctx, nil)
	if err != nil {
		return nil, err
	}
	if !cfg.Enabled {
		return nil, captureError(http.StatusForbidden, "抓包登录添加账号未启用")
	}
	p.cleanupExpired(ctx)
	// A user can only have one active capture proxy. Replacing an unfinished
	// flow also releases its remote session before allocating a new one.
	p.removeOwnerFlows(ctx, owner)
	platform := "qq"
	if strings.EqualFold(stringValue(payload, "platform"), "wx") {
		platform = "wx"
	}
	accountID := stringValue(payload, "accountId")
	if accountID != "" && p.accounts != nil {
		row, err := p.accounts.Get(ctx, accountID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, captureError(http.StatusNotFound, "目标账号不存在")
			}
			return nil, err
		}
		if row != nil && !strings.EqualFold(strings.TrimSpace(row.OwnerUser), owner) && stringValue(payload, "_admin") != "true" {
			return nil, captureError(http.StatusForbidden, "无权访问此账号")
		}
	}
	var random [18]byte
	if _, err := rand.Read(random[:]); err != nil {
		return nil, err
	}
	remoteID := fmt.Sprintf("capture-%x", random[:])
	flow := &captureFlow{ID: remoteID, Token: fmt.Sprintf("%x", random[:6]), Owner: owner, RemoteSessionID: remoteID, CertificateToken: fmt.Sprintf("%x", random[6:]), AccountID: accountID, Platform: platform, CaptureStatus: "idle", CreatedAt: time.Now().UnixMilli(), UpdatedAt: time.Now().UnixMilli(), PublicInfo: map[string]any{}, Proxy: map[string]any{}, Payload: payload}
	created, err := p.remote(ctx, cfg, http.MethodPost, "/api/sessions", remoteID, map[string]any{"sessionId": remoteID})
	if err != nil {
		return nil, err
	}
	p.addCapturedValues(flow, created)
	started, err := p.remote(ctx, cfg, http.MethodPost, "/api/capture/start", remoteID, map[string]any{"mode": platform})
	if err != nil {
		_ = p.stopRemoteFlow(context.Background(), flow)
		return nil, err
	}
	p.addCapturedValues(flow, started)
	p.mu.Lock()
	p.flows[flow.ID] = flow
	p.mu.Unlock()
	return p.serializeFlow(flow), nil
}

func (p *captureProvider) getFlow(ctx context.Context, id, owner string) (any, error) {
	p.mu.RLock()
	flow := p.flows[id]
	if flow == nil || flow.Owner != owner {
		p.mu.RUnlock()
		return nil, captureError(http.StatusNotFound, "抓取任务不存在或已过期")
	}
	completed, remoteID := flow.Completed, flow.RemoteSessionID
	p.mu.RUnlock()
	if !completed {
		cfg, err := p.loadCaptureConfig(ctx, nil)
		if err != nil {
			return nil, err
		}
		state, err := p.remote(ctx, cfg, http.MethodGet, "/api/sessions/"+url.PathEscape(remoteID)+"/state", remoteID, nil)
		if err != nil {
			return nil, err
		}
		p.mu.Lock()
		if current := p.flows[id]; current == flow && !flow.Cancelled {
			p.addCapturedValues(flow, state)
		} else {
			p.mu.Unlock()
			return nil, captureError(http.StatusNotFound, "抓取任务不存在或已过期")
		}
		p.mu.Unlock()
	}
	return p.serializeFlow(flow), nil
}

func (p *captureProvider) completeFlow(ctx context.Context, id string, payload map[string]any) (any, error) {
	p.mu.Lock()
	flow := p.flows[id]
	if flow == nil || flow.Owner != stringValue(payload, "_username") {
		p.mu.Unlock()
		return nil, captureError(http.StatusNotFound, "抓取任务不存在或已过期")
	}
	if flow.Completed {
		result := flow.Result
		p.mu.Unlock()
		return result, nil
	}
	if flow.Completing {
		p.mu.Unlock()
		return nil, captureError(http.StatusConflict, "账号正在添加中")
	}
	flow.Completing = true
	remoteID := flow.RemoteSessionID
	code := strings.TrimSpace(flow.Code)
	p.mu.Unlock()
	defer func() { p.mu.Lock(); flow.Completing = false; p.mu.Unlock() }()
	if code == "" {
		cfg, err := p.loadCaptureConfig(ctx, nil)
		if err != nil {
			return nil, err
		}
		state, err := p.remote(ctx, cfg, http.MethodGet, "/api/sessions/"+url.PathEscape(remoteID)+"/state", remoteID, nil)
		if err != nil {
			return nil, err
		}
		p.mu.Lock()
		if current := p.flows[id]; current != flow || flow.Cancelled {
			p.mu.Unlock()
			return nil, captureError(http.StatusNotFound, "抓取任务不存在或已过期")
		}
		p.addCapturedValues(flow, state)
		code = strings.TrimSpace(flow.Code)
		p.mu.Unlock()
	}
	if code == "" {
		return nil, captureError(http.StatusBadRequest, "尚未获取到 Code")
	}
	if p.accounts == nil {
		return nil, errors.New("account repository is not configured")
	}
	p.mu.RLock()
	if current := p.flows[id]; current != flow || flow.Cancelled {
		p.mu.RUnlock()
		return nil, captureError(http.StatusNotFound, "抓取任务不存在或已过期")
	}
	owner, accountID, platform := flow.Owner, flow.AccountID, flow.Platform
	accountGID, openID := strings.TrimSpace(flow.AccountGID), strings.TrimSpace(flow.OpenID)
	friends := append([]int64(nil), flow.FriendGIDs...)
	p.mu.RUnlock()
	accounts, err := p.accounts.List(ctx)
	if err != nil {
		return nil, err
	}
	var existing *store.Account
	duplicate := false
	for i := range accounts {
		if strings.TrimSpace(accounts[i].ID) == accountID && accountID != "" {
			existing = &accounts[i]
		}
		if !strings.EqualFold(strings.TrimSpace(accounts[i].ID), strings.TrimSpace(accountID)) && captureAccountDuplicate(accounts[i], platform, code, accountGID) {
			duplicate = true
			break
		}
	}
	if duplicate {
		p.mu.Lock()
		flow.Cancelled = true
		if current := p.flows[id]; current == flow {
			delete(p.flows, id)
		}
		p.mu.Unlock()
		go func() { _ = p.stopRemoteFlow(context.Background(), flow) }()
		return nil, captureErrorCode(http.StatusConflict, "DUPLICATE_CAPTURE_ACCOUNT", "检测到当前仍是已添加的账号，请先切换到目标 QQ，再重新开始抓取")
	}
	admin := stringValue(payload, "_admin") == "true"
	if existing != nil && !admin && !strings.EqualFold(existing.OwnerUser, owner) {
		return nil, captureError(http.StatusForbidden, "无权访问此账号")
	}
	if existing == nil && !admin {
		userAccounts, err := p.accounts.GetByUser(ctx, owner)
		if err != nil {
			return nil, err
		}
		user, err := p.userForCapture(ctx, owner)
		if err != nil {
			return nil, err
		}
		if user.AccountLimit > 0 && len(userAccounts) >= user.AccountLimit {
			return nil, captureError(http.StatusForbidden, "账号数量已达到配额上限")
		}
	}
	if existing == nil {
		var random [8]byte
		_, _ = rand.Read(random[:])
		existing = &store.Account{ID: "account-" + fmt.Sprintf("%x", random[:]), OwnerUser: owner}
	}
	row := *existing
	row.Name = stringValue(payload, "name")
	if row.Name == "" {
		row.Name = existing.Name
	}
	row.Code, row.Platform, row.LoginType = code, platform, "capture"
	if accountGID != "" {
		row.GID = accountGID
	}
	if openID != "" {
		row.OpenID = openID
	}
	row.OwnerUser = owner
	if err := p.accounts.Upsert(ctx, row); err != nil {
		return nil, err
	}
	importedFriendCount := 0
	if cfg, cfgErr := p.loadCaptureConfig(ctx, nil); cfgErr == nil && cfg.AutoImportQQGIDs && normalizeCapturePlatform(platform) == "qq" {
		friends = normalizeCaptureGIDs(friends, row.GID)
		importedFriendCount = len(friends)
		if p.cache != nil && len(friends) > 0 {
			raw, _ := json.Marshal(friends)
			_ = p.cache.PutKnownFriendGIDs(ctx, row.ID, store.CacheValue{Payload: raw})
		}
	}
	// Account startup performs a real game login and may take several seconds.
	// Complete the capture transaction first, then start asynchronously so the
	// browser can render the saved account and its eventual status.
	if p.manager != nil && !existing.Running && p.manager.Get(row.ID) == nil {
		go func(accountID string) {
			if err := p.manager.Start(accountID); err != nil {
				p.mu.Lock()
				if current := p.flows[id]; current == flow && current.Result != nil {
					if resultMap, ok := current.Result.(map[string]any); ok {
						resultMap["startError"] = err.Error()
					}
				}
				p.mu.Unlock()
			}
		}(row.ID)
	}
	result := map[string]any{"accountId": row.ID, "name": row.Name, "platform": row.Platform, "importedFriendCount": importedFriendCount, "startError": "", "updated": accountID != ""}
	p.mu.Lock()
	if current := p.flows[id]; current == flow && !flow.Cancelled {
		flow.Completed, flow.Result, flow.Payload, flow.UpdatedAt = true, result, payload, time.Now().UnixMilli()
	}
	p.mu.Unlock()
	go func() { _ = p.stopRemoteFlow(context.Background(), flow) }()
	return result, nil
}

func captureAccountDuplicate(account store.Account, platform, code, gid string) bool {
	if normalizeCapturePlatform(account.Platform) != normalizeCapturePlatform(platform) {
		return false
	}
	return (code != "" && strings.TrimSpace(account.Code) == code) ||
		(gid != "" && strings.TrimSpace(account.GID) == gid)
}

func normalizeCapturePlatform(value string) string {
	if strings.EqualFold(strings.TrimSpace(value), "wx") {
		return "wx"
	}
	return "qq"
}

func (p *captureProvider) userForCapture(ctx context.Context, username string) (store.User, error) {
	if p.users == nil {
		return store.User{}, errors.New("user repository is not configured")
	}
	user, err := p.users.Get(ctx, username)
	if err != nil {
		return store.User{}, err
	}
	if user == nil {
		return store.User{}, store.ErrUserNotFound
	}
	return *user, nil
}

func (p *captureProvider) certificate(ctx context.Context, id, token string) (any, error) {
	p.mu.RLock()
	flow := p.flows[id]
	valid := flow != nil && subtle.ConstantTimeCompare([]byte(flow.CertificateToken), []byte(token)) == 1
	certPath := "/cert/mitmproxy-ca-cert.cer"
	if valid {
		if value := stringValue(flow.PublicInfo, "certUrl"); value != "" {
			certPath = value
		}
	}
	p.mu.RUnlock()
	if !valid {
		return nil, captureError(http.StatusNotFound, "证书链接不存在或已过期")
	}
	cfg, err := p.loadCaptureConfig(ctx, nil)
	if err != nil {
		return nil, err
	}
	if !strings.HasPrefix(certPath, "/") || strings.HasPrefix(certPath, "//") {
		return nil, errors.New("抓包服务证书地址无效")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, cfg.APIBase+certPath, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+cfg.APIToken)
	client := p.client
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer func() { _ = response.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(response.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	if response.StatusCode >= 400 {
		return nil, fmt.Errorf("证书下载失败（HTTP %d）", response.StatusCode)
	}
	return handlers.BinaryResponse{Data: data, ContentType: "application/x-x509-ca-cert", Filename: "mitmproxy-ca-cert.cer"}, nil
}

func (p *captureProvider) serializeFlow(flow *captureFlow) map[string]any {
	p.mu.RLock()
	defer p.mu.RUnlock()
	proxy := make(map[string]any, len(flow.Proxy))
	for key, value := range flow.Proxy {
		proxy[key] = value
	}
	publicInfo := map[string]any{"host": stringValue(flow.PublicInfo, "host"), "mitmPort": numberValue(flow.PublicInfo, "mitmPort"), "certificateUrl": "/api/public/capture-certificate/" + url.PathEscape(flow.ID) + "/" + url.PathEscape(flow.CertificateToken)}
	return map[string]any{"id": flow.ID, "platform": flow.Platform, "codeCaptured": flow.Code != "", "accountGid": flow.AccountGID, "friendCount": len(flow.FriendGIDs), "captureStatus": flow.CaptureStatus, "proxy": proxy, "publicInfo": publicInfo, "completed": flow.Completed, "result": flow.Result}
}

func (p *captureProvider) deleteFlow(ctx context.Context, id, owner string) error {
	p.mu.Lock()
	flow := p.flows[id]
	if flow == nil {
		p.mu.Unlock()
		return nil
	}
	if flow.Owner != owner {
		p.mu.Unlock()
		return captureError(http.StatusNotFound, "抓取任务不存在或已过期")
	}
	flow.Cancelled = true
	delete(p.flows, id)
	p.mu.Unlock()
	// The local flow is already gone; a transient remote cleanup failure must
	// not make the browser retry a delete that can no longer be found.
	_ = p.stopRemoteFlow(ctx, flow)
	return nil
}

func normalizeCaptureGIDs(values []int64, own string) []int64 {
	ownID, _ := strconv.ParseInt(strings.TrimSpace(own), 10, 64)
	seen := map[int64]bool{}
	result := make([]int64, 0, len(values))
	for _, value := range values {
		if value <= 0 || value == ownID || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}

func numberValue(value map[string]any, key string) any {
	if value == nil {
		return float64(0)
	}
	if number, ok := value[key].(float64); ok {
		return number
	}
	return float64(0)
}

func arrayLength(value map[string]any, key string) int {
	items, _ := value[key].([]any)
	return len(items)
}

type proxyProvider struct {
	url    string
	key    string
	appID  string
	client *http.Client
}

func newProxyProvider(cfg config.Config) handlers.ProxyProvider {
	return &proxyProvider{url: strings.TrimSpace(cfg.WxProxy.APIURL), key: cfg.WxProxy.APIKey, appID: cfg.WxProxy.AppID, client: &http.Client{Timeout: 30 * time.Second}}
}
func (p *proxyProvider) Handle(ctx context.Context, body map[string]any) (any, error) {
	if p == nil || p.url == "" {
		return map[string]any{"configured": false, "ok": false}, nil
	}
	target, err := url.Parse(p.url)
	if err != nil || target.Scheme == "" || target.Host == "" {
		return nil, fmt.Errorf("invalid proxy URL")
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target.String(), bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if p.key != "" {
		req.Header.Set("x-proxy-api-key", p.key)
	}
	if p.appID != "" {
		req.Header.Set("x-proxy-app-id", p.appID)
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	var result any
	if json.Unmarshal(data, &result) != nil {
		result = string(data)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("proxy upstream returned HTTP %d", resp.StatusCode)
	}
	return result, nil
}

func stringValue(body map[string]any, key string) string {
	if body == nil {
		return ""
	}
	value, _ := body[key].(string)
	return strings.TrimSpace(value)
}

// runtimeProvider adapts AccountManager to the handler-facing runtime
// contract. It deliberately returns an offline snapshot for stopped accounts
// so read-only status pages can render without a toast-worthy server error.
type runtimeProvider struct {
	manager  *account.Manager
	accounts store.AccountRepo
}

func runtimeDomains(manager *account.Manager) handlers.DomainProviders {
	return handlers.DomainProviders{
		Farm: func(ctx context.Context, id string) (farm.Service, error) {
			return resolveRuntimeDomain[farm.Service](manager, id, "farm")
		},
		Friend: func(ctx context.Context, id string) (friend.Service, error) {
			return resolveRuntimeDomain[friend.Service](manager, id, "friend")
		},
		Warehouse: func(ctx context.Context, id string) (warehouse.Service, error) {
			return resolveRuntimeDomain[warehouse.Service](manager, id, "warehouse")
		},
		Mall: func(ctx context.Context, id string) (*mall.Domains, error) {
			return resolveRuntimeDomain[*mall.Domains](manager, id, "mall")
		},
		Task: func(ctx context.Context, id string) (task.Service, error) {
			return resolveRuntimeDomain[task.Service](manager, id, "task")
		},
		Activity: func(ctx context.Context, id string) (activity.Service, error) {
			return resolveRuntimeDomain[activity.Service](manager, id, "activity")
		},
		Career: func(ctx context.Context, id string) (career.Service, error) {
			return resolveRuntimeDomain[career.Service](manager, id, "career")
		},
		Illustrated: func(ctx context.Context, id string) (illustrated.Service, error) {
			return resolveRuntimeDomain[illustrated.Service](manager, id, "illustrated")
		},
		Social: func(ctx context.Context, id string) (social.Service, error) {
			return resolveRuntimeDomain[social.Service](manager, id, "social")
		},
	}
}

func resolveRuntimeDomain[T any](manager *account.Manager, id, name string) (T, error) {
	var zero T
	if manager == nil {
		return zero, fmt.Errorf("account manager is not configured")
	}
	runtime := manager.Get(strings.TrimSpace(id))
	if runtime == nil {
		return zero, fmt.Errorf("%w: account %q is offline", account.ErrAccountOffline, strings.TrimSpace(id))
	}
	service, ok := runtime.Domain(name).(T)
	if !ok || (any(service) == nil) {
		return zero, fmt.Errorf("account %q domain %q is not initialized", id, name)
	}
	return service, nil
}

func (p runtimeProvider) Status(ctx context.Context, id string) (any, error) {
	if p.manager != nil {
		if runtime := p.manager.Get(id); runtime != nil {
			return runtime.Status(), nil
		}
	}
	if p.accounts == nil {
		return account.StatusSnapshot{AccountID: id, Phase: account.PhaseOffline}, nil
	}
	row, err := p.accounts.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return account.StatusSnapshot{AccountID: row.ID, Phase: account.PhaseOffline, Name: row.Name, OpenID: row.OpenID}, nil
}

func (p runtimeProvider) Scheduler(ctx context.Context, id string) (any, error) {
	if p.manager == nil {
		return account.SchedulerStatus{}, nil
	}
	if runtime := p.manager.Get(id); runtime != nil {
		return runtime.SchedulerStatus(), nil
	}
	return account.SchedulerStatus{}, nil
}

func (p runtimeProvider) Automation(ctx context.Context, id string, patch json.RawMessage) (any, error) {
	if p.accounts == nil {
		return nil, errors.New("account repository is not configured")
	}
	current, err := p.accounts.GetConfig(ctx, id)
	if err != nil {
		return nil, err
	}
	var incoming map[string]any
	if err := json.Unmarshal(patch, &incoming); err != nil {
		return nil, fmt.Errorf("decode automation patch: %w", err)
	}
	automation := objectJSON(current.AutomationJSON)
	mergeJSON(automation, incoming)
	automationRaw, err := json.Marshal(automation)
	if err != nil {
		return nil, err
	}
	current.AutomationJSON = automationRaw
	value := objectJSON(current.ConfigJSON)
	value["automation"] = automation
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	current.ConfigJSON = raw
	if err := p.accounts.ApplyConfigSnapshot(ctx, id, *current); err != nil {
		return nil, err
	}
	if p.manager != nil && p.manager.Get(id) != nil {
		if err := p.manager.RestartContext(ctx, id); err != nil {
			return nil, fmt.Errorf("restart account after automation update: %w", err)
		}
	}
	return automation, nil
}

func initializeRuntimeDomains(ctx context.Context, runtime *account.Runtime, gameSession *session.Session, accountRow store.Account, accountConfig *store.AccountConfig, accounts store.AccountRepo, cache store.CacheRepo, configRepo store.ConfigRepo, stats store.StatsRepo) error {
	if runtime == nil || gameSession == nil {
		return errors.New("runtime and game session are required")
	}
	transport := gameSession
	state := gameSession.State()
	accountID := runtime.AccountID()
	automation := accountAutomation(accountConfig)
	fertilizerMode := accountAutomationMode(accountConfig, string(farm.FertilizerSmartNormal))
	automationOn := func(_ context.Context, key string) (bool, error) { return automation[key], nil }

	bag, err := warehouse.New(warehouse.Config{
		Transport: transport, ConfigRepo: configRepo, AccountID: accountID,
		AutomationOn: automationOn,
		IsFruit:      func(id int64) bool { return id >= 40000 && id < 50000 },
		Status: warehouse.StatusCallbacks{
			OnGold: func(value int64) { runtime.SetGold(value) },
			OnOperation: func(name string, value float64) {
				runtime.AddOperation(name, value)
				if stats != nil {
					_, _ = stats.IncrementOperation(context.Background(), accountID, time.Now().Format("2006-01-02"), name, value)
				}
			},
			CurrentGold: func() int64 { return runtime.Status().Gold },
		},
	})
	if err != nil {
		return fmt.Errorf("initialize warehouse: %w", err)
	}

	mallDomains, err := mall.New(mall.Config{
		Transport: transport, Warehouse: bag,
		Automation:            automationOn,
		MysteryAutoBuyEnabled: func(ctx context.Context) (bool, error) { return automationOn(ctx, "mystery_auto_buy") },
		MysteryAutoBuyCurrencies: func(context.Context) ([]int64, error) {
			return accountConfigInt64List(accountConfig, "mysteryAutoBuyCurrencies")
		},
	})
	if err != nil {
		return fmt.Errorf("initialize mall: %w", err)
	}

	landTypes := accountLandTypes(accountConfig)
	strategy := farm.PlantingStrategy(accountString(accountConfig, "plantingStrategy", string(farm.StrategyMaxExp)))
	farmService, err := farm.New(farm.ServiceConfig{
		Transport: transport, HostGID: state.GID, Warehouse: bag,
		PlantingConfig: farm.PlantingConfig{
			Strategy:         strategy,
			PreferredSeedID:  accountInt64(accountConfig, "preferredSeedID"),
			Prioritize2x2:    accountBool(accountConfig, "prioritize2x2Crops"),
			FallbackStrategy: farm.PlantingStrategy(accountString(accountConfig, "bagSeedFallbackStrategy", string(farm.StrategyLevel))),
		},
		FertilizerConfig: farm.FertilizerConfig{
			Mode:           farm.FertilizerMode(fertilizerMode),
			LandTypes:      landTypes,
			SmartThreshold: time.Duration(accountInt64Default(accountConfig, "fertilizerSmartSeconds", 3600)) * time.Second,
			BuyInterval:    time.Duration(accountInt64Default(accountConfig, "fertilizerBuyCheckIntervalMinutes", 10)) * time.Minute,
			BuyCheck: func(ctx context.Context) error {
				_, err := mallDomains.CheckAndBuyFertilizerBoth(ctx, mall.FertilizerCheckOptions{
					BuyOrganic: automation["fertilizer_buy_organic"], OrganicCount: accountInt64Default(accountConfig, "fertilizerBuyOrganicCount", 1), OrganicThresholdHrs: float64(accountInt64Default(accountConfig, "fertilizerBuyOrganicThresholdHours", 10)),
					BuyNormal: automation["fertilizer_buy_normal"], NormalCount: accountInt64Default(accountConfig, "fertilizerBuyNormalCount", 1), NormalThresholdHrs: float64(accountInt64Default(accountConfig, "fertilizerBuyNormalThresholdHours", 10)),
				})
				return err
			},
		},
		OrchestratorConfig: farm.OrchestratorConfig{Scheduler: runtime.DomainScheduler(), SkipOwnWeedBug: automation["skip_own_weed_bug"], GoldenBugClear: automation["golden_bug_clear"], LandUpgrade: automation["land_upgrade"], MultiSeasonFertilizer: automation["fertilizer_multi_season"]},
	})
	if err != nil {
		return fmt.Errorf("initialize farm: %w", err)
	}

	friendService, err := friend.New(friend.Config{
		Transport: transport, AccountID: accountID, MyGID: state.GID,
		Platform: friendPlatform(accountRow.Platform), Cache: cache, Farm: farmService, Warehouse: bag,
		QuietHours: accountQuietHours(accountConfig), Scheduler: nil,
	})
	if err != nil {
		return fmt.Errorf("initialize friend: %w", err)
	}
	taskService, err := task.New(task.Config{Transport: transport, Warehouse: bag, Scheduler: runtime.DomainScheduler(), AutomationOn: automationOn})
	if err != nil {
		return fmt.Errorf("initialize task: %w", err)
	}
	activityService, err := activity.New(activity.Config{Transport: transport, RawTransport: gameSession, Warehouse: bag, Connected: gameSession.Online})
	if err != nil {
		return fmt.Errorf("initialize activity: %w", err)
	}
	careerService, err := career.New(career.Config{Transport: transport})
	if err != nil {
		return fmt.Errorf("initialize career: %w", err)
	}
	illustratedService, err := illustrated.New(illustrated.Config{Transport: transport})
	if err != nil {
		return fmt.Errorf("initialize illustrated: %w", err)
	}
	socialService, err := social.New(social.Config{Transport: transport, HostGID: state.GID})
	if err != nil {
		return fmt.Errorf("initialize social: %w", err)
	}

	runtime.SetDomain("warehouse", bag)
	runtime.SetDomain("mall", mallDomains)
	runtime.SetDomain("farm", farmService)
	runtime.SetDomain("friend", friendService)
	runtime.SetDomain("task", taskService)
	runtime.SetDomain("activity", activityService)
	runtime.SetDomain("career", careerService)
	runtime.SetDomain("illustrated", illustratedService)
	runtime.SetDomain("social", socialService)
	runtime.AddCloser(farmService.Close)
	runtime.AddCloser(friendService.Stop)
	runtime.AddCloser(taskService.Close)
	if automation["mystery_auto_buy"] && mallDomains.Mystery != nil {
		if scheduler := runtime.DomainScheduler(); scheduler != nil {
			if err := mallDomains.Mystery.StartAutoBuy(ctx, scheduler); err != nil {
				return fmt.Errorf("start mystery scheduler: %w", err)
			}
			runtime.AddCloser(func() error {
				mallDomains.Mystery.StopAutoBuy(scheduler)
				return nil
			})
		}
	}
	if err := taskService.Start(ctx); err != nil {
		return fmt.Errorf("start task service: %w", err)
	}

	loops, err := runtime.NewLoops(account.LoopOptions{
		Scheduler: runtime.DomainScheduler(),
		Hooks: account.LoopHooks{
			Farm: func(ctx context.Context) error {
				if !automation["farm"] {
					return nil
				}
				result, err := farmService.Run(ctx, farm.OperationAll)
				if err == nil {
					_ = farmService.Fertilizer().CheckAndBuy(ctx)
				}
				return result.FirstError()
			},
			Help: func(ctx context.Context) error {
				if !automation["friend_help"] {
					return nil
				}
				_, err := friendService.Run(ctx, friend.RunOptions{Mode: friend.ModeHelp, Help: true})
				return err
			},
			Steal: func(ctx context.Context) error {
				if !automation["friend_steal"] {
					return nil
				}
				_, err := friendService.Run(ctx, friend.RunOptions{Mode: friend.ModeSteal, Steal: true})
				return err
			},
			Daily: func(ctx context.Context) error {
				var errs []error
				if automation["task"] {
					_, err := taskService.CheckAndClaimTasks(ctx)
					errs = append(errs, err)
				}
				if mallDomains.Mall != nil && automation["fertilizer_gift"] {
					_, err := mallDomains.BuyFreeGifts(ctx, false)
					errs = append(errs, err)
				}
				if mallDomains.MonthCard != nil {
					_, err := mallDomains.MonthCard.PerformDailyGift(ctx, false)
					errs = append(errs, err)
				}
				if mallDomains.QQVIP != nil {
					_, err := mallDomains.QQVIP.PerformDailyGift(ctx, false)
					errs = append(errs, err)
				}
				if socialService != nil {
					_, err := socialService.PerformDailyShare(ctx, false)
					errs = append(errs, err)
				}
				return errors.Join(nonNilErrors(errs)...)
			},
			OnTaskInfo: taskService.HandleEvent,
		},
	})
	if err != nil {
		return fmt.Errorf("initialize account loops: %w", err)
	}
	runtime.AddEventHandler(loops.HandleEvent)
	runtime.AddCloser(loops.Close)
	if err := loops.Start(ctx); err != nil {
		return fmt.Errorf("start account loops: %w", err)
	}
	return nil
}

type runtimeLogStore struct {
	mu   sync.RWMutex
	data map[string][]any
}

func newRuntimeLogStore() *runtimeLogStore { return &runtimeLogStore{data: make(map[string][]any)} }

func (s *runtimeLogStore) Append(accountID string, entry any) {
	if s == nil || strings.TrimSpace(accountID) == "" || entry == nil {
		return
	}
	s.mu.Lock()
	s.data[accountID] = append(s.data[accountID], entry)
	if len(s.data[accountID]) > 1000 {
		s.data[accountID] = s.data[accountID][len(s.data[accountID])-1000:]
	}
	s.mu.Unlock()
}
func (s *runtimeLogStore) Logs(_ context.Context, accountID string, limit int) (any, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []any
	if strings.TrimSpace(accountID) == "" || accountID == "all" {
		for _, entries := range s.data {
			result = append(result, entries...)
		}
	} else {
		result = append(result, s.data[accountID]...)
	}
	sort.SliceStable(result, func(i, j int) bool { return i > j })
	return limitEntries(result, limit), nil
}
func (s *runtimeLogStore) AccountLogs(ctx context.Context, accountID string, limit int) (any, error) {
	return s.Logs(ctx, accountID, limit)
}
func (s *runtimeLogStore) ClearLogs(_ context.Context, accountID string) error {
	s.mu.Lock()
	delete(s.data, accountID)
	s.mu.Unlock()
	return nil
}

func limitEntries(entries []any, limit int) []any {
	if limit <= 0 {
		limit = 100
	}
	if len(entries) > limit {
		entries = entries[len(entries)-limit:]
	}
	return append([]any(nil), entries...)
}

func nonNilErrors(values []error) []error {
	result := make([]error, 0, len(values))
	for _, err := range values {
		if err != nil {
			result = append(result, err)
		}
	}
	return result
}
func objectJSON(raw json.RawMessage) map[string]any {
	result := map[string]any{}
	_ = json.Unmarshal(raw, &result)
	return result
}
func mergeJSON(target map[string]any, patch map[string]any) {
	for key, value := range patch {
		target[key] = value
	}
}

func accountAutomation(cfg *store.AccountConfig) map[string]bool {
	defaults := map[string]bool{"farm": true, "friend": true, "friend_help": true, "friend_steal": true, "task": true, "sell": false, "fertilizer_buy_organic": false, "fertilizer_buy_normal": false, "fertilizer_gift": false, "mystery_auto_buy": false, "land_upgrade": false, "golden_bug_clear": true, "skip_own_weed_bug": true, "fertilizer_multi_season": true}
	if cfg == nil {
		return defaults
	}
	value := objectJSON(cfg.AutomationJSON)
	for key, raw := range value {
		if flag, ok := raw.(bool); ok {
			defaults[key] = flag
		}
	}
	return defaults
}
func accountAutomationMode(cfg *store.AccountConfig, fallback string) string {
	if cfg == nil {
		return fallback
	}
	value := objectJSON(cfg.AutomationJSON)
	if mode, ok := value["fertilizer"].(string); ok && strings.TrimSpace(mode) != "" {
		return strings.TrimSpace(mode)
	}
	return fallback
}
func accountString(cfg *store.AccountConfig, key, fallback string) string {
	if cfg == nil {
		return fallback
	}
	switch key {
	case "plantingStrategy":
		if strings.TrimSpace(cfg.PlantingStrategy) != "" {
			return cfg.PlantingStrategy
		}
	case "bagSeedFallbackStrategy":
		if strings.TrimSpace(cfg.BagSeedFallbackStrategy) != "" {
			return cfg.BagSeedFallbackStrategy
		}
	}
	value := objectJSON(cfg.ConfigJSON)
	if v, ok := value[key].(string); ok && strings.TrimSpace(v) != "" {
		return v
	}
	return fallback
}
func accountBool(cfg *store.AccountConfig, key string) bool {
	if cfg == nil {
		return false
	}
	switch key {
	case "prioritize2x2Crops":
		return cfg.Prioritize2x2Crops
	case "plantOrderRandom":
		return cfg.PlantOrderRandom
	case "friendHelpExpExhausted":
		return cfg.FriendHelpExpExhausted
	}
	value := objectJSON(cfg.ConfigJSON)
	v, _ := value[key].(bool)
	return v
}
func accountInt64(cfg *store.AccountConfig, key string) int64 {
	if cfg == nil {
		return 0
	}
	switch key {
	case "preferredSeedID":
		return cfg.PreferredSeedID
	case "fertilizerBuyOrganicCount":
		return cfg.FertilizerBuyOrganicCount
	case "fertilizerBuyOrganicThresholdHours":
		return cfg.FertilizerBuyOrganicThresholdHours
	case "fertilizerBuyNormalCount":
		return cfg.FertilizerBuyNormalCount
	case "fertilizerBuyNormalThresholdHours":
		return cfg.FertilizerBuyNormalThresholdHours
	case "fertilizerBuyCheckIntervalMinutes":
		return cfg.FertilizerBuyCheckIntervalMinutes
	case "autoAcceptFriendMinLevel":
		return cfg.AutoAcceptFriendMinLevel
	case "goldenBugKeepCount":
		return cfg.GoldenBugKeepCount
	case "goldenBugRoundLimit":
		return cfg.GoldenBugRoundLimit
	}
	value := objectJSON(cfg.ConfigJSON)
	switch v := value[key].(type) {
	case float64:
		return int64(v)
	case string:
		n, _ := strconv.ParseInt(v, 10, 64)
		return n
	}
	return 0
}
func accountInt64Default(cfg *store.AccountConfig, key string, fallback int64) int64 {
	if value := accountInt64(cfg, key); value > 0 {
		return value
	}
	return fallback
}
func accountConfigInt64List(cfg *store.AccountConfig, key string) ([]int64, error) {
	if cfg == nil {
		return nil, nil
	}
	var rawJSON json.RawMessage
	if key == "mysteryAutoBuyCurrencies" {
		rawJSON = cfg.MysteryAutoBuyCurrenciesJSON
	} else if key == "bagSeedPriority" {
		rawJSON = cfg.BagSeedPriorityJSON
	} else {
		value := objectJSON(cfg.ConfigJSON)
		rawJSON, _ = json.Marshal(value[key])
	}
	var values []any
	if err := json.Unmarshal(rawJSON, &values); err != nil {
		return nil, nil
	}
	result := make([]int64, 0, len(values))
	for _, item := range values {
		switch v := item.(type) {
		case float64:
			if v > 0 {
				result = append(result, int64(v))
			}
		case string:
			n, _ := strconv.ParseInt(v, 10, 64)
			if n > 0 {
				result = append(result, n)
			}
		}
	}
	return result, nil
}
func accountLandTypes(cfg *store.AccountConfig) []farm.LandType {
	if cfg == nil {
		return farm.AllFertilizerLandTypes()
	}
	automation := objectJSON(cfg.AutomationJSON)
	value, ok := automation["fertilizer_land_types"].([]any)
	if !ok {
		return farm.AllFertilizerLandTypes()
	}
	types := make([]farm.LandType, 0, len(value))
	for _, item := range value {
		if text, ok := item.(string); ok {
			types = append(types, farm.LandType(strings.ToLower(strings.TrimSpace(text))))
		}
	}
	return farm.NormalizeFertilizerLandTypes(types)
}
func accountQuietHours(cfg *store.AccountConfig) string {
	if cfg == nil {
		return ""
	}
	var value struct {
		Enabled bool   `json:"enabled"`
		Start   string `json:"start"`
		End     string `json:"end"`
	}
	if json.Unmarshal(cfg.FriendQuietHoursJSON, &value) != nil || !value.Enabled {
		return ""
	}
	if value.Start == "" || value.End == "" {
		return ""
	}
	return value.Start + "-" + value.End
}
func friendPlatform(platform string) friend.Platform {
	if strings.EqualFold(strings.TrimSpace(platform), string(friend.PlatformWechat)) || strings.EqualFold(strings.TrimSpace(platform), "wx") {
		return friend.PlatformWechat
	}
	return friend.PlatformQQ
}
