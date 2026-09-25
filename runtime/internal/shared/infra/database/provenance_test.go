package database

import (
	"context"
	"testing"
)

// Every domain module's table gets an author, a reviewer list, and both
// timestamps without hand-rolling the DDL, so a new table cannot forget one.
func TestCreateTableWithProvenanceAddsAllFourColumns(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close: %v", err)
		}
	})

	if err := CreateTableWithProvenance(ctx, db, "widgets", `
        id   TEXT PRIMARY KEY,
        name TEXT NOT NULL`); err != nil {
		t.Fatalf("create table: %v", err)
	}

	rows, err := db.QueryContext(ctx, `PRAGMA table_info(widgets)`)
	if err != nil {
		t.Fatalf("inspect schema: %v", err)
	}
	defer func() { _ = rows.Close() }()

	present := map[string]bool{}
	for rows.Next() {
		var (
			index      int
			name       string
			columnType string
			notNull    int
			preset     any
			primary    int
		)
		if err := rows.Scan(&index, &name, &columnType, &notNull, &preset, &primary); err != nil {
			t.Fatalf("read column: %v", err)
		}
		present[name] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("read schema: %v", err)
	}

	for _, want := range []string{"id", "name", "created_by", "reviewed_by_agents", "created_at", "updated_at"} {
		if !present[want] {
			t.Errorf("widgets has no column %q", want)
		}
	}
}

// A second call is what every caller does on every open; it must not fail
// against a table that already exists.
func TestCreateTableWithProvenanceIsIdempotent(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close: %v", err)
		}
	})

	for i := range 2 {
		if err := CreateTableWithProvenance(ctx, db, "widgets", `id TEXT PRIMARY KEY`); err != nil {
			t.Fatalf("create table (pass %d): %v", i, err)
		}
	}
}
