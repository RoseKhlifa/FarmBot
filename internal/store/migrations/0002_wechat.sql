-- P3-02: yyb WeChat identity and MMTLS state in the FarmBot database.
--
-- These tables intentionally live beside the FarmBot account tables. The
-- accounts.yyb_openid column from 0001_init.sql is the application-level link
-- to wechat_accounts.openid; credentials and session_blob remain plain here
-- until PC-02 adds the master-key encryption layer.

CREATE TABLE IF NOT EXISTS wechat_accounts (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    openid          TEXT    NOT NULL UNIQUE,
    uin             INTEGER,
    alias           TEXT,
    nickname        TEXT,
    avatar          TEXT,
    user_info       TEXT,
    login_buffer    TEXT    NOT NULL DEFAULT '',
    credentials     TEXT,
    status          TEXT,
    last_checked_at INTEGER,
    created_at      INTEGER NOT NULL DEFAULT 0,
    updated_at      INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_wechat_accounts_uin
    ON wechat_accounts(uin);

CREATE TABLE IF NOT EXISTS sessions (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    wechat_account_id INTEGER NOT NULL REFERENCES wechat_accounts(id) ON DELETE CASCADE,
    uin               INTEGER,
    tcp_proxy         TEXT    NOT NULL DEFAULT '',
    session_blob      TEXT    NOT NULL,
    expires_at        INTEGER NOT NULL,
    created_at        INTEGER NOT NULL DEFAULT 0,
    updated_at        INTEGER NOT NULL DEFAULT 0,
    UNIQUE(wechat_account_id, tcp_proxy)
);

CREATE INDEX IF NOT EXISTS idx_wechat_sessions_expires
    ON sessions(expires_at);

CREATE TABLE IF NOT EXISTS features (
    code        INTEGER PRIMARY KEY,
    name        TEXT    NOT NULL UNIQUE,
    description TEXT,
    enabled     INTEGER NOT NULL DEFAULT 1 CHECK (enabled IN (0, 1))
);

CREATE INDEX IF NOT EXISTS idx_wechat_accounts_openid
    ON wechat_accounts(openid);

CREATE INDEX IF NOT EXISTS idx_accounts_yyb_openid
    ON accounts(yyb_openid);
