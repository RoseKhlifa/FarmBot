-- P1-02: initial FarmBot persistence schema.
--
-- JSON payload columns intentionally preserve fields that are still evolving
-- during the Node-to-Go migration. Frequently queried account/config fields
-- remain expanded so repositories can add typed access without rewriting the
-- legacy snapshots.

CREATE TABLE IF NOT EXISTS users (
    username              TEXT PRIMARY KEY,
    pwd_hash              TEXT NOT NULL,
    salt                  TEXT NOT NULL DEFAULT '',
    role                  TEXT NOT NULL DEFAULT 'user',
    status                TEXT NOT NULL DEFAULT 'active',
    expire_at             INTEGER,
    account_limit         INTEGER NOT NULL DEFAULT 1,
    card_code             TEXT,
    card_json             TEXT NOT NULL DEFAULT '{}',
    must_change_password  INTEGER NOT NULL DEFAULT 0 CHECK (must_change_password IN (0, 1)),
    created_at            INTEGER NOT NULL DEFAULT 0,
    updated_at            INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_users_role ON users(role);
CREATE INDEX IF NOT EXISTS idx_users_status ON users(status);

CREATE TABLE IF NOT EXISTS accounts (
    id                    TEXT PRIMARY KEY,
    name                  TEXT NOT NULL DEFAULT '',
    code                  TEXT NOT NULL DEFAULT '',
    platform              TEXT NOT NULL DEFAULT 'qq',
    login_type            TEXT NOT NULL DEFAULT 'manual',
    provider              TEXT NOT NULL DEFAULT 'builtin',
    wxid                  TEXT NOT NULL DEFAULT '',
    uin                   TEXT NOT NULL DEFAULT '',
    qq                    TEXT NOT NULL DEFAULT '',
    gid                   TEXT NOT NULL DEFAULT '',
    open_id               TEXT NOT NULL DEFAULT '',
    avatar                TEXT NOT NULL DEFAULT '',
    owner_user            TEXT,
    yyb_openid            TEXT,
    remark                TEXT,
    thirdparty_json       TEXT NOT NULL DEFAULT '{}',
    created_at            INTEGER NOT NULL DEFAULT 0,
    updated_at            INTEGER NOT NULL DEFAULT 0,
    FOREIGN KEY (owner_user) REFERENCES users(username) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_accounts_owner_user ON accounts(owner_user);
CREATE INDEX IF NOT EXISTS idx_accounts_provider ON accounts(provider);
CREATE INDEX IF NOT EXISTS idx_accounts_yyb_openid ON accounts(yyb_openid);

CREATE TABLE IF NOT EXISTS account_config (
    account_id                              TEXT PRIMARY KEY,
    automation_json                         TEXT NOT NULL DEFAULT '{}',
    auto_code_refresh_json                  TEXT NOT NULL DEFAULT '{}',
    planting_strategy                       TEXT NOT NULL DEFAULT 'max_exp',
    preferred_seed_id                       INTEGER NOT NULL DEFAULT 0,
    prioritize_2x2_crops                    INTEGER NOT NULL DEFAULT 0 CHECK (prioritize_2x2_crops IN (0, 1)),
    friend_bad_retry_date                   TEXT NOT NULL DEFAULT '',
    intervals_json                          TEXT NOT NULL DEFAULT '{}',
    friend_quiet_hours_json                 TEXT NOT NULL DEFAULT '{}',
    known_friend_gids_json                  TEXT NOT NULL DEFAULT '[]',
    friend_blacklist_json                   TEXT NOT NULL DEFAULT '[]',
    plant_blacklist_json                    TEXT NOT NULL DEFAULT '[]',
    steal_delay_seconds                     REAL NOT NULL DEFAULT 1,
    plant_order_random                      INTEGER NOT NULL DEFAULT 1 CHECK (plant_order_random IN (0, 1)),
    plant_delay_seconds                     REAL NOT NULL DEFAULT 2,
    fertilizer_buy_organic_count            INTEGER NOT NULL DEFAULT 1,
    fertilizer_buy_organic_threshold_hours  INTEGER NOT NULL DEFAULT 10,
    fertilizer_buy_normal_count             INTEGER NOT NULL DEFAULT 1,
    fertilizer_buy_normal_threshold_hours   INTEGER NOT NULL DEFAULT 10,
    fertilizer_buy_check_interval_minutes   INTEGER NOT NULL DEFAULT 60,
    mystery_auto_buy_currencies_json        TEXT NOT NULL DEFAULT '[]',
    bag_seed_priority_json                  TEXT NOT NULL DEFAULT '[]',
    bag_seed_fallback_strategy               TEXT NOT NULL DEFAULT 'level',
    bag_priority_land_types_json            TEXT NOT NULL DEFAULT '[]',
    auto_accept_friend_min_level            INTEGER NOT NULL DEFAULT 0,
    golden_bug_keep_count                   INTEGER NOT NULL DEFAULT 0,
    golden_bug_round_limit                  INTEGER NOT NULL DEFAULT 24,
    friend_help_exp_exhausted               INTEGER NOT NULL DEFAULT 0 CHECK (friend_help_exp_exhausted IN (0, 1)),
    config_json                             TEXT NOT NULL DEFAULT '{}',
    updated_at                              INTEGER NOT NULL DEFAULT 0,
    FOREIGN KEY (account_id) REFERENCES accounts(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS cards (
    code                  TEXT PRIMARY KEY,
    description           TEXT NOT NULL DEFAULT '',
    type                  TEXT NOT NULL DEFAULT 'time',
    status                TEXT NOT NULL DEFAULT 'active',
    enabled               INTEGER NOT NULL DEFAULT 1 CHECK (enabled IN (0, 1)),
    days                  REAL,
    value                 INTEGER,
    duration_value        REAL,
    duration_unit         TEXT NOT NULL DEFAULT 'day',
    duration_ms           INTEGER,
    is_permanent          INTEGER NOT NULL DEFAULT 0 CHECK (is_permanent IN (0, 1)),
    bound_user            TEXT,
    used_at               INTEGER,
    claimed_at            INTEGER,
    created_at            INTEGER NOT NULL DEFAULT 0,
    updated_at            INTEGER NOT NULL DEFAULT 0,
    metadata_json         TEXT NOT NULL DEFAULT '{}',
    FOREIGN KEY (bound_user) REFERENCES users(username) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_cards_status ON cards(status, enabled);
CREATE INDEX IF NOT EXISTS idx_cards_bound_user ON cards(bound_user);

CREATE TABLE IF NOT EXISTS login_attempts (
    id                    INTEGER PRIMARY KEY AUTOINCREMENT,
    ip                    TEXT,
    username              TEXT,
    subject               TEXT NOT NULL UNIQUE,
    count                 INTEGER NOT NULL DEFAULT 0,
    window_start          INTEGER,
    first_attempt         INTEGER,
    last_attempt          INTEGER,
    locked_until          INTEGER
);

CREATE INDEX IF NOT EXISTS idx_login_attempts_ip ON login_attempts(ip);
CREATE INDEX IF NOT EXISTS idx_login_attempts_username ON login_attempts(username);
CREATE INDEX IF NOT EXISTS idx_login_attempts_locked_until ON login_attempts(locked_until);

CREATE TABLE IF NOT EXISTS login_logs (
    id                    INTEGER PRIMARY KEY AUTOINCREMENT,
    user                  TEXT NOT NULL DEFAULT '',
    username              TEXT,
    ip                    TEXT NOT NULL DEFAULT '',
    ua                    TEXT NOT NULL DEFAULT '',
    user_agent            TEXT,
    result                TEXT NOT NULL DEFAULT '',
    event                 TEXT,
    error_type            TEXT,
    ts                    INTEGER NOT NULL DEFAULT 0,
    metadata_json         TEXT NOT NULL DEFAULT '{}'
);

CREATE INDEX IF NOT EXISTS idx_login_logs_user ON login_logs(user);
CREATE INDEX IF NOT EXISTS idx_login_logs_ts ON login_logs(ts DESC);

CREATE TABLE IF NOT EXISTS friend_gid_cache (
    account_id            TEXT PRIMARY KEY,
    payload               TEXT NOT NULL DEFAULT '[]',
    updated_at            INTEGER NOT NULL DEFAULT 0,
    FOREIGN KEY (account_id) REFERENCES accounts(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS friend_dog_info (
    account_id            TEXT PRIMARY KEY,
    payload               TEXT NOT NULL DEFAULT '{}',
    updated_at            INTEGER NOT NULL DEFAULT 0,
    FOREIGN KEY (account_id) REFERENCES accounts(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS friend_list_cache (
    account_id            TEXT PRIMARY KEY,
    payload               TEXT NOT NULL DEFAULT '[]',
    updated_at            INTEGER NOT NULL DEFAULT 0,
    FOREIGN KEY (account_id) REFERENCES accounts(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS blacklist (
    account_id            TEXT NOT NULL,
    gid                   TEXT NOT NULL,
    reason                TEXT NOT NULL DEFAULT '',
    added_at              INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (account_id, gid),
    FOREIGN KEY (account_id) REFERENCES accounts(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_blacklist_account ON blacklist(account_id);

CREATE TABLE IF NOT EXISTS stats (
    account_id            TEXT NOT NULL,
    metric                TEXT NOT NULL,
    date                  TEXT NOT NULL DEFAULT '',
    value                 REAL NOT NULL DEFAULT 0,
    updated_at            INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (account_id, metric, date),
    FOREIGN KEY (account_id) REFERENCES accounts(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_stats_account ON stats(account_id);

CREATE TABLE IF NOT EXISTS global_config (
    key                   TEXT PRIMARY KEY,
    value                 TEXT NOT NULL DEFAULT '{}',
    updated_at            INTEGER NOT NULL DEFAULT 0
);
