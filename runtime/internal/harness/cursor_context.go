package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/infra/agentstate"
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

// cursorNodeState remembers what was last said, so the next call can tell
// whether anything is new.
//
// In memory.db's agent_state table because it is derived, disposable, and
// rebuildable: losing it costs one redundant reminder.
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
		run.Slug, orNotEntered(run.CurrentNode))
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

// legacyCursorNodeFile is the file this state lived in before agent_state.
// Disposable, so it is removed on first write rather than migrated.
const legacyCursorNodeFile = "cursor-node.json"

const (
	cursorStateNamespace = "cursor"
	cursorStateKey       = "last_node"
)

func readCursorNode(workspaceRoot string) cursorNodeState {
	var state cursorNodeState
	raw, ok, err := agentstate.Get(context.Background(), workspaceRoot, cursorStateNamespace, cursorStateKey)
	if err != nil || !ok {
		return state
	}
	_ = json.Unmarshal([]byte(raw), &state)
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
	if err := agentstate.Set(context.Background(), workspaceRoot,
		cursorStateNamespace, cursorStateKey, string(encoded)); err != nil {
		return false
	}
	_ = os.Remove(filepath.Join(workspace.StateDir(workspaceRoot), legacyCursorNodeFile))
	return true
}
