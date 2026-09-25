package harness

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

// legacyAmbientJournalName is the pre-database filename ambientJournal wrote
// to before this task, kept only so a one-time backfill can find and remove it.
const legacyAmbientJournalName = "journal.ndjson"

func legacyAmbientJournalPath(workspaceRoot string) string {
	return filepath.Join(workspace.StateDir(workspaceRoot), legacyAmbientJournalName)
}

// AmbientJournalBackfill moves every existing line of the legacy ambient
// journal file into journal_entries (run_id NULL, matching what ambientJournal
// itself would have written), then removes the file. Safe to run more than
// once: a workspace with nothing left at the legacy path migrates zero entries
// rather than erroring.
func AmbientJournalBackfill(ctx context.Context, workspaceRoot string) (int, error) {
	path := legacyAmbientJournalPath(workspaceRoot)
	events, err := state.ReadEvents(path)
	if err != nil {
		return 0, fmt.Errorf("read %s: %w", path, err)
	}
	if events == nil {
		if _, statErr := os.Stat(path); statErr != nil {
			return 0, nil
		}
	}

	dbPath := workspace.MemoryDBPath(workspaceRoot)
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o750); err != nil {
		return 0, fmt.Errorf("create state directory: %w", err)
	}
	db, err := database.Open(ctx, dbPath)
	if err != nil {
		return 0, fmt.Errorf("open journal: %w", err)
	}
	defer func() { _ = db.Close() }()
	if err := createJournalTable(ctx, db); err != nil {
		return 0, err
	}

	migrated, err := insertLegacyAmbientEvents(ctx, db, events)
	if err != nil {
		return migrated, err
	}
	if err := os.Remove(path); err != nil {
		return migrated, fmt.Errorf("remove %s: %w", path, err)
	}
	return migrated, nil
}

func insertLegacyAmbientEvents(ctx context.Context, db *sql.DB, events []state.Event) (int, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	migrated := 0
	for _, event := range events {
		at := event.At.UTC().Format(time.RFC3339)
		if _, err := db.ExecContext(ctx, `
            INSERT INTO journal_entries (run_id, type, node, at, payload, created_by, created_at, updated_at)
            VALUES (NULL, ?, ?, ?, ?, '', ?, ?)`,
			string(event.Type), event.Node, at, string(event.Payload), now, now); err != nil {
			return migrated, fmt.Errorf("insert journal_entries row %d: %w", event.Sequence, err)
		}
		migrated++
	}
	return migrated, nil
}
