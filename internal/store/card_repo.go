package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	cardTypeTime  = "time"
	cardTypeQuota = "quota"
	dayMillis     = int64(24 * time.Hour / time.Millisecond)
	hourMillis    = int64(time.Hour / time.Millisecond)
)

// CardRepo is the persistence contract for authorization cards. Operations
// that consume a card also update the user in the same SQLite transaction.
type CardRepo interface {
	Create(context.Context, CardSpec) (*Card, error)
	Get(context.Context, string) (*Card, error)
	List(context.Context) ([]Card, error)
	Update(context.Context, string, CardUpdate) (*Card, error)
	Delete(context.Context, string) error
	Claim(context.Context, string, string) (*Card, error)
	RegisterWithCard(context.Context, string, string, string) (*User, error)
	Renew(context.Context, string, string) (*User, error)
	ClaimByUA(context.Context, string, string) (*Card, error)
}

// SQLiteCardRepo stores authorization cards in the cards table created by
// P1-02. ClaimEnabled is process-local because the migration intentionally
// does not add an operational settings table to this card task.
type SQLiteCardRepo struct {
	DB *DB

	claimMu      sync.RWMutex
	claimEnabled bool
}

// CardRepository is a descriptive alias for callers that prefer the concrete
// repository name.
type CardRepository = SQLiteCardRepo

// Card is the normalized representation of a row in cards. Zero values for
// nullable numeric fields mean that the source column was NULL.
type Card struct {
	Code          string
	Description   string
	Type          string
	Status        string
	Enabled       bool
	Days          float64
	Value         int64
	DurationValue float64
	DurationUnit  string
	DurationMS    int64
	IsPermanent   bool
	BoundUser     string
	UsedBy        string
	UsedAt        int64
	ClaimedAt     int64
	CreatedAt     int64
	UpdatedAt     int64
	MetadataJSON  string
}

// CardSpec describes a new card. For time cards, days or
// durationValue/durationUnit may be used; durationMs takes precedence when
// supplied. A duration of -1 or IsPermanent creates a permanent card.
type CardSpec struct {
	Code          string
	Description   string
	Type          string
	Days          float64
	Value         int64
	DurationValue float64
	DurationUnit  string
	DurationMS    int64
	IsPermanent   bool
	Enabled       bool
	MetadataJSON  string
}

// CardUpdate contains mutable card fields. Empty fields are left unchanged.
type CardUpdate struct {
	Description  *string
	Enabled      *bool
	Status       *string
	MetadataJSON *string
}

// CardOptions is accepted by the compatibility CreateCard helper.
type CardOptions struct {
	Value         int64
	DurationValue float64
	DurationUnit  string
	DurationMS    int64
	IsPermanent   bool
}

// NewCardRepo creates a SQLite-backed card repository.
func NewCardRepo(db *DB) *SQLiteCardRepo {
	return &SQLiteCardRepo{DB: db, claimEnabled: true}
}

// NewCardRepository is an alias for NewCardRepo.
func NewCardRepository(db *DB) *SQLiteCardRepo { return NewCardRepo(db) }

func (r *SQLiteCardRepo) ensureDB() error {
	if r == nil || r.DB == nil || r.DB.DB == nil {
		return errors.New("card repository has no database")
	}
	return nil
}

