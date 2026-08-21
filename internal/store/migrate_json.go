package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ImportConflict controls what happens when a source key already exists in
// SQLite. The safe default is to keep the destination value unchanged.
type ImportConflict string

const (
	ConflictSkip      ImportConflict = "skip"
	ConflictOverwrite ImportConflict = "overwrite"
)

// JSONImportOptions controls one migration run. Overwrite is kept as a
// convenience for callers that do not want to construct an enum value.
type JSONImportOptions struct {
	Conflict  ImportConflict
	Overwrite bool
}

// JSONImportReport is both a machine-readable result and the source for the
// CLI's human-readable count summary. Counts are rows written, while Skipped
// contains source rows intentionally left unchanged due to a conflict.
type JSONImportReport struct {
	SourceDir string         `json:"sourceDir"`
	Counts    map[string]int `json:"counts"`
	Skipped   map[string]int `json:"skipped"`
	Files     map[string]int `json:"files"`
}

// JSONImporter migrates the legacy JSON data layout into the P1 SQLite
// schema. Repository dependencies are fields so tests can replace them with
// fakes; the default constructor wires the production SQLite implementations.
type JSONImporter struct {
	DB       *DB
	Accounts AccountRepo
	Caches   CacheRepo
	Config   ConfigRepo
	Stats    StatsRepo
	Users    UserRepo
	Cards    CardRepo

	// freshAccounts tracks rows created or explicitly overwritten during the
	// current run. AccountRepo creates a default account_config alongside a
	// new account, and that generated row is not a source conflict.
	freshAccounts map[string]bool
}

// NewJSONImporter creates an importer backed by the supplied SQLite database.
func NewJSONImporter(db *DB) *JSONImporter {
	return &JSONImporter{
		DB: db, Accounts: NewAccountRepo(db), Caches: NewCacheRepo(db),
		Config: NewConfigRepo(db), Stats: NewStatsRepo(db),
		Users: NewUserRepo(db), Cards: NewCardRepo(db),
	}
}

// ImportJSON is the package-level convenience entry point used by callers
// that do not need to customize repository implementations.
func ImportJSON(ctx context.Context, db *DB, sourceDir string, options JSONImportOptions) (JSONImportReport, error) {
	return NewJSONImporter(db).Import(ctx, sourceDir, options)
}

// MigrateJSON is a descriptive alias for ImportJSON.
func MigrateJSON(ctx context.Context, db *DB, sourceDir string, options JSONImportOptions) (JSONImportReport, error) {
	return ImportJSON(ctx, db, sourceDir, options)
}

// Import reads all supported legacy files below sourceDir. Missing optional
// files are normal; malformed files and rows with no stable key fail the run
// so an operator cannot mistake a partial migration for a complete one.
func (m *JSONImporter) Import(ctx context.Context, sourceDir string, options JSONImportOptions) (JSONImportReport, error) {
	report := JSONImportReport{SourceDir: sourceDir, Counts: map[string]int{}, Skipped: map[string]int{}, Files: map[string]int{}}
	if m == nil || m.DB == nil || m.DB.DB == nil {
		return report, errors.New("json importer has no database")
	}
	sourceDir = filepath.Clean(strings.TrimSpace(sourceDir))
	if sourceDir == "" || sourceDir == "." {
		return report, errors.New("json import source directory must not be empty")
	}
	info, err := os.Stat(sourceDir)
	if err != nil {
		return report, fmt.Errorf("stat JSON source directory: %w", err)
	}
	if !info.IsDir() {
		return report, fmt.Errorf("JSON source %q is not a directory", sourceDir)
	}
	report.SourceDir = sourceDir
	options = normalizeImportOptions(options)
	m.freshAccounts = make(map[string]bool)
	steps := []func(context.Context, string, JSONImportOptions, *JSONImportReport) error{
		m.importUsers, m.importAccounts, m.importStore, m.importCards,
		m.importLoginAttempts, m.importLoginLogs, m.importStats, m.importCaches,
	}
	for _, step := range steps {
		if err := step(ctx, sourceDir, options, &report); err != nil {
			return report, err
		}
	}
	return report, nil
}

func normalizeImportOptions(options JSONImportOptions) JSONImportOptions {
	if options.Overwrite {
		options.Conflict = ConflictOverwrite
	}
	if options.Conflict != ConflictOverwrite {
		options.Conflict = ConflictSkip
	}
	return options
}

func (r *JSONImportReport) wrote(table string)   { r.Counts[table]++ }
func (r *JSONImportReport) skipped(table string) { r.Skipped[table]++ }
func (r *JSONImportReport) file(name string)     { r.Files[name]++ }

