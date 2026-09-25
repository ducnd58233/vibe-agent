package harness

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/harness/infra/persistence"
	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/workspace"
)

// legacyAmbientJournalName is the pre-database filename ambientJournal wrote
// to before this task, kept only so a one-time backfill can find and remove it.
const legacyAmbientJournalName = "journal.ndjson"

func legacyAmbientJournalPath(workspaceRoot string) string {
	return filepath.Join(workspace.StateDir(workspaceRoot), legacyAmbientJournalName)
}

// corruptSuffix marks a legacy journal this backfill could not parse. Renaming
// it out of the way, rather than leaving it in place, is what keeps a single
// bad line from failing every future `migrate state` run identically forever:
// the next run sees no file at the original path and reports zero migrated,
// and the renamed file stays on disk for a person to inspect and repair.
const corruptSuffix = ".corrupt"

// AmbientJournalBackfill moves every existing line of the legacy ambient
// journal file into journal_entries (run_id NULL, matching what ambientJournal
// itself would have written), then removes the file. Safe to run more than
// once: a workspace with nothing left at the legacy path migrates zero entries
// rather than erroring.
func AmbientJournalBackfill(ctx context.Context, workspaceRoot string) (int, error) {
	path := legacyAmbientJournalPath(workspaceRoot)
	events, err := state.ReadEvents(path)
	if err != nil {
		renamed := path + corruptSuffix
		if renameErr := os.Rename(path, renamed); renameErr == nil {
			return 0, fmt.Errorf("read %s: %w (renamed to %s for manual repair)", path, err, renamed)
		}
		return 0, fmt.Errorf("read %s: %w", path, err)
	}
	if len(events) == 0 {
		_ = os.Remove(path)
		return 0, nil
	}

	db, err := persistence.Open(ctx, workspaceRoot)
	if err != nil {
		return 0, fmt.Errorf("open journal: %w", err)
	}
	defer func() { _ = db.Close() }()

	now := time.Now().UTC().Format(time.RFC3339)
	migrated := 0
	for _, event := range events {
		if err := persistence.InsertLegacyEvent(ctx, db, event, now); err != nil {
			return migrated, fmt.Errorf("insert journal_entries row %d: %w", event.Sequence, err)
		}
		migrated++
	}
	if err := os.Remove(path); err != nil {
		return migrated, fmt.Errorf("remove %s: %w", path, err)
	}
	return migrated, nil
}
