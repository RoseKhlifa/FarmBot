package store

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// CacheValue carries one JSON cache payload and its persistence timestamp.
type CacheValue struct {
	Payload   json.RawMessage
	UpdatedAt int64
}

// BlacklistEntry is one account-scoped friend blacklist record.
type BlacklistEntry struct {
	AccountID string
	GID       string
	Reason    string
	AddedAt   int64
}

// CacheRepo isolates friend caches and blacklist persistence.
type CacheRepo interface {
	GetKnownFriendGIDs(ctx context.Context, accountID string) (CacheValue, error)
	PutKnownFriendGIDs(ctx context.Context, accountID string, value CacheValue) error
	InvalidateKnownFriendGIDs(ctx context.Context, accountID string) error
	GetFriendDogInfo(ctx context.Context, accountID string) (CacheValue, error)
	PutFriendDogInfo(ctx context.Context, accountID string, value CacheValue) error
	InvalidateFriendDogInfo(ctx context.Context, accountID string) error
	GetFriendList(ctx context.Context, accountID string) (CacheValue, error)
	PutFriendList(ctx context.Context, accountID string, value CacheValue) error
	InvalidateFriendList(ctx context.Context, accountID string) error
	RemoveFriendFromCache(ctx context.Context, accountID, gid string) error
	DeleteAccountCaches(ctx context.Context, accountID string) error
	ListBlacklist(ctx context.Context, accountID string) ([]BlacklistEntry, error)
	UpsertBlacklist(ctx context.Context, entry BlacklistEntry) error
	DeleteBlacklist(ctx context.Context, accountID, gid string) error
}

// SQLiteCacheRepo stores account caches in the schema created by P1-02.
type SQLiteCacheRepo struct {
	DB *DB
}

// CacheRepository is a descriptive alias for callers that prefer the
// concrete repository name.
type CacheRepository = SQLiteCacheRepo

// NewCacheRepo creates a SQLite-backed CacheRepo implementation.
func NewCacheRepo(db *DB) *SQLiteCacheRepo {
	return &SQLiteCacheRepo{DB: db}
}

// NewCacheRepository is an alias for NewCacheRepo.
func NewCacheRepository(db *DB) *SQLiteCacheRepo { return NewCacheRepo(db) }

func (r *SQLiteCacheRepo) GetKnownFriendGIDs(ctx context.Context, accountID string) (CacheValue, error) {
	return r.readCache(ctx, "friend_gid_cache", accountID)
}

func (r *SQLiteCacheRepo) PutKnownFriendGIDs(ctx context.Context, accountID string, value CacheValue) error {
	return r.writeCache(ctx, "friend_gid_cache", accountID, value, "[]")
}

func (r *SQLiteCacheRepo) InvalidateKnownFriendGIDs(ctx context.Context, accountID string) error {
	return r.invalidateCache(ctx, "friend_gid_cache", accountID)
}

func (r *SQLiteCacheRepo) GetFriendDogInfo(ctx context.Context, accountID string) (CacheValue, error) {
	return r.readCache(ctx, "friend_dog_info", accountID)
}

func (r *SQLiteCacheRepo) PutFriendDogInfo(ctx context.Context, accountID string, value CacheValue) error {
	return r.writeCache(ctx, "friend_dog_info", accountID, value, "{}")
}

func (r *SQLiteCacheRepo) InvalidateFriendDogInfo(ctx context.Context, accountID string) error {
	return r.invalidateCache(ctx, "friend_dog_info", accountID)
}

func (r *SQLiteCacheRepo) GetFriendList(ctx context.Context, accountID string) (CacheValue, error) {
	return r.readCache(ctx, "friend_list_cache", accountID)
}

func (r *SQLiteCacheRepo) PutFriendList(ctx context.Context, accountID string, value CacheValue) error {
	return r.writeCache(ctx, "friend_list_cache", accountID, value, "[]")
}

func (r *SQLiteCacheRepo) InvalidateFriendList(ctx context.Context, accountID string) error {
	return r.invalidateCache(ctx, "friend_list_cache", accountID)
}

