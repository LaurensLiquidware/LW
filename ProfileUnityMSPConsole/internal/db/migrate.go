package db

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"sort"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

const migrationsTable = `
CREATE TABLE IF NOT EXISTS schema_migrations (
	filename    TEXT PRIMARY KEY,
	applied_at  TEXT NOT NULL
)`

// Migrate applies every embedded migration that has not yet been recorded
// as applied, in filename order, each inside its own transaction. It is
// idempotent: re-running it after a partial or previous run only applies
// what is missing.
func Migrate(sqlDB *sql.DB) error {
	if _, err := sqlDB.Exec(migrationsTable); err != nil {
		return fmt.Errorf("create schema_migrations table: %w", err)
	}

	entries, err := fs.Glob(migrationsFS, "migrations/*.sql")
	if err != nil {
		return fmt.Errorf("list embedded migrations: %w", err)
	}
	sort.Strings(entries)

	for _, path := range entries {
		filename := path[len("migrations/"):]

		var already bool
		row := sqlDB.QueryRow(`SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE filename = ?)`, filename)
		if err := row.Scan(&already); err != nil {
			return fmt.Errorf("check migration %s: %w", filename, err)
		}
		if already {
			continue
		}

		contents, err := migrationsFS.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", filename, err)
		}

		if err := applyMigration(sqlDB, filename, string(contents)); err != nil {
			return fmt.Errorf("apply migration %s: %w", filename, err)
		}
	}
	return nil
}

func applyMigration(sqlDB *sql.DB, filename, sqlText string) error {
	tx, err := sqlDB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(sqlText); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO schema_migrations (filename, applied_at) VALUES (?, strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))`, filename); err != nil {
		return err
	}
	return tx.Commit()
}
