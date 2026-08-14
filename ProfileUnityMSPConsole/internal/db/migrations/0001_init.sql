-- Phase 1 skeleton migration. Domain tables (tenants, snapshots,
-- collection_runs, ...) are added in Phase 2/3 as those features land.
CREATE TABLE app_meta (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

INSERT INTO app_meta (key, value) VALUES ('schema_initialized_at_utc', strftime('%Y-%m-%dT%H:%M:%SZ', 'now'));