// Create inserts one enabled, unbound card. The generated code is a
// cryptographically random 16-character value matching the Node format.
func (r *SQLiteCardRepo) Create(ctx context.Context, spec CardSpec) (*Card, error) {
	if err := r.ensureDB(); err != nil {
		return nil, err
	}
	spec = normalizeCardSpec(spec)
	if spec.Code == "" {
		var err error
		spec.Code, err = generateCardCode()
		if err != nil {
			return nil, err
		}
	}
	if spec.MetadataJSON == "" {
		spec.MetadataJSON = "{}"
	}
	now := time.Now().UnixMilli()
	_, err := r.DB.ExecContext(ctx, `
INSERT INTO cards (
    code, description, type, status, enabled, days, value,
    duration_value, duration_unit, duration_ms, is_permanent,
    created_at, updated_at, metadata_json
) VALUES (?, ?, ?, 'active', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		spec.Code, spec.Description, spec.Type, boolInt(spec.Enabled),
		nullableFloat64(spec.Days), nullableInt64Value(spec.Value),
		nullableFloat64(spec.DurationValue), spec.DurationUnit,
		nullableInt64Value(spec.DurationMS), boolInt(spec.IsPermanent),
		now, now, spec.MetadataJSON,
	)
	if err != nil {
		if isConstraintError(err) {
			return nil, fmt.Errorf("%w: %s", ErrCardExists, spec.Code)
		}
		return nil, fmt.Errorf("create card: %w", err)
	}
	return r.Get(ctx, spec.Code)
}

// CreateCard retains the source model's convenient positional API while also
// accepting a CardSpec. Supported forms are CreateCard(ctx, CardSpec) and
// CreateCard(ctx, description, days, type[, CardOptions]).
func (r *SQLiteCardRepo) CreateCard(ctx context.Context, args ...any) (*Card, error) {
	spec, err := cardSpecArgs(args...)
	if err != nil {
		return nil, err
	}
	return r.Create(ctx, spec)
}

// CreateCardsBatch creates up to 100 cards in one transaction.
func (r *SQLiteCardRepo) CreateCardsBatch(ctx context.Context, spec CardSpec, count int) ([]Card, error) {
	if err := r.ensureDB(); err != nil {
		return nil, err
	}
	if count < 1 {
		count = 1
	}
	if count > 100 {
		count = 100
	}
	spec = normalizeCardSpec(spec)
	if spec.MetadataJSON == "" {
		spec.MetadataJSON = "{}"
	}
	now := time.Now().UnixMilli()
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin card batch: %w", err)
	}
	defer tx.Rollback()
	created := make([]Card, 0, count)
	for i := 0; i < count; i++ {
		code, codeErr := generateCardCode()
		if codeErr != nil {
			return nil, codeErr
		}
		if _, err := tx.ExecContext(ctx, `
INSERT INTO cards (
    code, description, type, status, enabled, days, value,
    duration_value, duration_unit, duration_ms, is_permanent,
    created_at, updated_at, metadata_json
) VALUES (?, ?, ?, 'active', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			code, spec.Description, spec.Type, boolInt(spec.Enabled),
			nullableFloat64(spec.Days), nullableInt64Value(spec.Value),
			nullableFloat64(spec.DurationValue), spec.DurationUnit,
			nullableInt64Value(spec.DurationMS), boolInt(spec.IsPermanent),
			now, now, spec.MetadataJSON,
		); err != nil {
			return nil, fmt.Errorf("create card %q: %w", code, err)
		}
		card, scanErr := scanCard(tx.QueryRowContext(ctx, cardSelectSQL+" WHERE code = ?", code))
		if scanErr != nil {
			return nil, fmt.Errorf("read created card %q: %w", code, scanErr)
		}
		created = append(created, *card)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit card batch: %w", err)
	}
	return created, nil
}

// Get returns one card by its code.
func (r *SQLiteCardRepo) Get(ctx context.Context, code string) (*Card, error) {
	if err := r.ensureDB(); err != nil {
		return nil, err
	}
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, ErrInvalidCard
	}
	card, err := scanCard(r.DB.QueryRowContext(ctx, cardSelectSQL+" WHERE code = ?", code))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrCardNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get card %q: %w", code, err)
	}
	return card, nil
}

// GetCard is an alias for Get.
func (r *SQLiteCardRepo) GetCard(ctx context.Context, code string) (*Card, error) {
	return r.Get(ctx, code)
}

// List returns cards ordered newest first, matching the admin card view.
func (r *SQLiteCardRepo) List(ctx context.Context) ([]Card, error) {
	if err := r.ensureDB(); err != nil {
		return nil, err
	}
	rows, err := r.DB.QueryContext(ctx, cardSelectSQL+" ORDER BY created_at DESC, code")
	if err != nil {
		return nil, fmt.Errorf("list cards: %w", err)
	}
	defer rows.Close()
	cards := make([]Card, 0)
	for rows.Next() {
		card, scanErr := scanCard(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("scan card: %w", scanErr)
		}
		cards = append(cards, *card)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate cards: %w", err)
	}
	return cards, nil
}

