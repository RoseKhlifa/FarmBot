package store

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/subtle"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

const (
	passwordSaltBytes    = 32
	passwordIterations   = 100000
	passwordKeyBytes     = 64
	maxUserLoginAttempts = 5
	userLockoutDuration  = 15 * time.Minute
	maxIPLoginAttempts   = 6
	ipRateLimitWindow    = time.Minute
	ipLockoutDuration    = 10 * time.Minute
	defaultAccountLimit  = 2
	defaultAdminUsername = "admin"
	defaultAdminPassword = "admin"
	defaultLoginLogLimit = 100
	maxLoginLogLimit     = 500
)

var (
	// ErrUserNotFound and the other sentinel errors allow handlers to map
	// repository failures to stable HTTP responses without parsing messages.
	ErrUserNotFound       = errors.New("user not found")
	ErrUserExists         = errors.New("user already exists")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUserLocked         = errors.New("user is locked")
	ErrUserExpired        = errors.New("user entitlement expired")
	ErrRateLimited        = errors.New("login rate limited")
	ErrInvalidPassword    = errors.New("invalid password")
	ErrInvalidUsername    = errors.New("invalid username")
	ErrCardNotFound       = errors.New("card not found")
	ErrCardExists         = errors.New("card already exists")
	ErrCardUnavailable    = errors.New("card unavailable")
	ErrCardAlreadyBound   = errors.New("card already bound")
	ErrInvalidCard        = errors.New("invalid card")
	usernamePattern       = regexp.MustCompile(`^\w{3,32}$`)
)

// UserRepo is the persistence contract used by authentication handlers. The
// concrete implementation below is intentionally small and database-backed so
// tests and the composition root can depend on this interface.
type UserRepo interface {
	Create(context.Context, User) error
	Get(context.Context, string) (*User, error)
	List(context.Context) ([]User, error)
	Update(context.Context, User) error
	Delete(context.Context, string) error
	Authenticate(context.Context, string, string, string) (*User, error)
	InitializeDefaultAdmin(context.Context) error
	CheckRateLimit(context.Context, string) (RateLimitResult, error)
	CheckAccountLockout(context.Context, string) (RateLimitResult, error)
	RecordFailedAttempt(context.Context, string) (AttemptResult, error)
	ClearFailedAttempts(context.Context, string, string) error
	AddLoginLog(context.Context, LoginLog) error
	RecordLoginLog(context.Context, LoginLog) error
	GetLoginLogs(context.Context, int, int) ([]LoginLog, int, error)
	ClearLoginLogs(context.Context) error
}

// SQLiteUserRepo stores panel users, password credentials and login audit
// records in the main SQLite database.
type SQLiteUserRepo struct {
	DB *DB
}

// UserRepository is a descriptive alias retained for callers that prefer a
// concrete repository name.
type UserRepository = SQLiteUserRepo

// User is the typed representation of a row in users. Password is a legacy
// compatibility alias for the complete "salt:hash" value; new code should use
// PwdHash and Salt separately.
type User struct {
	Username           string
	PwdHash            string
	Salt               string
	Password           string
	Role               string
	Status             string
	ExpireAt           *int64
	AccountLimit       int
	CardCode           string
	CardJSON           string
	MustChangePassword bool
	CreatedAt          int64
	UpdatedAt          int64
}

// LoginLog is the normalized login audit record. Event/error fields preserve
// the shape of the existing login-logs.json data while user/ua/result/ts are
// convenient stable fields for Go handlers.
type LoginLog struct {
	ID           int64
	User         string
	Username     string
	IP           string
	UA           string
	UserAgent    string
	Result       string
	Event        string
	ErrorType    string
	TS           int64
	MetadataJSON string
}

// RateLimitResult reports the outcome of an IP or account attempt check.
type RateLimitResult struct {
	Allowed     bool
	Count       int
	LockedUntil int64
	Remaining   time.Duration
}