func (m *JSONImporter) importAccounts(ctx context.Context, dir string, options JSONImportOptions, report *JSONImportReport) error {
	raw, exists, err := readOptionalJSON(filepath.Join(dir, "accounts.json"))
	if err != nil || !exists {
		return err
	}
	entries, err := rawArrayField(raw, "accounts")
	if err != nil {
		return fmt.Errorf("accounts.json: %w", err)
	}
	report.file("accounts.json")
	for index, entry := range entries {
		account, err := decodeLegacyAccount(entry)
		if err != nil {
			return fmt.Errorf("accounts.json entry %d: %w", index, err)
		}
		existing, getErr := m.Accounts.Get(ctx, account.ID)
		if getErr == nil && existing != nil && options.Conflict == ConflictSkip {
			report.skipped("accounts")
			continue
		}
		if getErr != nil && !isMissing(getErr) {
			return fmt.Errorf("check account %q: %w", account.ID, getErr)
		}
		if err := m.Accounts.Upsert(ctx, account); err != nil {
			return fmt.Errorf("import account %q: %w", account.ID, err)
		}
		m.freshAccounts[account.ID] = true
		report.wrote("accounts")
	}
	return nil
}

func (m *JSONImporter) importStore(ctx context.Context, dir string, options JSONImportOptions, report *JSONImportReport) error {
	raw, exists, err := readOptionalJSON(filepath.Join(dir, "store.json"))
	if err != nil || !exists {
		return err
	}
	root, err := rawObject(raw)
	if err != nil {
		return fmt.Errorf("store.json: %w", err)
	}
	report.file("store.json")
	if err := m.setGlobal(ctx, "legacy:store", raw, options, report); err != nil {
		return err
	}
	for _, key := range []string{"systemConfig", "globalWxConfig", "antiResaleConfig", "offlineReminder", "loginLinks", "announcement", "superAdminAnnouncement", "captureConfig", "deviceProtocol", "userDeviceProtocols"} {
		value, ok := root[key]
		if !ok || !json.Valid(value) || string(value) == "null" {
			continue
		}
		if err := m.setGlobal(ctx, legacyConfigKey(key), value, options, report); err != nil {
			return err
		}
	}
	for key, target := range map[string]string{"adminPasswordHash": "admin_password_hash", "userDefaultAccountPlans": "legacy:user_default_account_plans", "defaultAccountConfig": "legacy:default_account_config"} {
		if value, ok := root[key]; ok && json.Valid(value) {
			if err := m.setGlobal(ctx, target, value, options, report); err != nil {
				return err
			}
		}
	}
	if ui, ok := root["ui"]; ok {
		if err := m.setGlobal(ctx, "legacy:ui", ui, options, report); err != nil {
			return err
		}
		if uiObject, objectErr := rawObject(ui); objectErr == nil {
			if theme, present := uiObject["theme"]; present {
				if err := m.setGlobal(ctx, ConfigKeyTheme, theme, options, report); err != nil {
					return err
				}
			}
		}
	}
	if reminders, ok := root["userOfflineReminders"]; ok {
		reminderObject, objectErr := rawObject(reminders)
		if objectErr != nil {
			return fmt.Errorf("store.json userOfflineReminders: %w", objectErr)
		}
		for _, username := range sortedRawKeys(reminderObject) {
			if err := m.setGlobal(ctx, ConfigKeyOfflineReminder+":"+username, reminderObject[username], options, report); err != nil {
				return err
			}
		}
	}
	configs, ok := root["accountConfigs"]
	if !ok {
		return nil
	}
	configObject, err := rawObject(configs)
	if err != nil {
		return fmt.Errorf("store.json accountConfigs: %w", err)
	}
	for _, accountID := range sortedRawKeys(configObject) {
		accountConfig, decodeErr := decodeLegacyAccountConfig(accountID, configObject[accountID])
		if decodeErr != nil {
			return fmt.Errorf("store.json accountConfigs[%q]: %w", accountID, decodeErr)
		}
		if options.Conflict == ConflictSkip && !m.freshAccounts[accountID] {
			if _, getErr := m.Accounts.GetConfig(ctx, accountID); getErr == nil {
				report.skipped("account_config")
				continue
			} else if !isMissing(getErr) {
				return fmt.Errorf("check account config %q: %w", accountID, getErr)
			}
		}
		if err := m.Accounts.ApplyConfigSnapshot(ctx, accountID, accountConfig); err != nil {
			return fmt.Errorf("import account config %q: %w", accountID, err)
		}
		report.wrote("account_config")
		if err := m.importBlacklist(ctx, accountID, accountConfig.FriendBlacklistJSON, options, report); err != nil {
			return err
		}
	}
	return nil
}

func (m *JSONImporter) setGlobal(ctx context.Context, key string, value json.RawMessage, options JSONImportOptions, report *JSONImportReport) error {
	if !json.Valid(value) {
		return fmt.Errorf("global config %q contains invalid JSON", key)
	}
	if options.Conflict == ConflictSkip {
		if _, err := m.Config.Get(ctx, key); err == nil {
			report.skipped("global_config")
			return nil
		} else if !isMissing(err) {
			return fmt.Errorf("check global config %q: %w", key, err)
		}
	}
	if err := m.Config.Set(ctx, ConfigEntry{Key: key, Value: append(json.RawMessage(nil), value...)}); err != nil {
		return fmt.Errorf("import global config %q: %w", key, err)
	}
	report.wrote("global_config")
	return nil
}

