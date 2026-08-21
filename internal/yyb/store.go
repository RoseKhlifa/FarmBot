package yyb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	secretcrypto "github.com/RoseKhlifa/FarmBot/internal/crypto"
	"github.com/RoseKhlifa/FarmBot/internal/store"
	"github.com/RoseKhlifa/FarmBot/internal/yyb/model"
)

var defaultFeatures = []Feature{
	{Code: 1001, Name: "getCode", Description: stringPtr("wx.login code"), Enabled: true},
	{Code: 1002, Name: "getPhoneNumber", Description: stringPtr("取手机号"), Enabled: true},
	{Code: 1003, Name: "operateWxData", Description: stringPtr("通用云函数代理"), Enabled: true},
}

type DB struct {
	*store.DB
	secretBox         *secretcrypto.SecretBox
	requireEncryption bool
}

// Aliases keep the original public yyb names stable while allowing protocol
// to depend on the independent model package.
type WechatAccount = model.WechatAccount
type AccountPublic = model.AccountPublic
type SessionRow = model.SessionRow

type Feature struct {
	Code        int     `json:"code"`
	Name        string  `json:"name"`
	Description *string `json:"description"`
	Enabled     bool    `json:"enabled"`
}

// NewDB creates a yyb storage facade over the already-open FarmBot database.
// The caller owns the underlying connection; closing this facade is therefore
// intentionally a no-op.
func NewDB(mainDB *store.DB) (*DB, error) {
	return newDB(mainDB, true)
}

func newDB(mainDB *store.DB, autoConfigureEnv bool) (*DB, error) {
	if mainDB == nil || mainDB.DB == nil {
		return nil, errors.New("main store database is required")
	}
	db := &DB{DB: mainDB}
	// Keep legacy callers working when no deployment secret is configured,
	// while automatically enabling encryption whenever FARM_MASTER_KEY exists.
	// NewDBFromEnv remains the strict fail-closed constructor.
	if autoConfigureEnv && strings.TrimSpace(os.Getenv(secretcrypto.MasterKeyEnv)) != "" {
		box, err := secretcrypto.NewSecretBoxFromEnv()
		if err != nil {
			return nil, err
		}
		db.secretBox = box
		db.requireEncryption = true
	}
	if err := db.EnsureDefaultFeatures(context.Background()); err != nil {
		return nil, fmt.Errorf("seed yyb features: %w", err)
	}
	return db, nil
}

// NewDBWithSecretBox enables mandatory encryption for credential and session
// blobs. Production composition roots should use this constructor.
func NewDBWithSecretBox(mainDB *store.DB, box *secretcrypto.SecretBox) (*DB, error) {
	if box == nil {
		return nil, secretcrypto.ErrMasterKeyMissing
	}
	db, err := newDB(mainDB, false)
	if err != nil {
		return nil, err
	}
	db.secretBox = box
	db.requireEncryption = true
	return db, nil
}

// NewDBFromEnv is the fail-closed constructor for deployments using the
// FARM_MASTER_KEY environment secret.
func NewDBFromEnv(mainDB *store.DB) (*DB, error) {
	box, err := secretcrypto.NewSecretBoxFromEnv()
	if err != nil {
		return nil, err
	}
	return NewDBWithSecretBox(mainDB, box)
}

func (db *DB) SetSecretBox(box *secretcrypto.SecretBox) error {
	if db == nil {
		return errors.New("yyb database is nil")
	}
	if box == nil {
		return secretcrypto.ErrMasterKeyMissing
	}
	db.secretBox = box
	db.requireEncryption = true
	return nil
}

func (db *DB) EncryptionRequired() bool { return db != nil && db.requireEncryption }

// Open is retained as the composition-root entry point, but now accepts the
// main store handle rather than a filesystem path. No independent yyb SQLite
// database is created.
func Open(mainDB *store.DB) (*DB, error) { return NewDB(mainDB) }

// Close does not close the shared main database. The store package owns it.
func (db *DB) Close() error { return nil }

