package tenant

import (
	"context"
	"testing"

	"github.com/RoseKhlifa/FarmBot/internal/config"
	"github.com/RoseKhlifa/FarmBot/internal/store"
)

func TestSQLStorePersistsTenantAndCountsUsage(t *testing.T) {
	db, err := store.Open(config.Config{DataDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	ctx := context.Background()
	sqlStore := NewSQLStore(db)
	if err := sqlStore.UpsertTenant(ctx, Tenant{ID: "team-1", Name: "Team", Plan: Plan{Name: "starter", MaxAccounts: 1, MaxConcurrent: 1}}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO users(username, pwd_hash, tenant_id) VALUES ('owner', 'hash', 'team-1')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO accounts(id, tenant_id, running) VALUES ('account-1', 'team-1', 1)`); err != nil {
		t.Fatal(err)
	}
	manager := NewManager(sqlStore)
	if err := manager.CheckAccountCreate(ctx, "team-1"); err == nil {
		t.Fatal("account quota allowed an over-limit tenant")
	}
	if err := manager.CheckAccountStart(ctx, "team-1", "account-2"); err == nil {
		t.Fatal("concurrent quota allowed an over-limit tenant")
	}
	if got, err := sqlStore.GetUserTenant(ctx, "owner"); err != nil || got != "team-1" {
		t.Fatalf("user tenant = %q, %v", got, err)
	}
}
