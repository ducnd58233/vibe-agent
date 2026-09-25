package database

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
)

// tableName matches a safe SQL identifier: lowercase letters, digits, and
// underscores, not starting with a digit. name is interpolated into the
// statement directly (SQL has no placeholder for an identifier), so this is
// what keeps a future caller's mistake - a name built from anything other
// than a fixed string in code - from reaching the database as part of a
// query instead of failing closed here.
var tableName = regexp.MustCompile(`^[a-z_][a-z0-9_]*$`)

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
// from a file, an argument, or any other input outside this binary. Rejected
// if it does not look like one, rather than trusting every caller to remember.
func CreateTableWithProvenance(ctx context.Context, db *sql.DB, name, ownColumns string) error {
	if !tableName.MatchString(name) {
		return fmt.Errorf("table name %q is not a safe SQL identifier", name)
	}
	stmt := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (%s,%s)", name, ownColumns, ProvenanceColumns)
	if _, err := db.ExecContext(ctx, stmt); err != nil {
		return fmt.Errorf("create table %s: %w", name, err)
	}
	return nil
}
