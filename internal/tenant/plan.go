// Package tenant contains tenant identity, plan and quota enforcement logic.
package tenant

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/RoseKhlifa/FarmBot/internal/store"
)

var (
	ErrTenantRequired          = errors.New("tenant is required")
	ErrTenantNotFound          = errors.New("tenant not found")
	ErrTenantInactive          = errors.New("tenant is inactive")
	ErrAccountQuotaExceeded    = errors.New("tenant account quota exceeded")
	ErrConcurrentQuotaExceeded = errors.New("tenant concurrent quota exceeded")
	ErrFeatureDisabled         = errors.New("tenant feature is disabled")
)

// Plan is the effective quota and feature set for a tenant. A zero limit is
// treated as unlimited, which is useful for the enterprise plan.
type Plan struct {
	Name          string
	MaxAccounts   int
	MaxConcurrent int
	Features      map[string]bool
}

func (p Plan) Normalize() Plan {
	p.Name = strings.ToLower(strings.TrimSpace(p.Name))
	if p.Name == "" {
		p.Name = "starter"
	}
	if p.Features == nil {
		p.Features = map[string]bool{}
	}
	return p
}

// BuiltinPlan provides conservative defaults when a tenant row only stores a
// plan name. Product-specific rows can override every limit and feature.
func BuiltinPlan(name string) Plan {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "pro":
		return Plan{Name: "pro", MaxAccounts: 20, MaxConcurrent: 5, Features: map[string]bool{"realtime": true}}
	case "business", "team":
		return Plan{Name: "business", MaxAccounts: 100, MaxConcurrent: 25, Features: map[string]bool{"realtime": true, "yyb": true}}
	case "enterprise":
		return Plan{Name: "enterprise", Features: map[string]bool{"realtime": true, "yyb": true, "export": true}}
	default:
		return Plan{Name: "starter", MaxAccounts: 3, MaxConcurrent: 1, Features: map[string]bool{"realtime": true}}
	}
}

// Tenant is the durable isolation boundary. TenantID is deliberately a
// separate key from user/account IDs so future shard routing can use it.
type Tenant struct {
	ID        string
	Name      string
	Plan      Plan
	Status    string
	CreatedAt int64
	UpdatedAt int64
}

func (t Tenant) Active() bool {
	return strings.EqualFold(strings.TrimSpace(t.Status), "active") || strings.TrimSpace(t.Status) == ""
}

type Usage struct {
	Accounts   int
	Concurrent int
}

// Store is the minimal persistence surface used by Manager. Keeping it small
// allows middleware tests to use an in-memory fake without a SQLite database.
type Store interface {
	GetTenant(context.Context, string) (*Tenant, error)
	GetUserTenant(context.Context, string) (string, error)
	CountAccounts(context.Context, string) (int, error)
	CountConcurrent(context.Context, string) (int, error)
}

// SQLStore implements Store over the shared FarmBot database.
type SQLStore struct{ DB *store.DB }

func NewSQLStore(db *store.DB) *SQLStore { return &SQLStore{DB: db} }

func (s *SQLStore) UpsertTenant(ctx context.Context, tenant Tenant) error {
	if s == nil || s.DB == nil || s.DB.DB == nil {
		return errors.New("tenant store database is nil")
	}
	tenant.ID = strings.TrimSpace(tenant.ID)
	if tenant.ID == "" {
		return ErrTenantRequired
	}
	plan := tenant.Plan.Normalize()
	features, err := json.Marshal(plan.Features)
	if err != nil {
		return fmt.Errorf("encode tenant features: %w", err)
	}
	status := strings.TrimSpace(tenant.Status)
	if status == "" {
		status = "active"
	}
	if tenant.CreatedAt == 0 {
		tenant.CreatedAt = nowUnixMilli()
	}
	if tenant.UpdatedAt == 0 {
		tenant.UpdatedAt = tenant.CreatedAt
	}
	_, err = s.DB.ExecContext(ctx, `INSERT INTO tenants
        (id, name, plan, account_limit, concurrent_limit, features_json, status, created_at, updated_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
        ON CONFLICT(id) DO UPDATE SET name=excluded.name, plan=excluded.plan,
        account_limit=excluded.account_limit, concurrent_limit=excluded.concurrent_limit,
        features_json=excluded.features_json, status=excluded.status, updated_at=excluded.updated_at`,
		tenant.ID, tenant.Name, plan.Name, plan.MaxAccounts, plan.MaxConcurrent, string(features), status, tenant.CreatedAt, tenant.UpdatedAt)
	return err
}

