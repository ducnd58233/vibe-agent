package database

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestOpenAppliesBaselineMigrations(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "memory.db")
	db, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	for _, table := range []string{
		"runs", "run_events", "task_lists", "fetch_cache",
		"journal_entries", "sdd_cache", "memories",
	} {
		var n int
		if err := db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&n); err != nil {
			t.Fatalf("%s: %v", table, err)
		}
		if n != 1 {
			t.Errorf("table %s missing after Open", table)
		}
	}

	second, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("second Open: %v", err)
	}
	_ = second.Close()
}

func TestOpenMigratesPreMigrateDatabaseWithoutLosingTables(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "legacy.db")

	raw, err := sql.Open(Driver, path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.ExecContext(ctx, `CREATE TABLE memories (
        id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, kind TEXT NOT NULL,
        content TEXT NOT NULL, tags TEXT NOT NULL DEFAULT '',
        confidence REAL NOT NULL, status TEXT NOT NULL,
        source_type TEXT NOT NULL, source_ref TEXT, evidence TEXT NOT NULL,
        supersedes_id TEXT, used_count INTEGER NOT NULL DEFAULT 0,
        expires_at TEXT, valid_from TEXT NOT NULL DEFAULT '', valid_to TEXT,
        created_by TEXT NOT NULL DEFAULT '', reviewed_by_agents TEXT NOT NULL DEFAULT '',
        created_at TEXT NOT NULL, updated_at TEXT NOT NULL)`); err != nil {
		_ = raw.Close()
		t.Fatal(err)
	}
	if err := raw.Close(); err != nil {
		t.Fatal(err)
	}

	db, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("Open legacy DB: %v", err)
	}
	defer func() { _ = db.Close() }()

	var version uint
	var dirty bool
	if err := db.QueryRowContext(ctx, `SELECT version, dirty FROM schema_migrations`).Scan(&version, &dirty); err != nil {
		t.Fatal(err)
	}
	if version != 1 || dirty {
		t.Errorf("schema_migrations = %d dirty=%v, want 1 false", version, dirty)
	}
	var runs int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='runs'`).Scan(&runs); err != nil {
		t.Fatal(err)
	}
	if runs != 1 {
		t.Error("pre-migrate Open did not create the rest of the baseline tables")
	}
}
