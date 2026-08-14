-- The console's own operators (project brief §9: "at minimum
-- operator/viewer... never rely on network position"). Distinct from
-- ProfileUnity tenant credentials, which live in the tenants table.
CREATE TABLE users (
    id             TEXT PRIMARY KEY,
    username       TEXT NOT NULL UNIQUE,
    password_hash  TEXT NOT NULL,
    role           TEXT NOT NULL, -- 'operator' or 'viewer'
    created_at_utc TEXT NOT NULL,
    updated_at_utc TEXT NOT NULL
);

-- Server-side session state, so a session can be revoked (logout) and an
-- idle/absolute timeout enforced without trusting the cookie alone.
CREATE TABLE sessions (
    id               TEXT PRIMARY KEY, -- hash of the token in the cookie, never the raw token
    user_id          TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at_utc   TEXT NOT NULL,
    last_seen_at_utc TEXT NOT NULL,
    expires_at_utc   TEXT NOT NULL
);

CREATE INDEX idx_sessions_user ON sessions (user_id);