func (s *SQLStore) AssignUserTenant(ctx context.Context, username, tenantID string) error {
	return s.assign(ctx, "users", "username", username, tenantID)
}

func (s *SQLStore) AssignAccountTenant(ctx context.Context, accountID, tenantID string) error {
	return s.assign(ctx, "accounts", "id", accountID, tenantID)
}

func (s *SQLStore) assign(ctx context.Context, table, key, value, tenantID string) error {
	if s == nil || s.DB == nil || s.DB.DB == nil {
		return errors.New("tenant store database is nil")
	}
	if strings.TrimSpace(value) == "" || strings.TrimSpace(tenantID) == "" {
		return ErrTenantRequired
	}
	query := fmt.Sprintf("UPDATE %s SET tenant_id = ? WHERE %s = ?", table, key)
	_, err := s.DB.ExecContext(ctx, query, strings.TrimSpace(tenantID), strings.TrimSpace(value))
	return err
}

func (s *SQLStore) GetTenant(ctx context.Context, tenantID string) (*Tenant, error) {
	if s == nil || s.DB == nil || s.DB.DB == nil {
		return nil, errors.New("tenant store database is nil")
	}
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return nil, ErrTenantRequired
	}
	var t Tenant
	var planName, features, status string
	var maxAccounts, maxConcurrent int
	err := s.DB.QueryRowContext(ctx, `SELECT id, name, plan, account_limit, concurrent_limit, features_json, status, created_at, updated_at FROM tenants WHERE id = ?`, tenantID).
		Scan(&t.ID, &t.Name, &planName, &maxAccounts, &maxConcurrent, &features, &status, &t.CreatedAt, &t.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrTenantNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get tenant %q: %w", tenantID, err)
	}
	plan := BuiltinPlan(planName)
	if maxAccounts >= 0 {
		plan.MaxAccounts = maxAccounts
	}
	if maxConcurrent >= 0 {
		plan.MaxConcurrent = maxConcurrent
	}
	if strings.TrimSpace(features) != "" {
		if err := json.Unmarshal([]byte(features), &plan.Features); err != nil {
			return nil, fmt.Errorf("decode tenant features: %w", err)
		}
	}
	plan.Name = planName
	t.Plan, t.Status = plan.Normalize(), status
	return &t, nil
}

func (s *SQLStore) GetUserTenant(ctx context.Context, username string) (string, error) {
	if s == nil || s.DB == nil || s.DB.DB == nil {
		return "", errors.New("tenant store database is nil")
	}
	var tenantID sql.NullString
	err := s.DB.QueryRowContext(ctx, "SELECT tenant_id FROM users WHERE username = ?", strings.TrimSpace(username)).Scan(&tenantID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrTenantNotFound
	}
	if err != nil {
		return "", fmt.Errorf("get tenant for user: %w", err)
	}
	if !tenantID.Valid || strings.TrimSpace(tenantID.String) == "" {
		return "", ErrTenantRequired
	}
	return strings.TrimSpace(tenantID.String), nil
}

func (s *SQLStore) CountAccounts(ctx context.Context, tenantID string) (int, error) {
	return s.count(ctx, "SELECT count(*) FROM accounts WHERE tenant_id = ?", tenantID)
}

func (s *SQLStore) CountConcurrent(ctx context.Context, tenantID string) (int, error) {
	// Account status is the durable fallback for process restarts. A runtime
	// manager can supply a more precise live count through Manager's callback.
	return s.count(ctx, "SELECT count(*) FROM accounts WHERE tenant_id = ? AND running = 1", tenantID)
}

func (s *SQLStore) IsAccountRunning(ctx context.Context, tenantID, accountID string) (bool, error) {
	if s == nil || s.DB == nil || s.DB.DB == nil {
		return false, errors.New("tenant store database is nil")
	}
	var running int
	err := s.DB.QueryRowContext(ctx, "SELECT running FROM accounts WHERE tenant_id = ? AND id = ?", strings.TrimSpace(tenantID), strings.TrimSpace(accountID)).Scan(&running)
	if errors.Is(err, sql.ErrNoRows) {
		return false, ErrTenantNotFound
	}
	return running != 0, err
}

