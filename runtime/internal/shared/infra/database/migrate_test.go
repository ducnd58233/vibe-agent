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
		"runs", "run_events", "run_checks", "task_lists", "fetch_cache",
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
	if version != 4 || dirty {
		t.Errorf("schema_migrations = %d dirty=%v, want 4 false", version, dirty)
	}
	var runs int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='runs'`).Scan(&runs); err != nil {
		t.Fatal(err)
	}
	if runs != 1 {
		t.Error("pre-migrate Open did not create the rest of the baseline tables")
	}
	var checks int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='run_checks'`).Scan(&checks); err != nil {
		t.Fatal(err)
	}
	if checks != 1 {
		t.Error("pre-migrate Open did not apply run_checks migration")
	}
}

// An upgrade from version 2 must carry memories into the stemmed index and add
// the agent-memory tables without touching existing rows.
func TestUpgradeFromVersionTwoRebuildsTheMemoryIndex(t *testing.T) {
	ctx := t.Context()
	path := filepath.Join(t.TempDir(), "memory.db")

	first, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := first.ExecContext(ctx, `INSERT INTO memories (id, workspace_id, kind, content, tags, confidence,
        status, source_type, evidence, created_at, updated_at)
        VALUES ('m1', '.', 'semantic', 'the suite failed on redis', 'ci', 0.9, 'confirmed', 'command_result', 'x', 't', 't')`); err != nil {
		t.Fatal(err)
	}
	// Roll the ledger back one step and drop what version 3 added, as a
	// version-2 database would look.
	for _, stmt := range []string{
		`DROP TABLE memories_fts`,
		`CREATE VIRTUAL TABLE memories_fts USING fts5(memory_id UNINDEXED, content, tags)`,
		`INSERT INTO memories_fts (memory_id, content, tags) VALUES ('m1', 'the suite failed on redis', 'ci')`,
		`DROP TABLE session_events_fts`, `DROP TABLE session_events`, `DROP TABLE agent_state`,
		`DROP TABLE memory_links`, `DROP TABLE memory_events`,
		`UPDATE schema_migrations SET version = 2`,
	} {
		if _, err := first.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	_ = first.Close()

	db, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("upgrade: %v", err)
	}
	defer func() { _ = db.Close() }()

	var matched int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM memories_fts WHERE memories_fts MATCH 'failing'`).Scan(&matched); err != nil {
		t.Fatal(err)
	}
	if matched != 1 {
		t.Errorf("stemmed match count = %d, want 1 (porter tokenizer missing or index not repopulated)", matched)
	}
	for _, table := range []string{"session_events", "agent_state", "memory_links", "memory_events"} {
		var n int
		if err := db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&n); err != nil || n != 1 {
			t.Errorf("table %s missing after upgrade (%v)", table, err)
		}
	}
}
