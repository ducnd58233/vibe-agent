package harness

import (
	"context"
	"encoding/json"
	"fmt"
	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
	"strings"

	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/infra/agentstate"
)

// Some hosts never learn which node a run is at.
//
// Most inject on every prompt. A host whose prompt-submit event can only
// validate or block a prompt (Dialect.PromptInjection is false) is deliberately
// sent nothing there rather than interrupted to deliver a status line. The
// consequence went unrecorded for a long time, and commands/goal.md still
// claimed injection happened "on every prompt", which was true only for the
// hosts that can.
//
// A tool call's outcome is the way in: that reply already carries context for
// the host when a guard has something to say, so the node travels on the back of
// an event the host does honour (Dialect.ToolUseNodeReminder).
//
// Once per change, not once per tool call. A run at the same node for twenty
// edits has nothing new to report, and repeating it twenty times trains a
// reader to skip exactly the line that matters when it does change.

// nodeReminderState remembers what was last said, so the next call can tell
// whether anything is new.
//
// In memory.db's agent_state table because it is derived, disposable, and
// rebuildable: losing it costs one redundant reminder.
type nodeReminderState struct {
	Slug string `json:"slug"`
	Node string `json:"node"`
}

// nodeReminder returns the line to append to a tool-call reply, or "" when
// there is nothing new.
//
// Every failure path returns "": this is a convenience on top of a hook that
// must not fail a session, and a workspace that cannot write the marker is
// better served by a repeated reminder than by an error.
func nodeReminder(req Request) string {
	runs := state.Active(req.WorkspaceRoot)
	if len(runs) != 1 {
		// With none there is nothing to report. With several, naming one would
		// be choosing which goal the person is working on, which is the same
		// judgement session-start declines to make.
		return ""
	}
	run := runs[0]

	current := nodeReminderState{Slug: run.Slug, Node: run.CurrentNode}
	if readNodeReminder(req.WorkspaceRoot) == current {
		return ""
	}
	if !writeNodeReminder(req.WorkspaceRoot, current) {
		return ""
	}
	return fmt.Sprintf(
		"Run %s is at node %s. This host receives no prompt-time injection, so this arrives here instead. "+
			"Ask the runtime for run state; do not infer or manually advance it.",
		run.Slug, orNotEntered(run.CurrentNode))
}

// joinNonEmpty joins the parts that have something to say.
//
// A host reads one context field, so guard advice and the node reminder share
// it. Blank-joining them would leave a leading newline whenever only one is
// present, which reads as a truncated message.
func joinNonEmpty(parts ...string) string {
	var kept []string
	for _, part := range parts {
		if part != "" {
			kept = append(kept, part)
		}
	}
	return strings.Join(kept, "\n")
}

const (
	nodeReminderNamespace = "node_reminder"
	nodeReminderKey       = "last_node"
)

func readNodeReminder(workspaceRoot string) nodeReminderState {
	var state nodeReminderState
	raw, ok, err := agentstate.Get(context.Background(), workspaceRoot, nodeReminderNamespace, nodeReminderKey)
	if err != nil || !ok {
		return state
	}
	_ = json.Unmarshal([]byte(raw), &state)
	return state
}

// writeNodeReminder records what was just said. It reports whether the write
// succeeded, because announcing a node this package could not remember would
// announce it again on the next tool call, and every one after that.
func writeNodeReminder(workspaceRoot string, state nodeReminderState) bool {
	encoded, err := json.Marshal(state)
	if err != nil {
		return false
	}
	if err := agentstate.Set(context.Background(), workspaceRoot,
		nodeReminderNamespace, nodeReminderKey, string(encoded)); err != nil {
		return false
	}
	return true
}
