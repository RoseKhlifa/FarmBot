package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/RoseKhlifa/FarmBot/internal/config"
)

func TestJSONImporterRoundTripAndConflictModes(t *testing.T) {
	source := t.TempDir()
	writeJSONFixture(t, filepath.Join(source, "accounts.json"), `{"accounts":[{"id":"account-1","name":"legacy","platform":"qq","updatedAt":1710000000000}]}`)
	writeJSONFixture(t, filepath.Join(source, "store.json"), `{"accountConfigs":{"account-1":{"plantingStrategy":"level","friendBlacklist":[{"gid":"gid-1","reason":"keep"}]}}}`)
	writeJSONFixture(t, filepath.Join(source, "stats", "account-1.json"), `{"date":"2026-08-19","operations":{"harvest":2}}`)
	writeJSONFixture(t, filepath.Join(source, "known_friend_gids", "account-1.json"), `{"gids":["gid-1"],"updatedAt":1710000000000}`)
	originalAccounts, err := os.ReadFile(filepath.Join(source, "accounts.json"))
	if err != nil {
		t.Fatal(err)
	}

	dataDir := t.TempDir()
	db, err := Open(config.Config{DataDir: dataDir})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	report, err := ImportJSON(ctx, db, source, JSONImportOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for table, want := range map[string]int{"accounts": 1, "account_config": 1, "blacklist": 1, "stats": 1, "known_friend_gids": 1} {
		if report.Counts[table] != want {
			t.Fatalf("first import count %s = %d, want %d; report=%+v", table, report.Counts[table], want, report)
		}
	}
	account, err := NewAccountRepo(db).Get(ctx, "account-1")
	if err != nil || account.Name != "legacy" {
		t.Fatalf("imported account = %+v, %v", account, err)
	}
	configRow, err := NewAccountRepo(db).GetConfig(ctx, "account-1")
	if err != nil || configRow.PlantingStrategy != "level" {
		t.Fatalf("imported account config = %+v, %v", configRow, err)
	}
	if got, err := NewStatsRepo(db).Get(ctx, "account-1", "harvest", "2026-08-19"); err != nil || got.Value != 2 {
		t.Fatalf("imported stat = %+v, %v", got, err)
	}
	if _, err := os.Stat(filepath.Join(source, "accounts.json")); err != nil {
		t.Fatalf("source JSON was removed: %v", err)
	}
	if after, err := os.ReadFile(filepath.Join(source, "accounts.json")); err != nil || string(after) != string(originalAccounts) {
		t.Fatalf("source JSON changed during import: %v", err)
	}

	skipped, err := ImportJSON(ctx, db, source, JSONImportOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if skipped.Counts["accounts"] != 0 || skipped.Skipped["accounts"] != 1 {
		t.Fatalf("repeat import did not skip account: %+v", skipped)
	}

	writeJSONFixture(t, filepath.Join(source, "accounts.json"), `{"accounts":[{"id":"account-1","name":"updated"}]}`)
	overwritten, err := ImportJSON(ctx, db, source, JSONImportOptions{Conflict: ConflictOverwrite})
	if err != nil {
		t.Fatal(err)
	}
	if overwritten.Counts["accounts"] != 1 {
		t.Fatalf("overwrite import count = %+v", overwritten)
	}
	account, err = NewAccountRepo(db).Get(ctx, "account-1")
	if err != nil || account.Name != "updated" {
		t.Fatalf("overwrite did not update account = %+v, %v", account, err)
	}
}

func writeJSONFixture(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}
