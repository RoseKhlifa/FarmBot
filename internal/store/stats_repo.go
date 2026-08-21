package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// Stat is one per-account metric for a date key (normally YYYY-MM-DD).
type Stat struct {
	AccountID string
	Metric    string
	Date      string
	Value     float64
	UpdatedAt int64
}

// StatsRepo stores operation counters and supports atomic increments for
// concurrent account tasks.
type StatsRepo interface {
	Get(ctx context.Context, accountID, metric, date string) (Stat, error)
	List(ctx context.Context, accountID string, date ...string) ([]Stat, error)
	Set(ctx context.Context, stat Stat) error
	Increment(ctx context.Context, accountID, metric, date string, delta float64) (Stat, error)
	Delete(ctx context.Context, accountID, metric, date string) error
	GetOperations(ctx context.Context, accountID, date string) (map[string]float64, error)
	SetOperation(ctx context.Context, accountID, date, operation string, value float64) error
	IncrementOperation(ctx context.Context, accountID, date, operation string, delta float64) (float64, error)
}

// SQLiteStatsRepo stores statistics in the schema created by P1-02.
type SQLiteStatsRepo struct {
	DB *DB
}

// StatsRepository is a descriptive alias for callers that prefer the
// concrete repository name.
type StatsRepository = SQLiteStatsRepo

// NewStatsRepo creates a SQLite-backed StatsRepo implementation.
func NewStatsRepo(db *DB) *SQLiteStatsRepo {
	return &SQLiteStatsRepo{DB: db}
}

// NewStatsRepository is an alias for NewStatsRepo.
func NewStatsRepository(db *DB) *SQLiteStatsRepo { return NewStatsRepo(db) }

func (r *SQLiteStatsRepo) Get(ctx context.Context, accountID, metric, date string) (Stat, error) {
	accountID, metric, date, err := statsRequiredKeyParts(accountID, metric, date)
	if err != nil {
		return Stat{}, err
	}
	if err := r.checkDB(); err != nil {
		return Stat{}, err
	}
	var stat Stat
	err = r.DB.QueryRowContext(ctx, `
SELECT account_id, metric, date, value, updated_at
FROM stats WHERE account_id = ? AND metric = ? AND date = ?`, accountID, metric, date,
	).Scan(&stat.AccountID, &stat.Metric, &stat.Date, &stat.Value, &stat.UpdatedAt)
	if err != nil {
		return Stat{}, fmt.Errorf("get stat %q/%q for account %q: %w", metric, date, accountID, err)
	}
	return stat, nil
}

func (r *SQLiteStatsRepo) List(ctx context.Context, accountID string, dates ...string) ([]Stat, error) {
	accountID, err := accountRequiredText("accountID", accountID)
	if err != nil {
		return nil, err
	}
	if len(dates) > 1 {
		return nil, fmt.Errorf("stats list accepts at most one date")
	}
	if err := r.checkDB(); err != nil {
		return nil, err
	}
	query := `SELECT account_id, metric, date, value, updated_at FROM stats WHERE account_id = ?`
	args := []any{accountID}
	if len(dates) == 1 {
		date := strings.TrimSpace(dates[0])
		if date == "" {
			return nil, fmt.Errorf("date must not be empty")
		}
		query += " AND date = ?"
		args = append(args, date)
	}
	query += " ORDER BY date, metric"
	rows, err := r.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list stats for account %q: %w", accountID, err)
	}
	defer func() { _ = rows.Close() }()
	stats := make([]Stat, 0)
	for rows.Next() {
		var stat Stat
		if err := rows.Scan(&stat.AccountID, &stat.Metric, &stat.Date, &stat.Value, &stat.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan stats for account %q: %w", accountID, err)
		}
		stats = append(stats, stat)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list stats for account %q: %w", accountID, err)
	}
	return stats, nil
}