// AttemptResult reports account-level failed-login state.
type AttemptResult struct {
	Locked            bool
	Count             int
	RemainingAttempts int
	LockedUntil       int64
}

// NewUserRepo creates a SQLite-backed user repository. Initialization of the
// default admin is explicit via InitializeDefaultAdmin so opening a database
// never silently mutates authentication state.
func NewUserRepo(db *DB) *SQLiteUserRepo { return &SQLiteUserRepo{DB: db} }

// NewUserRepository is an alias for NewUserRepo.
func NewUserRepository(db *DB) *SQLiteUserRepo { return NewUserRepo(db) }

// HashPassword returns the current PBKDF2-SHA512 password format used by the
// Node implementation: a hexadecimal salt, a colon, then a hexadecimal key.
func HashPassword(password string) (string, error) {
	salt := make([]byte, passwordSaltBytes)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate password salt: %w", err)
	}
	saltText := hex.EncodeToString(salt)
	hash := derivePassword(password, saltText)
	return saltText + ":" + hex.EncodeToString(hash), nil
}

// VerifyPassword accepts both the current salt:hash format and the legacy
// SHA-256-only format used before the PBKDF2 migration.
func VerifyPassword(password, stored string) bool {
	parts := strings.SplitN(stored, ":", 2)
	if len(parts) != 2 {
		legacy := sha256.Sum256([]byte(password))
		return subtle.ConstantTimeCompare([]byte(hex.EncodeToString(legacy[:])), []byte(stored)) == 1
	}
	computed := hex.EncodeToString(derivePassword(password, parts[0]))
	return subtle.ConstantTimeCompare([]byte(computed), []byte(parts[1])) == 1
}

// VerifyPasswordParts is useful when a repository has already separated the
// salt and hash columns from the database row.
func VerifyPasswordParts(password, pwdHash, salt string) bool {
	if salt == "" {
		return VerifyPassword(password, pwdHash)
	}
	computed := hex.EncodeToString(derivePassword(password, salt))
	return subtle.ConstantTimeCompare([]byte(computed), []byte(pwdHash)) == 1
}

func derivePassword(password, salt string) []byte {
	return pbkdf2SHA512([]byte(password), []byte(salt), passwordIterations, passwordKeyBytes)
}

// pbkdf2SHA512 is the RFC 8018 PBKDF2 construction. It is kept local so the
// storage package remains buildable with the module's declared Go 1.23 toolchain
// without adding another dependency solely for password hashing.
func pbkdf2SHA512(password, salt []byte, iterations, keyLength int) []byte {
	hashSize := sha512.Size
	blocks := (keyLength + hashSize - 1) / hashSize
	derived := make([]byte, 0, blocks*hashSize)
	for block := 1; block <= blocks; block++ {
		mac := hmac.New(sha512.New, password)
		_, _ = mac.Write(salt)
		var counter [4]byte
		binary.BigEndian.PutUint32(counter[:], uint32(block))
		_, _ = mac.Write(counter[:])
		u := mac.Sum(nil)
		t := append([]byte(nil), u...)
		for round := 1; round < iterations; round++ {
			mac.Reset()
			_, _ = mac.Write(u)
			u = mac.Sum(nil)
			for i := range t {
				t[i] ^= u[i]
			}
		}
		derived = append(derived, t...)
	}
	return derived[:keyLength]
}

func (r *SQLiteUserRepo) ensureDB() error {
	if r == nil || r.DB == nil || r.DB.DB == nil {
		return errors.New("user repository has no database")
	}
	return nil
}

