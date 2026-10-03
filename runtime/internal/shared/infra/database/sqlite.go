package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
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
//
// The busy timeout and immediate transactions are DSN parameters so every
// pooled connection gets them, not only the one a PRAGMA happened to run on.
// Immediate matters: a deferred transaction reads first and upgrades to a
// write lock later, and SQLite answers that upgrade with SQLITE_BUSY at once,
// without waiting, because two readers waiting on each other would deadlock.
// Taking the write lock at BEGIN lets the busy timeout do its job.
func Open(ctx context.Context, path string) (*sql.DB, error) {
	db, err := sql.Open(Driver, dsn(path))
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}
	if err := retryBusy(ctx, func() error { return useWAL(ctx, db) }); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("set journal_mode: %w", err)
	}
	if err := Up(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func dsn(path string) string {
	sep := "?"
	if strings.Contains(path, "?") {
		sep = "&"
	}
	return fmt.Sprintf("%s%s_pragma=busy_timeout(%d)&_txlock=immediate", path, sep, BusyTimeout.Milliseconds())
}

// useWAL switches the file to WAL once. The mode is stored in the file, so a
// database that already has it needs no write lock to be told again.
func useWAL(ctx context.Context, db *sql.DB) error {
	var mode string
	if err := db.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&mode); err != nil {
		return err
	}
	if strings.EqualFold(mode, "wal") || strings.EqualFold(mode, "memory") {
		return nil
	}
	_, err := db.ExecContext(ctx, "PRAGMA journal_mode = WAL")
	return err
}

// retryBusy repeats fn while SQLite reports the file locked without having
// waited (the journal-mode switch, a lock upgrade), up to BusyTimeout.
func retryBusy(ctx context.Context, fn func() error) error {
	deadline := time.Now().Add(BusyTimeout)
	for {
		err := fn()
		if err == nil || !isBusy(err) || time.Now().After(deadline) {
			return err
		}
		select {
		case <-ctx.Done():
			return err
		case <-time.After(busyBackoff):
		}
	}
}

const busyBackoff = 20 * time.Millisecond

func isBusy(err error) bool {
	var serr *sqlite.Error
	if !errors.As(err, &serr) {
		return false
	}
	code := serr.Code() & 0xff
	return code == sqlite3.SQLITE_BUSY || code == sqlite3.SQLITE_LOCKED
}