func (m *JSONImporter) importUsers(ctx context.Context, dir string, options JSONImportOptions, report *JSONImportReport) error {
	raw, exists, err := readOptionalJSON(filepath.Join(dir, "users.json"))
	if err != nil || !exists {
		return err
	}
	entries, err := rawArrayField(raw, "users")
	if err != nil {
		return fmt.Errorf("users.json: %w", err)
	}
	report.file("users.json")
	for index, entry := range entries {
		user, err := decodeLegacyUser(entry)
		if err != nil {
			return fmt.Errorf("users.json entry %d: %w", index, err)
		}
		existing, getErr := m.Users.Get(ctx, user.Username)
		if getErr == nil && existing != nil {
			if options.Conflict == ConflictSkip {
				report.skipped("users")
				continue
			}
			if err := m.Users.Update(ctx, user); err != nil {
				return fmt.Errorf("overwrite user %q: %w", user.Username, err)
			}
			report.wrote("users")
			continue
		}
		if getErr != nil && !isMissing(getErr) {
			return fmt.Errorf("check user %q: %w", user.Username, getErr)
		}
		if err := m.Users.Create(ctx, user); err != nil {
			return fmt.Errorf("import user %q: %w", user.Username, err)
		}
		report.wrote("users")
	}
	return nil
}

type legacyCardRow struct {
	code, description, cardType, status, durationUnit, boundUser, metadata string
	enabled, permanent                                                     bool
	days, value, durationValue                                             float64
	durationMS, usedAt, claimedAt, createdAt, updatedAt                    int64
}

func (m *JSONImporter) importCards(ctx context.Context, dir string, options JSONImportOptions, report *JSONImportReport) error {
	raw, exists, err := readOptionalJSON(filepath.Join(dir, "cards.json"))
	if err != nil || !exists {
		return err
	}
	entries, err := rawArrayField(raw, "cards")
	if err != nil {
		return fmt.Errorf("cards.json: %w", err)
	}
	report.file("cards.json")
	for index, entry := range entries {
		card, err := decodeLegacyCard(entry)
		if err != nil {
			return fmt.Errorf("cards.json entry %d: %w", index, err)
		}
		existing, getErr := m.Cards.Get(ctx, card.code)
		if getErr == nil && existing != nil && options.Conflict == ConflictSkip {
			report.skipped("cards")
			continue
		}
		if getErr != nil && !isMissing(getErr) {
			return fmt.Errorf("check card %q: %w", card.code, getErr)
		}
		if err := m.writeCard(ctx, card, options); err != nil {
			return fmt.Errorf("import card %q: %w", card.code, err)
		}
		report.wrote("cards")
	}
	return nil
}

func (m *JSONImporter) writeCard(ctx context.Context, card legacyCardRow, options JSONImportOptions) error {
	args := []any{card.code, card.description, card.cardType, card.status, boolToInt(card.enabled), nullableImportFloat(card.days), nullableImportFloat(card.value), nullableImportFloat(card.durationValue), card.durationUnit, nullableImportInt(card.durationMS), boolToInt(card.permanent), nullableImportString(card.boundUser), nullableImportInt(card.usedAt), nullableImportInt(card.claimedAt), card.createdAt, card.updatedAt, card.metadata}
	query := `INSERT INTO cards (code, description, type, status, enabled, days, value, duration_value, duration_unit, duration_ms, is_permanent, bound_user, used_at, claimed_at, created_at, updated_at, metadata_json) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	if options.Conflict == ConflictOverwrite {
		query += ` ON CONFLICT(code) DO UPDATE SET description = excluded.description, type = excluded.type, status = excluded.status, enabled = excluded.enabled, days = excluded.days, value = excluded.value, duration_value = excluded.duration_value, duration_unit = excluded.duration_unit, duration_ms = excluded.duration_ms, is_permanent = excluded.is_permanent, bound_user = excluded.bound_user, used_at = excluded.used_at, claimed_at = excluded.claimed_at, created_at = excluded.created_at, updated_at = excluded.updated_at, metadata_json = excluded.metadata_json`
	}
	_, err := m.DB.ExecContext(ctx, query, args...)
	return err
}

func (m *JSONImporter) importLoginAttempts(ctx context.Context, dir string, options JSONImportOptions, report *JSONImportReport) error {
	raw, exists, err := readOptionalJSON(filepath.Join(dir, "login-attempts.json"))
	if err != nil || !exists {
		return err
	}
	entries, err := rawObject(raw)
	if err != nil {
		return fmt.Errorf("login-attempts.json: %w", err)
	}
	if nested, ok := entries["attempts"]; ok {
		if nestedObject, nestedErr := rawObject(nested); nestedErr == nil {
			entries = nestedObject
		} else {
			return fmt.Errorf("login-attempts.json attempts: %w", nestedErr)
		}
	}
	report.file("login-attempts.json")
	for _, subject := range sortedRawKeys(entries) {
		entry, entryErr := rawObject(entries[subject])
		if entryErr != nil {
			return fmt.Errorf("login-attempts.json[%q]: %w", subject, entryErr)
		}
		canonicalSubject := rawStringOr(entry, subject, "subject")
		ip, username := rawString(entry, "ip"), rawString(entry, "username", "user")
		if strings.HasPrefix(canonicalSubject, "ip:") && ip == "" {
			ip = strings.TrimPrefix(canonicalSubject, "ip:")
		}
		if strings.HasPrefix(canonicalSubject, "user:") && username == "" {
			username = strings.TrimPrefix(canonicalSubject, "user:")
		}
		if canonicalSubject == "" {
			return fmt.Errorf("login-attempts.json[%q]: subject is empty", subject)
		}
		if options.Conflict == ConflictSkip {
			var found int
			checkErr := m.DB.QueryRowContext(ctx, "SELECT 1 FROM login_attempts WHERE subject = ?", canonicalSubject).Scan(&found)
			if checkErr == nil {
				report.skipped("login_attempts")
				continue
			}
			if !isMissing(checkErr) {
				return fmt.Errorf("check login attempt %q: %w", canonicalSubject, checkErr)
			}
		}
		count := rawInt64(entry, "count")
		windowStart := rawInt64(entry, "windowStart", "window_start")
		firstAttempt := rawInt64(entry, "firstAttempt", "first_attempt")
		lastAttempt := rawInt64(entry, "lastAttempt", "last_attempt")
		lockedUntil := rawInt64(entry, "lockedUntil", "locked_until")
		query := `INSERT INTO login_attempts (ip, username, subject, count, window_start, first_attempt, last_attempt, locked_until)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
		if options.Conflict == ConflictOverwrite {
			query += ` ON CONFLICT(subject) DO UPDATE SET ip = excluded.ip, username = excluded.username, count = excluded.count,
window_start = excluded.window_start, first_attempt = excluded.first_attempt, last_attempt = excluded.last_attempt, locked_until = excluded.locked_until`
		}
		if _, execErr := m.DB.ExecContext(ctx, query, nullableImportString(ip), nullableImportString(username), canonicalSubject, count,
			nullableImportInt(windowStart), nullableImportInt(firstAttempt), nullableImportInt(lastAttempt), nullableImportInt(lockedUntil)); execErr != nil {
			return fmt.Errorf("import login attempt %q: %w", canonicalSubject, execErr)
		}
		report.wrote("login_attempts")
	}
	return nil
}

