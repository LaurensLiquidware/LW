package auth

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"profileunity-msp-console/internal/db"
)

func newTestDB(t *testing.T) *UserRepo {
	t.Helper()
	sqlDB, err := db.Open("sqlite", filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	return NewUserRepo(sqlDB)
}

func TestUserRepo_CreateAndAuthenticate(t *testing.T) {
	repo := newTestDB(t)
	ctx := context.Background()

	created, err := repo.CreateUser(ctx, "jane", "correct-horse-battery-staple", RoleOperator)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if created.Role != RoleOperator {
		t.Errorf("Role = %q, want operator", created.Role)
	}

	u, err := repo.Authenticate(ctx, "jane", "correct-horse-battery-staple")
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if u.ID != created.ID {
		t.Errorf("ID = %q, want %q", u.ID, created.ID)
	}
}

func TestUserRepo_Authenticate_WrongPassword(t *testing.T) {
	repo := newTestDB(t)
	ctx := context.Background()

	if _, err := repo.CreateUser(ctx, "jane", "correct-horse-battery-staple", RoleViewer); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Authenticate(ctx, "jane", "wrong-password-entirely"); err != ErrUserNotFound {
		t.Errorf("err = %v, want ErrUserNotFound", err)
	}
}

func TestUserRepo_Authenticate_UnknownUsername(t *testing.T) {
	repo := newTestDB(t)
	if _, err := repo.Authenticate(context.Background(), "nobody", "whatever-password"); err != ErrUserNotFound {
		t.Errorf("err = %v, want ErrUserNotFound", err)
	}
}

func TestUserRepo_CreateUser_RejectsDuplicateUsername(t *testing.T) {
	repo := newTestDB(t)
	ctx := context.Background()

	if _, err := repo.CreateUser(ctx, "jane", "correct-horse-battery-staple", RoleOperator); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CreateUser(ctx, "jane", "another-long-password", RoleViewer); err != ErrUsernameTaken {
		t.Errorf("err = %v, want ErrUsernameTaken", err)
	}
}

func TestUserRepo_CreateUser_RejectsShortPassword(t *testing.T) {
	repo := newTestDB(t)
	if _, err := repo.CreateUser(context.Background(), "jane", "short", RoleOperator); err == nil {
		t.Fatal("expected an error for a too-short password")
	}
}

func TestUserRepo_PasswordNeverStoredInPlaintext(t *testing.T) {
	repo := newTestDB(t)
	ctx := context.Background()

	u, err := repo.CreateUser(ctx, "jane", "correct-horse-battery-staple", RoleOperator)
	if err != nil {
		t.Fatal(err)
	}

	var hash string
	if err := repo.db.QueryRowContext(ctx, `SELECT password_hash FROM users WHERE id = ?`, u.ID).Scan(&hash); err != nil {
		t.Fatal(err)
	}
	if hash == "correct-horse-battery-staple" {
		t.Fatal("password stored in plaintext")
	}
}

func newTestSessionRepo(t *testing.T, idle, absolute time.Duration) (*UserRepo, *SessionRepo) {
	t.Helper()
	sqlDB, err := db.Open("sqlite", filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	return NewUserRepo(sqlDB), NewSessionRepo(sqlDB, idle, absolute)
}

func TestSessionRepo_CreateAndValidate(t *testing.T) {
	users, sessions := newTestSessionRepo(t, time.Hour, 12*time.Hour)
	ctx := context.Background()

	u, err := users.CreateUser(ctx, "jane", "correct-horse-battery-staple", RoleOperator)
	if err != nil {
		t.Fatal(err)
	}

	token, err := sessions.Create(ctx, u.ID)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if token == "" {
		t.Fatal("expected a non-empty token")
	}

	sess, err := sessions.Validate(ctx, token)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if sess.UserID != u.ID {
		t.Errorf("UserID = %q, want %q", sess.UserID, u.ID)
	}
}

func TestSessionRepo_Validate_RejectsUnknownToken(t *testing.T) {
	_, sessions := newTestSessionRepo(t, time.Hour, 12*time.Hour)
	if _, err := sessions.Validate(context.Background(), "not-a-real-token"); err != ErrSessionInvalid {
		t.Errorf("err = %v, want ErrSessionInvalid", err)
	}
}

func TestSessionRepo_Revoke(t *testing.T) {
	users, sessions := newTestSessionRepo(t, time.Hour, 12*time.Hour)
	ctx := context.Background()

	u, err := users.CreateUser(ctx, "jane", "correct-horse-battery-staple", RoleOperator)
	if err != nil {
		t.Fatal(err)
	}
	token, err := sessions.Create(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := sessions.Revoke(ctx, token); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	if _, err := sessions.Validate(ctx, token); err != ErrSessionInvalid {
		t.Errorf("err = %v, want ErrSessionInvalid after revoke", err)
	}
}

func TestSessionRepo_Validate_EnforcesIdleTimeout(t *testing.T) {
	users, sessions := newTestSessionRepo(t, 10*time.Millisecond, time.Hour)
	ctx := context.Background()

	u, err := users.CreateUser(ctx, "jane", "correct-horse-battery-staple", RoleOperator)
	if err != nil {
		t.Fatal(err)
	}
	token, err := sessions.Create(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(30 * time.Millisecond)
	if _, err := sessions.Validate(ctx, token); err != ErrSessionInvalid {
		t.Errorf("err = %v, want ErrSessionInvalid after idle timeout", err)
	}
}

func TestSessionRepo_Validate_EnforcesAbsoluteTimeout(t *testing.T) {
	users, sessions := newTestSessionRepo(t, time.Hour, 10*time.Millisecond)
	ctx := context.Background()

	u, err := users.CreateUser(ctx, "jane", "correct-horse-battery-staple", RoleOperator)
	if err != nil {
		t.Fatal(err)
	}
	token, err := sessions.Create(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(30 * time.Millisecond)
	if _, err := sessions.Validate(ctx, token); err != ErrSessionInvalid {
		t.Errorf("err = %v, want ErrSessionInvalid after absolute timeout", err)
	}
}
