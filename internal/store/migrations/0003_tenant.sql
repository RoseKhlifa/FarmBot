-- PC-02: tenant isolation, plans and quota metadata.
-- Existing rows remain valid with NULL tenant_id and are assigned explicitly by
-- the application migration path before tenant-scoped access is enabled.

CREATE TABLE IF NOT EXISTS tenants (
    id                TEXT PRIMARY KEY,
    name              TEXT NOT NULL DEFAULT '',
    plan              TEXT NOT NULL DEFAULT 'starter',
    account_limit     INTEGER NOT NULL DEFAULT 3,
    concurrent_limit  INTEGER NOT NULL DEFAULT 1,
    features_json     TEXT NOT NULL DEFAULT '{}',
    status            TEXT NOT NULL DEFAULT 'active',
    created_at        INTEGER NOT NULL DEFAULT 0,
    updated_at        INTEGER NOT NULL DEFAULT 0
);

ALTER TABLE users ADD COLUMN tenant_id TEXT REFERENCES tenants(id) ON DELETE SET NULL;
ALTER TABLE users ADD COLUMN plan TEXT NOT NULL DEFAULT 'starter';
ALTER TABLE accounts ADD COLUMN tenant_id TEXT REFERENCES tenants(id) ON DELETE SET NULL;
ALTER TABLE accounts ADD COLUMN running INTEGER NOT NULL DEFAULT 0 CHECK (running IN (0, 1));

CREATE INDEX IF NOT EXISTS idx_users_tenant_id ON users(tenant_id);
CREATE INDEX IF NOT EXISTS idx_accounts_tenant_id ON accounts(tenant_id);
CREATE INDEX IF NOT EXISTS idx_tenants_status ON tenants(status);