func (db *DB) EnsureDefaultFeatures(ctx context.Context) error {
	for _, f := range defaultFeatures {
		desc := nullableString(f.Description)
		if _, err := db.DB.ExecContext(ctx,
			"INSERT OR IGNORE INTO features(code, name, description, enabled) VALUES(?,?,?,1)",
			f.Code, f.Name, desc,
		); err != nil {
			return err
		}
	}
	return nil
}

func (db *DB) UpsertAccount(ctx context.Context, openid, loginBuffer string, alias, nickname, avatar *string, userInfo map[string]any, credentials map[string]any, status *string) (*WechatAccount, error) {
	now := time.Now().Unix()
	userJSON, err := marshalNullable(userInfo)
	if err != nil {
		return nil, err
	}
	credJSON, err := marshalNullable(credentials)
	if err != nil {
		return nil, err
	}
	loginBuffer, err = db.sealSecret(loginBuffer)
	if err != nil {
		return nil, err
	}
	credJSON, err = db.sealNullable(credJSON)
	if err != nil {
		return nil, err
	}
	_, err = db.DB.ExecContext(ctx,
		`INSERT INTO wechat_accounts
		(openid, login_buffer, alias, nickname, avatar, user_info, credentials, status, created_at, updated_at)
		VALUES(?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(openid) DO UPDATE SET
		login_buffer=excluded.login_buffer, alias=excluded.alias, nickname=excluded.nickname,
		avatar=excluded.avatar, user_info=excluded.user_info, credentials=excluded.credentials,
		status=excluded.status, updated_at=excluded.updated_at`,
		openid, loginBuffer, nullableString(alias), nullableString(nickname), nullableString(avatar),
		userJSON, credJSON, nullableString(status), now, now,
	)
	if err != nil {
		return nil, err
	}
	return db.GetAccountByOpenID(ctx, openid)
}

func (db *DB) GetAccount(ctx context.Context, id int64) (*WechatAccount, error) {
	account, legacy, err := db.scanAccount(db.DB.QueryRowContext(ctx, selectAccountSQL+" WHERE id=?", id))
	if err != nil {
		return nil, err
	}
	if legacy {
		if err := db.upgradeAccountSecrets(ctx, account); err != nil {
			return nil, err
		}
	}
	return account, nil
}

func (db *DB) GetAccountByOpenID(ctx context.Context, openid string) (*WechatAccount, error) {
	account, legacy, err := db.scanAccount(db.DB.QueryRowContext(ctx, selectAccountSQL+" WHERE openid=?", openid))
	if err != nil {
		return nil, err
	}
	if legacy {
		if err := db.upgradeAccountSecrets(ctx, account); err != nil {
			return nil, err
		}
	}
	return account, nil
}

func (db *DB) GetAccountByUIN(ctx context.Context, uin int64) (*WechatAccount, error) {
	account, legacy, err := db.scanAccount(db.DB.QueryRowContext(ctx, selectAccountSQL+" WHERE uin=?", uin))
	if err != nil {
		return nil, err
	}
	if legacy {
		if err := db.upgradeAccountSecrets(ctx, account); err != nil {
			return nil, err
		}
	}
	return account, nil
}

func (db *DB) ResolveAccount(ctx context.Context, ref string) (*WechatAccount, error) {
	if ref == "" {
		return nil, sql.ErrNoRows
	}
	if isDigits(ref) {
		n, _ := strconv.ParseInt(ref, 10, 64)
		if acc, err := db.GetAccountByUIN(ctx, n); err == nil {
			return acc, nil
		} else if !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		return db.GetAccount(ctx, n)
	}
	return db.GetAccountByOpenID(ctx, ref)
}

