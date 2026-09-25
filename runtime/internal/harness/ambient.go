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

// The journal used to record nothing unless a run was in flight, and only
// /goal ever starts a run. Every other command therefore ran with the control
// plane awake and mute: hooks fired, returned nothing, and the memory store
// stayed empty however long the work went on. The next session then began with
// exactly what the last one began with, which is the state this whole design
// exists to avoid.
//
// The fix is a second destination rather than a second policy. An entry outside
// a run is the same entry; what it lacks is a run to belong to. So it goes to
// the workspace, next to the memory database that reads it, and everything else
// about journalling stays where it was.
//
// Refusal deliberately did not move with it. stop and the pre-tool gate still
// require an active run, so this adds a record and no new way for a session to
// be blocked.

// journalTable is the shared row store for tool-use events, both a run's own
// (a later task's job - see runs/run_events in the migration SPEC) and the
// ambient case this file writes: run_id is empty for an entry outside any run.
//
// Beside memory.db under .agent-state/ rather than under tmp/, because the two
// directories mean different things: tmp/ holds a run's evidence, which a person
// reads and a run owns, and .agent-state/ holds what the workspace derives and
// can rebuild. An entry belonging to no run belongs to the second.
const journalTable = "journal_entries"

func createJournalTable(ctx context.Context, db *sql.DB) error {
	return database.CreateTableWithProvenance(ctx, db, journalTable, `
        id      INTEGER PRIMARY KEY AUTOINCREMENT,
        run_id  TEXT,
        type    TEXT NOT NULL,
        node    TEXT NOT NULL DEFAULT '',
        at      TEXT NOT NULL,
        payload TEXT NOT NULL DEFAULT ''`)
}

// openJournalDB opens memory.db (creating its directory and journal_entries if
// needed) for the two callers that write to it directly: the live ambient path
// here and the one-time backfill. Shared so the four-step open sequence has one
// place to change rather than two copies that can drift.
func openJournalDB(ctx context.Context, workspaceRoot string) (*sql.DB, error) {
	path := workspace.MemoryDBPath(workspaceRoot)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, err
	}
	db, err := database.Open(ctx, path)
	if err != nil {
		return nil, err
	}
	if err := createJournalTable(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

// ambientJournal records one entry outside any run and returns the reference a
// memory can cite, or "" when nothing was written.
//
// The reference names this table and the row's id rather than a byte offset in
// a file, since there is no file any more. A memory citing the wrong place is
// worse than one citing none: it points a reader at something that does not
// contain the line.
//
// Every failure below is real (returned, not logged and dropped); this is the
// one place that turns "real" into "silent", because a hook that fails a tool
// call over its own bookkeeping is worse than one that records nothing.
func ambientJournal(workspaceRoot string, entry []byte) string {
	ref, err := insertAmbientJournalRow(workspaceRoot, entry)
	if err != nil {
		// Bookkeeping never fails a session; see the doc comment above.
		return ""
	}
	return ref
}

func insertAmbientJournalRow(workspaceRoot string, entry []byte) (string, error) {
	ctx := context.Background()
	db, err := openJournalDB(ctx, workspaceRoot)
	if err != nil {
		return "", err
	}
	defer func() { _ = db.Close() }()

	now := time.Now().UTC().Format(time.RFC3339)
	result, err := db.ExecContext(ctx, `
        INSERT INTO journal_entries (run_id, type, node, at, payload, created_by, created_at, updated_at)
        VALUES (NULL, ?, '', ?, ?, '', ?, ?)`,
		string(state.EventToolUse), now, string(entry), now, now)
	if err != nil {
		return "", err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s#%d", journalTable, id), nil
}
