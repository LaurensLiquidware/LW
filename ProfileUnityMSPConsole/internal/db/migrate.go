package db

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"
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

// ExpectedMigrationFilenames returns every migration filename embedded in
// this binary, sorted the same way Migrate applies them. Exposed so a
// read-only consumer (the demo-database loader in OpenDemo) can compare
// against a file's applied schema_migrations without running Migrate.
func ExpectedMigrationFilenames() ([]string, error) {
	entries, err := fs.Glob(migrationsFS, "migrations/*.sql")
	if err != nil {
		return nil, fmt.Errorf("list embedded migrations: %w", err)
	}
	sort.Strings(entries)
	names := make([]string, len(entries))
	for i, path := range entries {
		names[i] = path[len("migrations/"):]
	}
	return names, nil
}

// SchemaMismatchError reports that a demo database's applied migrations
// don't match what this binary embeds -- the set-diff of migration
// filenames, since this repo tracks schema state by filename ledger, not a
// single version number (see schema_migrations above).
type SchemaMismatchError struct {
	Missing []string // this binary expects these, the file doesn't have them
	Extra   []string // the file has these, this binary doesn't embed them
}

func (e *SchemaMismatchError) Error() string {
	var b strings.Builder
	b.WriteString("schema mismatch")
	if len(e.Missing) > 0 {
		fmt.Fprintf(&b, "; missing: %s", strings.Join(e.Missing, ", "))
	}
	if len(e.Extra) > 0 {
		fmt.Fprintf(&b, "; unexpected: %s", strings.Join(e.Extra, ", "))
	}
	return b.String()
}

func diffMigrations(expected, applied []string) *SchemaMismatchError {
	expectedSet := make(map[string]bool, len(expected))
	for _, f := range expected {
		expectedSet[f] = true
	}
	appliedSet := make(map[string]bool, len(applied))
	for _, f := range applied {
		appliedSet[f] = true
	}

	var missing, extra []string
	for _, f := range expected {
		if !appliedSet[f] {
			missing = append(missing, f)
		}
	}
	for _, f := range applied {
		if !expectedSet[f] {
			extra = append(extra, f)
		}
	}
	if len(missing) == 0 && len(extra) == 0 {
		return nil
	}
	sort.Strings(missing)
	sort.Strings(extra)
	return &SchemaMismatchError{Missing: missing, Extra: extra}
}

// OpenDemo opens dsn as a read-write sqlite connection *without* applying
// migrations, and verifies its schema_migrations ledger exactly matches
// what this binary expects. A demo database is never migrated -- it's a
// disposable, regenerable artifact (see cmd/gendemodb), and silently
// upgrading it in place is how it would stop being reproducible. Any
// mismatch, or a missing/unreadable schema_migrations table (not a valid
// demo database, or corrupt), fails loudly rather than falling back to the
// real database -- callers must not treat an error here as "use dsn
// anyway."
func OpenDemo(dsn string) (*sql.DB, error) {
	sqlDB, err := sql.Open("sqlite", withPragmas(dsn))
	if err != nil {
		return nil, fmt.Errorf("open demo database %q: %w", dsn, err)
	}
	if err := sqlDB.Ping(); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("ping demo database %q: %w", dsn, err)
	}

	expected, err := ExpectedMigrationFilenames()
	if err != nil {
		sqlDB.Close()
		return nil, err
	}

	rows, err := sqlDB.Query(`SELECT filename FROM schema_migrations`)
	if err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("demo database %q has no readable schema_migrations table (not a valid demo database, or corrupt): %w", dsn, err)
	}
	var applied []string
	for rows.Next() {
		var f string
		if err := rows.Scan(&f); err != nil {
			rows.Close()
			sqlDB.Close()
			return nil, fmt.Errorf("read schema_migrations from demo database %q: %w", dsn, err)
		}
		applied = append(applied, f)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		sqlDB.Close()
		return nil, fmt.Errorf("read schema_migrations from demo database %q: %w", dsn, err)
	}
	rows.Close()

	if mismatch := diffMigrations(expected, applied); mismatch != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("demo database %q: %w -- regenerate it with cmd/gendemodb or remove the file", dsn, mismatch)
	}
	return sqlDB, nil
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
