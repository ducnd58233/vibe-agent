package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/memory"
	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
)

// commandLimit keeps one journal entry to one line's worth of command. A
// heredoc or a generated script belongs in the transcript, not the event log.
const commandLimit = 500

// memorableCommandLimit is the longest command this package will remember.
//
// A journal entry records what ran. A memory goes further and says the command
// is worth recognising when it comes back, which is a claim about the workspace
// rather than about the moment somebody typed it. Length separates the two well
// in practice: the commands a project runs again and again are short, and the
// long ones are assembled to answer a single question and never appear in that
// shape twice.
const memorableCommandLimit = 120

// toolUse is what a PostToolUse hook records: the tool, what it was aimed at,
// and how it ended. Nothing here is interpreted; the log stores what happened.
type toolUse struct {
	Tool    string `json:"tool"`
	Command string `json:"command,omitempty"`
	File    string `json:"file,omitempty"`
	// ExitCode is a pointer so "the host did not report one" stays different
	// from "it exited 0".
	ExitCode *int `json:"exitCode,omitempty"`
	// Failed is the host's own verdict, carried by which event fired. Claude
	// Code reports no exit code at all, so without this the log could not tell
	// a green command from a red one.
	Failed bool `json:"failed,omitempty"`
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

// readResponse parses whatever the host put in tool_response.
//
// Claude Code sends a bare JSON string for Bash, which is why the earlier
// struct-only parse produced an empty response every time and cost this package
// its evidence. A string carries no exit code, so it lands in Stderr, where the
// only caller uses it: as the detail line on a failure memory.
func readResponse(raw json.RawMessage) response {
	if len(raw) == 0 {
		return response{}
	}
	var structured response
	if err := json.Unmarshal(raw, &structured); err == nil {
		return structured
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return response{Stderr: text}
	}
	return response{}
}

// journal records tool use against every active run, or against the workspace
// when no run is in flight, and proposes a memory when a command reported a
// real failure.
//
// It fires after the tool ran, so it never refuses anything and never returns
// an error: a control plane that fails a session over its own bookkeeping is
// worse than one that records nothing.
func journal(req Request, body payload, failed bool) error {
	if body.ToolName == "" {
		return nil
	}

	result := readResponse(body.ToolResponse)

	// Fold in the fields Claude puts beside the response rather than inside it.
	// A failure payload has no tool_response, so without this the detail line
	// every failure memory is supposed to carry would always be empty.
	if result.Stderr == "" {
		result.Stderr = body.failureText()
	}
	if body.declined() {
		result.Interrupted = true
	}

	// A host that reports an exit code has said the same thing twice. Trust
	// either witness: Cursor supplies the number, Claude supplies the event.
	if exit := result.exit(); exit != nil && *exit != 0 {
		failed = true
	}

	command := truncate(body.shellCommand(), commandLimit)
	entry := encodeToolUse(toolUse{
		Tool:     body.ToolName,
		Command:  command,
		File:     body.writeTarget(),
		ExitCode: result.exit(),
		Failed:   failed,
	})
	if entry == nil {
		return nil
	}

	// Read the runs here rather than at the top. The entry is the same either
	// way, and computing it first is what lets the no-run case record instead of
	// return.
	runs := activeRuns(req.WorkspaceRoot)
	if len(runs) == 0 {
		if ref := ambientJournal(req.WorkspaceRoot, entry); ref != "" && failed {
			proposeFailure(req.WorkspaceRoot, "", "", command, result, ref)
		}
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
		if failed {
			proposeFailure(req.WorkspaceRoot, run.Slug, run.CurrentNode, command, result, recorded.Ref())
		}
	}
	return nil
}

// encodeToolUse renders a journal entry, or nil if it somehow cannot.
//
// A struct of strings and an int does not fail to marshal. The journal also
// never fails a session over its own bookkeeping, so this returns bytes rather
// than an error nobody upstream would act on.
func encodeToolUse(use toolUse) []byte {
	encoded, err := json.Marshal(use)
	if err != nil {
		return nil
	}
	return encoded
}

// memorable reports whether a failed command says something about the workspace
// rather than about the session that happened to type it.
//
// Debugging is mostly failing commands by design, so without this guard the
// store fills with a record of somebody probing rather than of anything that
// broke, and retrieval spends its small budget replaying that. A command
// spanning lines was composed for one moment - a loop, a heredoc, a pipeline
// chained together to answer one question - and it will not arrive again in
// that shape for a memory of it to match.
//
// The event log is unaffected. It records what ran either way; only the claim
// that the command is worth carrying forward is withheld.
func memorable(command string) bool {
	if command == "" || len(command) > memorableCommandLimit {
		return false
	}
	return !strings.ContainsAny(command, "\n\r")
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
// The caller decides that the call failed, from the event the host fired or the
// exit code it reported. Both are observations. What is still refused is reading
// an outcome out of result text, which would be a guess wearing evidence's
// clothes.
//
// A command that fails is not automatically one worth remembering; memorable
// decides that part.
//
// Slug and node are passed rather than a *state.Run because a failure outside
// any run has neither, and is worth remembering for exactly the same reason one
// inside a run is: the host reported the exit code. An empty slug is that case,
// and it changes the wording of the evidence line and drops the run tag. It does
// not change whether the memory is written.
func proposeFailure(workspaceRoot, slug, node, command string, result response, ref string) {
	if !memorable(command) || result.Interrupted {
		return
	}

	store, err := memory.Open(context.Background(), workspaceRoot)
	if err != nil {
		return
	}
	defer func() { _ = store.Close() }()

	evidence := []string{failureContext(slug, node, command, result)}
	if detail := truncate(singleLine(result.Stderr), 300); detail != "" {
		evidence = append(evidence, detail)
	}

	now := time.Now().UTC()
	expires := now.Add(FailureMemoryLife)
	ctx := context.Background()

	stored, decision, err := store.Propose(ctx, memory.Record{
		WorkspaceID: workspaceRoot,
		Kind:        memory.KindEpisodic,
		Content:     fmt.Sprintf("%s %s in this workspace", command, exitsPhrase(result.exit())),
		Tags:        failureTags(slug),
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

// failureContext says where the command failed, in whichever terms exist.
//
// A run gives the line a slug and a node to point at. Outside one there is no
// such anchor, and inventing a placeholder would put a fact in the record that
// nothing observed. Saying plainly that it happened outside a run is both
// shorter and true.
func failureContext(slug, node, command string, result response) string {
	if slug == "" {
		return fmt.Sprintf("%s %s outside any run", command, exitedPhrase(result.exit()))
	}
	return fmt.Sprintf("%s %s during run %s at node %s",
		command, exitedPhrase(result.exit()), slug, orDash(node))
}

// failureTags labels the memory so retrieval can narrow by run.
//
// The empty slug is dropped rather than stored. A tag of "" matches nothing a
// person would search for and would still occupy the row.
func failureTags(slug string) []string {
	if slug == "" {
		return []string{"command-failure"}
	}
	return []string{"command-failure", slug}
}

// exitedPhrase and exitsPhrase name the outcome as precisely as the host allowed.
//
// A number is worth keeping where there is one: "exits 2" tells the next session
// which failure this was, and "fails" only that there was one. Claude Code
// supplies no exit code, so the vaguer phrasing is the honest ceiling there
// rather than a default.
func exitedPhrase(exit *int) string {
	if exit == nil {
		return "failed"
	}
	return fmt.Sprintf("exited %d", *exit)
}

func exitsPhrase(exit *int) string {
	if exit == nil {
		return "fails"
	}
	return fmt.Sprintf("exits %d", *exit)
}

func truncate(text string, limit int) string {
	trimmed := strings.TrimSpace(text)
	if len(trimmed) <= limit {
		return trimmed
	}
	return trimmed[:limit] + "..."
}
