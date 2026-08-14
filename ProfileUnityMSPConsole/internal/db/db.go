// Package db owns the SQLite/Postgres connection and schema migrations.
package db

import (
	"database/sql"
	"fmt"
	"strings"

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
		// SQLite allows only one writer at a time; the scheduler polls
		// tenants concurrently and each writes its own snapshot, so
		// without a busy timeout concurrent writers get SQLITE_BUSY
		// immediately instead of waiting their turn. WAL mode lets reads
		// proceed alongside a writer. _pragma is per-connection, applied
		// by the driver on every new pooled connection, not just the first.
		sqlDB, err := sql.Open("sqlite", withPragmas(dsn))
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

func withPragmas(dsn string) string {
	pragmas := "_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"
	if strings.Contains(dsn, "?") {
		return dsn + "&" + pragmas
	}
	return dsn + "?" + pragmas
}
