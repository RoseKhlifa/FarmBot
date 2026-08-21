package backup

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/RoseKhlifa/FarmBot/internal/config"
	"github.com/RoseKhlifa/FarmBot/internal/store"
	_ "modernc.org/sqlite"
)

func TestSnapshotAndExport(t *testing.T) {
	dir := t.TempDir()
	db, err := store.Open(config.Config{DataDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	ctx := context.Background()
	accounts := store.NewAccountRepo(db)
	if err := accounts.Upsert(ctx, store.Account{ID: "a1", Name: "demo", Code: "private", OwnerUser: "admin"}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, "INSERT INTO stats(account_id,metric,date,value,updated_at) VALUES('a1','harvest','2026-08-20',2,1)"); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(dir, "backups", "farmbot-test.db")
	if err := (Snapshotter{DB: db}).Snapshot(ctx, output); err != nil {
		t.Fatal(err)
	}
	independent, err := sql.Open("sqlite", output)
	if err != nil {
		t.Fatal(err)
	}
	var count int
	if err := independent.QueryRow("SELECT count(*) FROM accounts").Scan(&count); err != nil || count != 1 {
		t.Fatalf("snapshot count=%d err=%v", count, err)
	}
	if err := independent.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := ExportAccount(ctx, db, "a1")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) == "" || string(data) == "private" {
		t.Fatal("export must not include login code")
	}
	if err := os.Remove(output); err != nil {
		t.Fatal(err)
	}
}
