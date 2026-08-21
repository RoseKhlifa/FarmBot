-- PC-05: append-only administrative operation audit trail.
CREATE TABLE IF NOT EXISTS audit_log (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    actor_user      TEXT NOT NULL DEFAULT '',
    action          TEXT NOT NULL DEFAULT '',
    target_account  TEXT NOT NULL DEFAULT '',
    ip              TEXT NOT NULL DEFAULT '',
    ua              TEXT NOT NULL DEFAULT '',
    detail_json     TEXT NOT NULL DEFAULT '{}',
    ts              INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_audit_log_ts ON audit_log(ts DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_audit_log_target ON audit_log(target_account, ts DESC);
CREATE TRIGGER IF NOT EXISTS audit_log_no_update
BEFORE UPDATE ON audit_log BEGIN SELECT RAISE(ABORT, 'audit_log is append-only'); END;
CREATE TRIGGER IF NOT EXISTS audit_log_no_delete
BEFORE DELETE ON audit_log BEGIN SELECT RAISE(ABORT, 'audit_log is append-only'); END;
