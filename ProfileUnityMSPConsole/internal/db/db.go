// Package db owns the SQLite/Postgres connection and schema migrations.
package db

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// Open opens the configured database, applies any pending migrations, and
// returns a ready-to-use connection pool.
//
// Only the sqlite driver is implemented so far. Postgres is a planned
// configuration option (see project brief §5) but is not wired up yet —
// erroring clearly here beats silently connecting to the wrong thing.
func Open(driver, dsn string) (*sql.DB, error) {
	switch driver {
	case "sqlite":
		sqlDB, err := sql.Open("sqlite", dsn)
		if err != nil {
			return nil, fmt.Errorf("open sqlite database %q: %w", dsn, err)
		}
		if err := sqlDB.Ping(); err != nil {
			sqlDB.Close()
			return nil, fmt.Errorf("ping sqlite database %q: %w", dsn, err)
		}
		if err := Migrate(sqlDB); err != nil {
			sqlDB.Close()
			return nil, fmt.Errorf("migrate sqlite database %q: %w", dsn, err)
		}
		return sqlDB, nil
	case "postgres":
		return nil, fmt.Errorf("postgres support is not implemented yet")
	default:
		return nil, fmt.Errorf("unknown db driver %q", driver)
	}
}
