package store

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/RoseKhlifa/FarmBot/internal/config"
	_ "modernc.org/sqlite"
)

// migrationFiles contains the SQL migrations shipped with the binary. The
// directory is intentionally empty of numbered migrations until P1-02 adds
// the initial schema.
//
//go:embed migrations
var migrationFiles embed.FS

const (
	databaseFilename = "farmbot.db"
	openTimeout      = 10 * time.Second
	busyTimeout      = 5 * time.Second
)

// DB owns the process-wide SQLite connection pool. Embedding sql.DB keeps the
// repository package able to use the standard database/sql operations without
// exposing another database abstraction.
type DB struct {
	*sql.DB
}

// Open creates or opens the SQLite database below cfg's data directory and
// applies all embedded migrations. SQLite is deliberately kept to one open
// connection because writes are serialized by the database itself.
func Open(cfg config.Config) (*DB, error) {
	dataDir := cfg.DataDir
	if dataDir == "" {
		dataDir = cfg.Paths.DataDir
	}
	if dataDir == "" {
		dataDir = config.ResolvePaths().DataDir
	}

	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("create data directory %q: %w", dataDir, err)
	}
	databasePath := filepath.Join(dataDir, databaseFilename)

	conn, err := sql.Open("sqlite", databasePath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}
	conn.SetMaxOpenConns(1)
	conn.SetMaxIdleConns(1)

	db := &DB{DB: conn}
	ctx, cancel := context.WithTimeout(context.Background(), openTimeout)
	defer cancel()
	if err := db.configure(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := runMigrations(ctx, conn); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

// Close releases the underlying SQLite connection. It is safe to call on a
// nil DB so shutdown paths can defer cleanup without extra guards.
func (db *DB) Close() error {
	if db == nil || db.DB == nil {
		return nil
	}
	return db.DB.Close()
}

func (db *DB) configure(ctx context.Context) error {
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("ping sqlite database: %w", err)
	}
	if _, err := db.ExecContext(ctx, "PRAGMA busy_timeout=5000"); err != nil {
		return fmt.Errorf("set sqlite busy timeout: %w", err)
	}
	var journalMode string
	if err := db.QueryRowContext(ctx, "PRAGMA journal_mode=WAL").Scan(&journalMode); err != nil {
		return fmt.Errorf("enable sqlite WAL: %w", err)
	}
	if !strings.EqualFold(journalMode, "wal") {
		return fmt.Errorf("enable sqlite WAL: database reported journal mode %q", journalMode)
	}
	return nil
}

func runMigrations(ctx context.Context, conn *sql.DB) error {
	if _, err := conn.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS schema_migrations (
	version    INTEGER PRIMARY KEY,
	name       TEXT NOT NULL,
	applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
)`); err != nil {
		return fmt.Errorf("create schema_migrations table: %w", err)
	}

	migrations, err := loadMigrations()
	if err != nil {
		return err
	}
	for _, migration := range migrations {
		var appliedName string
		err := conn.QueryRowContext(ctx,
			"SELECT name FROM schema_migrations WHERE version = ?", migration.version,
		).Scan(&appliedName)
		switch {
		case err == nil:
			if appliedName != migration.name {
				return fmt.Errorf("migration version %d already applied as %q, found %q", migration.version, appliedName, migration.name)
			}
			continue
		case errors.Is(err, sql.ErrNoRows):
			// Apply below in a transaction.
		default:
			return fmt.Errorf("check migration %s: %w", migration.name, err)
		}

		tx, err := conn.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin migration %s: %w", migration.name, err)
		}
		if _, err = tx.ExecContext(ctx, string(migration.sql)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("apply migration %s: %w", migration.name, err)
		}
		if _, err = tx.ExecContext(ctx,
			"INSERT INTO schema_migrations(version, name) VALUES (?, ?)", migration.version, migration.name,
		); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record migration %s: %w", migration.name, err)
		}
		if err = tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %s: %w", migration.name, err)
		}
	}
	return nil
}

type migration struct {
	version int64
	name    string
	sql     []byte
}

func loadMigrations() ([]migration, error) {
	entries, err := fs.Glob(migrationFiles, "migrations/*.sql")
	if err != nil {
		return nil, fmt.Errorf("list embedded migrations: %w", err)
	}
	migrations := make([]migration, 0, len(entries))
	seenVersions := make(map[int64]string, len(entries))
	for _, entry := range entries {
		name := path.Base(entry)
		version, err := migrationVersion(name)
		if err != nil {
			return nil, err
		}
		if previous, exists := seenVersions[version]; exists {
			return nil, fmt.Errorf("duplicate migration version %d in %q and %q", version, previous, name)
		}
		script, err := fs.ReadFile(migrationFiles, entry)
		if err != nil {
			return nil, fmt.Errorf("read embedded migration %s: %w", name, err)
		}
		seenVersions[version] = name
		migrations = append(migrations, migration{version: version, name: name, sql: script})
	}
	sort.Slice(migrations, func(i, j int) bool {
		if migrations[i].version == migrations[j].version {
			return migrations[i].name < migrations[j].name
		}
		return migrations[i].version < migrations[j].version
	})
	return migrations, nil
}

func migrationVersion(name string) (int64, error) {
	separator := strings.IndexAny(name, "_-")
	if separator <= 0 || !strings.HasSuffix(name, ".sql") {
		return 0, fmt.Errorf("invalid migration filename %q: expected NNNN_name.sql", name)
	}
	version, err := strconv.ParseInt(name[:separator], 10, 64)
	if err != nil || version < 1 {
		return 0, fmt.Errorf("invalid migration version in %q", name)
	}
	return version, nil
}