func (m *JSONImporter) importLoginLogs(ctx context.Context, dir string, options JSONImportOptions, report *JSONImportReport) error {
	raw, exists, err := readOptionalJSON(filepath.Join(dir, "login-logs.json"))
	if err != nil || !exists {
		return err
	}
	entries, err := rawArrayField(raw, "logs")
	if err != nil {
		return fmt.Errorf("login-logs.json: %w", err)
	}
	report.file("login-logs.json")
	for index, entry := range entries {
		object, objectErr := rawObject(entry)
		if objectErr != nil {
			return fmt.Errorf("login-logs.json entry %d: %w", index, objectErr)
		}
		logID := rawInt64(object, "id")
		user := rawString(object, "user", "username")
		username := rawString(object, "username", "user")
		ip := rawString(object, "ip")
		ua := rawString(object, "ua")
		userAgent := rawString(object, "userAgent", "user_agent")
		result := rawString(object, "result")
		event := rawString(object, "event")
		errorType := rawString(object, "errorType", "error_type")
		ts := rawInt64(object, "ts", "timestamp")
		metadata := append(json.RawMessage(nil), entry...)
		if logID > 0 && options.Conflict == ConflictSkip {
			var found int
			checkErr := m.DB.QueryRowContext(ctx, "SELECT 1 FROM login_logs WHERE id = ?", logID).Scan(&found)
			if checkErr == nil {
				report.skipped("login_logs")
				continue
			}
			if !isMissing(checkErr) {
				return fmt.Errorf("check login log %d: %w", logID, checkErr)
			}
		}
		if logID > 0 {
			query := `INSERT INTO login_logs (id, user, username, ip, ua, user_agent, result, event, error_type, ts, metadata_json)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
			if options.Conflict == ConflictOverwrite {
				query += ` ON CONFLICT(id) DO UPDATE SET user = excluded.user, username = excluded.username, ip = excluded.ip,
ua = excluded.ua, user_agent = excluded.user_agent, result = excluded.result, event = excluded.event,
error_type = excluded.error_type, ts = excluded.ts, metadata_json = excluded.metadata_json`
			}
			_, err = m.DB.ExecContext(ctx, query, logID, user, nullableImportString(username), ip, ua,
				nullableImportString(userAgent), result, nullableImportString(event), nullableImportString(errorType), ts, metadata)
		} else {
			_, err = m.DB.ExecContext(ctx, `INSERT INTO login_logs (user, username, ip, ua, user_agent, result, event, error_type, ts, metadata_json)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, user, nullableImportString(username), ip, ua,
				nullableImportString(userAgent), result, nullableImportString(event), nullableImportString(errorType), ts, metadata)
		}
		if err != nil {
			return fmt.Errorf("import login log entry %d: %w", index, err)
		}
		report.wrote("login_logs")
	}
	return nil
}

