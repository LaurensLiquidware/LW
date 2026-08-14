-- Tenant registrations (one per customer ProfileUnity console) and the
-- daily license snapshots collected from them. See project brief §7.1/§7.2.
CREATE TABLE tenants (
    id                  TEXT PRIMARY KEY,
    display_name        TEXT NOT NULL,
    hostname             TEXT NOT NULL,
    port                 INTEGER NOT NULL DEFAULT 8000,
    username             TEXT NOT NULL DEFAULT '',
    encrypted_password   BLOB,
    tls_skip_verify      INTEGER NOT NULL DEFAULT 0,
    enabled              INTEGER NOT NULL DEFAULT 1,
    tags                 TEXT NOT NULL DEFAULT '',
    notes                TEXT NOT NULL DEFAULT '',
    created_at_utc       TEXT NOT NULL,
    updated_at_utc       TEXT NOT NULL
);

-- One row per tenant per collection day (the day boundary is computed in
-- the configured collection timezone, per §11.2 -- comparison and storage
-- always use the ISO date/UTC timestamp columns below, never a formatted
-- string in the display locale). The unique constraint is what makes
-- collection idempotent: re-running the same day upserts this row instead
-- of creating a duplicate.
CREATE TABLE snapshots (
    id                   TEXT PRIMARY KEY,
    tenant_id            TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    collection_date      TEXT NOT NULL, -- YYYY-MM-DD in the configured collection timezone
    collected_at_utc     TEXT NOT NULL,

    -- Outcome of this collection attempt. "success" is the only status
    -- with meaningful license figures below -- every other status means
    -- the figures are absent, and must be rendered as unknown, never as
    -- zero usage (project brief §2).
    status               TEXT NOT NULL,
    auth_path            TEXT NOT NULL DEFAULT '',
    error_message        TEXT NOT NULL DEFAULT '',

    -- The full raw API response body, retained so a future field-mapping
    -- change never has to work from parsed-and-possibly-wrong history.
    raw_payload          TEXT NOT NULL DEFAULT '',

    registered_to        TEXT NOT NULL DEFAULT '',
    license_mode         TEXT NOT NULL DEFAULT '',
    license_product      TEXT NOT NULL DEFAULT '',
    total_licenses        INTEGER,
    used_licenses         INTEGER,
    evaluation            INTEGER,
    console_version       TEXT NOT NULL DEFAULT '',
    is_trial_expired      INTEGER,
    is_trial              INTEGER,
    is_pro_u_only          INTEGER,
    is_flex_only           INTEGER,
    support_ends_iso       TEXT NOT NULL DEFAULT '',

    UNIQUE (tenant_id, collection_date)
);

CREATE INDEX idx_snapshots_tenant_date ON snapshots (tenant_id, collection_date);