func (r *SQLiteCacheRepo) RemoveFriendFromCache(ctx context.Context, accountID, gid string) error {
	accountID, err := accountRequiredText("accountID", accountID)
	if err != nil {
		return err
	}
	targetGID, err := cacheRequiredGID(gid)
	if err != nil {
		return err
	}
	if err := r.checkDB(); err != nil {
		return err
	}

	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin friend cache update: %w", err)
	}
	defer tx.Rollback()
	if err := cacheEnsureAccount(ctx, tx, accountID); err != nil {
		return err
	}

	var friendList string
	err = tx.QueryRowContext(ctx, "SELECT payload FROM friend_list_cache WHERE account_id = ?", accountID).Scan(&friendList)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("read friend list cache for account %q: %w", accountID, err)
	}
	if err == nil {
		var friends []json.RawMessage
		if err := json.Unmarshal([]byte(friendList), &friends); err != nil {
			return fmt.Errorf("decode friend list cache for account %q: %w", accountID, err)
		}
		filtered := friends[:0]
		changed := false
		for _, friend := range friends {
			if cacheObjectGID(friend) == targetGID {
				changed = true
				continue
			}
			filtered = append(filtered, friend)
		}
		if changed {
			payload, err := json.Marshal(filtered)
			if err != nil {
				return fmt.Errorf("encode friend list cache: %w", err)
			}
			if _, err := tx.ExecContext(ctx,
				"UPDATE friend_list_cache SET payload = ?, updated_at = ? WHERE account_id = ?",
				string(payload), time.Now().UnixMilli(), accountID,
			); err != nil {
				return fmt.Errorf("update friend list cache for account %q: %w", accountID, err)
			}
		}
	}

	var dogInfo string
	err = tx.QueryRowContext(ctx, "SELECT payload FROM friend_dog_info WHERE account_id = ?", accountID).Scan(&dogInfo)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("read friend dog cache for account %q: %w", accountID, err)
	}
	if err == nil {
		var dogs map[string]json.RawMessage
		if err := json.Unmarshal([]byte(dogInfo), &dogs); err != nil {
			return fmt.Errorf("decode friend dog cache for account %q: %w", accountID, err)
		}
		changed := false
		for key := range dogs {
			if cacheCanonicalGID(key) == targetGID {
				delete(dogs, key)
				changed = true
			}
		}
		if changed {
			payload, err := json.Marshal(dogs)
			if err != nil {
				return fmt.Errorf("encode friend dog cache: %w", err)
			}
			if _, err := tx.ExecContext(ctx,
				"UPDATE friend_dog_info SET payload = ?, updated_at = ? WHERE account_id = ?",
				string(payload), time.Now().UnixMilli(), accountID,
			); err != nil {
				return fmt.Errorf("update friend dog cache for account %q: %w", accountID, err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit friend cache update: %w", err)
	}
	return nil
}

func (r *SQLiteCacheRepo) DeleteAccountCaches(ctx context.Context, accountID string) error {
	accountID, err := accountRequiredText("accountID", accountID)
	if err != nil {
		return err
	}
	if err := r.checkDB(); err != nil {
		return err
	}
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin account cache delete: %w", err)
	}
	defer tx.Rollback()
	for _, table := range []string{"friend_gid_cache", "friend_dog_info", "friend_list_cache"} {
		if _, err := tx.ExecContext(ctx, "DELETE FROM "+table+" WHERE account_id = ?", accountID); err != nil {
			return fmt.Errorf("delete %s for account %q: %w", table, accountID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit account cache delete: %w", err)
	}
	return nil
}

func (r *SQLiteCacheRepo) ListBlacklist(ctx context.Context, accountID string) ([]BlacklistEntry, error) {
	accountID, err := accountRequiredText("accountID", accountID)
	if err != nil {
		return nil, err
	}
	if err := r.checkDB(); err != nil {
		return nil, err
	}
	rows, err := r.DB.QueryContext(ctx, `
SELECT account_id, gid, reason, added_at
FROM blacklist WHERE account_id = ? ORDER BY added_at, gid`, accountID)
	if err != nil {
		return nil, fmt.Errorf("list blacklist for account %q: %w", accountID, err)
	}
	defer rows.Close()

	entries := make([]BlacklistEntry, 0)
	for rows.Next() {
		var entry BlacklistEntry
		if err := rows.Scan(&entry.AccountID, &entry.GID, &entry.Reason, &entry.AddedAt); err != nil {
			return nil, fmt.Errorf("scan blacklist for account %q: %w", accountID, err)
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list blacklist for account %q: %w", accountID, err)
	}
	return entries, nil
}

func (r *SQLiteCacheRepo) UpsertBlacklist(ctx context.Context, entry BlacklistEntry) error {
	accountID, err := accountRequiredText("accountID", entry.AccountID)
	if err != nil {
		return err
	}
	gid, err := cacheRequiredGID(entry.GID)
	if err != nil {
		return err
	}
	if err := r.checkDB(); err != nil {
		return err
	}
	explicitTimestamp := entry.AddedAt != 0
	addedAt := entry.AddedAt
	if addedAt == 0 {
		addedAt = time.Now().UnixMilli()
	}

	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin blacklist write: %w", err)
	}
	defer tx.Rollback()
	if err := cacheEnsureAccount(ctx, tx, accountID); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
INSERT INTO blacklist(account_id, gid, reason, added_at) VALUES (?, ?, ?, ?)
ON CONFLICT(account_id, gid) DO UPDATE SET
    reason = excluded.reason,
    added_at = CASE WHEN ? THEN excluded.added_at ELSE blacklist.added_at END`,
		accountID, gid, entry.Reason, addedAt, explicitTimestamp,
	)
	if err != nil {
		return fmt.Errorf("upsert blacklist entry %q for account %q: %w", gid, accountID, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit blacklist write: %w", err)
	}
	return nil
}

func (r *SQLiteCacheRepo) DeleteBlacklist(ctx context.Context, accountID, gid string) error {
	accountID, err := accountRequiredText("accountID", accountID)
	if err != nil {
		return err
	}
	gid, err = cacheRequiredGID(gid)
	if err != nil {
		return err
	}
	if err := r.checkDB(); err != nil {
		return err
	}
	if _, err := r.DB.ExecContext(ctx,
		"DELETE FROM blacklist WHERE account_id = ? AND gid = ?", accountID, gid,
	); err != nil {
		return fmt.Errorf("delete blacklist entry %q for account %q: %w", gid, accountID, err)
	}
	return nil
}

func (r *SQLiteCacheRepo) readCache(ctx context.Context, table, accountID string) (CacheValue, error) {
	accountID, err := accountRequiredText("accountID", accountID)
	if err != nil {
		return CacheValue{}, err
	}
	if err := r.checkDB(); err != nil {
		return CacheValue{}, err
	}
	var payload string
	var updatedAt int64
	err = r.DB.QueryRowContext(ctx,
		"SELECT payload, updated_at FROM "+table+" WHERE account_id = ?", accountID,
	).Scan(&payload, &updatedAt)
	if err != nil {
		return CacheValue{}, fmt.Errorf("read %s for account %q: %w", table, accountID, err)
	}
	return CacheValue{Payload: json.RawMessage(payload), UpdatedAt: updatedAt}, nil
}

func (r *SQLiteCacheRepo) writeCache(
	ctx context.Context,
	table, accountID string,
	value CacheValue,
	fallback string,
) error {
	accountID, err := accountRequiredText("accountID", accountID)
	if err != nil {
		return err
	}
	payload, err := accountJSONText(value.Payload, fallback)
	if err != nil {
		return fmt.Errorf("%s payload for account %q: %w", table, accountID, err)
	}
	if err := r.checkDB(); err != nil {
		return err
	}
	updatedAt := value.UpdatedAt
	if updatedAt == 0 {
		updatedAt = time.Now().UnixMilli()
	}
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin %s write: %w", table, err)
	}
	defer tx.Rollback()
	if err := cacheEnsureAccount(ctx, tx, accountID); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx,
		"INSERT INTO "+table+"(account_id, payload, updated_at) VALUES (?, ?, ?) "+
			"ON CONFLICT(account_id) DO UPDATE SET payload = excluded.payload, updated_at = excluded.updated_at",
		accountID, payload, updatedAt,
	)
	if err != nil {
		return fmt.Errorf("write %s for account %q: %w", table, accountID, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit %s write: %w", table, err)
	}
	return nil
}

func (r *SQLiteCacheRepo) invalidateCache(ctx context.Context, table, accountID string) error {
	accountID, err := accountRequiredText("accountID", accountID)
	if err != nil {
		return err
	}
	if err := r.checkDB(); err != nil {
		return err
	}
	if _, err := r.DB.ExecContext(ctx, "DELETE FROM "+table+" WHERE account_id = ?", accountID); err != nil {
		return fmt.Errorf("invalidate %s for account %q: %w", table, accountID, err)
	}
	return nil
}

func (r *SQLiteCacheRepo) checkDB() error {
	if r == nil || r.DB == nil || r.DB.DB == nil {
		return fmt.Errorf("cache repository database is nil")
	}
	return nil
}

func cacheEnsureAccount(ctx context.Context, tx *sql.Tx, accountID string) error {
	var exists int
	if err := tx.QueryRowContext(ctx, "SELECT 1 FROM accounts WHERE id = ?", accountID).Scan(&exists); err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("account %q: %w", accountID, sql.ErrNoRows)
		}
		return fmt.Errorf("check account %q: %w", accountID, err)
	}
	return nil
}

func cacheRequiredGID(value string) (string, error) {
	value = cacheCanonicalGID(value)
	if value == "" || value == "0" {
		return "", fmt.Errorf("gid must not be empty")
	}
	return value, nil
}

func cacheCanonicalGID(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if parsed, err := strconv.ParseUint(value, 10, 64); err == nil {
		return strconv.FormatUint(parsed, 10)
	}
	return value
}

func cacheObjectGID(value json.RawMessage) string {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(value, &object); err != nil {
		return ""
	}
	raw, ok := object["gid"]
	if !ok {
		return ""
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return cacheCanonicalGID(text)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var number json.Number
	if err := decoder.Decode(&number); err == nil {
		return cacheCanonicalGID(number.String())
	}
	return ""
}

var _ CacheRepo = (*SQLiteCacheRepo)(nil)