func (m *JSONImporter) importStats(ctx context.Context, dir string, options JSONImportOptions, report *JSONImportReport) error {
	statsDir := filepath.Join(dir, "stats")
	entries, err := os.ReadDir(statsDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read stats directory: %w", err)
	}
	for _, file := range entries {
		if file.IsDir() || !strings.HasSuffix(strings.ToLower(file.Name()), ".json") {
			continue
		}
		accountID := strings.TrimSuffix(file.Name(), filepath.Ext(file.Name()))
		raw, readErr := os.ReadFile(filepath.Join(statsDir, file.Name()))
		if readErr != nil {
			return fmt.Errorf("read stats/%s: %w", file.Name(), readErr)
		}
		if !json.Valid(raw) {
			return fmt.Errorf("stats/%s: invalid JSON", file.Name())
		}
		object, objectErr := rawObject(raw)
		if objectErr != nil {
			return fmt.Errorf("stats/%s: %w", file.Name(), objectErr)
		}
		if value := rawString(object, "accountId", "account_id"); value != "" {
			accountID = value
		}
		date := rawString(object, "date")
		if date == "" {
			date = time.UnixMilli(rawInt64(object, "updatedAt", "updated_at")).UTC().Format("2006-01-02")
		}
		if date == "1970-01-01" {
			date = "legacy"
		}
		operations := map[string]json.RawMessage{}
		if operationsRaw, ok := object["operations"]; ok {
			operations, objectErr = rawObject(operationsRaw)
			if objectErr != nil {
				return fmt.Errorf("stats/%s operations: %w", file.Name(), objectErr)
			}
		}
		for metric, valueRaw := range operations {
			value, valueErr := rawFloat64Value(valueRaw)
			if valueErr != nil {
				return fmt.Errorf("stats/%s operations[%q]: %w", file.Name(), metric, valueErr)
			}
			if err := m.importStat(ctx, accountID, metric, date, value, rawInt64(object, "updatedAt", "updated_at"), options, report); err != nil {
				return err
			}
		}
		for metric, valueRaw := range object {
			if metric == "date" || metric == "operations" || metric == "accountId" || metric == "account_id" || metric == "updatedAt" || metric == "updated_at" {
				continue
			}
			value, valueErr := rawFloat64Value(valueRaw)
			if valueErr != nil {
				continue
			}
			if err := m.importStat(ctx, accountID, metric, date, value, rawInt64(object, "updatedAt", "updated_at"), options, report); err != nil {
				return err
			}
		}
		report.file(filepath.ToSlash(filepath.Join("stats", file.Name())))
	}
	return nil
}

func (m *JSONImporter) importStat(ctx context.Context, accountID, metric, date string, value float64, updatedAt int64, options JSONImportOptions, report *JSONImportReport) error {
	if strings.TrimSpace(accountID) == "" || strings.TrimSpace(metric) == "" {
		return fmt.Errorf("stats entry has empty account or metric")
	}
	if options.Conflict == ConflictSkip {
		if _, err := m.Stats.Get(ctx, accountID, metric, date); err == nil {
			report.skipped("stats")
			return nil
		} else if !isMissing(err) {
			return fmt.Errorf("check stat %q/%q: %w", accountID, metric, err)
		}
	}
	if err := m.Stats.Set(ctx, Stat{AccountID: accountID, Metric: metric, Date: date, Value: value, UpdatedAt: updatedAt}); err != nil {
		return fmt.Errorf("import stat %q/%q: %w", accountID, metric, err)
	}
	report.wrote("stats")
	return nil
}

func (m *JSONImporter) importCaches(ctx context.Context, dir string, options JSONImportOptions, report *JSONImportReport) error {
	if err := m.importCacheKind(ctx, dir, "known_friend_gids", "gids", options, report); err != nil {
		return err
	}
	if err := m.importCacheKind(ctx, dir, "friend_dog_info", "dogInfo", options, report); err != nil {
		return err
	}
	return m.importCacheKind(ctx, dir, "friend_list_cache", "friends", options, report)
}

func (m *JSONImporter) importCacheKind(ctx context.Context, dir, directory, field string, options JSONImportOptions, report *JSONImportReport) error {
	cacheDir := filepath.Join(dir, directory)
	entries, err := os.ReadDir(cacheDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read %s directory: %w", directory, err)
	}
	for _, file := range entries {
		if file.IsDir() || !strings.HasSuffix(strings.ToLower(file.Name()), ".json") {
			continue
		}
		accountID := strings.TrimSuffix(file.Name(), filepath.Ext(file.Name()))
		raw, readErr := os.ReadFile(filepath.Join(cacheDir, file.Name()))
		if readErr != nil {
			return fmt.Errorf("read %s/%s: %w", directory, file.Name(), readErr)
		}
		if !json.Valid(raw) {
			return fmt.Errorf("%s/%s contains invalid JSON", directory, file.Name())
		}
		object, objectErr := rawObject(raw)
		if objectErr != nil {
			return fmt.Errorf("%s/%s: %w", directory, file.Name(), objectErr)
		}
		payload, ok := object[field]
		if !ok {
			payload = json.RawMessage(`[]`)
			if directory == "friend_dog_info" {
				payload = json.RawMessage(`{}`)
			}
		}
		updatedAt := rawInt64(object, "updatedAt", "updated_at")
		if options.Conflict == ConflictSkip {
			var getErr error
			switch directory {
			case "known_friend_gids":
				_, getErr = m.Caches.GetKnownFriendGIDs(ctx, accountID)
			case "friend_dog_info":
				_, getErr = m.Caches.GetFriendDogInfo(ctx, accountID)
			default:
				_, getErr = m.Caches.GetFriendList(ctx, accountID)
			}
			if getErr == nil {
				report.skipped(directory)
				continue
			}
			if !isMissing(getErr) {
				return fmt.Errorf("check %s cache for %q: %w", directory, accountID, getErr)
			}
		}
		value := CacheValue{Payload: append(json.RawMessage(nil), payload...), UpdatedAt: updatedAt}
		switch directory {
		case "known_friend_gids":
			err = m.Caches.PutKnownFriendGIDs(ctx, accountID, value)
		case "friend_dog_info":
			err = m.Caches.PutFriendDogInfo(ctx, accountID, value)
		default:
			err = m.Caches.PutFriendList(ctx, accountID, value)
		}
		if err != nil {
			return fmt.Errorf("import %s cache for %q: %w", directory, accountID, err)
		}
		report.wrote(directory)
		report.file(filepath.ToSlash(filepath.Join(directory, file.Name())))
	}
	return nil
}

