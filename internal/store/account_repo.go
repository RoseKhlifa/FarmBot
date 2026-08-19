package store

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Account is the persistence representation of one FarmBot account.
type Account struct {
	ID             string
	Name           string
	Code           string
	Platform       string
	LoginType      string
	Provider       string
	WXID           string
	UIN            string
	QQ             string
	GID            string
	OpenID         string
	Avatar         string
	OwnerUser      string
	YYBOpenID      string
	Remark         string
	ThirdPartyJSON json.RawMessage
	CreatedAt      int64
	UpdatedAt      int64
}

// AccountConfig is a full per-account configuration snapshot. Evolving
// nested values remain JSON so repositories persist them without owning their
// business validation rules.
type AccountConfig struct {
	AccountID                          string
	AutomationJSON                     json.RawMessage
	AutoCodeRefreshJSON                json.RawMessage
	PlantingStrategy                   string
	PreferredSeedID                    int64
	Prioritize2x2Crops                 bool
	FriendBadRetryDate                 string
	IntervalsJSON                      json.RawMessage
	FriendQuietHoursJSON               json.RawMessage
	KnownFriendGIDsJSON                json.RawMessage
	FriendBlacklistJSON                json.RawMessage
	PlantBlacklistJSON                 json.RawMessage
	StealDelaySeconds                  float64
	PlantOrderRandom                   bool
	PlantDelaySeconds                  float64
	FertilizerBuyOrganicCount          int64
	FertilizerBuyOrganicThresholdHours int64
	FertilizerBuyNormalCount           int64
	FertilizerBuyNormalThresholdHours  int64
	FertilizerBuyCheckIntervalMinutes  int64
	MysteryAutoBuyCurrenciesJSON       json.RawMessage
	BagSeedPriorityJSON                json.RawMessage
	BagSeedFallbackStrategy            string
	BagPriorityLandTypesJSON           json.RawMessage
	AutoAcceptFriendMinLevel           int64
	GoldenBugKeepCount                 int64
	GoldenBugRoundLimit                int64
	FriendHelpExpExhausted             bool
	ConfigJSON                         json.RawMessage
	UpdatedAt                          int64
}

// AccountRepo isolates account and per-account configuration persistence.
// Every account-scoped method requires an explicit account ID.
type AccountRepo interface {
	List(ctx context.Context) ([]Account, error)
	Get(ctx context.Context, accountID string) (*Account, error)
	Upsert(ctx context.Context, account Account) error
	Delete(ctx context.Context, accountID string) error
	GetByUser(ctx context.Context, username string) ([]Account, error)
	GetConfig(ctx context.Context, accountID string) (*AccountConfig, error)
	ApplyConfigSnapshot(ctx context.Context, accountID string, config AccountConfig) error
}

// SQLiteAccountRepo stores accounts in the schema created by P1-02.
type SQLiteAccountRepo struct {
	DB *DB
}

// AccountRepository is a descriptive alias for callers that prefer the
// concrete repository name.
type AccountRepository = SQLiteAccountRepo

// NewAccountRepo creates a SQLite-backed AccountRepo implementation.
func NewAccountRepo(db *DB) *SQLiteAccountRepo {
	return &SQLiteAccountRepo{DB: db}
}

// NewAccountRepository is an alias for NewAccountRepo.
func NewAccountRepository(db *DB) *SQLiteAccountRepo { return NewAccountRepo(db) }

func (r *SQLiteAccountRepo) List(ctx context.Context) ([]Account, error) {
	if err := r.checkDB(); err != nil {
		return nil, err
	}
	rows, err := r.DB.QueryContext(ctx, accountSelectSQL+" ORDER BY created_at, id")
	if err != nil {
		return nil, fmt.Errorf("list accounts: %w", err)
	}
	defer rows.Close()

	accounts := make([]Account, 0)
	for rows.Next() {
		account, err := scanAccount(rows)
		if err != nil {
			return nil, fmt.Errorf("scan account: %w", err)
		}
		accounts = append(accounts, account)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list accounts: %w", err)
	}
	return accounts, nil
}

func (r *SQLiteAccountRepo) Get(ctx context.Context, accountID string) (*Account, error) {
	accountID, err := accountRequiredText("accountID", accountID)
	if err != nil {
		return nil, err
	}
	if err := r.checkDB(); err != nil {
		return nil, err
	}
	account, err := scanAccount(r.DB.QueryRowContext(ctx, accountSelectSQL+" WHERE id = ?", accountID))
	if err != nil {
		return nil, fmt.Errorf("get account %q: %w", accountID, err)
	}
	return &account, nil
}

