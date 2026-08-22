package harness

import (
	"fmt"
	"path/filepath"

	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
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

// ambientJournalName is the workspace-level log for tool use outside any run.
//
// Beside memory.db under .agent-state/ rather than under tmp/, because the two
// directories mean different things: tmp/ holds a run's evidence, which a person
// reads and a run owns, and .agent-state/ holds what the workspace derives and
// can rebuild. An entry belonging to no run belongs to the second.
const ambientJournalName = "journal.ndjson"

// ambientJournalPath is the log's location for a workspace.
func ambientJournalPath(workspaceRoot string) string {
	return filepath.Join(workspace.StateDir(workspaceRoot), ambientJournalName)
}

// ambientJournal records one entry outside any run and returns the reference a
// memory can cite, or "" when nothing was written.
//
// The reference is built here rather than taken from Event.Ref, which names
// events.ndjson unconditionally. A memory citing the wrong file is worse than
// one citing none: it points a reader at a log that does not contain the line.
func ambientJournal(workspaceRoot string, entry []byte) string {
	recorded, err := state.AppendRunEvent(ambientJournalPath(workspaceRoot), state.Event{
		Type:    state.EventToolUse,
		Payload: entry,
	})
	if err != nil {
		// Same rule as the run path: bookkeeping never fails a session.
		return ""
	}
	return fmt.Sprintf("%s#%d", ambientJournalName, recorded.Sequence)
}
