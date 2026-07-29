// Package harness adapts the control plane to each host's lifecycle hooks.
//
// Hooks are the deterministic surface: Claude Code and Cursor always fire them,
// unlike an MCP tool call, which the model decides to make. That is why the
// same capabilities exist in both places and why this one is preferred where it
// is available.
//
// Every hook here informs rather than interferes, with one exception. A control
// plane that can wedge a coding session is worse than one that occasionally
// says nothing, so a missing run, an unreadable manifest, or an unknown event
// all end in a quiet exit 0.
//
// The exception is gate.go, which refuses the two commands that reach other
// people: a push to a protected branch and a pull request merge. Those are the
// irreversible step the delivery graph already guards with approve_merge, and a
// reminder is not enough in front of an action that cannot be undone.
package harness

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ducnd58233/vibe-agent/runtime/internal/graph"
	"github.com/ducnd58233/vibe-agent/runtime/internal/state"
)

// Client is the host whose hook is firing. Payload shapes differ between them.
type Client string

const (
	ClientClaude Client = "claude"
	ClientCursor Client = "cursor"
)

// Event is a lifecycle moment, named in the vendor-neutral form the CLI accepts.
type Event string

const (
	EventSessionStart     Event = "session-start"
	EventUserPromptSubmit Event = "user-prompt-submit"
	EventStop             Event = "stop"
	EventSubagentStop     Event = "subagent-stop"
	// EventPreToolUse is the one event that can refuse. Claude Code fires it as
	// PreToolUse and Cursor as beforeShellExecution.
	EventPreToolUse Event = "pre-tool-use"
)

// Request is a hook invocation.
type Request struct {
	Event         Event
	Client        Client
	WorkspaceRoot string
	ToolkitRoot   string
	Stdin         io.Reader
}

// payload is the union of the fields this package reads from either host.
type payload struct {
	Prompt string `json:"prompt"`
	// Claude sends transcript_path; Cursor sends agent_transcript_path.
	TranscriptPath      string `json:"transcript_path"`
	AgentTranscriptPath string `json:"agent_transcript_path"`
	Slug                string `json:"slug"`

	// Claude nests the shell command under the tool input; Cursor's
	// beforeShellExecution puts it at the top level.
	ToolInput struct {
		Command string `json:"command"`
	} `json:"tool_input"`
	Command string `json:"command"`
}

// shellCommand returns whichever field the host filled in.
func (p payload) shellCommand() string {
	if p.ToolInput.Command != "" {
		return p.ToolInput.Command
	}
	return p.Command
}

// Run handles one hook invocation and writes any response to out.
func Run(req Request, out io.Writer) error {
	body := readPayload(req.Stdin)

	switch req.Event {
	case EventSessionStart:
		return emitContext(out, req.Client, sessionContext(req))
	case EventUserPromptSubmit:
		// Cursor's beforeSubmitPrompt cannot inject context: its output is
		// {continue, user_message} and only validates or blocks. Injecting
		// nothing is correct there; blocking the user to deliver a reminder
		// would be a different and worse behavior.
		if req.Client == ClientCursor {
			return nil
		}
		text := promptContext(req, body.Prompt)
		if text == "" {
			return nil
		}
		return emitContext(out, req.Client, text)
	case EventStop, EventSubagentStop:
		text := runReminder(req)
		if text == "" {
			return nil
		}
		return emitMessage(out, text)
	case EventPreToolUse:
		return gate(req, body.shellCommand(), out)
	default:
		return fmt.Errorf("unknown hook event %q", req.Event)
	}
}

func readPayload(reader io.Reader) payload {
	var body payload
	if reader == nil {
		return body
	}
	raw, err := io.ReadAll(reader)
	if err != nil || len(raw) == 0 {
		return body
	}
	_ = json.Unmarshal(raw, &body)
	return body
}

// sessionContext tells a new session where the rules are and whether a run is
// already in flight, so it resumes rather than starting over.
func sessionContext(req Request) string {
	var lines []string
	lines = append(lines, "vibe-agent control plane is available.")

	if rules := presentFiles(req.WorkspaceRoot, "AGENTS.md", "CLAUDE.md", "CLAUDE.local.md"); len(rules) > 0 {
		lines = append(lines, "Workspace rules: "+strings.Join(rules, ", ")+". Read them before applying any toolkit default.")
	}
	lines = append(lines,
		"Source of truth, most authoritative first: repository code and config, git-backed project rules, current run state, retrieved memory, model assumptions.")

	active := activeRuns(req.WorkspaceRoot)
	if len(active) == 0 {
		return strings.Join(lines, " ")
	}

	lines = append(lines, "Active runs:")
	for _, run := range active {
		lines = append(lines, fmt.Sprintf(
			"  %s at node %s (%s, iteration %d/%d). Do not infer or manually advance workflow state; ask the runtime.",
			run.Slug, orDash(run.CurrentNode), run.Status, run.Iteration, run.MaxTransitions))
	}
	return strings.Join(lines, "\n")
}

