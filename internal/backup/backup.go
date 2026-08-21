// Package backup contains externally-triggered persistence snapshots and
// account export primitives. It deliberately owns no scheduler or clock.
package backup

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/RoseKhlifa/FarmBot/internal/store"
)

type Snapshotter struct{ DB *store.DB }

func (s Snapshotter) Snapshot(ctx context.Context, destination string) error {
	if s.DB == nil || s.DB.DB == nil {
		return errors.New("backup database is nil")
	}
	destination, err := filepath.Abs(strings.TrimSpace(destination))
	if err != nil || destination == "" {
		return fmt.Errorf("invalid backup destination: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return fmt.Errorf("create backup directory: %w", err)
	}
	if _, err := os.Stat(destination); err == nil {
		return fmt.Errorf("backup destination already exists: %s", destination)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("check backup destination: %w", err)
	}
	source, err := databaseFilename(ctx, s.DB)
	if err != nil {
		return err
	}
	if samePath(source, destination) {
		return errors.New("backup destination must differ from source database")
	}
	if _, err := s.DB.ExecContext(ctx, "VACUUM INTO '"+strings.ReplaceAll(destination, "'", "''")+"'"); err != nil {
		return fmt.Errorf("vacuum sqlite database: %w", err)
	}
	return nil
}

func databaseFilename(ctx context.Context, db *store.DB) (string, error) {
	var filename string
	if err := db.QueryRowContext(ctx, "PRAGMA database_list").Scan(new(int), new(string), &filename); err != nil {
		return "", fmt.Errorf("inspect sqlite database: %w", err)
	}
	return filename, nil
}

func samePath(left, right string) bool {
	if left == "" {
		return false
	}
	a, _ := filepath.Abs(left)
	b, _ := filepath.Abs(right)
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}

func PruneSnapshots(directory string, keep int) error {
	if keep < 0 {
		return errors.New("keep must be non-negative")
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		return err
	}
	type item struct {
		path string
		mod  int64
	}
	items := make([]item, 0)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "farmbot-") || !strings.HasSuffix(entry.Name(), ".db") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		items = append(items, item{filepath.Join(directory, entry.Name()), info.ModTime().UnixNano()})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].mod > items[j].mod })
	for _, old := range items[keep:] {
		if err := os.Remove(old.path); err != nil {
			return fmt.Errorf("prune snapshot %s: %w", old.path, err)
		}
	}
	return nil
}

type Export struct {
	Version int `json:"version"`
	Account any `json:"account"`
	Config  any `json:"config,omitempty"`
	Cache   any `json:"cache,omitempty"`
	Stats   any `json:"stats,omitempty"`
}

type accountRow struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Platform  string `json:"platform"`
	LoginType string `json:"loginType"`
	Provider  string `json:"provider"`
	WXID      string `json:"wxid,omitempty"`
	UIN       string `json:"uin,omitempty"`
	QQ        string `json:"qq,omitempty"`
	GID       string `json:"gid,omitempty"`
	Avatar    string `json:"avatar,omitempty"`
	OwnerUser string `json:"ownerUser,omitempty"`
	Remark    string `json:"remark,omitempty"`
	CreatedAt int64  `json:"createdAt"`
	UpdatedAt int64  `json:"updatedAt"`
}

func ExportAccount(ctx context.Context, db *store.DB, accountID string) ([]byte, error) {
	if db == nil || db.DB == nil {
		return nil, errors.New("export database is nil")
	}
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return nil, errors.New("account ID is required")
	}
	var a accountRow
	err := db.QueryRowContext(ctx, `SELECT id,name,platform,login_type,provider,wxid,uin,qq,gid,avatar,COALESCE(owner_user,''),COALESCE(remark,''),created_at,updated_at FROM accounts WHERE id = ?`, accountID).Scan(
		&a.ID, &a.Name, &a.Platform, &a.LoginType, &a.Provider, &a.WXID, &a.UIN, &a.QQ, &a.GID, &a.Avatar, &a.OwnerUser, &a.Remark, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("load account %q: %w", accountID, err)
	}
	var config any
	if row, err := store.NewAccountRepo(db).GetConfig(ctx, accountID); err == nil && row != nil {
		config = row
	}
	cache := map[string]any{}
	for _, table := range []string{"friend_gid_cache", "friend_dog_info", "friend_list_cache"} {
		var payload string
		if err := db.QueryRowContext(ctx, "SELECT payload FROM "+table+" WHERE account_id = ?", accountID).Scan(&payload); err == nil && json.Valid([]byte(payload)) {
			var value any
			_ = json.Unmarshal([]byte(payload), &value)
			cache[table] = value
		}
	}
	rows, err := db.QueryContext(ctx, "SELECT metric,date,value,updated_at FROM stats WHERE account_id = ? ORDER BY date,metric", accountID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	stats := make([]map[string]any, 0)
	for rows.Next() {
		var metric, date string
		var value float64
		var updated int64
		if err := rows.Scan(&metric, &date, &value, &updated); err != nil {
			return nil, err
		}
		stats = append(stats, map[string]any{"metric": metric, "date": date, "value": value, "updatedAt": updated})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return json.MarshalIndent(Export{Version: 1, Account: a, Config: config, Cache: cache, Stats: stats}, "", "  ")
}
