package harness

import (
	"encoding/json"
	"strings"

	"github.com/ducnd58233/vibe-agent/runtime/internal/harness/infra/memport"
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

// response is the host's report of how a tool call ended. Fields are optional
// because hosts disagree on what they put in tool_response.
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
			proposeFailure(req.WorkspaceRoot, string(req.Client), "", "", command, result, ref)
		}
		return nil
	}

	for _, run := range runs {
		recorded, err := state.AppendRunEvent(state.EventLogPath(req.WorkspaceRoot, run.Slug), state.Event{
			Type:    state.EventToolUse,
			Node:    run.CurrentNode,
			Payload: entry,
		})
		if err != nil {
			continue
		}
		if failed {
			proposeFailure(req.WorkspaceRoot, string(req.Client), run.Slug, run.CurrentNode, command, result, recorded.Ref())
		}
	}
	return nil
}

// encodeToolUse renders a journal entry, or nil if it somehow cannot.
//
// A struct of strings and an int does not fail to marshal. The journal also
// never invents a tool name; an empty Tool means the hook body was incomplete.
func encodeToolUse(use toolUse) []byte {
	if use.Tool == "" {
		return nil
	}
	raw, err := json.Marshal(use)
	if err != nil {
		return nil
	}
	return raw
}

// memorable reports whether a command is short and single-line enough to be
// worth recognising next time. memorableCommandLimit is the length ceiling.
func memorable(command string) bool {
	command = strings.TrimSpace(command)
	if command == "" || len(command) > memorableCommandLimit {
		return false
	}
	return !strings.ContainsAny(command, "\n\r")
}

// FailureMemoryLife is how long a recorded command failure stays retrievable.
// Re-exported from the memory adapter so existing tests keep a stable name.
const FailureMemoryLife = memport.FailureMemoryLife

// proposeFailure turns a failed command into a memory via the harness memory
// adapter. memorable still decides at this call site whether the command is
// worth remembering; Interrupted and store errors stay silent inside the
// adapter so a hook never fails a tool call over bookkeeping.
func proposeFailure(workspaceRoot, client, slug, node, command string, result response, ref string) {
	if !memorable(command) {
		return
	}
	memport.ProposeFailure(memport.FailureProposal{
		WorkspaceRoot: workspaceRoot,
		Client:        client,
		Slug:          slug,
		Node:          node,
		Command:       command,
		ExitCode:      result.exit(),
		Stderr:        result.Stderr,
		SourceRef:     ref,
		Interrupted:   result.Interrupted,
	})
}

func truncate(text string, limit int) string {
	trimmed := strings.TrimSpace(text)
	if len(trimmed) <= limit {
		return trimmed
	}
	return trimmed[:limit] + "..."
}
