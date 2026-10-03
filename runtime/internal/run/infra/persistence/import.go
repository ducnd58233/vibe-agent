package persistence

import (
	"context"
	"fmt"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/run/domain"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/validate"
)

// ImportRun stores a run and its event log as given, replacing whatever the
// database held for that run. date and version place the revision when the run
// itself does not carry them. Only the database is written: this is how state
// produced elsewhere (an older build's files, a restore) becomes current state,
// and nothing about where it came from belongs in this package.
func ImportRun(ctx context.Context, workspaceRoot string, run *domain.Run, date string, version int, events []domain.Event) error {
	if err := run.Validate(); err != nil {
		return fmt.Errorf("refusing to import invalid run state: %w", err)
	}
	if !validate.Date(date) || version < 1 {
		return fmt.Errorf("import %s: date %q and version %d do not place a revision", run.Slug, date, version)
	}
	db, err := openDB(ctx, workspaceRoot)
	if err != nil {
		return fmt.Errorf("open runs database: %w", err)
	}
	defer func() { _ = db.Close() }()

	loc := runLocation{WorkspaceRoot: workspaceRoot, Date: date, Slug: run.Slug, Version: version}
	if err := upsertRun(ctx, db, run, loc); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `DELETE FROM run_events WHERE run_id = ?`, run.RunID); err != nil {
		return fmt.Errorf("clear run_events for %s: %w", run.RunID, err)
	}
	for _, event := range events {
		at := time.Now().UTC().Format(time.RFC3339)
		if !event.At.IsZero() {
			at = event.At.UTC().Format(time.RFC3339)
		}
		if _, err := db.ExecContext(ctx, `
            INSERT INTO run_events (
                run_id, sequence, type, node, at, payload,
                created_by, reviewed_by_agents, created_at, updated_at)
            VALUES (?, ?, ?, ?, ?, ?, '', '', ?, ?)`,
			run.RunID, event.Sequence, string(event.Type), event.Node, at, string(event.Payload), at, at); err != nil {
			return fmt.Errorf("insert event %d for %s: %w", event.Sequence, run.RunID, err)
		}
	}
	return nil
}
