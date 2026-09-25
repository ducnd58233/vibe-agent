package database

import (
	"context"
	"database/sql"
	"fmt"
)

// ProvenanceColumns is the column DDL every state table in this database
// carries, matching memories' own shape: who wrote a row, who has reviewed
// it, and when.
const ProvenanceColumns = `
    created_by TEXT NOT NULL DEFAULT '',
    reviewed_by_agents TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL`

// CreateTableWithProvenance creates a table if it does not already exist,
// with ownColumns first and the four provenance columns appended after.
//
// A new table gets provenance from the start: there is no "add it later"
// migration the way memories needed, because a brand-new table has no
// pre-provenance rows to backfill.
//
// name is not a query parameter - SQL has no placeholder for an identifier -
// so it must be a fixed string a caller writes in code, never a value derived
// from a file, an argument, or any other input outside this binary.
func CreateTableWithProvenance(ctx context.Context, db *sql.DB, name, ownColumns string) error {
	stmt := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (%s,%s)", name, ownColumns, ProvenanceColumns)
	if _, err := db.ExecContext(ctx, stmt); err != nil {
		return fmt.Errorf("create table %s: %w", name, err)
	}
	return nil
}