// Create inserts a user atomically. If Password is supplied it is normalized
// into PwdHash/Salt; callers may also supply an existing legacy hash directly.
func (r *SQLiteUserRepo) Create(ctx context.Context, user User) error {
	if err := r.ensureDB(); err != nil {
		return err
	}
	user.Username = strings.TrimSpace(user.Username)
	if !usernamePattern.MatchString(user.Username) {
		return ErrInvalidUsername
	}
	pwdHash, salt, err := credentialParts(user)
	if err != nil {
		return err
	}
	if user.Role == "" {
		user.Role = "user"
	}
	if user.Status == "" {
		user.Status = "active"
	}
	if user.AccountLimit <= 0 {
		user.AccountLimit = defaultAccountLimit
	}
	if user.CardJSON == "" {
		user.CardJSON = "{}"
	}
	now := time.Now().UnixMilli()
	if user.CreatedAt == 0 {
		user.CreatedAt = now
	}
	if user.UpdatedAt == 0 {
		user.UpdatedAt = now
	}
	_, err = r.DB.ExecContext(ctx, `
INSERT INTO users
    (username, pwd_hash, salt, role, status, expire_at, account_limit,
     card_code, card_json, must_change_password, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		user.Username, pwdHash, salt, user.Role, user.Status, nullableInt64(user.ExpireAt),
		user.AccountLimit, nullableString(user.CardCode), user.CardJSON,
		boolInt(user.MustChangePassword), user.CreatedAt, user.UpdatedAt,
	)
	if err != nil {
		if isConstraintError(err) {
			return fmt.Errorf("%w: %s", ErrUserExists, user.Username)
		}
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

// Get returns one user by username.
func (r *SQLiteUserRepo) Get(ctx context.Context, username string) (*User, error) {
	if err := r.ensureDB(); err != nil {
		return nil, err
	}
	user, err := scanUser(r.DB.QueryRowContext(ctx, userSelectSQL+" WHERE username = ?", username))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	return user, nil
}

// List returns users ordered by creation time. It includes the hard-coded
// super-admin only if such a row was explicitly imported.
func (r *SQLiteUserRepo) List(ctx context.Context) ([]User, error) {
	if err := r.ensureDB(); err != nil {
		return nil, err
	}
	rows, err := r.DB.QueryContext(ctx, userSelectSQL+" ORDER BY created_at, username")
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()
	users := make([]User, 0)
	for rows.Next() {
		user, scanErr := scanUser(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("scan user: %w", scanErr)
		}
		users = append(users, *user)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate users: %w", err)
	}
	return users, nil
}

// Update replaces mutable user fields while retaining existing credentials or
// timestamps when the caller leaves those fields empty.
func (r *SQLiteUserRepo) Update(ctx context.Context, user User) error {
	if err := r.ensureDB(); err != nil {
		return err
	}
	current, err := r.Get(ctx, user.Username)
	if err != nil {
		return err
	}
	merged := mergeUser(*current, user)
	pwdHash, salt, err := credentialParts(merged)
	if err != nil {
		return err
	}
	if merged.UpdatedAt == 0 {
		merged.UpdatedAt = time.Now().UnixMilli()
	}
	_, err = r.DB.ExecContext(ctx, `
UPDATE users SET pwd_hash = ?, salt = ?, role = ?, status = ?, expire_at = ?,
    account_limit = ?, card_code = ?, card_json = ?, must_change_password = ?,
    created_at = ?, updated_at = ?
WHERE username = ?`,
		pwdHash, salt, merged.Role, merged.Status, nullableInt64(merged.ExpireAt),
		merged.AccountLimit, nullableString(merged.CardCode), merged.CardJSON,
		boolInt(merged.MustChangePassword), merged.CreatedAt, merged.UpdatedAt,
		merged.Username,
	)
	if err != nil {
		return fmt.Errorf("update user: %w", err)
	}
	return nil
}

// Delete removes a user and releases any cards bound to it in one transaction.
func (r *SQLiteUserRepo) Delete(ctx context.Context, username string) error {
	if err := r.ensureDB(); err != nil {
		return err
	}
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin delete user: %w", err)
	}
	if _, err = tx.ExecContext(ctx, "UPDATE cards SET bound_user = NULL, status = 'active', used_at = NULL, claimed_at = NULL WHERE bound_user = ?", username); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("release user cards: %w", err)
	}
	result, err := tx.ExecContext(ctx, "DELETE FROM users WHERE username = ?", username)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("delete user: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		_ = tx.Rollback()
		return ErrUserNotFound
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit delete user: %w", err)
	}
	return nil
}

// InitializeDefaultAdmin creates the documented admin/admin account if it is
// absent. Existing credentials are never overwritten.
func (r *SQLiteUserRepo) InitializeDefaultAdmin(ctx context.Context) error {
	if err := r.ensureDB(); err != nil {
		return err
	}
	stored, err := HashPassword(defaultAdminPassword)
	if err != nil {
		return err
	}
	pwdHash, salt, err := splitStoredPassword(stored)
	if err != nil {
		return err
	}
	_, err = r.DB.ExecContext(ctx, `
INSERT OR IGNORE INTO users
    (username, pwd_hash, salt, role, status, account_limit, card_json, created_at, updated_at)
VALUES (?, ?, ?, 'admin', 'active', ?, '{}', ?, ?)`,
		defaultAdminUsername, pwdHash, salt, defaultAccountLimit, time.Now().UnixMilli(), time.Now().UnixMilli(),
	)
	if err != nil {
		return fmt.Errorf("initialize default admin: %w", err)
	}
	return nil
}

// CheckRateLimit applies the legacy six attempts per IP per minute rule. The
// seventh attempt locks the IP for ten minutes, matching user-store.js.
func (r *SQLiteUserRepo) CheckRateLimit(ctx context.Context, ip string) (RateLimitResult, error) {
	return r.checkAttempt(ctx, "ip:"+strings.TrimSpace(ip), ip, "", maxIPLoginAttempts, ipRateLimitWindow, ipLockoutDuration)
}

// CheckAccountLockout reports whether a username is currently locked.
func (r *SQLiteUserRepo) CheckAccountLockout(ctx context.Context, username string) (RateLimitResult, error) {
	if err := r.ensureDB(); err != nil {
		return RateLimitResult{}, err
	}
	result, err := r.readAttempt(ctx, "user:"+strings.TrimSpace(username))
	if err != nil {
		return RateLimitResult{}, err
	}
	if result.LockedUntil > time.Now().UnixMilli() {
		result.Allowed = false
		result.Remaining = time.Until(time.UnixMilli(result.LockedUntil))
		return result, nil
	}
	if result.LockedUntil != 0 {
		_, err = r.DB.ExecContext(ctx, "DELETE FROM login_attempts WHERE subject = ?", "user:"+strings.TrimSpace(username))
		if err != nil {
			return RateLimitResult{}, fmt.Errorf("clear expired lock: %w", err)
		}
	}
	result.Allowed = true
	return result, nil
}

// RecordFailedAttempt records a failed username login and locks after five
// failures for fifteen minutes.
func (r *SQLiteUserRepo) RecordFailedAttempt(ctx context.Context, username string) (AttemptResult, error) {
	if err := r.ensureDB(); err != nil {
		return AttemptResult{}, err
	}
	return r.recordFailure(ctx, "user:"+strings.TrimSpace(username), "", username, maxUserLoginAttempts, userLockoutDuration)
}

// ClearFailedAttempts removes account and IP attempt state after a successful
// login.
func (r *SQLiteUserRepo) ClearFailedAttempts(ctx context.Context, username, ip string) error {
	if err := r.ensureDB(); err != nil {
		return err
	}
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin clear login attempts: %w", err)
	}
	for _, subject := range []string{"user:" + strings.TrimSpace(username), "ip:" + strings.TrimSpace(ip)} {
		if subject == "user:" || subject == "ip:" {
			continue
		}
		if _, err = tx.ExecContext(ctx, "DELETE FROM login_attempts WHERE subject = ?", subject); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("clear login attempts: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit clear login attempts: %w", err)
	}
	return nil
}

// Authenticate enforces IP and account lockout, verifies legacy/current
// credentials, upgrades SHA-256 credentials, and records a login audit row.
func (r *SQLiteUserRepo) Authenticate(ctx context.Context, username, password, ip string) (*User, error) {
	rate, err := r.CheckRateLimit(ctx, ip)
	if err != nil {
		return nil, err
	}
	if !rate.Allowed {
		_ = r.AddLoginLog(ctx, LoginLog{User: username, IP: ip, Result: "rate_limited", Event: "login", TS: time.Now().UnixMilli()})
		return nil, ErrRateLimited
	}

	user, err := r.Get(ctx, username)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return nil, r.invalidLogin(ctx, username, ip, ErrInvalidCredentials)
		}
		return nil, err
	}
	lock, err := r.CheckAccountLockout(ctx, username)
	if err != nil {
		return nil, err
	}
	if !lock.Allowed {
		_ = r.AddLoginLog(ctx, LoginLog{User: username, IP: ip, Result: "locked", Event: "login", TS: time.Now().UnixMilli()})
		return nil, ErrUserLocked
	}
	if user.Status != "" && user.Status != "active" {
		return nil, r.invalidLogin(ctx, username, ip, ErrInvalidCredentials)
	}
	if user.ExpireAt != nil && *user.ExpireAt > 0 && *user.ExpireAt <= time.Now().UnixMilli() {
		_ = r.AddLoginLog(ctx, LoginLog{User: username, IP: ip, Result: "expired", Event: "login", TS: time.Now().UnixMilli()})
		return nil, ErrUserExpired
	}
	if !VerifyPasswordParts(password, user.PwdHash, user.Salt) {
		return nil, r.invalidLogin(ctx, username, ip, ErrInvalidCredentials)
	}

	if user.Salt == "" {
		stored, hashErr := HashPassword(password)
		if hashErr == nil {
			if newHash, newSalt, splitErr := splitStoredPassword(stored); splitErr == nil {
				_, _ = r.DB.ExecContext(ctx, "UPDATE users SET pwd_hash = ?, salt = ?, updated_at = ? WHERE username = ?", newHash, newSalt, time.Now().UnixMilli(), username)
				user.PwdHash, user.Salt = newHash, newSalt
			}
		}
	}
	if err := r.ClearFailedAttempts(ctx, username, ip); err != nil {
		return nil, err
	}
	if err := r.AddLoginLog(ctx, LoginLog{User: username, IP: ip, Result: "success", Event: "login", TS: time.Now().UnixMilli()}); err != nil {
		return nil, err
	}
	return user, nil
}

func (r *SQLiteUserRepo) invalidLogin(ctx context.Context, username, ip string, base error) error {
	result, err := r.RecordFailedAttempt(ctx, username)
	if err != nil {
		return err
	}
	resultText := "invalid_credentials"
	if result.Locked {
		resultText = "locked"
	}
	_ = r.AddLoginLog(ctx, LoginLog{User: username, IP: ip, Result: resultText, Event: "login", TS: time.Now().UnixMilli()})
	if result.Locked {
		return ErrUserLocked
	}
	return base
}

// AddLoginLog writes one login audit record in a single INSERT.
func (r *SQLiteUserRepo) AddLoginLog(ctx context.Context, log LoginLog) error {
	if err := r.ensureDB(); err != nil {
		return err
	}
	if log.User == "" {
		log.User = log.Username
	}
	if log.Username == "" {
		log.Username = log.User
	}
	if log.UA == "" {
		log.UA = log.UserAgent
	}
	if log.UserAgent == "" {
		log.UserAgent = log.UA
	}
	if log.TS == 0 {
		log.TS = time.Now().UnixMilli()
	}
	if log.MetadataJSON == "" {
		log.MetadataJSON = "{}"
	}
	_, err := r.DB.ExecContext(ctx, `
INSERT INTO login_logs (user, username, ip, ua, user_agent, result, event, error_type, ts, metadata_json)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		log.User, nullableString(log.Username), log.IP, log.UA, nullableString(log.UserAgent), log.Result,
		nullableString(log.Event), nullableString(log.ErrorType), log.TS, log.MetadataJSON,
	)
	if err != nil {
		return fmt.Errorf("add login log: %w", err)
	}
	return nil
}