// ListCards is an alias for List.
func (r *SQLiteCardRepo) ListCards(ctx context.Context) ([]Card, error) {
	return r.List(ctx)
}

// Update changes only fields exposed by the admin card editor.
func (r *SQLiteCardRepo) Update(ctx context.Context, code string, update CardUpdate) (*Card, error) {
	if err := r.ensureDB(); err != nil {
		return nil, err
	}
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, ErrInvalidCard
	}
	sets := make([]string, 0, 4)
	args := make([]any, 0, 4)
	if update.Description != nil {
		sets = append(sets, "description = ?")
		args = append(args, *update.Description)
	}
	if update.Enabled != nil {
		sets = append(sets, "enabled = ?")
		args = append(args, boolInt(*update.Enabled))
	}
	if update.Status != nil {
		sets = append(sets, "status = ?")
		args = append(args, *update.Status)
	}
	if update.MetadataJSON != nil {
		sets = append(sets, "metadata_json = ?")
		args = append(args, *update.MetadataJSON)
	}
	if len(sets) == 0 {
		return r.Get(ctx, code)
	}
	sets = append(sets, "updated_at = ?")
	args = append(args, time.Now().UnixMilli(), code)
	result, err := r.DB.ExecContext(ctx, "UPDATE cards SET "+strings.Join(sets, ", ")+" WHERE code = ?", args...)
	if err != nil {
		return nil, fmt.Errorf("update card %q: %w", code, err)
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return nil, ErrCardNotFound
	}
	return r.Get(ctx, code)
}

// UpdateCard is an alias for Update.
func (r *SQLiteCardRepo) UpdateCard(ctx context.Context, code string, update CardUpdate) (*Card, error) {
	return r.Update(ctx, code, update)
}

// Delete removes a card. The source model permits deleting used cards too.
func (r *SQLiteCardRepo) Delete(ctx context.Context, code string) error {
	if err := r.ensureDB(); err != nil {
		return err
	}
	result, err := r.DB.ExecContext(ctx, "DELETE FROM cards WHERE code = ?", strings.TrimSpace(code))
	if err != nil {
		return fmt.Errorf("delete card %q: %w", code, err)
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return ErrCardNotFound
	}
	return nil
}

// DeleteCard is an alias for Delete.
func (r *SQLiteCardRepo) DeleteCard(ctx context.Context, code string) error {
	return r.Delete(ctx, code)
}

// Claim atomically consumes a card for an existing user. Time cards extend
// the user's entitlement; quota cards increase account_limit.
func (r *SQLiteCardRepo) Claim(ctx context.Context, code, username string) (*Card, error) {
	if strings.TrimSpace(username) == "" {
		return nil, ErrInvalidUsername
	}
	if err := r.ensureDB(); err != nil {
		return nil, err
	}
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin card claim: %w", err)
	}
	defer tx.Rollback()
	card, err := scanCard(tx.QueryRowContext(ctx, cardSelectSQL+" WHERE code = ?", strings.TrimSpace(code)))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrCardNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("read card for claim: %w", err)
	}
	if err := validateCardAvailable(*card); err != nil {
		return nil, err
	}
	user, err := scanUser(tx.QueryRowContext(ctx, userSelectSQL+" WHERE username = ?", username))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("read user for card claim: %w", err)
	}
	if err := applyCardToUserTx(ctx, tx, user, *card); err != nil {
		return nil, err
	}
	now := time.Now().UnixMilli()
	result, err := tx.ExecContext(ctx, `UPDATE cards
SET bound_user = ?, status = 'used', used_at = ?, claimed_at = ?, updated_at = ?
WHERE code = ? AND bound_user IS NULL AND enabled = 1 AND status = 'active'`,
		username, now, now, now, card.Code)
	if err != nil {
		return nil, fmt.Errorf("consume card %q: %w", card.Code, err)
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return nil, ErrCardAlreadyBound
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit card claim: %w", err)
	}
	card.BoundUser, card.UsedBy = username, username
	card.Status, card.UsedAt, card.ClaimedAt, card.UpdatedAt = "used", now, now, now
	return card, nil
}

// ClaimCard is an alias for Claim.
func (r *SQLiteCardRepo) ClaimCard(ctx context.Context, code, username string) (*Card, error) {
	return r.Claim(ctx, code, username)
}

