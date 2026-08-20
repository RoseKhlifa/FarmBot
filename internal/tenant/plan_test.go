package tenant

import (
	"context"
	"errors"
	"testing"
)

type fakeStore struct {
	tenant     Tenant
	accounts   int
	concurrent int
}

func (f fakeStore) GetTenant(context.Context, string) (*Tenant, error)    { return &f.tenant, nil }
func (f fakeStore) GetUserTenant(context.Context, string) (string, error) { return f.tenant.ID, nil }
func (f fakeStore) CountAccounts(context.Context, string) (int, error)    { return f.accounts, nil }
func (f fakeStore) CountConcurrent(context.Context, string) (int, error)  { return f.concurrent, nil }

func TestManagerRejectsAccountAndConcurrentQuota(t *testing.T) {
	store := fakeStore{tenant: Tenant{ID: "t1", Status: "active", Plan: Plan{Name: "starter", MaxAccounts: 2, MaxConcurrent: 1}}, accounts: 2, concurrent: 1}
	m := NewManager(store)
	if err := m.CheckAccountCreate(context.Background(), "t1"); !errors.Is(err, ErrAccountQuotaExceeded) {
		t.Fatalf("create quota error = %v", err)
	}
	if err := m.CheckAccountStart(context.Background(), "t1", "a1"); !errors.Is(err, ErrConcurrentQuotaExceeded) {
		t.Fatalf("start quota error = %v", err)
	}
}

func TestManagerRejectsInactiveTenant(t *testing.T) {
	m := NewManager(fakeStore{tenant: Tenant{ID: "t1", Status: "suspended"}})
	if err := m.CheckAccountCreate(context.Background(), "t1"); !errors.Is(err, ErrTenantInactive) {
		t.Fatalf("inactive tenant error = %v", err)
	}
}
