package db

import (
	"path/filepath"
	"testing"
)

func TestOpen_AppliesMigrationsIdempotently(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "test.db")

	sqlDB, err := Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("first open: %v", err)
	}
	var value string
	if err := sqlDB.QueryRow(`SELECT value FROM app_meta WHERE key = 'schema_initialized_at_utc'`).Scan(&value); err != nil {
		t.Fatalf("query app_meta: %v", err)
	}
	if value == "" {
		t.Error("expected schema_initialized_at_utc to be set")
	}
	sqlDB.Close()

	// Re-opening the same database must not fail or duplicate the migration.
	sqlDB2, err := Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("second open: %v", err)
	}
	defer sqlDB2.Close()

	var afterFirstOpen int
	if err := sqlDB2.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&afterFirstOpen); err != nil {
		t.Fatalf("count migrations: %v", err)
	}
	if afterFirstOpen == 0 {
		t.Fatal("expected at least one migration to be recorded")
	}

	// A third open must not re-apply anything: the count must be
	// unchanged from the second open.
	sqlDB3, err := Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("third open: %v", err)
	}
	defer sqlDB3.Close()

	var afterThirdOpen int
	if err := sqlDB3.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&afterThirdOpen); err != nil {
		t.Fatalf("count migrations: %v", err)
	}
	if afterThirdOpen != afterFirstOpen {
		t.Errorf("schema_migrations count changed across idempotent re-opens: %d -> %d", afterFirstOpen, afterThirdOpen)
	}
}

func TestOpen_RejectsUnknownDriver(t *testing.T) {
	if _, err := Open("mysql", ""); err == nil {
		t.Fatal("expected error for unknown driver, got nil")
	}
}
