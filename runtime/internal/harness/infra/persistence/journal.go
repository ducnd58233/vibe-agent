package persistence

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/infra/database"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/workspace"
)

// Table is the SQLite name for ambient (and future run) journal rows.
const Table = "journal_entries"

// Open opens memory.db for journal reads and writes. Schema comes from
// runtime/migrations via database.Open; this package does not CREATE TABLE.
func Open(ctx context.Context, workspaceRoot string) (*sql.DB, error) {
	path := workspace.MemoryDBPath(workspaceRoot)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, err
	}
	return database.Open(ctx, path)
}

// InsertAmbient records one tool-use entry outside any run and returns a
// citation like journal_entries#N, or an error when the write fails.
func InsertAmbient(ctx context.Context, db *sql.DB, payload []byte) (string, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	result, err := db.ExecContext(ctx, `
        INSERT INTO journal_entries (run_id, type, node, at, payload, created_by, created_at, updated_at)
        VALUES (NULL, ?, '', ?, ?, '', ?, ?)`,
		string(state.EventToolUse), now, string(payload), now, now)
	if err != nil {
		return "", err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s#%d", Table, id), nil
}

// InsertLegacyEvent writes one pre-database journal line during backfill.
func InsertLegacyEvent(ctx context.Context, db *sql.DB, event state.Event, now string) error {
	at := event.At.UTC().Format(time.RFC3339)
	_, err := db.ExecContext(ctx, `
        INSERT INTO journal_entries (run_id, type, node, at, payload, created_by, created_at, updated_at)
        VALUES (NULL, ?, ?, ?, ?, '', ?, ?)`,
		string(event.Type), event.Node, at, string(event.Payload), now, now)
	return err
}
