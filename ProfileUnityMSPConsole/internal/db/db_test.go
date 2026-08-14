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

	var count int
	if err := sqlDB2.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&count); err != nil {
		t.Fatalf("count migrations: %v", err)
	}
	if count != 1 {
		t.Errorf("schema_migrations count = %d, want 1", count)
	}
}

func TestOpen_RejectsUnknownDriver(t *testing.T) {
	if _, err := Open("mysql", ""); err == nil {
		t.Fatal("expected error for unknown driver, got nil")
	}
}
