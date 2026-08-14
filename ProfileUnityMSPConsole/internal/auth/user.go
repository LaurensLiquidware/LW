// Package auth is the console's own operator/viewer authentication and
// session handling (project brief §9) — entirely separate from tenant
// ProfileUnity credentials, which live in internal/tenant. Nothing here
// ever talks to a ProfileUnity console.
package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// Role is a console operator's permission level. There is no read-only
// role on the ProfileUnity side (project brief §3.6); this one is ours.
type Role string

const (
	RoleOperator Role = "operator"
	RoleViewer   Role = "viewer"
)

// ErrUserNotFound is returned when a username has no matching account.
var ErrUserNotFound = errors.New("auth: user not found")

// ErrUsernameTaken is returned by CreateUser on a duplicate username.
var ErrUsernameTaken = errors.New("auth: username already taken")

// User is a console operator/viewer account. It never carries a password
// hash outside this package.
type User struct {
	ID        string
	Username  string
	Role      Role
	CreatedAt time.Time
	UpdatedAt time.Time
}

const isoUTC = "2006-01-02T15:04:05Z"

// UserRepo stores console operator accounts.
type UserRepo struct {
	db *sql.DB
}

func NewUserRepo(db *sql.DB) *UserRepo {
	return &UserRepo{db: db}
}

// CreateUser hashes password with bcrypt and stores a new account.
func (r *UserRepo) CreateUser(ctx context.Context, username, password string, role Role) (User, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return User{}, fmt.Errorf("auth: username is required")
	}
	if len(password) < 12 {
		return User{}, fmt.Errorf("auth: password must be at least 12 characters")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return User{}, fmt.Errorf("auth: hash password: %w", err)
	}

	now := time.Now().UTC()
	u := User{ID: uuid.NewString(), Username: username, Role: role, CreatedAt: now, UpdatedAt: now}

	_, err = r.db.ExecContext(ctx, `
		INSERT INTO users (id, username, password_hash, role, created_at_utc, updated_at_utc)
		VALUES (?, ?, ?, ?, ?, ?)`,
		u.ID, u.Username, string(hash), string(u.Role), now.Format(isoUTC), now.Format(isoUTC))
	if err != nil {
		if isUniqueConstraintErr(err) {
			return User{}, ErrUsernameTaken
		}
		return User{}, fmt.Errorf("auth: create user: %w", err)
	}
	return u, nil
}

// Authenticate checks username/password and returns the matching User on
// success. It takes the same amount of time whether the username exists
// or not, by always running a bcrypt comparison — against a fixed dummy
// hash when the account is missing — so a login failure never reveals
// which case occurred through timing.
func (r *UserRepo) Authenticate(ctx context.Context, username, password string) (User, error) {
	var id, storedUsername, hash, role, createdAt, updatedAt string
	row := r.db.QueryRowContext(ctx, `SELECT id, username, password_hash, role, created_at_utc, updated_at_utc FROM users WHERE username = ?`, username)
	err := row.Scan(&id, &storedUsername, &hash, &role, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		bcrypt.CompareHashAndPassword([]byte(dummyHash), []byte(password))
		return User{}, ErrUserNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("auth: authenticate: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return User{}, ErrUserNotFound
	}

	createdAtTime, err := time.Parse(isoUTC, createdAt)
	if err != nil {
		return User{}, fmt.Errorf("auth: parse created_at_utc: %w", err)
	}
	updatedAtTime, err := time.Parse(isoUTC, updatedAt)
	if err != nil {
		return User{}, fmt.Errorf("auth: parse updated_at_utc: %w", err)
	}
	return User{ID: id, Username: storedUsername, Role: Role(role), CreatedAt: createdAtTime, UpdatedAt: updatedAtTime}, nil
}

// GetByID looks up a user for an active session.
func (r *UserRepo) GetByID(ctx context.Context, id string) (User, error) {
	var u User
	var role, createdAt, updatedAt string
	row := r.db.QueryRowContext(ctx, `SELECT id, username, role, created_at_utc, updated_at_utc FROM users WHERE id = ?`, id)
	if err := row.Scan(&u.ID, &u.Username, &role, &createdAt, &updatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, ErrUserNotFound
		}
		return User{}, fmt.Errorf("auth: get user: %w", err)
	}
	u.Role = Role(role)
	var err error
	u.CreatedAt, err = time.Parse(isoUTC, createdAt)
	if err != nil {
		return User{}, err
	}
	u.UpdatedAt, err = time.Parse(isoUTC, updatedAt)
	if err != nil {
		return User{}, err
	}
	return u, nil
}

// Count returns how many operator accounts exist, so the server can
// decide whether to bootstrap one at startup.
func (r *UserRepo) Count(ctx context.Context) (int, error) {
	var n int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&n); err != nil {
		return 0, fmt.Errorf("auth: count users: %w", err)
	}
	return n, nil
}

// dummyHash is a valid bcrypt hash of an arbitrary fixed string, compared
// against on every "unknown username" path purely to keep that path's
// timing indistinguishable from a real (failing) comparison.
const dummyHash = "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"

func isUniqueConstraintErr(err error) bool {
	return strings.Contains(err.Error(), "UNIQUE constraint failed")
}