// RegisterWithCard creates a new user and consumes a time card atomically.
// Quota cards intentionally cannot be used for registration, matching the
// source business rule.
func (r *SQLiteCardRepo) RegisterWithCard(ctx context.Context, username, password, code string) (*User, error) {
	username = strings.TrimSpace(username)
	if !usernamePattern.MatchString(username) {
		return nil, ErrInvalidUsername
	}
	if err := validateRegistrationPassword(password); err != nil {
		return nil, err
	}
	if err := r.ensureDB(); err != nil {
		return nil, err
	}
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin user registration: %w", err)
	}
	defer tx.Rollback()
	var exists int
	if err := tx.QueryRowContext(ctx, "SELECT 1 FROM users WHERE username = ?", username).Scan(&exists); err == nil {
		return nil, ErrUserExists
	} else if err != sql.ErrNoRows {
		return nil, fmt.Errorf("check registered user: %w", err)
	}
	card, err := scanCard(tx.QueryRowContext(ctx, cardSelectSQL+" WHERE code = ?", strings.TrimSpace(code)))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrCardNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("read registration card: %w", err)
	}
	if err := validateCardAvailable(*card); err != nil {
		return nil, err
	}
	if card.Type == cardTypeQuota {
		return nil, ErrInvalidCard
	}
	hashed, err := HashPassword(password)
	if err != nil {
		return nil, err
	}
	pwdHash, salt, err := splitStoredPassword(hashed)
	if err != nil {
		return nil, err
	}
	now := time.Now().UnixMilli()
	user := &User{Username: username, PwdHash: pwdHash, Salt: salt, Password: hashed,
		Role: "user", Status: "active", AccountLimit: defaultAccountLimit,
		CardCode: card.Code, CardJSON: membershipJSON(*card, now, nil), CreatedAt: now, UpdatedAt: now}
	if !card.IsPermanent {
		expires := now + cardDurationMillis(*card)
		user.ExpireAt = &expires
		user.CardJSON = membershipJSON(*card, now, &expires)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO users
    (username, pwd_hash, salt, role, status, expire_at, account_limit,
     card_code, card_json, must_change_password, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 0, ?, ?)`,
		user.Username, user.PwdHash, user.Salt, user.Role, user.Status,
		nullableInt64(user.ExpireAt), user.AccountLimit, user.CardCode,
		user.CardJSON, user.CreatedAt, user.UpdatedAt); err != nil {
		if isConstraintError(err) {
			return nil, fmt.Errorf("%w: %s", ErrUserExists, username)
		}
		return nil, fmt.Errorf("create registered user: %w", err)
	}
	result, err := tx.ExecContext(ctx, `UPDATE cards
SET bound_user = ?, status = 'used', used_at = ?, claimed_at = ?, updated_at = ?
WHERE code = ? AND bound_user IS NULL AND enabled = 1 AND status = 'active'`,
		username, now, now, now, card.Code)
	if err != nil {
		return nil, fmt.Errorf("consume registration card: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return nil, ErrCardAlreadyBound
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit user registration: %w", err)
	}
	return user, nil
}

// Renew atomically consumes an unused card for an existing user.
func (r *SQLiteCardRepo) Renew(ctx context.Context, username, code string) (*User, error) {
	if strings.TrimSpace(username) == "" {
		return nil, ErrInvalidUsername
	}
	if err := r.ensureDB(); err != nil {
		return nil, err
	}
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin user renewal: %w", err)
	}
	defer tx.Rollback()
	user, err := scanUser(tx.QueryRowContext(ctx, userSelectSQL+" WHERE username = ?", username))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("read user for renewal: %w", err)
	}
	card, err := scanCard(tx.QueryRowContext(ctx, cardSelectSQL+" WHERE code = ?", strings.TrimSpace(code)))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrCardNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("read renewal card: %w", err)
	}
	if err := validateCardAvailable(*card); err != nil {
		return nil, err
	}
	if err := applyCardToUserTx(ctx, tx, user, *card); err != nil {
		return nil, err
	}
	now := time.Now().UnixMilli()
	result, err := tx.ExecContext(ctx, `UPDATE cards
SET bound_user = ?, status = 'used', used_at = ?, claimed_at = ?, updated_at = ?
WHERE code = ? AND bound_user IS NULL AND enabled = 1 AND status = 'active'`,
		username, now, now, now, card.Code)
	if err != nil {
		return nil, fmt.Errorf("consume renewal card: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return nil, ErrCardAlreadyBound
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit user renewal: %w", err)
	}
	return user, nil
}

// RenewCard is an alias for Renew.
func (r *SQLiteCardRepo) RenewCard(ctx context.Context, username, code string) (*User, error) {
	return r.Renew(ctx, username, code)
}

// ClaimByUA returns one available time card and records a 24-hour UA claim
// in login_logs. The card remains unbound so the caller can use it to register;
// RegisterWithCard performs the consuming transaction.
func (r *SQLiteCardRepo) ClaimByUA(ctx context.Context, ua, username string) (*Card, error) {
	if strings.TrimSpace(ua) == "" {
		return nil, ErrInvalidCard
	}
	if err := r.ensureDB(); err != nil {
		return nil, err
	}
	r.claimMu.RLock()
	enabled := r.claimEnabled
	r.claimMu.RUnlock()
	if !enabled {
		return nil, ErrCardUnavailable
	}
	uaHash := hashUA(ua)
	now := time.Now().UnixMilli()
	var previous int64
	err := r.DB.QueryRowContext(ctx, `SELECT ts FROM login_logs
WHERE event = 'card_claim' AND error_type = ? AND ts > ?
ORDER BY ts DESC LIMIT 1`, uaHash, now-int64(24*time.Hour/time.Millisecond)).Scan(&previous)
	if err == nil {
		return nil, ErrCardAlreadyBound
	}
	if err != sql.ErrNoRows {
		return nil, fmt.Errorf("check UA card claim: %w", err)
	}
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin UA card claim: %w", err)
	}
	defer tx.Rollback()
	var claimedAt int64
	err = tx.QueryRowContext(ctx, `SELECT ts FROM login_logs
WHERE event = 'card_claim' AND error_type = ? AND ts > ?
ORDER BY ts DESC LIMIT 1`, uaHash, now-int64(24*time.Hour/time.Millisecond)).Scan(&claimedAt)
	if err == nil {
		return nil, ErrCardAlreadyBound
	}
	if err != sql.ErrNoRows {
		return nil, fmt.Errorf("recheck UA card claim: %w", err)
	}
	card, err := scanCard(tx.QueryRowContext(ctx, cardSelectSQL+`
 WHERE type = 'time' AND enabled = 1 AND status = 'active' AND bound_user IS NULL
 ORDER BY RANDOM() LIMIT 1`))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrCardUnavailable
	}
	if err != nil {
		return nil, fmt.Errorf("select UA card: %w", err)
	}
	metadata, _ := json.Marshal(map[string]string{"uaHash": uaHash})
	if _, err := tx.ExecContext(ctx, `INSERT INTO login_logs
    (user, username, ip, ua, user_agent, result, event, error_type, ts, metadata_json)
VALUES (?, ?, '', ?, ?, 'claimed', 'card_claim', ?, ?, ?)`,
		username, nullableString(username), ua, ua, uaHash, now, string(metadata)); err != nil {
		return nil, fmt.Errorf("record UA card claim: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit UA card claim: %w", err)
	}
	return card, nil
}

// SetClaimEnabled controls the public UA claim endpoint for this repository
// instance. It defaults to true, matching the legacy store.
func (r *SQLiteCardRepo) SetClaimEnabled(enabled bool) {
	if r == nil {
		return
	}
	r.claimMu.Lock()
	r.claimEnabled = enabled
	r.claimMu.Unlock()
}

// SetCardClaimStatus is a source-compatible alias for SetClaimEnabled.
func (r *SQLiteCardRepo) SetCardClaimStatus(enabled bool) { r.SetClaimEnabled(enabled) }

// GetClaimStatus reports whether UA claims are enabled.
func (r *SQLiteCardRepo) GetClaimStatus() bool {
	if r == nil {
		return false
	}
	r.claimMu.RLock()
	defer r.claimMu.RUnlock()
	return r.claimEnabled
}

// GetCardClaimStatus is a source-compatible alias for GetClaimStatus.
func (r *SQLiteCardRepo) GetCardClaimStatus() bool { return r.GetClaimStatus() }

// GetAvailableTimeCardCount returns the number of currently claimable time
// cards.
func (r *SQLiteCardRepo) GetAvailableTimeCardCount(ctx context.Context) (int, error) {
	if err := r.ensureDB(); err != nil {
		return 0, err
	}
	var count int
	if err := r.DB.QueryRowContext(ctx, `SELECT count(*) FROM cards
WHERE type = 'time' AND enabled = 1 AND status = 'active' AND bound_user IS NULL`).Scan(&count); err != nil {
		return 0, fmt.Errorf("count available cards: %w", err)
	}
	return count, nil
}

const cardSelectSQL = `
SELECT code, description, type, status, enabled, days, value,
       duration_value, duration_unit, duration_ms, is_permanent,
       bound_user, used_at, claimed_at, created_at, updated_at, metadata_json
FROM cards`

func scanCard(row rowScanner) (*Card, error) {
	var card Card
	var enabled, permanent sql.NullInt64
	var days, value, durationValue, durationMS sql.NullFloat64
	var boundUser sql.NullString
	var usedAt, claimedAt sql.NullInt64
	if err := row.Scan(&card.Code, &card.Description, &card.Type, &card.Status,
		&enabled, &days, &value, &durationValue, &card.DurationUnit,
		&durationMS, &permanent, &boundUser, &usedAt, &claimedAt,
		&card.CreatedAt, &card.UpdatedAt, &card.MetadataJSON); err != nil {
		return nil, err
	}
	card.Enabled = enabled.Int64 != 0
	card.IsPermanent = permanent.Int64 != 0
	if days.Valid {
		card.Days = days.Float64
	}
	if value.Valid {
		card.Value = int64(value.Float64)
	}
	if durationValue.Valid {
		card.DurationValue = durationValue.Float64
	}
	if durationMS.Valid {
		card.DurationMS = int64(durationMS.Float64)
	}
	if boundUser.Valid {
		card.BoundUser, card.UsedBy = boundUser.String, boundUser.String
	}
	if usedAt.Valid {
		card.UsedAt = usedAt.Int64
	}
	if claimedAt.Valid {
		card.ClaimedAt = claimedAt.Int64
	}
	return &card, nil
}

func normalizeCardSpec(spec CardSpec) CardSpec {
	if spec.Type != cardTypeQuota {
		spec.Type = cardTypeTime
	}
	if spec.Type == cardTypeQuota {
		if spec.Value <= 0 {
			spec.Value = maxInt64(1, int64(spec.Days))
		}
		spec.Days = float64(spec.Value)
		spec.DurationValue, spec.DurationMS, spec.IsPermanent = 0, 0, false
		spec.DurationUnit = "day"
	} else {
		duration := normalizeDuration(spec.DurationValue, spec.DurationUnit, spec.DurationMS, spec.Days, spec.IsPermanent)
		spec.Days, spec.DurationValue, spec.DurationUnit, spec.DurationMS, spec.IsPermanent = duration.days, duration.value, duration.unit, duration.ms, duration.permanent
	}
	if !spec.Enabled {
		// New cards are enabled by default; callers can disable them through
		// Update after creation without making the common zero value unusable.
		spec.Enabled = true
	}
	return spec
}

type normalizedDuration struct {
	days      float64
	value     float64
	unit      string
	ms        int64
	permanent bool
}

func normalizeDuration(value float64, unit string, durationMS int64, days float64, permanent bool) normalizedDuration {
	if permanent || value == -1 || days == -1 {
		return normalizedDuration{days: -1, value: -1, unit: "day", permanent: true}
	}
	if unit != "hour" {
		unit = "day"
	}
	if durationMS > 0 {
		if value <= 0 {
			base := float64(dayMillis)
			if unit == "hour" {
				base = float64(hourMillis)
			}
			value = float64(durationMS) / base
		}
	} else {
		if value <= 0 {
			value = days
		}
		if value <= 0 {
			value = 1
		}
		base := float64(dayMillis)
		if unit == "hour" {
			base = float64(hourMillis)
		}
		durationMS = int64(math.Round(value * base))
	}
	return normalizedDuration{days: float64(durationMS) / float64(dayMillis), value: value, unit: unit, ms: durationMS}
}

func cardDurationMillis(card Card) int64 {
	if card.IsPermanent || card.DurationValue == -1 || card.Days == -1 {
		return 0
	}
	if card.DurationMS > 0 {
		return card.DurationMS
	}
	value := card.DurationValue
	if value <= 0 {
		value = card.Days
	}
	if value <= 0 {
		value = 1
	}
	if card.DurationUnit == "hour" {
		return int64(math.Round(value * float64(hourMillis)))
	}
	return int64(math.Round(value * float64(dayMillis)))
}

func validateCardAvailable(card Card) error {
	if !card.Enabled || card.Status != "active" {
		return ErrCardUnavailable
	}
	if card.BoundUser != "" || card.UsedAt != 0 {
		return ErrCardAlreadyBound
	}
	return nil
}

type cardMembership struct {
	Code          string  `json:"code"`
	Description   string  `json:"description"`
	Days          float64 `json:"days"`
	DurationValue float64 `json:"durationValue"`
	DurationUnit  string  `json:"durationUnit"`
	DurationMS    int64   `json:"durationMs"`
	IsPermanent   bool    `json:"isPermanent"`
	Enabled       bool    `json:"enabled"`
	ExpiresAt     *int64  `json:"expiresAt"`
}

func membershipJSON(card Card, now int64, expiresAt *int64) string {
	membership := cardMembership{Code: card.Code, Description: card.Description,
		Days: card.Days, DurationValue: card.DurationValue, DurationUnit: card.DurationUnit,
		DurationMS: card.DurationMS, IsPermanent: card.IsPermanent, Enabled: true,
		ExpiresAt: expiresAt}
	if card.IsPermanent {
		membership.Days, membership.DurationValue, membership.DurationMS = -1, -1, 0
		membership.ExpiresAt = nil
	}
	if membership.DurationUnit == "" {
		membership.DurationUnit = "day"
	}
	_ = now
	payload, _ := json.Marshal(membership)
	return string(payload)
}

func applyCardToUserTx(ctx context.Context, tx *sql.Tx, user *User, card Card) error {
	now := time.Now().UnixMilli()
	if user.AccountLimit <= 0 {
		user.AccountLimit = defaultAccountLimit
	}
	user.CardCode = card.Code
	if card.Type == cardTypeQuota {
		increment := card.Value
		if increment <= 0 {
			increment = int64(maxInt64(1, int64(card.Days)))
		}
		user.AccountLimit += int(increment)
	} else {
		var previous cardMembership
		if user.CardJSON != "" {
			_ = json.Unmarshal([]byte(user.CardJSON), &previous)
		}
		duration := cardDurationMillis(card)
		if card.IsPermanent || card.DurationValue == -1 || card.Days == -1 || previous.IsPermanent {
			user.ExpireAt = nil
			user.CardJSON = membershipJSON(card, now, nil)
			if previous.IsPermanent {
				user.CardJSON = membershipJSON(Card{Code: card.Code, Description: card.Description, Days: -1, DurationValue: -1, DurationUnit: "day", IsPermanent: true}, now, nil)
			}
		} else {
			previousDuration := membershipDurationMillis(previous)
			totalDuration := previousDuration + duration
			var expires int64
			if previous.ExpiresAt != nil && *previous.ExpiresAt > now {
				expires = *previous.ExpiresAt + duration
			} else if user.ExpireAt != nil && *user.ExpireAt > now {
				expires = *user.ExpireAt + duration
			} else {
				expires = now + duration
			}
			user.ExpireAt = &expires
			membership := card
			membership.DurationMS = totalDuration
			if card.DurationUnit == "hour" && totalDuration%hourMillis == 0 {
				membership.DurationValue = float64(totalDuration / hourMillis)
				membership.DurationUnit = "hour"
			} else {
				membership.DurationValue = float64(totalDuration) / float64(dayMillis)
				membership.DurationUnit = "day"
			}
			membership.Days = float64(totalDuration) / float64(dayMillis)
			user.CardJSON = membershipJSON(membership, now, &expires)
		}
	}
	user.UpdatedAt = now
	_, err := tx.ExecContext(ctx, `UPDATE users SET expire_at = ?, account_limit = ?, card_code = ?, card_json = ?, updated_at = ? WHERE username = ?`,
		nullableInt64(user.ExpireAt), user.AccountLimit, user.CardCode, user.CardJSON, user.UpdatedAt, user.Username)
	if err != nil {
		return fmt.Errorf("apply card to user %q: %w", user.Username, err)
	}
	return nil
}

func membershipDurationMillis(membership cardMembership) int64 {
	if membership.IsPermanent {
		return 0
	}
	if membership.DurationMS > 0 {
		return membership.DurationMS
	}
	value := membership.DurationValue
	if value <= 0 {
		value = membership.Days
	}
	if value <= 0 {
		return 0
	}
	if membership.DurationUnit == "hour" {
		return int64(math.Round(value * float64(hourMillis)))
	}
	return int64(math.Round(value * float64(dayMillis)))
}

func validateRegistrationPassword(password string) error {
	if len(password) < 6 || len(password) > 128 {
		return ErrInvalidPassword
	}
	complexity := 0
	for _, class := range []string{"abcdefghijklmnopqrstuvwxyz", "ABCDEFGHIJKLMNOPQRSTUVWXYZ", "0123456789", `!@#$%^&*(),.?":{}|<>_-+=[\\];'/\x60~`} {
		if strings.ContainsAny(password, class) {
			complexity++
		}
	}
	if complexity < 2 || containsWeakPassword(password) {
		return ErrInvalidPassword
	}
	return nil
}