func (r *SQLiteAccountRepo) Upsert(ctx context.Context, account Account) error {
	accountID, err := accountRequiredText("account ID", account.ID)
	if err != nil {
		return err
	}
	if err := r.checkDB(); err != nil {
		return err
	}
	thirdParty, err := accountJSONText(account.ThirdPartyJSON, "{}")
	if err != nil {
		return fmt.Errorf("account %q third-party config: %w", accountID, err)
	}

	now := time.Now().UnixMilli()
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin account upsert: %w", err)
	}
	defer tx.Rollback()

	createdAt := account.CreatedAt
	if createdAt == 0 {
		err := tx.QueryRowContext(ctx, "SELECT created_at FROM accounts WHERE id = ?", accountID).Scan(&createdAt)
		if err != nil && err != sql.ErrNoRows {
			return fmt.Errorf("read account creation time: %w", err)
		}
		if createdAt == 0 {
			createdAt = now
		}
	}
	updatedAt := account.UpdatedAt
	if updatedAt == 0 {
		updatedAt = now
	}
	platform := account.Platform
	if platform == "" {
		platform = "qq"
	}
	loginType := account.LoginType
	if loginType == "" {
		loginType = "manual"
	}
	provider := account.Provider
	if provider == "" {
		provider = "builtin"
	}

	_, err = tx.ExecContext(ctx, `
INSERT INTO accounts (
    id, name, code, platform, login_type, provider, wxid, uin, qq, gid,
    open_id, avatar, owner_user, yyb_openid, remark, thirdparty_json,
    created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
    name = excluded.name,
    code = excluded.code,
    platform = excluded.platform,
    login_type = excluded.login_type,
    provider = excluded.provider,
    wxid = excluded.wxid,
    uin = excluded.uin,
    qq = excluded.qq,
    gid = excluded.gid,
    open_id = excluded.open_id,
    avatar = excluded.avatar,
    owner_user = excluded.owner_user,
    yyb_openid = excluded.yyb_openid,
    remark = excluded.remark,
    thirdparty_json = excluded.thirdparty_json,
    created_at = excluded.created_at,
    updated_at = excluded.updated_at`,
		accountID, account.Name, account.Code, platform, loginType, provider,
		account.WXID, account.UIN, account.QQ, account.GID, account.OpenID,
		account.Avatar, accountNullableText(account.OwnerUser), accountNullableText(account.YYBOpenID),
		accountNullableText(account.Remark), thirdParty, createdAt, updatedAt,
	)
	if err != nil {
		return fmt.Errorf("upsert account %q: %w", accountID, err)
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO account_config(account_id, updated_at) VALUES (?, ?)
ON CONFLICT(account_id) DO NOTHING`, accountID, updatedAt); err != nil {
		return fmt.Errorf("ensure account config %q: %w", accountID, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit account upsert: %w", err)
	}
	return nil
}

func (r *SQLiteAccountRepo) Delete(ctx context.Context, accountID string) error {
	accountID, err := accountRequiredText("accountID", accountID)
	if err != nil {
		return err
	}
	if err := r.checkDB(); err != nil {
		return err
	}
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin account delete: %w", err)
	}
	defer tx.Rollback()

	// Delete dependants explicitly as well as relying on foreign keys. This
	// keeps cleanup correct for databases opened by older clients without the
	// SQLite foreign_keys pragma enabled.
	for _, table := range []string{
		"friend_gid_cache", "friend_dog_info", "friend_list_cache",
		"blacklist", "stats", "account_config",
	} {
		if _, err := tx.ExecContext(ctx, "DELETE FROM "+table+" WHERE account_id = ?", accountID); err != nil {
			return fmt.Errorf("delete %s for account %q: %w", table, accountID, err)
		}
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM accounts WHERE id = ?", accountID); err != nil {
		return fmt.Errorf("delete account %q: %w", accountID, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit account delete: %w", err)
	}
	return nil
}

func (r *SQLiteAccountRepo) GetByUser(ctx context.Context, username string) ([]Account, error) {
	username, err := accountRequiredText("username", username)
	if err != nil {
		return nil, err
	}
	if err := r.checkDB(); err != nil {
		return nil, err
	}
	rows, err := r.DB.QueryContext(ctx, accountSelectSQL+" WHERE owner_user = ? ORDER BY created_at, id", username)
	if err != nil {
		return nil, fmt.Errorf("list accounts for user %q: %w", username, err)
	}
	defer rows.Close()

	accounts := make([]Account, 0)
	for rows.Next() {
		account, err := scanAccount(rows)
		if err != nil {
			return nil, fmt.Errorf("scan account for user %q: %w", username, err)
		}
		accounts = append(accounts, account)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list accounts for user %q: %w", username, err)
	}
	return accounts, nil
}

func (r *SQLiteAccountRepo) GetConfig(ctx context.Context, accountID string) (*AccountConfig, error) {
	accountID, err := accountRequiredText("accountID", accountID)
	if err != nil {
		return nil, err
	}
	if err := r.checkDB(); err != nil {
		return nil, err
	}
	config, err := scanAccountConfig(r.DB.QueryRowContext(ctx, accountConfigSelectSQL+" WHERE account_id = ?", accountID))
	if err != nil {
		return nil, fmt.Errorf("get config for account %q: %w", accountID, err)
	}
	return &config, nil
}

func (r *SQLiteAccountRepo) ApplyConfigSnapshot(ctx context.Context, accountID string, config AccountConfig) error {
	accountID, err := accountRequiredText("accountID", accountID)
	if err != nil {
		return err
	}
	if err := r.checkDB(); err != nil {
		return err
	}
	jsonValues := make([]string, 0, 11)
	for _, field := range []struct {
		name     string
		value    json.RawMessage
		fallback string
	}{
		{"automation", config.AutomationJSON, "{}"},
		{"auto code refresh", config.AutoCodeRefreshJSON, "{}"},
		{"intervals", config.IntervalsJSON, "{}"},
		{"friend quiet hours", config.FriendQuietHoursJSON, "{}"},
		{"known friend GIDs", config.KnownFriendGIDsJSON, "[]"},
		{"friend blacklist", config.FriendBlacklistJSON, "[]"},
		{"plant blacklist", config.PlantBlacklistJSON, "[]"},
		{"mystery currencies", config.MysteryAutoBuyCurrenciesJSON, "[]"},
		{"bag seed priority", config.BagSeedPriorityJSON, "[]"},
		{"bag priority land types", config.BagPriorityLandTypesJSON, "[]"},
		{"config", config.ConfigJSON, "{}"},
	} {
		value, err := accountJSONText(field.value, field.fallback)
		if err != nil {
			return fmt.Errorf("account %q %s: %w", accountID, field.name, err)
		}
		jsonValues = append(jsonValues, value)
	}

	plantingStrategy := config.PlantingStrategy
	if plantingStrategy == "" {
		plantingStrategy = "max_exp"
	}
	bagFallback := config.BagSeedFallbackStrategy
	if bagFallback == "" {
		bagFallback = "level"
	}
	updatedAt := config.UpdatedAt
	if updatedAt == 0 {
		updatedAt = time.Now().UnixMilli()
	}

	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin config write: %w", err)
	}
	defer tx.Rollback()
	var exists int
	if err := tx.QueryRowContext(ctx, "SELECT 1 FROM accounts WHERE id = ?", accountID).Scan(&exists); err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("account %q: %w", accountID, sql.ErrNoRows)
		}
		return fmt.Errorf("check account %q: %w", accountID, err)
	}

	_, err = tx.ExecContext(ctx, `
INSERT INTO account_config (
    account_id, automation_json, auto_code_refresh_json, planting_strategy,
    preferred_seed_id, prioritize_2x2_crops, friend_bad_retry_date,
    intervals_json, friend_quiet_hours_json, known_friend_gids_json,
    friend_blacklist_json, plant_blacklist_json, steal_delay_seconds,
    plant_order_random, plant_delay_seconds, fertilizer_buy_organic_count,
    fertilizer_buy_organic_threshold_hours, fertilizer_buy_normal_count,
    fertilizer_buy_normal_threshold_hours, fertilizer_buy_check_interval_minutes,
    mystery_auto_buy_currencies_json, bag_seed_priority_json,
    bag_seed_fallback_strategy, bag_priority_land_types_json,
    auto_accept_friend_min_level, golden_bug_keep_count, golden_bug_round_limit,
    friend_help_exp_exhausted, config_json, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(account_id) DO UPDATE SET
    automation_json = excluded.automation_json,
    auto_code_refresh_json = excluded.auto_code_refresh_json,
    planting_strategy = excluded.planting_strategy,
    preferred_seed_id = excluded.preferred_seed_id,
    prioritize_2x2_crops = excluded.prioritize_2x2_crops,
    friend_bad_retry_date = excluded.friend_bad_retry_date,
    intervals_json = excluded.intervals_json,
    friend_quiet_hours_json = excluded.friend_quiet_hours_json,
    known_friend_gids_json = excluded.known_friend_gids_json,
    friend_blacklist_json = excluded.friend_blacklist_json,
    plant_blacklist_json = excluded.plant_blacklist_json,
    steal_delay_seconds = excluded.steal_delay_seconds,
    plant_order_random = excluded.plant_order_random,
    plant_delay_seconds = excluded.plant_delay_seconds,
    fertilizer_buy_organic_count = excluded.fertilizer_buy_organic_count,
    fertilizer_buy_organic_threshold_hours = excluded.fertilizer_buy_organic_threshold_hours,
    fertilizer_buy_normal_count = excluded.fertilizer_buy_normal_count,
    fertilizer_buy_normal_threshold_hours = excluded.fertilizer_buy_normal_threshold_hours,
    fertilizer_buy_check_interval_minutes = excluded.fertilizer_buy_check_interval_minutes,
    mystery_auto_buy_currencies_json = excluded.mystery_auto_buy_currencies_json,
    bag_seed_priority_json = excluded.bag_seed_priority_json,
    bag_seed_fallback_strategy = excluded.bag_seed_fallback_strategy,
    bag_priority_land_types_json = excluded.bag_priority_land_types_json,
    auto_accept_friend_min_level = excluded.auto_accept_friend_min_level,
    golden_bug_keep_count = excluded.golden_bug_keep_count,
    golden_bug_round_limit = excluded.golden_bug_round_limit,
    friend_help_exp_exhausted = excluded.friend_help_exp_exhausted,
    config_json = excluded.config_json,
    updated_at = excluded.updated_at`,
		accountID, jsonValues[0], jsonValues[1], plantingStrategy,
		config.PreferredSeedID, accountBoolInt(config.Prioritize2x2Crops), config.FriendBadRetryDate,
		jsonValues[2], jsonValues[3], jsonValues[4], jsonValues[5], jsonValues[6],
		config.StealDelaySeconds, accountBoolInt(config.PlantOrderRandom), config.PlantDelaySeconds,
		config.FertilizerBuyOrganicCount, config.FertilizerBuyOrganicThresholdHours,
		config.FertilizerBuyNormalCount, config.FertilizerBuyNormalThresholdHours,
		config.FertilizerBuyCheckIntervalMinutes, jsonValues[7], jsonValues[8], bagFallback,
		jsonValues[9], config.AutoAcceptFriendMinLevel, config.GoldenBugKeepCount,
		config.GoldenBugRoundLimit, accountBoolInt(config.FriendHelpExpExhausted), jsonValues[10], updatedAt,
	)
	if err != nil {
		return fmt.Errorf("apply config for account %q: %w", accountID, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit config for account %q: %w", accountID, err)
	}
	return nil
}

func (r *SQLiteAccountRepo) checkDB() error {
	if r == nil || r.DB == nil || r.DB.DB == nil {
		return fmt.Errorf("account repository database is nil")
	}
	return nil
}

const accountSelectSQL = `SELECT
    id, name, code, platform, login_type, provider, wxid, uin, qq, gid,
    open_id, avatar, owner_user, yyb_openid, remark, thirdparty_json,
    created_at, updated_at
FROM accounts`

const accountConfigSelectSQL = `SELECT
    account_id, automation_json, auto_code_refresh_json, planting_strategy,
    preferred_seed_id, prioritize_2x2_crops, friend_bad_retry_date,
    intervals_json, friend_quiet_hours_json, known_friend_gids_json,
    friend_blacklist_json, plant_blacklist_json, steal_delay_seconds,
    plant_order_random, plant_delay_seconds, fertilizer_buy_organic_count,
    fertilizer_buy_organic_threshold_hours, fertilizer_buy_normal_count,
    fertilizer_buy_normal_threshold_hours, fertilizer_buy_check_interval_minutes,
    mystery_auto_buy_currencies_json, bag_seed_priority_json,
    bag_seed_fallback_strategy, bag_priority_land_types_json,
    auto_accept_friend_min_level, golden_bug_keep_count, golden_bug_round_limit,
    friend_help_exp_exhausted, config_json, updated_at
FROM account_config`

type accountScanner interface {
	Scan(dest ...any) error
}

func scanAccount(scanner accountScanner) (Account, error) {
	var account Account
	var ownerUser, yybOpenID, remark sql.NullString
	var thirdParty string
	err := scanner.Scan(
		&account.ID, &account.Name, &account.Code, &account.Platform,
		&account.LoginType, &account.Provider, &account.WXID, &account.UIN,
		&account.QQ, &account.GID, &account.OpenID, &account.Avatar,
		&ownerUser, &yybOpenID, &remark, &thirdParty,
		&account.CreatedAt, &account.UpdatedAt,
	)
	if err != nil {
		return Account{}, err
	}
	account.OwnerUser = ownerUser.String
	account.YYBOpenID = yybOpenID.String
	account.Remark = remark.String
	account.ThirdPartyJSON = json.RawMessage(thirdParty)
	return account, nil
}

func scanAccountConfig(scanner accountScanner) (AccountConfig, error) {
	var config AccountConfig
	var automation, autoCodeRefresh, intervals, quietHours string
	var knownGIDs, friendBlacklist, plantBlacklist string
	var mysteryCurrencies, bagSeedPriority, bagLandTypes, configJSON string
	var prioritize2x2, plantOrderRandom, friendHelpExpExhausted int
	err := scanner.Scan(
		&config.AccountID, &automation, &autoCodeRefresh, &config.PlantingStrategy,
		&config.PreferredSeedID, &prioritize2x2, &config.FriendBadRetryDate,
		&intervals, &quietHours, &knownGIDs, &friendBlacklist, &plantBlacklist,
		&config.StealDelaySeconds, &plantOrderRandom, &config.PlantDelaySeconds,
		&config.FertilizerBuyOrganicCount, &config.FertilizerBuyOrganicThresholdHours,
		&config.FertilizerBuyNormalCount, &config.FertilizerBuyNormalThresholdHours,
		&config.FertilizerBuyCheckIntervalMinutes, &mysteryCurrencies, &bagSeedPriority,
		&config.BagSeedFallbackStrategy, &bagLandTypes, &config.AutoAcceptFriendMinLevel,
		&config.GoldenBugKeepCount, &config.GoldenBugRoundLimit, &friendHelpExpExhausted,
		&configJSON, &config.UpdatedAt,
	)
	if err != nil {
		return AccountConfig{}, err
	}
	config.AutomationJSON = json.RawMessage(automation)
	config.AutoCodeRefreshJSON = json.RawMessage(autoCodeRefresh)
	config.IntervalsJSON = json.RawMessage(intervals)
	config.FriendQuietHoursJSON = json.RawMessage(quietHours)
	config.KnownFriendGIDsJSON = json.RawMessage(knownGIDs)
	config.FriendBlacklistJSON = json.RawMessage(friendBlacklist)
	config.PlantBlacklistJSON = json.RawMessage(plantBlacklist)
	config.MysteryAutoBuyCurrenciesJSON = json.RawMessage(mysteryCurrencies)
	config.BagSeedPriorityJSON = json.RawMessage(bagSeedPriority)
	config.BagPriorityLandTypesJSON = json.RawMessage(bagLandTypes)
	config.ConfigJSON = json.RawMessage(configJSON)
	config.Prioritize2x2Crops = prioritize2x2 != 0
	config.PlantOrderRandom = plantOrderRandom != 0
	config.FriendHelpExpExhausted = friendHelpExpExhausted != 0
	return config, nil
}

func accountRequiredText(name, value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("%s must not be empty", name)
	}
	return value, nil
}

func accountJSONText(value json.RawMessage, fallback string) (string, error) {
	value = bytes.TrimSpace(value)
	if len(value) == 0 {
		return fallback, nil
	}
	if !json.Valid(value) {
		return "", fmt.Errorf("invalid JSON")
	}
	return string(value), nil
}

func accountNullableText(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func accountBoolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

var _ AccountRepo = (*SQLiteAccountRepo)(nil)