func (m *JSONImporter) importBlacklist(ctx context.Context, accountID string, raw json.RawMessage, options JSONImportOptions, report *JSONImportReport) error {
	entries, err := rawArray(raw)
	if err != nil {
		return fmt.Errorf("friend blacklist for account %q: %w", accountID, err)
	}
	for index, entry := range entries {
		gid, reason, addedAt := "", "", int64(0)
		if object, objectErr := rawObject(entry); objectErr == nil {
			gid = rawString(object, "gid", "id")
			reason = rawString(object, "reason")
			addedAt = rawInt64(object, "addedAt", "added_at")
		} else {
			gid = rawScalarString(entry)
		}
		if gid == "" {
			return fmt.Errorf("friend blacklist entry %d for account %q has no gid", index, accountID)
		}
		if options.Conflict == ConflictSkip {
			existing, listErr := m.Caches.ListBlacklist(ctx, accountID)
			if listErr != nil {
				return fmt.Errorf("check blacklist for account %q: %w", accountID, listErr)
			}
			found := false
			for _, item := range existing {
				if item.GID == gid {
					found = true
					break
				}
			}
			if found {
				report.skipped("blacklist")
				continue
			}
		}
		if err := m.Caches.UpsertBlacklist(ctx, BlacklistEntry{AccountID: accountID, GID: gid, Reason: reason, AddedAt: addedAt}); err != nil {
			return fmt.Errorf("import blacklist %q for account %q: %w", gid, accountID, err)
		}
		report.wrote("blacklist")
	}
	return nil
}

func decodeLegacyAccount(raw json.RawMessage) (Account, error) {
	object, err := rawObject(raw)
	if err != nil {
		return Account{}, err
	}
	account := Account{
		ID: rawString(object, "id", "accountId", "account_id"), Name: rawString(object, "name"), Code: rawString(object, "code"),
		Platform: rawString(object, "platform"), LoginType: rawString(object, "loginType", "login_type"), Provider: rawString(object, "provider"),
		WXID: rawString(object, "wxid", "wxId"), UIN: rawString(object, "uin"), QQ: rawString(object, "qq"), GID: rawString(object, "gid"),
		OpenID: rawString(object, "openId", "open_id"), Avatar: rawString(object, "avatar", "avatarUrl", "avatar_url"),
		OwnerUser: rawString(object, "username", "ownerUser", "owner_user"), YYBOpenID: rawString(object, "yybOpenid", "yyb_openid"), Remark: rawString(object, "remark"),
		CreatedAt: rawInt64(object, "createdAt", "created_at"), UpdatedAt: rawInt64(object, "updatedAt", "updated_at"), ThirdPartyJSON: rawJSON(object, "thirdparty", "thirdParty", "thirdpartyConfig"),
	}
	if account.ID == "" {
		return Account{}, errors.New("account id is empty")
	}
	if len(account.ThirdPartyJSON) == 0 || string(account.ThirdPartyJSON) == "null" {
		account.ThirdPartyJSON = json.RawMessage(`{}`)
	}
	return account, nil
}

