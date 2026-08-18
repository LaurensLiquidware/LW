-- Per-tenant ProfileUnity License Server connection -- a distinct host
-- from the tenant's own ProfileUnity console (see the tenants table
-- above), authenticated with that server's own Mongo-connection-string
-- username/password (its only identity store). One License Server per
-- tenant, matching the server's own one-license-per-server constraint.
ALTER TABLE tenants ADD COLUMN license_server_hostname TEXT NOT NULL DEFAULT '';
ALTER TABLE tenants ADD COLUMN license_server_port INTEGER NOT NULL DEFAULT 443;
ALTER TABLE tenants ADD COLUMN license_server_username TEXT NOT NULL DEFAULT '';
ALTER TABLE tenants ADD COLUMN license_server_encrypted_password BLOB;
ALTER TABLE tenants ADD COLUMN license_server_tls_skip_verify INTEGER NOT NULL DEFAULT 0;

-- Audit trail for pushes -- the License Server itself keeps no history
-- (every push destroys the prior license and purges its seats), so this
-- is the only record of "what replaced what, when, by whom." Stores the
-- full pushed license code so a past push can be redone without having
-- to re-locate the original license file.
CREATE TABLE license_pushes (
    id                    TEXT PRIMARY KEY,
    tenant_id             TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    pushed_at_utc         TEXT NOT NULL,
    operator_username     TEXT NOT NULL,
    outcome               TEXT NOT NULL, -- success | auth_failed | rejected | unreachable | error
    error_message         TEXT NOT NULL DEFAULT '',
    license_code_base64   TEXT NOT NULL,
    organization          TEXT NOT NULL DEFAULT '',
    contact_name          TEXT NOT NULL DEFAULT '',
    contact_email         TEXT NOT NULL DEFAULT '',
    valid_until           TEXT NOT NULL DEFAULT '',
    license_type          TEXT NOT NULL DEFAULT '',
    max_users             INTEGER,
    is_machine            INTEGER NOT NULL DEFAULT 0,
    is_concurrent         INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX idx_license_pushes_tenant ON license_pushes(tenant_id, pushed_at_utc);
