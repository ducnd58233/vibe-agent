package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

const Driver = "sqlite"

// BusyTimeout is how long a connection waits on a lock before giving up. Run
// state is written on every graph transition and shares this file with memory
// search; without this a second writer fails immediately with SQLITE_BUSY
// instead of waiting the fraction of a second the first write usually takes.
const BusyTimeout = 5 * time.Second

// Open opens a SQLite database at path, with WAL journaling and a busy
// timeout so concurrent writers wait for a lock instead of failing on it.
// Both are no-ops SQLite already handles gracefully for ":memory:".
// Pending embedded migrations under runtime/migrations are applied before
// the connection is returned so callers never CREATE TABLE themselves.
func Open(ctx context.Context, path string) (*sql.DB, error) {
	db, err := sql.Open(Driver, path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}
	if _, err := db.ExecContext(ctx, fmt.Sprintf("PRAGMA busy_timeout = %d", BusyTimeout.Milliseconds())); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("set busy_timeout: %w", err)
	}
	if _, err := db.ExecContext(ctx, "PRAGMA journal_mode = WAL"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("set journal_mode: %w", err)
	}
	if err := Up(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}
