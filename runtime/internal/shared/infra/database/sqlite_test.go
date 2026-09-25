package database

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestOpenInMemory(t *testing.T) {
	db, err := Open(context.Background(), ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	if err := db.PingContext(context.Background()); err != nil {
		t.Fatal(err)
	}
}

// A file-backed database gets WAL and a busy_timeout so a second connection
// waits for a lock instead of failing the moment one is held. This is what
// makes it safe for run state, written on every graph transition, to share
// this file with memory search.
func TestOpenSetsWALAndBusyTimeoutOnAFileDatabase(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "provenance.db")
	db, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close: %v", err)
		}
	})

	var mode string
	if err := db.QueryRowContext(ctx, `PRAGMA journal_mode`).Scan(&mode); err != nil {
		t.Fatalf("read journal_mode: %v", err)
	}
	if mode != "wal" {
		t.Errorf("journal_mode = %q, want wal", mode)
	}

	var timeoutMS int
	if err := db.QueryRowContext(ctx, `PRAGMA busy_timeout`).Scan(&timeoutMS); err != nil {
		t.Fatalf("read busy_timeout: %v", err)
	}
	if timeoutMS <= 0 {
		t.Errorf("busy_timeout = %d, want a positive value", timeoutMS)
	}
}

// A second connection holding a write lock must make the first one wait, not
// fail. Without WAL and a busy_timeout this returns "database is locked"
// immediately; the run-state writer at every graph transition would then race
// memory search for the same file.
func TestOpenLetsAConcurrentWriterWaitInsteadOfFailing(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "concurrent.db")

	first, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := first.Close(); err != nil {
			t.Errorf("close first: %v", err)
		}
	})
	if _, err := first.ExecContext(ctx, `CREATE TABLE rows (id INTEGER PRIMARY KEY)`); err != nil {
		t.Fatalf("create table: %v", err)
	}

	second, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := second.Close(); err != nil {
			t.Errorf("close second: %v", err)
		}
	})

	tx, err := first.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO rows DEFAULT VALUES`); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	wg.Add(1)
	writeErr := make(chan error, 1)
	go func() {
		defer wg.Done()
		_, err := second.ExecContext(ctx, `INSERT INTO rows DEFAULT VALUES`)
		writeErr <- err
	}()

	// Give the second connection time to actually attempt the write and start
	// waiting on the lock before this releases it.
	time.Sleep(50 * time.Millisecond)
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	wg.Wait()

	if err := <-writeErr; err != nil {
		t.Errorf("concurrent writer was refused instead of waiting: %v", err)
	}
}
