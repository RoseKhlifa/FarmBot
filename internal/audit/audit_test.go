package audit

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/RoseKhlifa/FarmBot/internal/config"
	"github.com/RoseKhlifa/FarmBot/internal/store"
)

func TestAppendListExportRedactsAndUsesCallerTimestamp(t *testing.T) {
	db, err := store.Open(config.Config{DataDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	repo := &Repository{DB: db}
	ts := int64(1234)
	if err := repo.Append(context.Background(), Entry{ActorUser: "admin", Action: "post settings", DetailJSON: json.RawMessage(`{"password":"x","ok":true}`), Timestamp: ts}); err != nil {
		t.Fatal(err)
	}
	rows, err := repo.List(context.Background(), Filter{})
	if err != nil || len(rows) != 1 {
		t.Fatalf("rows=%v err=%v", rows, err)
	}
	if rows[0].Timestamp != ts || string(rows[0].DetailJSON) == "" || string(rows[0].DetailJSON) == `{"password":"x","ok":true}` {
		t.Fatalf("row=%+v", rows[0])
	}
	data, err := repo.ExportJSON(context.Background(), Filter{})
	if err != nil || len(data) == 0 {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(context.Background(), "UPDATE audit_log SET action='tampered'"); err == nil {
		t.Fatal("audit table should be append-only at repository API")
	}
}
