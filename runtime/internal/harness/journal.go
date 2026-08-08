package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/memory"
	"github.com/ducnd58233/vibe-agent/runtime/internal/state"
)

// commandLimit keeps one journal entry to one line's worth of command. A
// heredoc or a generated script belongs in the transcript, not the event log.
const commandLimit = 500

// toolUse is what a PostToolUse hook records: the tool, what it was aimed at,
// and how it ended. Nothing here is interpreted; the log stores what happened.
type toolUse struct {
	Tool    string `json:"tool"`
	Command string `json:"command,omitempty"`
	File    string `json:"file,omitempty"`
	// ExitCode is a pointer so "the host did not report one" stays different
	// from "it exited 0".
	ExitCode *int `json:"exitCode,omitempty"`
}

// response is the subset of a tool result this package can read.
//
// Hosts disagree about the shape and change it between versions, so every
// field is optional and an unparseable response is simply one without an
// outcome. Guessing an exit code from result text would put an inference where
// this design only accepts evidence.
type response struct {
	ExitCode    *int   `json:"exit_code"`
	ExitCodeAlt *int   `json:"exitCode"`
	Stderr      string `json:"stderr"`
	Interrupted bool   `json:"interrupted"`
}

func (r response) exit() *int {
	if r.ExitCode != nil {
		return r.ExitCode
	}
	return r.ExitCodeAlt
}

// journal records tool use against every active run, and proposes a memory when
// a command reported a real failure.
//
// It fires after the tool ran, so it never refuses anything and never returns
// an error: a control plane that fails a session over its own bookkeeping is
// worse than one that records nothing.
func journal(req Request, body payload) error {
	if body.ToolName == "" {
		return nil
	}
	runs := activeRuns(req.WorkspaceRoot)
	if len(runs) == 0 {
		return nil
	}

	var result response
	if len(body.ToolResponse) > 0 {
		_ = json.Unmarshal(body.ToolResponse, &result)
	}

	command := truncate(body.shellCommand(), commandLimit)
	entry, err := json.Marshal(toolUse{
		Tool:     body.ToolName,
		Command:  command,
		File:     body.writeTarget(),
		ExitCode: result.exit(),
	})
	if err != nil {
		return nil
	}

	for _, run := range runs {
		recorded, err := state.AppendEvent(state.EventLogPath(req.WorkspaceRoot, run.Slug), state.Event{
			Type:    "tool_use",
			Node:    run.CurrentNode,
			Payload: entry,
		})
		if err != nil {
			continue
		}
		proposeFailure(req.WorkspaceRoot, run, command, result, recorded.Ref())
	}
	return nil
}

// FailureMemoryLife is how long a recorded command failure stays retrievable.
//
// "go build ./... exits 2" is true about a moment, not about the repository.
// Left permanent it becomes the stale memory the whole design warns about, so
// it retires itself instead of waiting to be contradicted.
const FailureMemoryLife = 7 * 24 * time.Hour

// proposeFailure turns a failed command into a memory, and confirms it.
//
// Confirming from inside a hook looks like it breaks this package's rule that
// only a verifier or a human promotes a memory. It does not, because the rule
// exists to stop model output from validating itself, and none of this came
// from a model: the host reported the exit code, the same provenance the run
// manifest accepts as CheckSource exit_code, and the event log already holds
// the entry this cites as its source.
//
// The alternative was leaving it proposed, which is what retrieval filters
// out. That is a memory the runtime writes and can never read.
//
// When the host does not report an exit code, nothing is written. An outcome
// read out of result text would be a guess wearing evidence's clothes.
func proposeFailure(workspaceRoot string, run *state.Run, command string, result response, ref string) {
	exit := result.exit()
	if exit == nil || *exit == 0 || command == "" || result.Interrupted {
		return
	}

	store, err := memory.Open(workspaceRoot)
	if err != nil {
		return
	}
	defer store.Close()

	evidence := []string{fmt.Sprintf("%s exited %d during run %s at node %s",
		command, *exit, run.Slug, orDash(run.CurrentNode))}
	if detail := truncate(singleLine(result.Stderr), 300); detail != "" {
		evidence = append(evidence, detail)
	}

	now := time.Now().UTC()
	expires := now.Add(FailureMemoryLife)
	ctx := context.Background()

	stored, decision, err := store.Propose(ctx, memory.Record{
		WorkspaceID: workspaceRoot,
		Kind:        memory.KindEpisodic,
		Content:     fmt.Sprintf("%s exits %d in this workspace", command, *exit),
		Tags:        []string{"command-failure", run.Slug},
		Confidence:  0.6,
		SourceType:  memory.SourceCommandResult,
		SourceRef:   ref,
		Evidence:    evidence,
		ExpiresAt:   &expires,
	}, now)
	if err != nil || decision.Verdict == memory.VerdictReject {
		return
	}
	_, _ = store.Confirm(ctx, stored.ID, memory.SourceCommandResult, ref, now)
}

func truncate(text string, limit int) string {
	trimmed := strings.TrimSpace(text)
	if len(trimmed) <= limit {
		return trimmed
	}
	return trimmed[:limit] + "..."
}