// RecordLoginLog is a descriptive alias for AddLoginLog.
func (r *SQLiteUserRepo) RecordLoginLog(ctx context.Context, log LoginLog) error {
	return r.AddLoginLog(ctx, log)
}

// GetLoginLogs returns newest-first logs and the unpaged total.
func (r *SQLiteUserRepo) GetLoginLogs(ctx context.Context, limit, offset int) ([]LoginLog, int, error) {
	if err := r.ensureDB(); err != nil {
		return nil, 0, err
	}
	if limit <= 0 {
		limit = defaultLoginLogLimit
	}
	if limit > maxLoginLogLimit {
		limit = maxLoginLogLimit
	}
	if offset < 0 {
		offset = 0
	}
	var total int
	if err := r.DB.QueryRowContext(ctx, "SELECT count(*) FROM login_logs").Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count login logs: %w", err)
	}
	rows, err := r.DB.QueryContext(ctx, `
SELECT id, user, username, ip, ua, user_agent, result, event, error_type, ts, metadata_json
FROM login_logs ORDER BY ts DESC, id DESC LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list login logs: %w", err)
	}
	defer rows.Close()
	logs := make([]LoginLog, 0)
	for rows.Next() {
		var log LoginLog
		var username, userAgent, event, errorType, metadata sql.NullString
		if err := rows.Scan(&log.ID, &log.User, &username, &log.IP, &log.UA, &userAgent, &log.Result, &event, &errorType, &log.TS, &metadata); err != nil {
			return nil, 0, fmt.Errorf("scan login log: %w", err)
		}
		log.Username, log.UserAgent, log.Event, log.ErrorType, log.MetadataJSON = username.String, userAgent.String, event.String, errorType.String, metadata.String
		logs = append(logs, log)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate login logs: %w", err)
	}
	return logs, total, nil
}

// ClearLoginLogs removes all login audit records.
func (r *SQLiteUserRepo) ClearLoginLogs(ctx context.Context) error {
	if err := r.ensureDB(); err != nil {
		return err
	}
	_, err := r.DB.ExecContext(ctx, "DELETE FROM login_logs")
	if err != nil {
		return fmt.Errorf("clear login logs: %w", err)
	}
	return nil
}

const userSelectSQL = `
SELECT username, pwd_hash, salt, role, status, expire_at, account_limit,
       card_code, card_json, must_change_password, created_at, updated_at
FROM users`

type rowScanner interface {
	Scan(...any) error
}

func scanUser(row rowScanner) (*User, error) {
	var user User
	var expireAt sql.NullInt64
	var cardCodeText sql.NullString
	var mustChange int
	if err := row.Scan(
		&user.Username, &user.PwdHash, &user.Salt, &user.Role, &user.Status,
		&expireAt, &user.AccountLimit, &cardCodeText, &user.CardJSON, &mustChange,
		&user.CreatedAt, &user.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if expireAt.Valid {
		user.ExpireAt = &expireAt.Int64
	}
	if cardCodeText.Valid {
		user.CardCode = cardCodeText.String
	}
	user.MustChangePassword = mustChange != 0
	user.Password = storedCredential(user.PwdHash, user.Salt)
	return &user, nil
}

func credentialParts(user User) (string, string, error) {
	if user.Password != "" {
		return splitStoredPasswordOrLegacy(user.Password)
	}
	if strings.Contains(user.PwdHash, ":") && user.Salt == "" {
		return splitStoredPasswordOrLegacy(user.PwdHash)
	}
	if user.PwdHash == "" {
		return "", "", ErrInvalidPassword
	}
	return user.PwdHash, user.Salt, nil
}

func splitStoredPassword(stored string) (string, string, error) {
	parts := strings.SplitN(stored, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", ErrInvalidPassword
	}
	return parts[1], parts[0], nil
}

func splitStoredPasswordOrLegacy(stored string) (string, string, error) {
	if strings.Contains(stored, ":") {
		return splitStoredPassword(stored)
	}
	if stored == "" {
		return "", "", ErrInvalidPassword
	}
	return stored, "", nil
}

func storedCredential(pwdHash, salt string) string {
	if salt == "" {
		return pwdHash
	}
	return salt + ":" + pwdHash
}

func mergeUser(current, update User) User {
	if update.PwdHash != "" || update.Password != "" {
		current.PwdHash, current.Salt, _ = credentialParts(update)
		current.Password = storedCredential(current.PwdHash, current.Salt)
	}
	if update.Role != "" {
		current.Role = update.Role
	}
	if update.Status != "" {
		current.Status = update.Status
	}
	if update.ExpireAt != nil {
		current.ExpireAt = update.ExpireAt
	}
	if update.AccountLimit > 0 {
		current.AccountLimit = update.AccountLimit
	}
	if update.CardCode != "" {
		current.CardCode = update.CardCode
	}
	if update.CardJSON != "" {
		current.CardJSON = update.CardJSON
	}
	if update.MustChangePassword {
		current.MustChangePassword = true
	}
	if update.CreatedAt != 0 {
		current.CreatedAt = update.CreatedAt
	}
	if update.UpdatedAt != 0 {
		current.UpdatedAt = update.UpdatedAt
	}
	return current
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func nullableInt64(value *int64) any {
	if value == nil {
		return nil
	}
	return *value
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func (r *SQLiteUserRepo) readAttempt(ctx context.Context, subject string) (RateLimitResult, error) {
	var result RateLimitResult
	var locked sql.NullInt64
	err := r.DB.QueryRowContext(ctx, "SELECT count, locked_until FROM login_attempts WHERE subject = ?", subject).Scan(&result.Count, &locked)
	if errors.Is(err, sql.ErrNoRows) {
		return result, nil
	}
	if err != nil {
		return RateLimitResult{}, fmt.Errorf("read login attempt: %w", err)
	}
	if locked.Valid {
		result.LockedUntil = locked.Int64
	}
	return result, nil
}

func (r *SQLiteUserRepo) checkAttempt(ctx context.Context, subject, ip, username string, limit int, window, lockout time.Duration) (RateLimitResult, error) {
	if err := r.ensureDB(); err != nil {
		return RateLimitResult{}, err
	}
	now := time.Now().UnixMilli()
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return RateLimitResult{}, fmt.Errorf("begin rate limit: %w", err)
	}
	var result RateLimitResult
	var count, windowStart int64
	var lockedUntil sql.NullInt64
	err = tx.QueryRowContext(ctx, "SELECT count, window_start, locked_until FROM login_attempts WHERE subject = ?", subject).Scan(&count, &windowStart, &lockedUntil)
	if errors.Is(err, sql.ErrNoRows) {
		_, err = tx.ExecContext(ctx, `INSERT INTO login_attempts (ip, username, subject, count, window_start) VALUES (?, ?, ?, 1, ?)`, nullableString(ip), nullableString(username), subject, now)
		if err == nil {
			err = tx.Commit()
		}
		if err != nil {
			_ = tx.Rollback()
			return RateLimitResult{}, fmt.Errorf("record initial rate limit attempt: %w", err)
		}
		return RateLimitResult{Allowed: true, Count: 1}, nil
	}
	if err != nil {
		_ = tx.Rollback()
		return RateLimitResult{}, fmt.Errorf("read rate limit attempt: %w", err)
	}
	if lockedUntil.Valid && lockedUntil.Int64 > now {
		result = RateLimitResult{Allowed: false, Count: int(count), LockedUntil: lockedUntil.Int64, Remaining: time.Until(time.UnixMilli(lockedUntil.Int64))}
		_ = tx.Rollback()
		return result, nil
	}
	if lockedUntil.Valid || now-windowStart > window.Milliseconds() {
		if _, err = tx.ExecContext(ctx, `UPDATE login_attempts SET ip = ?, username = ?, count = 1, window_start = ?, first_attempt = ?, last_attempt = ?, locked_until = NULL WHERE subject = ?`, nullableString(ip), nullableString(username), now, now, now, subject); err != nil {
			_ = tx.Rollback()
			return RateLimitResult{}, fmt.Errorf("reset rate limit attempt: %w", err)
		}
		if err = tx.Commit(); err != nil {
			return RateLimitResult{}, fmt.Errorf("commit rate limit reset: %w", err)
		}
		return RateLimitResult{Allowed: true, Count: 1}, nil
	}
	if count >= int64(limit) {
		locked := now + lockout.Milliseconds()
		if _, err = tx.ExecContext(ctx, "UPDATE login_attempts SET locked_until = ?, last_attempt = ? WHERE subject = ?", locked, now, subject); err != nil {
			_ = tx.Rollback()
			return RateLimitResult{}, fmt.Errorf("lock rate limit subject: %w", err)
		}
		if err = tx.Commit(); err != nil {
			return RateLimitResult{}, fmt.Errorf("commit rate limit lock: %w", err)
		}
		return RateLimitResult{Allowed: false, Count: int(count), LockedUntil: locked, Remaining: lockout}, nil
	}
	count++
	if _, err = tx.ExecContext(ctx, "UPDATE login_attempts SET count = ?, last_attempt = ? WHERE subject = ?", count, now, subject); err != nil {
		_ = tx.Rollback()
		return RateLimitResult{}, fmt.Errorf("increment rate limit attempt: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return RateLimitResult{}, fmt.Errorf("commit rate limit attempt: %w", err)
	}
	return RateLimitResult{Allowed: true, Count: int(count)}, nil
}

func (r *SQLiteUserRepo) recordFailure(ctx context.Context, subject, ip, username string, limit int, lockout time.Duration) (AttemptResult, error) {
	if err := r.ensureDB(); err != nil {
		return AttemptResult{}, err
	}
	now := time.Now().UnixMilli()
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return AttemptResult{}, fmt.Errorf("begin failed-login record: %w", err)
	}
	var count int64
	var firstAttempt sql.NullInt64
	var lockedUntil sql.NullInt64
	err = tx.QueryRowContext(ctx, "SELECT count, first_attempt, locked_until FROM login_attempts WHERE subject = ?", subject).Scan(&count, &firstAttempt, &lockedUntil)
	if errors.Is(err, sql.ErrNoRows) {
		count = 1
		_, err = tx.ExecContext(ctx, `INSERT INTO login_attempts (ip, username, subject, count, first_attempt, last_attempt) VALUES (?, ?, ?, 1, ?, ?)`, nullableString(ip), nullableString(username), subject, now, now)
	} else if err == nil {
		count++
		if count >= int64(limit) {
			locked := now + lockout.Milliseconds()
			lockedUntil = sql.NullInt64{Int64: locked, Valid: true}
			_, err = tx.ExecContext(ctx, "UPDATE login_attempts SET count = ?, last_attempt = ?, locked_until = ? WHERE subject = ?", count, now, locked, subject)
		} else {
			_, err = tx.ExecContext(ctx, "UPDATE login_attempts SET count = ?, last_attempt = ? WHERE subject = ?", count, now, subject)
		}
	}
	if err != nil {
		_ = tx.Rollback()
		return AttemptResult{}, fmt.Errorf("record failed login: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return AttemptResult{}, fmt.Errorf("commit failed-login record: %w", err)
	}
	result := AttemptResult{Count: int(count), RemainingAttempts: maxInt(0, limit-int(count))}
	if lockedUntil.Valid {
		result.Locked = lockedUntil.Int64 > now
		result.LockedUntil = lockedUntil.Int64
	}
	return result, nil
}

func maxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}

func isConstraintError(err error) bool {
	return strings.Contains(strings.ToLower(err.Error()), "constraint") || strings.Contains(strings.ToLower(err.Error()), "unique")
}
