package legacy

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/workspace"
)

// ambientJournalName is the tool-use journal written outside any run, before
// journal_entries.
const ambientJournalName = "journal.ndjson"

func ambientJournalPath(root string) string {
	return filepath.Join(workspace.StateDir(root), ambientJournalName)
}

var ambientJournalSource = Source{
	Name:    "ambient journal",
	Into:    "journal_entries",
	Pending: func(root string) bool { return exists(ambientJournalPath(root)) },
	Migrate: migrateAmbientJournal,
}

func migrateAmbientJournal(ctx context.Context, root string) (int, error) {
	path := ambientJournalPath(root)
	if !exists(path) {
		return 0, nil
	}
	events, err := readEventLog(path)
	if err != nil {
		return 0, setAside(path, fmt.Errorf("read %s: %w", path, err))
	}
	if len(events) == 0 {
		return 0, removeMoved(path)
	}
	db, err := openDB(ctx, root)
	if err != nil {
		return 0, err
	}
	defer func() { _ = db.Close() }()

	now := time.Now().UTC().Format(time.RFC3339)
	for i, event := range events {
		if _, err := db.ExecContext(ctx, `
            INSERT INTO journal_entries (run_id, type, node, at, payload, created_by, created_at, updated_at)
            VALUES (NULL, ?, ?, ?, ?, '', ?, ?)`,
			string(event.Type), event.Node, event.At.UTC().Format(time.RFC3339), string(event.Payload), now, now); err != nil {
			return i, fmt.Errorf("insert journal line %d: %w", event.Sequence, err)
		}
	}
	return len(events), removeMoved(path)
}

// sessionSource: session.ndjson, one per run directory plus one for the
// workspace, before session_events.
var sessionSource = Source{
	Name: "session logs",
	Into: "session_events",
	Pending: func(root string) bool {
		files, _ := sessionLogs(root)
		return len(files) > 0
	},
	Migrate: migrateSessions,
}

func sessionLogs(root string) ([]string, error) {
	files, err := findNamed(workspace.RunsDir(root), state.SessionLogName)
	if err != nil {
		return nil, err
	}
	ambient := filepath.Join(workspace.StateDir(root), state.SessionLogName)
	if exists(ambient) {
		files = append(files, ambient)
	}
	return files, nil
}

func migrateSessions(ctx context.Context, root string) (int, error) {
	files, err := sessionLogs(root)
	if err != nil {
		return 0, err
	}
	moved := 0
	for _, path := range files {
		events, err := readEventLog(path)
		if err != nil {
			return moved, setAside(path, fmt.Errorf("read %s: %w", path, err))
		}
		stored, err := state.ImportSessionEvents(ctx, path, events)
		moved += stored
		if err != nil {
			// A session.ndjson outside a versioned run directory is not one this
			// layout ever wrote; leave it where it is.
			fmt.Fprintf(os.Stderr, "migrate sessions: skip %s: %v\n", path, err)
			continue
		}
		if err := removeMoved(path); err != nil {
			return moved, err
		}
	}
	return moved, nil
}