// promptContext surfaces the current node when the prompt suggests the person
// is asking about delivery progress.
func promptContext(req Request, prompt string) string {
	active := activeRuns(req.WorkspaceRoot)
	if len(active) == 0 {
		return ""
	}
	lowered := strings.ToLower(prompt)
	interested := false
	for _, keyword := range []string{"goal", "task", "next step", "status", "ship", "continue", "tiếp", "trạng thái"} {
		if strings.Contains(lowered, keyword) {
			interested = true
			break
		}
	}
	if !interested {
		return ""
	}

	var lines []string
	for _, run := range active {
		line := fmt.Sprintf("Run %s is at node %s (%s).", run.Slug, orDash(run.CurrentNode), run.Status)
		if node, ok := nodeFor(req, run); ok && node.Description != "" {
			line += " " + node.Description
		}
		lines = append(lines, line)
	}
	lines = append(lines, "Follow the current node the runtime reports. Do not advance workflow state by inference.")
	return strings.Join(lines, "\n")
}

// runReminder is emitted at the end of a turn so unfinished verification is
// visible rather than quietly forgotten.
func runReminder(req Request) string {
	active := activeRuns(req.WorkspaceRoot)
	if len(active) == 0 {
		return ""
	}
	var lines []string
	for _, run := range active {
		if len(run.Blockers) > 0 {
			blocker := run.Blockers[len(run.Blockers)-1]
			lines = append(lines, fmt.Sprintf(
				"Run %s is blocked at %s: %s (attempt %d).", run.Slug, blocker.Node, blocker.Reason, blocker.Attempts))
			continue
		}
		lines = append(lines, fmt.Sprintf(
			"Run %s is still at node %s. Record evidence with vibe-agent checkpoint rather than assuming the step is done.",
			run.Slug, orDash(run.CurrentNode)))
	}
	return strings.Join(lines, "\n")
}

func nodeFor(req Request, run *state.Run) (graph.Node, bool) {
	loaded, err := graph.LoadByID(graph.DefaultDir(req.ToolkitRoot), run.GraphID)
	if err != nil {
		return graph.Node{}, false
	}
	return loaded.Node(run.CurrentNode)
}

// activeRuns finds manifests under tmp/ that have not finished. An unreadable
// or invalid manifest is skipped rather than reported: a hook is not the place
// to fail a session over a stale file.
func activeRuns(workspaceRoot string) []*state.Run {
	entries, err := os.ReadDir(filepath.Join(workspaceRoot, "tmp"))
	if err != nil {
		return nil
	}
	var runs []*state.Run
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		run, err := state.Load(state.ManifestPath(workspaceRoot, entry.Name()))
		if err != nil {
			continue
		}
		switch run.Status {
		case state.StatusRunning, state.StatusAwaitingHuman:
			runs = append(runs, run)
		}
	}
	sort.Slice(runs, func(i, j int) bool { return runs[i].Slug < runs[j].Slug })
	return runs
}

// emitContext writes host-specific JSON that adds text to the model's context.
func emitContext(out io.Writer, client Client, text string) error {
	if text == "" {
		return nil
	}
	switch client {
	case ClientCursor:
		// Cursor surfaces session context through the agent-visible field only.
		return write(out, map[string]any{"agentMessage": text})
	default:
		return write(out, map[string]any{
			"hookSpecificOutput": map[string]any{
				"hookEventName":     "SessionStart",
				"additionalContext": text,
			},
		})
	}
}

func emitMessage(out io.Writer, text string) error {
	return write(out, map[string]any{"systemMessage": text})
}

func write(out io.Writer, body any) error {
	encoder := json.NewEncoder(out)
	return encoder.Encode(body)
}

func presentFiles(root string, names ...string) []string {
	var present []string
	for _, name := range names {
		if _, err := os.Stat(filepath.Join(root, name)); err == nil {
			present = append(present, name)
		}
	}
	return present
}

func orDash(value string) string {
	if value == "" {
		return "(not entered)"
	}
	return value
}
