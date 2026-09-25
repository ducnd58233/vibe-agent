package harness

import (
	"context"

	"github.com/ducnd58233/vibe-agent/runtime/internal/harness/infra/persistence"
)

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

// journalTable is the citation prefix ambient journal SourceRefs use.
const journalTable = persistence.Table

func insertAmbientJournalRow(workspaceRoot string, entry []byte) (string, error) {
	ctx := context.Background()
	db, err := persistence.Open(ctx, workspaceRoot)
	if err != nil {
		return "", err
	}
	defer func() { _ = db.Close() }()
	return persistence.InsertAmbient(ctx, db, entry)
}
