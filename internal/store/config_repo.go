package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const (
	ConfigKeySystem          = "system_config"
	ConfigKeyWX              = "global_wx_config"
	ConfigKeyTheme           = "ui_theme"
	ConfigKeyAntiResale      = "anti_resale_config"
	ConfigKeyOfflineReminder = "offline_reminder"
)

// ConfigEntry is one JSON value in the global configuration namespace.
type ConfigEntry struct {
	Key       string
	Value     json.RawMessage
	UpdatedAt int64
}

// ConfigRepo persists global and system-level settings. Account-scoped
// configuration belongs to AccountRepo and is intentionally not duplicated
// here.
type ConfigRepo interface {
	Get(ctx context.Context, key string) (ConfigEntry, error)
	List(ctx context.Context) ([]ConfigEntry, error)
	Set(ctx context.Context, entry ConfigEntry) error
	Delete(ctx context.Context, key string) error
	GetGlobal(ctx context.Context, key string) (json.RawMessage, error)
	SetGlobal(ctx context.Context, key string, value json.RawMessage) error
	DeleteGlobal(ctx context.Context, key string) error
	GetSystemConfig(ctx context.Context) (json.RawMessage, error)
	SetSystemConfig(ctx context.Context, value json.RawMessage) error
	GetWXConfig(ctx context.Context) (json.RawMessage, error)
	SetWXConfig(ctx context.Context, value json.RawMessage) error
	GetTheme(ctx context.Context) (string, error)
	SetTheme(ctx context.Context, theme string) error
	GetAntiResaleConfig(ctx context.Context) (json.RawMessage, error)
	SetAntiResaleConfig(ctx context.Context, value json.RawMessage) error
	GetOfflineReminder(ctx context.Context, username string) (json.RawMessage, error)
	SetOfflineReminder(ctx context.Context, username string, value json.RawMessage) error
	DeleteOfflineReminder(ctx context.Context, username string) error
}

// SQLiteConfigRepo stores global configuration in the schema created by
// P1-02.
type SQLiteConfigRepo struct {
	DB *DB
}

// ConfigRepository is a descriptive alias for callers that prefer the
// concrete repository name.
type ConfigRepository = SQLiteConfigRepo

// NewConfigRepo creates a SQLite-backed ConfigRepo implementation.
func NewConfigRepo(db *DB) *SQLiteConfigRepo {
	return &SQLiteConfigRepo{DB: db}
}

// NewConfigRepository is an alias for NewConfigRepo.
func NewConfigRepository(db *DB) *SQLiteConfigRepo { return NewConfigRepo(db) }

func (r *SQLiteConfigRepo) Get(ctx context.Context, key string) (ConfigEntry, error) {
	key, err := configRequiredKey(key)
	if err != nil {
		return ConfigEntry{}, err
	}
	if err := r.checkDB(); err != nil {
		return ConfigEntry{}, err
	}
	var entry ConfigEntry
	var value string
	err = r.DB.QueryRowContext(ctx,
		"SELECT key, value, updated_at FROM global_config WHERE key = ?", key,
	).Scan(&entry.Key, &value, &entry.UpdatedAt)
	if err != nil {
		return ConfigEntry{}, fmt.Errorf("get global config %q: %w", key, err)
	}
	entry.Value = json.RawMessage(value)
	return entry, nil
}

func (r *SQLiteConfigRepo) List(ctx context.Context) ([]ConfigEntry, error) {
	if err := r.checkDB(); err != nil {
		return nil, err
	}
	rows, err := r.DB.QueryContext(ctx,
		"SELECT key, value, updated_at FROM global_config ORDER BY key",
	)
	if err != nil {
		return nil, fmt.Errorf("list global config: %w", err)
	}
	defer rows.Close()
	entries := make([]ConfigEntry, 0)
	for rows.Next() {
		var entry ConfigEntry
		var value string
		if err := rows.Scan(&entry.Key, &value, &entry.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan global config: %w", err)
		}
		entry.Value = json.RawMessage(value)
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list global config: %w", err)
	}
	return entries, nil
}