func (db *DB) ListAccounts(ctx context.Context) ([]*WechatAccount, error) {
	rows, err := db.DB.QueryContext(ctx, selectAccountSQL+" ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []*WechatAccount
	legacyAccounts := make([]*WechatAccount, 0)
	for rows.Next() {
		acc, legacy, err := db.scanAccount(rows)
		if err != nil {
			return nil, err
		}
		if legacy {
			legacyAccounts = append(legacyAccounts, acc)
		}
		out = append(out, acc)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	for _, acc := range legacyAccounts {
		if err := db.upgradeAccountSecrets(ctx, acc); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (db *DB) SetAccountUIN(ctx context.Context, id, uin int64) error {
	_, err := db.DB.ExecContext(ctx, "UPDATE wechat_accounts SET uin=?, updated_at=? WHERE id=?", uin, time.Now().Unix(), id)
	return err
}

func (db *DB) SetAccountProfile(ctx context.Context, id int64, nickname, avatar *string, userInfo map[string]any) error {
	userJSON, err := marshalNullable(userInfo)
	if err != nil {
		return err
	}
	_, err = db.DB.ExecContext(ctx,
		"UPDATE wechat_accounts SET nickname=?, avatar=?, user_info=?, updated_at=? WHERE id=?",
		nullableString(nickname), nullableString(avatar), userJSON, time.Now().Unix(), id,
	)
	return err
}

func (db *DB) SetAccountCredential(ctx context.Context, id int64, loginBuffer string, credentials map[string]any) error {
	credJSON, err := marshalNullable(credentials)
	if err != nil {
		return err
	}
	loginBuffer, err = db.sealSecret(loginBuffer)
	if err != nil {
		return err
	}
	credJSON, err = db.sealNullable(credJSON)
	if err != nil {
		return err
	}
	_, err = db.DB.ExecContext(ctx,
		"UPDATE wechat_accounts SET login_buffer=?, credentials=?, updated_at=? WHERE id=?",
		loginBuffer, credJSON, time.Now().Unix(), id,
	)
	return err
}

func (db *DB) SetAccountStatus(ctx context.Context, id int64, status string) error {
	now := time.Now().Unix()
	_, err := db.DB.ExecContext(ctx,
		"UPDATE wechat_accounts SET status=?, last_checked_at=?, updated_at=? WHERE id=?",
		status, now, now, id,
	)
	return err
}

func (db *DB) DeleteAccount(ctx context.Context, id int64) error {
	tx, err := db.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, "DELETE FROM sessions WHERE wechat_account_id=?", id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		"UPDATE accounts SET yyb_openid=NULL WHERE yyb_openid=(SELECT openid FROM wechat_accounts WHERE id=?)", id,
	); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM wechat_accounts WHERE id=?", id); err != nil {
		return err
	}
	return tx.Commit()
}

func (db *DB) GetSession(ctx context.Context, accountID int64, tcpProxy string) (*SessionRow, error) {
	row := db.DB.QueryRowContext(ctx,
		"SELECT id, wechat_account_id, uin, tcp_proxy, session_blob, expires_at, created_at, updated_at FROM sessions WHERE wechat_account_id=? AND tcp_proxy=? AND expires_at>?",
		accountID, tcpProxy, time.Now().Unix(),
	)
	session, legacy, err := db.scanSession(row)
	if err != nil {
		return nil, err
	}
	if legacy {
		if err := db.upgradeSession(ctx, session); err != nil {
			return nil, err
		}
	}
	return session, nil
}

func (db *DB) PutSession(ctx context.Context, accountID int64, uin *int64, sessionBlob map[string]any, expiresAt int64, tcpProxy string) error {
	now := time.Now().Unix()
	blob, err := json.Marshal(sessionBlob)
	if err != nil {
		return err
	}
	protected, err := db.sealSecret(string(blob))
	if err != nil {
		return err
	}
	_, err = db.DB.ExecContext(ctx,
		`INSERT INTO sessions
		(wechat_account_id, uin, tcp_proxy, session_blob, expires_at, created_at, updated_at)
		VALUES(?,?,?,?,?,?,?)
		ON CONFLICT(wechat_account_id, tcp_proxy) DO UPDATE SET
		uin=excluded.uin, session_blob=excluded.session_blob,
		expires_at=excluded.expires_at, updated_at=excluded.updated_at`,
		accountID, nullableInt(uin), tcpProxy, protected, expiresAt, now, now,
	)
	return err
}

func (db *DB) InvalidateSession(ctx context.Context, accountID int64, tcpProxy string) error {
	_, err := db.DB.ExecContext(ctx, "DELETE FROM sessions WHERE wechat_account_id=? AND tcp_proxy=?", accountID, tcpProxy)
	return err
}

func (db *DB) PurgeExpiredSessions(ctx context.Context) (int64, error) {
	res, err := db.DB.ExecContext(ctx, "DELETE FROM sessions WHERE expires_at<=?", time.Now().Unix())
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (db *DB) ListFeatures(ctx context.Context, onlyEnabled bool) ([]Feature, error) {
	sqlText := "SELECT code, name, description, enabled FROM features"
	if onlyEnabled {
		sqlText += " WHERE enabled=1"
	}
	sqlText += " ORDER BY code"
	rows, err := db.DB.QueryContext(ctx, sqlText)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []Feature
	for rows.Next() {
		f, err := scanFeature(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func (db *DB) ResolveFeature(ctx context.Context, ref any) (*Feature, error) {
	switch v := ref.(type) {
	case float64:
		return db.GetFeature(ctx, int(v))
	case int:
		return db.GetFeature(ctx, v)
	case string:
		if isDigits(v) {
			n, _ := strconv.Atoi(v)
			return db.GetFeature(ctx, n)
		}
		return db.GetFeatureByName(ctx, v)
	default:
		return nil, sql.ErrNoRows
	}
}

func (db *DB) GetFeature(ctx context.Context, code int) (*Feature, error) {
	row := db.DB.QueryRowContext(ctx, "SELECT code, name, description, enabled FROM features WHERE code=?", code)
	f, err := scanFeature(row)
	if err != nil {
		return nil, err
	}
	return &f, nil
}

func (db *DB) GetFeatureByName(ctx context.Context, name string) (*Feature, error) {
	row := db.DB.QueryRowContext(ctx, "SELECT code, name, description, enabled FROM features WHERE name=? COLLATE NOCASE", name)
	f, err := scanFeature(row)
	if err != nil {
		return nil, err
	}
	return &f, nil
}

const selectAccountSQL = `SELECT id, openid, uin, alias, nickname, avatar, user_info, login_buffer, credentials, status, last_checked_at, created_at, updated_at FROM wechat_accounts`

type accountScanner interface {
	Scan(dest ...any) error
}

type featureScanner interface {
	Scan(dest ...any) error
}

func (db *DB) scanAccount(row accountScanner) (*WechatAccount, bool, error) {
	return scanAccountRowsWithSecrets(row, db)
}

func scanAccountRowsWithSecrets(row accountScanner, db *DB) (*WechatAccount, bool, error) {
	var (
		a                       WechatAccount
		uin, lastChecked        sql.NullInt64
		alias, nickname, avatar sql.NullString
		userJSON, credJSON      sql.NullString
		status                  sql.NullString
		loginBuffer             string
	)
	err := row.Scan(
		&a.ID, &a.OpenID, &uin, &alias, &nickname, &avatar, &userJSON,
		&loginBuffer, &credJSON, &status, &lastChecked, &a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		return nil, false, err
	}
	decoded, legacy, err := db.decodeSecret(loginBuffer)
	if err != nil {
		return nil, false, err
	}
	a.LoginBuffer = decoded
	if uin.Valid {
		a.UIN = &uin.Int64
	}
	a.Alias = stringPtrFromNull(alias)
	a.Nickname = stringPtrFromNull(nickname)
	a.Avatar = stringPtrFromNull(avatar)
	a.Status = stringPtrFromNull(status)
	if lastChecked.Valid {
		a.LastCheckedAt = &lastChecked.Int64
	}
	if userJSON.Valid && userJSON.String != "" {
		_ = json.Unmarshal([]byte(userJSON.String), &a.UserInfo)
	}
	if credJSON.Valid && credJSON.String != "" {
		decoded, credLegacy, err := db.decodeSecret(credJSON.String)
		if err != nil {
			return nil, false, err
		}
		legacy = legacy || credLegacy
		if err := json.Unmarshal([]byte(decoded), &a.Credentials); err != nil {
			return nil, false, fmt.Errorf("decode credentials: %w", err)
		}
	}
	return &a, legacy, nil
}

func (db *DB) scanSession(row accountScanner) (*SessionRow, bool, error) {
	var s SessionRow
	var uin sql.NullInt64
	var blob string
	if err := row.Scan(&s.ID, &s.WechatAccountID, &uin, &s.TCPProxy, &blob, &s.ExpiresAt, &s.CreatedAt, &s.UpdatedAt); err != nil {
		return nil, false, err
	}
	decoded, legacy, err := db.decodeSecret(blob)
	if err != nil {
		return nil, false, err
	}
	if uin.Valid {
		s.UIN = &uin.Int64
	}
	if err := json.Unmarshal([]byte(decoded), &s.SessionBlob); err != nil {
		return nil, false, fmt.Errorf("decode session_blob: %w", err)
	}
	return &s, legacy, nil
}

func (db *DB) decodeSecret(value string) (string, bool, error) {
	if strings.HasPrefix(value, secretcrypto.CiphertextPrefix) {
		if db == nil || db.secretBox == nil {
			return "", false, secretcrypto.ErrMasterKeyMissing
		}
		plain, err := db.secretBox.DecryptString(value)
		return plain, false, err
	}
	if db != nil && db.requireEncryption {
		if db.secretBox == nil {
			return "", false, secretcrypto.ErrMasterKeyMissing
		}
		return value, true, nil
	}
	return value, false, nil
}

func (db *DB) sealSecret(value string) (string, error) {
	if strings.HasPrefix(value, secretcrypto.CiphertextPrefix) {
		if db == nil || db.secretBox == nil {
			return "", secretcrypto.ErrMasterKeyMissing
		}
		if _, err := db.secretBox.DecryptString(value); err != nil {
			return "", err
		}
		return value, nil
	}
	if db == nil || db.secretBox == nil {
		if db != nil && db.requireEncryption {
			return "", secretcrypto.ErrMasterKeyMissing
		}
		return value, nil
	}
	return db.secretBox.EncryptString(value)
}

func (db *DB) sealNullable(value sql.NullString) (sql.NullString, error) {
	if !value.Valid {
		return value, nil
	}
	sealed, err := db.sealSecret(value.String)
	if err != nil {
		return sql.NullString{}, err
	}
	return sql.NullString{String: sealed, Valid: true}, nil
}

func (db *DB) upgradeAccountSecrets(ctx context.Context, account *WechatAccount) error {
	if db == nil || db.secretBox == nil || account == nil {
		return nil
	}
	return db.SetAccountCredential(ctx, account.ID, account.LoginBuffer, account.Credentials)
}

func (db *DB) upgradeSession(ctx context.Context, session *SessionRow) error {
	if db == nil || db.secretBox == nil || session == nil {
		return nil
	}
	return db.PutSession(ctx, session.WechatAccountID, session.UIN, session.SessionBlob, session.ExpiresAt, session.TCPProxy)
}

func scanFeature(row featureScanner) (Feature, error) {
	var f Feature
	var desc sql.NullString
	var enabled int
	if err := row.Scan(&f.Code, &f.Name, &desc, &enabled); err != nil {
		return Feature{}, err
	}
	f.Description = stringPtrFromNull(desc)
	f.Enabled = enabled != 0
	return f, nil
}

func marshalNullable(v map[string]any) (sql.NullString, error) {
	if v == nil {
		return sql.NullString{}, nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return sql.NullString{}, err
	}
	return sql.NullString{String: string(b), Valid: true}, nil
}

func nullableString(s *string) sql.NullString {
	if s == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *s, Valid: true}
}

func nullableInt(v *int64) sql.NullInt64 {
	if v == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *v, Valid: true}
}

func stringPtrFromNull(v sql.NullString) *string {
	if !v.Valid {
		return nil
	}
	return &v.String
}

func stringPtr(s string) *string { return &s }

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