func (s *SQLStore) count(ctx context.Context, query, tenantID string) (int, error) {
	if s == nil || s.DB == nil || s.DB.DB == nil {
		return 0, errors.New("tenant store database is nil")
	}
	var count int
	if err := s.DB.QueryRowContext(ctx, query, strings.TrimSpace(tenantID)).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

// Manager performs fail-closed quota and feature checks.
type Manager struct {
	Store           Store
	ConcurrentCount func(context.Context, string) (int, error)
}

func NewManager(s Store) *Manager { return &Manager{Store: s} }

func (m *Manager) TenantForUser(ctx context.Context, username string) (*Tenant, error) {
	if m == nil || m.Store == nil {
		return nil, ErrTenantRequired
	}
	id, err := m.Store.GetUserTenant(ctx, username)
	if err != nil {
		return nil, err
	}
	t, err := m.Store.GetTenant(ctx, id)
	if err != nil {
		return nil, err
	}
	if !t.Active() {
		return nil, ErrTenantInactive
	}
	return t, nil
}

func (m *Manager) CheckAccountCreate(ctx context.Context, tenantID string) error {
	t, usage, err := m.tenantUsage(ctx, tenantID)
	if err != nil {
		return err
	}
	if t.Plan.MaxAccounts > 0 && usage.Accounts >= t.Plan.MaxAccounts {
		return fmt.Errorf("%w: %d/%d", ErrAccountQuotaExceeded, usage.Accounts, t.Plan.MaxAccounts)
	}
	return nil
}

func (m *Manager) CheckAccountStart(ctx context.Context, tenantID, accountID string) error {
	t, usage, err := m.tenantUsage(ctx, tenantID)
	if err != nil {
		return err
	}
	if runningStore, ok := m.Store.(interface {
		IsAccountRunning(context.Context, string, string) (bool, error)
	}); ok {
		running, checkErr := runningStore.IsAccountRunning(ctx, tenantID, accountID)
		if checkErr != nil {
			return fmt.Errorf("check tenant account state: %w", checkErr)
		}
		if running {
			return nil
		}
	}
	if t.Plan.MaxConcurrent > 0 && usage.Concurrent >= t.Plan.MaxConcurrent {
		return fmt.Errorf("%w: %d/%d", ErrConcurrentQuotaExceeded, usage.Concurrent, t.Plan.MaxConcurrent)
	}
	return nil
}

func (m *Manager) CheckFeature(ctx context.Context, tenantID, feature string) error {
	t, err := m.tenant(ctx, tenantID)
	if err != nil {
		return err
	}
	if enabled, exists := t.Plan.Features[strings.ToLower(strings.TrimSpace(feature))]; exists && !enabled {
		return fmt.Errorf("%w: %s", ErrFeatureDisabled, feature)
	}
	return nil
}

func (m *Manager) tenantUsage(ctx context.Context, tenantID string) (*Tenant, Usage, error) {
	t, err := m.tenant(ctx, tenantID)
	if err != nil {
		return nil, Usage{}, err
	}
	accounts, err := m.Store.CountAccounts(ctx, tenantID)
	if err != nil {
		return nil, Usage{}, fmt.Errorf("count tenant accounts: %w", err)
	}
	concurrent := 0
	if m.ConcurrentCount != nil {
		concurrent, err = m.ConcurrentCount(ctx, tenantID)
	} else {
		concurrent, err = m.Store.CountConcurrent(ctx, tenantID)
	}
	if err != nil {
		return nil, Usage{}, fmt.Errorf("count tenant concurrency: %w", err)
	}
	return t, Usage{Accounts: accounts, Concurrent: concurrent}, nil
}

func (m *Manager) tenant(ctx context.Context, tenantID string) (*Tenant, error) {
	if m == nil || m.Store == nil {
		return nil, ErrTenantRequired
	}
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return nil, ErrTenantRequired
	}
	t, err := m.Store.GetTenant(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	if !t.Active() {
		return nil, ErrTenantInactive
	}
	t.Plan = t.Plan.Normalize()
	return t, nil
}

var _ Store = (*SQLStore)(nil)

func nowUnixMilli() int64 { return time.Now().UnixMilli() }
