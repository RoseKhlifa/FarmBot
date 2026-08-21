package yyb

import (
	"context"
	"database/sql"
	"testing"

	"github.com/RoseKhlifa/FarmBot/internal/config"
	"github.com/RoseKhlifa/FarmBot/internal/store"
)

func openSharedTestDB(t *testing.T) (*store.DB, *DB) {
	t.Helper()
	dataDir := t.TempDir()
	mainDB, err := store.Open(config.Config{DataDir: dataDir, Paths: config.NewPaths(dataDir, "", nil)})
	if err != nil {
		t.Fatal(err)
	}
	// Keep package fixtures deterministic even when the caller runs the suite
	// with a production FARM_MASTER_KEY. Secure-store coverage supplies its own
	// explicit key and should not inherit process-global test environment.
	yybDB, err := newDB(mainDB, false)
	if err != nil {
		_ = mainDB.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = mainDB.Close() })
	return mainDB, yybDB
}

func TestStoreUsesFarmBotDatabase(t *testing.T) {
	mainDB, db := openSharedTestDB(t)
	ctx := context.Background()
	alias, nickname, avatar, status := "demo", "Demo", "/tmp/avatar.jpg", "alive"
	account, err := db.UpsertAccount(ctx, "openid-1", "buffer", &alias, &nickname, &avatar, map[string]any{"level": 3}, map[string]any{"access": "token"}, &status)
	if err != nil {
		t.Fatal(err)
	}
	if account.ID == 0 || account.OpenID != "openid-1" || account.UserInfo["level"] != float64(3) {
		t.Fatalf("unexpected account: %+v", account)
	}

	var count int
	if err := mainDB.QueryRowContext(ctx, "SELECT count(*) FROM wechat_accounts").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("shared database has %d accounts, want 1", count)
	}

	if err := db.PutSession(ctx, account.ID, nil, map[string]any{"send": "key"}, 4102444800, ""); err != nil {
		t.Fatal(err)
	}
	session, err := db.GetSession(ctx, account.ID, "")
	if err != nil || session.SessionBlob["send"] != "key" {
		t.Fatalf("session round trip failed: %+v, %v", session, err)
	}
	if err := db.DeleteAccount(ctx, account.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.GetAccount(ctx, account.ID); err != sql.ErrNoRows {
		t.Fatalf("deleted account lookup = %v, want sql.ErrNoRows", err)
	}
	if _, err := db.GetSession(ctx, account.ID, ""); err != sql.ErrNoRows {
		t.Fatalf("deleted session lookup = %v, want sql.ErrNoRows", err)
	}
}

func TestNewDBRejectsNilMainStore(t *testing.T) {
	if _, err := NewDB(nil); err == nil {
		t.Fatal("NewDB accepted nil main store")
	}
}