func containsWeakPassword(password string) bool {
	switch strings.ToLower(password) {
	case "password", "123456", "qwerty", "abc123", "111111", "000000":
		return true
	default:
		return false
	}
}

func cardSpecArgs(args ...any) (CardSpec, error) {
	if len(args) == 1 {
		if spec, ok := args[0].(CardSpec); ok {
			return spec, nil
		}
	}
	if len(args) < 3 || len(args) > 4 {
		return CardSpec{}, ErrInvalidCard
	}
	description, ok := args[0].(string)
	if !ok {
		return CardSpec{}, ErrInvalidCard
	}
	days, ok := numberFloat(args[1])
	if !ok {
		return CardSpec{}, ErrInvalidCard
	}
	cardType, ok := args[2].(string)
	if !ok {
		return CardSpec{}, ErrInvalidCard
	}
	spec := CardSpec{Description: description, Days: days, Type: cardType}
	if len(args) == 4 {
		switch options := args[3].(type) {
		case CardOptions:
			spec.Value, spec.DurationValue, spec.DurationUnit, spec.DurationMS, spec.IsPermanent = options.Value, options.DurationValue, options.DurationUnit, options.DurationMS, options.IsPermanent
		case *CardOptions:
			if options != nil {
				spec.Value, spec.DurationValue, spec.DurationUnit, spec.DurationMS, spec.IsPermanent = options.Value, options.DurationValue, options.DurationUnit, options.DurationMS, options.IsPermanent
			}
		default:
			return CardSpec{}, ErrInvalidCard
		}
	}
	return spec, nil
}

func numberFloat(value any) (float64, bool) {
	switch number := value.(type) {
	case int:
		return float64(number), true
	case int64:
		return float64(number), true
	case float64:
		return number, true
	case float32:
		return float64(number), true
	case string:
		parsed, err := strconv.ParseFloat(number, 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}

func generateCardCode() (string, error) {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		return "", fmt.Errorf("generate card code: %w", err)
	}
	for i := range random {
		random[i] = alphabet[int(random[i])%len(alphabet)]
	}
	return string(random), nil
}

func hashUA(ua string) string {
	digest := sha256.Sum256([]byte(ua))
	return hex.EncodeToString(digest[:])
}

func nullableFloat64(value float64) any {
	if value == 0 {
		return nil
	}
	return value
}

func nullableInt64Value(value int64) any {
	if value == 0 {
		return nil
	}
	return value
}

func maxInt64(left, right int64) int64 {
	if left > right {
		return left
	}
	return right
}