func decodeLegacyAccountConfig(accountID string, raw json.RawMessage) (AccountConfig, error) {
	object, err := rawObject(raw)
	if err != nil {
		return AccountConfig{}, err
	}
	config := AccountConfig{AccountID: accountID, PlantingStrategy: rawString(object, "plantingStrategy", "planting_strategy"), PreferredSeedID: rawInt64(object, "preferredSeedId", "preferred_seed_id"),
		Prioritize2x2Crops: rawBool(object, "prioritize2x2Crops", "prioritize_2x2_crops"), FriendBadRetryDate: rawString(object, "friendBadRetryDate", "friend_bad_retry_date"),
		StealDelaySeconds: rawFloat64(object, "stealDelaySeconds", "steal_delay_seconds"), PlantOrderRandom: rawBool(object, "plantOrderRandom", "plant_order_random"), PlantDelaySeconds: rawFloat64(object, "plantDelaySeconds", "plant_delay_seconds"),
		FertilizerBuyOrganicCount: rawInt64(object, "fertilizerBuyOrganicCount", "fertilizer_buy_organic_count"), FertilizerBuyOrganicThresholdHours: rawInt64(object, "fertilizerBuyOrganicThresholdHours", "fertilizer_buy_organic_threshold_hours"),
		FertilizerBuyNormalCount: rawInt64(object, "fertilizerBuyNormalCount", "fertilizer_buy_normal_count"), FertilizerBuyNormalThresholdHours: rawInt64(object, "fertilizerBuyNormalThresholdHours", "fertilizer_buy_normal_threshold_hours"), FertilizerBuyCheckIntervalMinutes: rawInt64(object, "fertilizerBuyCheckIntervalMinutes", "fertilizer_buy_check_interval_minutes"),
		AutoAcceptFriendMinLevel: rawInt64(object, "autoAcceptFriendMinLevel", "auto_accept_friend_min_level"), GoldenBugKeepCount: rawInt64(object, "goldenBugKeepCount", "golden_bug_keep_count"), GoldenBugRoundLimit: rawInt64(object, "goldenBugRoundLimit", "golden_bug_round_limit"), FriendHelpExpExhausted: rawBool(object, "friendHelpExpExhausted", "friend_help_exp_exhausted"), UpdatedAt: rawInt64(object, "updatedAt", "updated_at"), ConfigJSON: append(json.RawMessage(nil), raw...)}
	config.AutomationJSON = rawJSONOr(object, "automation", `{}`)
	config.AutoCodeRefreshJSON = rawJSONOr(object, "autoCodeRefresh", "{}")
	config.IntervalsJSON = rawJSONOr(object, "intervals", `{}`)
	config.FriendQuietHoursJSON = rawJSONOr(object, "friendQuietHours", `{}`)
	config.KnownFriendGIDsJSON = rawJSONOr(object, "knownFriendGids", `[]`)
	config.FriendBlacklistJSON = rawJSONOr(object, "friendBlacklist", `[]`)
	config.PlantBlacklistJSON = rawJSONOr(object, "plantBlacklist", `[]`)
	config.MysteryAutoBuyCurrenciesJSON = rawJSONOr(object, "mysteryAutoBuyCurrencies", `[]`)
	config.BagSeedPriorityJSON = rawJSONOr(object, "bagSeedPriority", `[]`)
	config.BagSeedFallbackStrategy = rawString(object, "bagSeedFallbackStrategy", "bag_seed_fallback_strategy")
	config.BagPriorityLandTypesJSON = rawJSONOr(object, "bagPriorityLandTypes", `[]`)
	return config, nil
}

func decodeLegacyUser(raw json.RawMessage) (User, error) {
	object, err := rawObject(raw)
	if err != nil {
		return User{}, err
	}
	cardJSON := rawJSON(object, "card", "cardJson", "card_json")
	if len(cardJSON) == 0 {
		cardJSON = json.RawMessage(`{}`)
	}
	user := User{Username: rawString(object, "username", "user"), PwdHash: rawString(object, "pwdHash", "pwd_hash", "password"), Salt: rawString(object, "salt"), Password: rawString(object, "password"), Role: rawString(object, "role"), Status: rawString(object, "status"), AccountLimit: int(rawInt64(object, "accountLimit", "account_limit")), CardCode: rawString(object, "cardCode", "card_code"), CardJSON: string(cardJSON), MustChangePassword: rawBool(object, "mustChangePassword", "must_change_password"), CreatedAt: rawInt64(object, "createdAt", "created_at"), UpdatedAt: rawInt64(object, "updatedAt", "updated_at")}
	if expire := rawInt64(object, "expireAt", "expire_at"); expire != 0 {
		user.ExpireAt = &expire
	}
	if user.PwdHash == "" {
		user.PwdHash = user.Password
	}
	if user.Username == "" || user.PwdHash == "" {
		return User{}, errors.New("username and password hash are required")
	}
	return user, nil
}

func decodeLegacyCard(raw json.RawMessage) (legacyCardRow, error) {
	object, err := rawObject(raw)
	if err != nil {
		return legacyCardRow{}, err
	}
	card := legacyCardRow{code: rawString(object, "code"), description: rawString(object, "description"), cardType: rawString(object, "type", "cardType"), status: rawString(object, "status"), durationUnit: rawString(object, "durationUnit", "duration_unit"), boundUser: rawString(object, "usedBy", "boundUser", "bound_user"), enabled: rawBoolDefault(object, true, "enabled"), permanent: rawBool(object, "isPermanent", "permanent"), days: rawFloat64(object, "days"), value: rawFloat64(object, "value"), durationValue: rawFloat64(object, "durationValue", "duration_value"), durationMS: rawInt64(object, "durationMs", "duration_ms"), usedAt: rawInt64(object, "usedAt", "used_at"), claimedAt: rawInt64(object, "claimedAt", "claimed_at"), createdAt: rawInt64(object, "createdAt", "created_at"), updatedAt: rawInt64(object, "updatedAt", "updated_at"), metadata: string(rawJSONOr(object, "metadata", string(raw)))}
	if card.code == "" {
		return legacyCardRow{}, errors.New("card code is empty")
	}
	if card.cardType == "" {
		card.cardType = "time"
	}
	if card.status == "" {
		card.status = "active"
	}
	if card.durationUnit == "" {
		card.durationUnit = "day"
	}
	if card.metadata == "" || !json.Valid([]byte(card.metadata)) {
		card.metadata = string(raw)
	}
	return card, nil
}

