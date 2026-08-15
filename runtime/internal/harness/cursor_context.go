package harness

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/workspace"
)

// Cursor is the only host that never learns which node a run is at.
//
// The other three inject on every prompt. Cursor cannot: beforeSubmitPrompt
// returns {continue, user_message} and can validate or block a prompt, nothing
// more, so the runtime deliberately sends it nothing rather than interrupting
// someone to deliver a status line. The consequence went unrecorded for a long
// time, and commands/goal.md still claimed injection happened "on every
// prompt", which was true on three hosts out of four.
//
// postToolUse is the way in. Cursor documents it reading additional_context,
// and this package already writes that field there when a guard has something
// to say. So the node travels on the back of an event Cursor does honour.
//
// Once per change, not once per tool call. A run at the same node for twenty
// edits has nothing new to report, and repeating it twenty times trains a
// reader to skip exactly the line that matters when it does change.

// cursorNodeFile remembers what was last said, so the next call can tell
// whether anything is new.
//
// In .agent-state/ because it is derived, disposable, and rebuildable: losing
// it costs one redundant reminder, which is why it does not belong in tmp/
// beside evidence a person reads.
const cursorNodeFile = "cursor-node.json"

type cursorNodeState struct {
	Slug string `json:"slug"`
	Node string `json:"node"`
}

// cursorNodeReminder returns the line to append for Cursor, or "" when there is
// nothing new.
//
// Every failure path returns "": this is a convenience on top of a hook that
// must not fail a session, and a workspace that cannot write the marker is
// better served by a repeated reminder than by an error.
func cursorNodeReminder(req Request) string {
	if req.Client != ClientCursor {
		return ""
	}
	runs := activeRuns(req.WorkspaceRoot)
	if len(runs) != 1 {
		// With none there is nothing to report. With several, naming one would
		// be choosing which goal the person is working on, which is the same
		// judgement session-start declines to make.
		return ""
	}
	run := runs[0]

	current := cursorNodeState{Slug: run.Slug, Node: run.CurrentNode}
	if readCursorNode(req.WorkspaceRoot) == current {
		return ""
	}
	if !writeCursorNode(req.WorkspaceRoot, current) {
		return ""
	}
	return fmt.Sprintf(
		"Run %s is at node %s. Cursor receives no prompt-time injection, so this arrives here instead. "+
			"Ask the runtime for run state; do not infer or manually advance it.",
		run.Slug, orDash(run.CurrentNode))
}

// joinNonEmpty joins the parts that have something to say.
//
// Cursor reads one additional_context field, so guard advice and the node
// reminder share it. Blank-joining them would leave a leading newline whenever
// only one is present, which reads as a truncated message.
func joinNonEmpty(parts ...string) string {
	var kept []string
	for _, part := range parts {
		if part != "" {
			kept = append(kept, part)
		}
	}
	return strings.Join(kept, "\n")
}

func cursorNodePath(workspaceRoot string) string {
	return filepath.Join(workspace.StateDir(workspaceRoot), cursorNodeFile)
}

func readCursorNode(workspaceRoot string) cursorNodeState {
	var state cursorNodeState
	raw, err := os.ReadFile(filepath.Clean(cursorNodePath(workspaceRoot)))
	if err != nil {
		return state
	}
	_ = json.Unmarshal(raw, &state)
	return state
}

// writeCursorNode records what was just said. It reports whether the write
// succeeded, because announcing a node this package could not remember would
// announce it again on the next tool call, and every one after that.
func writeCursorNode(workspaceRoot string, state cursorNodeState) bool {
	encoded, err := json.Marshal(state)
	if err != nil {
		return false
	}
	if err := os.MkdirAll(workspace.StateDir(workspaceRoot), 0o750); err != nil {
		return false
	}
	return os.WriteFile(cursorNodePath(workspaceRoot), encoded, 0o600) == nil
}