func (r *SQLiteConfigRepo) Set(ctx context.Context, entry ConfigEntry) error {
	key, err := configRequiredKey(entry.Key)
	if err != nil {
		return err
	}
	value, err := accountJSONText(entry.Value, "{}")
	if err != nil {
		return fmt.Errorf("global config %q: %w", key, err)
	}
	if err := r.checkDB(); err != nil {
		return err
	}
	updatedAt := entry.UpdatedAt
	if updatedAt == 0 {
		updatedAt = time.Now().UnixMilli()
	}
	if _, err := r.DB.ExecContext(ctx, `
INSERT INTO global_config(key, value, updated_at) VALUES (?, ?, ?)
ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
		key, value, updatedAt,
	); err != nil {
		return fmt.Errorf("set global config %q: %w", key, err)
	}
	return nil
}

func (r *SQLiteConfigRepo) Delete(ctx context.Context, key string) error {
	key, err := configRequiredKey(key)
	if err != nil {
		return err
	}
	if err := r.checkDB(); err != nil {
		return err
	}
	if _, err := r.DB.ExecContext(ctx, "DELETE FROM global_config WHERE key = ?", key); err != nil {
		return fmt.Errorf("delete global config %q: %w", key, err)
	}
	return nil
}

func (r *SQLiteConfigRepo) GetGlobal(ctx context.Context, key string) (json.RawMessage, error) {
	entry, err := r.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	return append(json.RawMessage(nil), entry.Value...), nil
}

func (r *SQLiteConfigRepo) SetGlobal(ctx context.Context, key string, value json.RawMessage) error {
	return r.Set(ctx, ConfigEntry{Key: key, Value: value})
}

func (r *SQLiteConfigRepo) DeleteGlobal(ctx context.Context, key string) error {
	return r.Delete(ctx, key)
}

func (r *SQLiteConfigRepo) GetSystemConfig(ctx context.Context) (json.RawMessage, error) {
	return r.GetGlobal(ctx, ConfigKeySystem)
}

func (r *SQLiteConfigRepo) SetSystemConfig(ctx context.Context, value json.RawMessage) error {
	return r.SetGlobal(ctx, ConfigKeySystem, value)
}

func (r *SQLiteConfigRepo) GetWXConfig(ctx context.Context) (json.RawMessage, error) {
	return r.GetGlobal(ctx, ConfigKeyWX)
}

func (r *SQLiteConfigRepo) SetWXConfig(ctx context.Context, value json.RawMessage) error {
	return r.SetGlobal(ctx, ConfigKeyWX, value)
}

func (r *SQLiteConfigRepo) GetTheme(ctx context.Context) (string, error) {
	value, err := r.GetGlobal(ctx, ConfigKeyTheme)
	if err != nil {
		return "", err
	}
	var theme string
	if err := json.Unmarshal(value, &theme); err != nil {
		return "", fmt.Errorf("decode theme config: %w", err)
	}
	return theme, nil
}

func (r *SQLiteConfigRepo) SetTheme(ctx context.Context, theme string) error {
	value, err := json.Marshal(theme)
	if err != nil {
		return fmt.Errorf("encode theme config: %w", err)
	}
	return r.SetGlobal(ctx, ConfigKeyTheme, value)
}

func (r *SQLiteConfigRepo) GetAntiResaleConfig(ctx context.Context) (json.RawMessage, error) {
	return r.GetGlobal(ctx, ConfigKeyAntiResale)
}

func (r *SQLiteConfigRepo) SetAntiResaleConfig(ctx context.Context, value json.RawMessage) error {
	return r.SetGlobal(ctx, ConfigKeyAntiResale, value)
}

func (r *SQLiteConfigRepo) GetOfflineReminder(ctx context.Context, username string) (json.RawMessage, error) {
	return r.GetGlobal(ctx, configOfflineReminderKey(username))
}

func (r *SQLiteConfigRepo) SetOfflineReminder(ctx context.Context, username string, value json.RawMessage) error {
	return r.SetGlobal(ctx, configOfflineReminderKey(username), value)
}

func (r *SQLiteConfigRepo) DeleteOfflineReminder(ctx context.Context, username string) error {
	return r.DeleteGlobal(ctx, configOfflineReminderKey(username))
}

func (r *SQLiteConfigRepo) checkDB() error {
	if r == nil || r.DB == nil || r.DB.DB == nil {
		return fmt.Errorf("config repository database is nil")
	}
	return nil
}

func configRequiredKey(key string) (string, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return "", fmt.Errorf("config key must not be empty")
	}
	return key, nil
}

func configOfflineReminderKey(username string) string {
	username = strings.TrimSpace(username)
	if username == "" {
		return ConfigKeyOfflineReminder
	}
	return ConfigKeyOfflineReminder + ":" + username
}

var _ ConfigRepo = (*SQLiteConfigRepo)(nil)