func readOptionalJSON(path string) (json.RawMessage, bool, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("read %s: %w", path, err)
	}
	if !json.Valid(raw) {
		return nil, true, fmt.Errorf("%s contains invalid JSON", path)
	}
	return json.RawMessage(raw), true, nil
}

func rawObject(raw json.RawMessage) (map[string]json.RawMessage, error) {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil || object == nil {
		if err == nil {
			err = errors.New("expected JSON object")
		}
		return nil, err
	}
	return object, nil
}

func rawArray(raw json.RawMessage) ([]json.RawMessage, error) {
	var entries []json.RawMessage
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

func rawArrayField(raw json.RawMessage, field string) ([]json.RawMessage, error) {
	object, err := rawObject(raw)
	if err != nil {
		return nil, err
	}
	value, ok := object[field]
	if !ok {
		return []json.RawMessage{}, nil
	}
	return rawArray(value)
}

func rawField(object map[string]json.RawMessage, names ...string) (json.RawMessage, bool) {
	for _, name := range names {
		if value, ok := object[name]; ok {
			return value, true
		}
	}
	return nil, false
}

func rawString(object map[string]json.RawMessage, names ...string) string {
	value, ok := rawField(object, names...)
	if !ok {
		return ""
	}
	return rawScalarString(value)
}

func rawStringOr(object map[string]json.RawMessage, fallback string, names ...string) string {
	if value := rawString(object, names...); value != "" {
		return value
	}
	return fallback
}

func rawScalarString(raw json.RawMessage) string {
	var value string
	if json.Unmarshal(raw, &value) == nil {
		return strings.TrimSpace(value)
	}
	var number json.Number
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.UseNumber()
	if decoder.Decode(&number) == nil {
		return number.String()
	}
	return ""
}

func rawInt64(object map[string]json.RawMessage, names ...string) int64 {
	value, ok := rawField(object, names...)
	if !ok {
		return 0
	}
	if parsed, err := strconv.ParseInt(rawScalarString(value), 10, 64); err == nil {
		return parsed
	}
	if parsed, err := strconv.ParseFloat(rawScalarString(value), 64); err == nil && !math.IsNaN(parsed) && !math.IsInf(parsed, 0) {
		return int64(parsed)
	}
	return 0
}

func rawFloat64(object map[string]json.RawMessage, names ...string) float64 {
	value, ok := rawField(object, names...)
	if !ok {
		return 0
	}
	parsed, _ := rawFloat64Value(value)
	return parsed
}

func rawFloat64Value(raw json.RawMessage) (float64, error) {
	parsed, err := strconv.ParseFloat(rawScalarString(raw), 64)
	if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
		return 0, errors.New("expected finite number")
	}
	return parsed, nil
}

func rawBool(object map[string]json.RawMessage, names ...string) bool {
	value, ok := rawField(object, names...)
	if !ok {
		return false
	}
	var parsed bool
	if json.Unmarshal(value, &parsed) == nil {
		return parsed
	}
	return strings.EqualFold(rawScalarString(value), "true") || rawScalarString(value) == "1"
}

func rawBoolDefault(object map[string]json.RawMessage, fallback bool, names ...string) bool {
	if _, ok := rawField(object, names...); !ok {
		return fallback
	}
	return rawBool(object, names...)
}

func rawJSON(object map[string]json.RawMessage, names ...string) json.RawMessage {
	if value, ok := rawField(object, names...); ok && json.Valid(value) {
		return append(json.RawMessage(nil), value...)
	}
	return nil
}

func rawJSONOr(object map[string]json.RawMessage, name, fallback string) json.RawMessage {
	if value := rawJSON(object, name); len(value) > 0 {
		return value
	}
	return json.RawMessage(fallback)
}

func sortedRawKeys(object map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(object))
	for key := range object {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func nullableImportString(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func nullableImportInt(value int64) any {
	if value == 0 {
		return nil
	}
	return value
}

func nullableImportFloat(value float64) any {
	if value == 0 {
		return nil
	}
	return value
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func isMissing(err error) bool {
	return errors.Is(err, sql.ErrNoRows) || errors.Is(err, ErrUserNotFound) || errors.Is(err, ErrCardNotFound)
}

func legacyConfigKey(key string) string {
	switch key {
	case "systemConfig":
		return ConfigKeySystem
	case "globalWxConfig":
		return ConfigKeyWX
	case "antiResaleConfig":
		return ConfigKeyAntiResale
	case "offlineReminder":
		return ConfigKeyOfflineReminder
	default:
		return "legacy:" + key
	}
}
