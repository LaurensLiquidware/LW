-- A single row of operator-editable runtime settings (project brief-style
-- follow-up: a Settings screen for everything that ISN'T needed to boot
-- the process in the first place). Bootstrap-only settings -- listen
-- address, DB driver/DSN, credential encryption key, the initial admin
-- account -- stay env-var-only and never appear here, since the process
-- needs them before it can even open this database.
--
-- Seeded once from the PUMC_* environment variables on first startup
-- (see internal/settings.Store.EnsureSeeded); after that, this row is
-- the sole source of truth -- an operator's change in the Settings
-- screen survives every future restart, and the env vars for these
-- particular fields are only ever consulted again if this row is
-- somehow missing.
CREATE TABLE runtime_settings (
    id INTEGER PRIMARY KEY CHECK (id = 1), -- singleton row, enforced by the check

    smtp_host     TEXT NOT NULL DEFAULT '',
    smtp_port     INTEGER NOT NULL DEFAULT 587,
    smtp_username TEXT NOT NULL DEFAULT '',
    smtp_password TEXT NOT NULL DEFAULT '',
    smtp_from     TEXT NOT NULL DEFAULT '',
    smtp_security TEXT NOT NULL DEFAULT 'starttls',

    report_recipients TEXT NOT NULL DEFAULT '', -- comma-separated
    report_email_day  INTEGER NOT NULL DEFAULT 1,

    collection_interval_seconds       INTEGER NOT NULL DEFAULT 3600,
    collection_timezone               TEXT NOT NULL DEFAULT 'UTC',
    collection_concurrency            INTEGER NOT NULL DEFAULT 5,
    collection_tenant_timeout_seconds INTEGER NOT NULL DEFAULT 30,

    session_idle_timeout_seconds     INTEGER NOT NULL DEFAULT 1800,
    session_absolute_timeout_seconds INTEGER NOT NULL DEFAULT 43200,

    -- The active TLS certificate/key, PEM-encoded. Empty until either the
    -- self-signed bootstrap cert is generated (copied in here so a
    -- restart doesn't need to re-read the files) or an operator uploads
    -- a real one via the Settings screen.
    tls_cert_pem TEXT NOT NULL DEFAULT '',
    tls_key_pem  TEXT NOT NULL DEFAULT '',

    updated_at_utc TEXT NOT NULL
);
