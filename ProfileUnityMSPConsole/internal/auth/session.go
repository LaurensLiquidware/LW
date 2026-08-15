package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"sync/atomic"
	"time"
)

// ErrSessionInvalid covers a missing, expired, or idled-out session —
// deliberately one error, so a caller can't distinguish "never existed"
// from "expired" from a timing/response-shape difference.
var ErrSessionInvalid = errors.New("auth: session invalid or expired")

// Session is server-side session state for one logged-in user.
type Session struct {
	UserID string
}

// SessionRepo stores sessions keyed by a hash of the bearer token — the
// raw token exists only in the cookie, never in the database, so a
// database read alone can never impersonate a user.
//
// idleTimeout/absoluteTimeout are atomic.Int64 (nanoseconds) rather than
// plain time.Duration fields so the Settings screen can change them at
// runtime from an HTTP handler goroutine while Validate reads them
// concurrently from every other request.
type SessionRepo struct {
	db              *sql.DB
	idleTimeout     atomic.Int64
	absoluteTimeout atomic.Int64
}

func NewSessionRepo(db *sql.DB, idleTimeout, absoluteTimeout time.Duration) *SessionRepo {
	r := &SessionRepo{db: db}
	r.SetTimeouts(idleTimeout, absoluteTimeout)
	return r
}

// SetTimeouts changes the idle and absolute session timeouts, effective
// immediately for idle timeout (checked fresh on every Validate call)
// and for the absolute timeout of any session created from now on —
// already-issued sessions keep the absolute expiry that was computed
// and stored at their own Create time. Safe to call from any goroutine.
func (r *SessionRepo) SetTimeouts(idleTimeout, absoluteTimeout time.Duration) {
	r.idleTimeout.Store(int64(idleTimeout))
	r.absoluteTimeout.Store(int64(absoluteTimeout))
}

func (r *SessionRepo) IdleTimeout() time.Duration {
	return time.Duration(r.idleTimeout.Load())
}

func (r *SessionRepo) AbsoluteTimeout() time.Duration {
	return time.Duration(r.absoluteTimeout.Load())
}

// Create starts a new session for userID and returns the raw token to
// set in the session cookie.
func (r *SessionRepo) Create(ctx context.Context, userID string) (token string, err error) {
	token, err = newToken()
	if err != nil {
		return "", err
	}
	now := time.Now().UTC()
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO sessions (id, user_id, created_at_utc, last_seen_at_utc, expires_at_utc)
		VALUES (?, ?, ?, ?, ?)`,
		hashToken(token), userID, now.Format(isoUTC), now.Format(isoUTC), now.Add(r.AbsoluteTimeout()).Format(isoUTC))
	if err != nil {
		return "", fmt.Errorf("auth: create session: %w", err)
	}
	return token, nil
}

// Validate checks token against stored session state, enforcing both the
// idle timeout (time since last activity) and the absolute timeout (time
// since login), and — on success — refreshes last-seen to now.
func (r *SessionRepo) Validate(ctx context.Context, token string) (Session, error) {
	id := hashToken(token)

	var userID, lastSeenAt, expiresAt string
	row := r.db.QueryRowContext(ctx, `SELECT user_id, last_seen_at_utc, expires_at_utc FROM sessions WHERE id = ?`, id)
	if err := row.Scan(&userID, &lastSeenAt, &expiresAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Session{}, ErrSessionInvalid
		}
		return Session{}, fmt.Errorf("auth: validate session: %w", err)
	}

	lastSeen, err := time.Parse(isoUTC, lastSeenAt)
	if err != nil {
		return Session{}, fmt.Errorf("auth: parse last_seen_at_utc: %w", err)
	}
	expires, err := time.Parse(isoUTC, expiresAt)
	if err != nil {
		return Session{}, fmt.Errorf("auth: parse expires_at_utc: %w", err)
	}

	now := time.Now().UTC()
	if now.After(expires) || now.Sub(lastSeen) > r.IdleTimeout() {
		_, _ = r.db.ExecContext(ctx, `DELETE FROM sessions WHERE id = ?`, id)
		return Session{}, ErrSessionInvalid
	}

	if _, err := r.db.ExecContext(ctx, `UPDATE sessions SET last_seen_at_utc = ? WHERE id = ?`, now.Format(isoUTC), id); err != nil {
		return Session{}, fmt.Errorf("auth: refresh session: %w", err)
	}
	return Session{UserID: userID}, nil
}

// Revoke ends one session (logout).
func (r *SessionRepo) Revoke(ctx context.Context, token string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM sessions WHERE id = ?`, hashToken(token))
	if err != nil {
		return fmt.Errorf("auth: revoke session: %w", err)
	}
	return nil
}

// RevokeAllForUser ends every session belonging to userID -- called
// right after a user account is deleted. sessions.user_id declares
// ON DELETE CASCADE, but this database never enables
// PRAGMA foreign_keys, so that constraint is not actually enforced;
// this explicit delete is what actually invalidates the deleted
// account's sessions rather than leaving them valid until they expire
// naturally.
func (r *SessionRepo) RevokeAllForUser(ctx context.Context, userID string) error {
	if _, err := r.db.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = ?`, userID); err != nil {
		return fmt.Errorf("auth: revoke sessions for user: %w", err)
	}
	return nil
}

func newToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("auth: generate token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