func (r *SQLiteStatsRepo) Set(ctx context.Context, stat Stat) error {
	accountID, metric, date, err := statsRequiredKeyParts(stat.AccountID, stat.Metric, stat.Date)
	if err != nil {
		return err
	}
	if err := r.checkDB(); err != nil {
		return err
	}
	updatedAt := stat.UpdatedAt
	if updatedAt == 0 {
		updatedAt = time.Now().UnixMilli()
	}
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin stat write: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := statsEnsureAccount(ctx, tx, accountID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO stats(account_id, metric, date, value, updated_at) VALUES (?, ?, ?, ?, ?)
ON CONFLICT(account_id, metric, date) DO UPDATE SET
    value = excluded.value, updated_at = excluded.updated_at`,
		accountID, metric, date, stat.Value, updatedAt,
	); err != nil {
		return fmt.Errorf("set stat %q/%q for account %q: %w", metric, date, accountID, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit stat write: %w", err)
	}
	return nil
}

func (r *SQLiteStatsRepo) Increment(ctx context.Context, accountID, metric, date string, delta float64) (Stat, error) {
	accountID, metric, date, err := statsRequiredKeyParts(accountID, metric, date)
	if err != nil {
		return Stat{}, err
	}
	if err := r.checkDB(); err != nil {
		return Stat{}, err
	}
	updatedAt := time.Now().UnixMilli()
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return Stat{}, fmt.Errorf("begin stat increment: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := statsEnsureAccount(ctx, tx, accountID); err != nil {
		return Stat{}, err
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO stats(account_id, metric, date, value, updated_at) VALUES (?, ?, ?, ?, ?)
ON CONFLICT(account_id, metric, date) DO UPDATE SET
    value = stats.value + excluded.value, updated_at = excluded.updated_at`,
		accountID, metric, date, delta, updatedAt,
	); err != nil {
		return Stat{}, fmt.Errorf("increment stat %q/%q for account %q: %w", metric, date, accountID, err)
	}
	var stat Stat
	if err := tx.QueryRowContext(ctx, `
SELECT account_id, metric, date, value, updated_at
FROM stats WHERE account_id = ? AND metric = ? AND date = ?`, accountID, metric, date,
	).Scan(&stat.AccountID, &stat.Metric, &stat.Date, &stat.Value, &stat.UpdatedAt); err != nil {
		return Stat{}, fmt.Errorf("read incremented stat %q/%q for account %q: %w", metric, date, accountID, err)
	}
	if err := tx.Commit(); err != nil {
		return Stat{}, fmt.Errorf("commit stat increment: %w", err)
	}
	return stat, nil
}

func (r *SQLiteStatsRepo) Delete(ctx context.Context, accountID, metric, date string) error {
	accountID, metric, date, err := statsRequiredKeyParts(accountID, metric, date)
	if err != nil {
		return err
	}
	if err := r.checkDB(); err != nil {
		return err
	}
	if _, err := r.DB.ExecContext(ctx,
		"DELETE FROM stats WHERE account_id = ? AND metric = ? AND date = ?",
		accountID, metric, date,
	); err != nil {
		return fmt.Errorf("delete stat %q/%q for account %q: %w", metric, date, accountID, err)
	}
	return nil
}

func (r *SQLiteStatsRepo) GetOperations(ctx context.Context, accountID, date string) (map[string]float64, error) {
	stats, err := r.List(ctx, accountID, date)
	if err != nil {
		return nil, err
	}
	operations := make(map[string]float64, len(stats))
	for _, stat := range stats {
		operations[stat.Metric] = stat.Value
	}
	return operations, nil
}

func (r *SQLiteStatsRepo) SetOperation(ctx context.Context, accountID, date, operation string, value float64) error {
	return r.Set(ctx, Stat{AccountID: accountID, Metric: operation, Date: date, Value: value})
}

func (r *SQLiteStatsRepo) IncrementOperation(ctx context.Context, accountID, date, operation string, delta float64) (float64, error) {
	stat, err := r.Increment(ctx, accountID, operation, date, delta)
	if err != nil {
		return 0, err
	}
	return stat.Value, nil
}

func (r *SQLiteStatsRepo) checkDB() error {
	if r == nil || r.DB == nil || r.DB.DB == nil {
		return fmt.Errorf("stats repository database is nil")
	}
	return nil
}

func statsEnsureAccount(ctx context.Context, tx *sql.Tx, accountID string) error {
	var exists int
	if err := tx.QueryRowContext(ctx, "SELECT 1 FROM accounts WHERE id = ?", accountID).Scan(&exists); err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("account %q: %w", accountID, sql.ErrNoRows)
		}
		return fmt.Errorf("check account %q: %w", accountID, err)
	}
	return nil
}

func statsRequiredKeyParts(accountID, metric, date string) (string, string, string, error) {
	var err error
	if accountID, err = accountRequiredText("accountID", accountID); err != nil {
		return "", "", "", err
	}
	if metric, err = accountRequiredText("metric", metric); err != nil {
		return "", "", "", err
	}
	if date, err = accountRequiredText("date", date); err != nil {
		return "", "", "", err
	}
	return accountID, metric, date, nil
}

var _ StatsRepo = (*SQLiteStatsRepo)(nil)
