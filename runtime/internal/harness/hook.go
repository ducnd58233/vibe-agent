// Package harness adapts the control plane to each host's lifecycle hooks.
//
// Hooks are the deterministic surface: Claude Code and Cursor always fire them,
// unlike an MCP tool call, which the model decides to make. That is why the
// same capabilities exist in both places and why this one is preferred where it
// is available.
//
// Three of these hooks interfere rather than inform, because a reminder is not
// enough in front of the thing it is guarding:
//
//   - gate.go refuses a push to a protected branch, a pull request merge, and
//     any write to a run's own state files. The first two are the irreversible
//     step approve_merge already guards; the third is the way past that guard.
//   - Stop refuses to end a turn while a run sits mid-graph with no evidence
//     recorded, so an unfinished run is resumed rather than quietly dropped.
//
// Everything else stays advisory, and every failure path is a quiet exit 0. A
// control plane that can wedge a coding session is worse than one that
// occasionally says nothing, so a missing run, an unreadable manifest, an
// absent memory database, or an unknown event all end without complaint.
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
	"github.com/ducnd58233/vibe-agent/runtime/internal/loop"
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
	// EventPreToolUse is the one event that can refuse a tool call. Claude Code
	// fires it as PreToolUse and Cursor as beforeShellExecution.
	EventPreToolUse Event = "pre-tool-use"
	// EventPostToolUse records what a tool actually did. It never refuses:
	// the work already happened by the time it fires.
	EventPostToolUse Event = "post-tool-use"
)

// Events is every event this build handles.
//
// One list, because the same set was previously written out three times: here as
// constants, again as a prose error message in the CLI, and again in each host's
// config. A build whose config registered an event it did not implement was
// therefore possible, and it happened: a binary predating post-tool-use kept
// answering the other five while rejecting that one, which reads as a broken hook
// rather than as an out-of-date install.
//
// `doctor` diffs this against what the host configs register, so the mismatch is
// reported before a session runs into it.
func Events() []Event {
	return []Event{
		EventSessionStart,
		EventUserPromptSubmit,
		EventPreToolUse,
		EventPostToolUse,
		EventStop,
		EventSubagentStop,
	}
}

// Handles reports whether this build knows an event.
func Handles(event Event) bool {
	for _, known := range Events() {
		if known == event {
			return true
		}
	}
	return false
}

// EventNames is Events as strings, for messages and comparisons.
func EventNames() []string {
	names := make([]string, 0, len(Events()))
	for _, event := range Events() {
		names = append(names, string(event))
	}
	return names
}

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
	// Claude Code sends user_prompt; older builds and Cursor send prompt.
	// Reading both keeps one adapter working across versions.
	Prompt     string `json:"prompt"`
	UserPrompt string `json:"user_prompt"`

	// Claude sends transcript_path; Cursor sends agent_transcript_path.
	TranscriptPath      string `json:"transcript_path"`
	AgentTranscriptPath string `json:"agent_transcript_path"`
	Slug                string `json:"slug"`

	// Source distinguishes a fresh session from a resume, a clear, or the
	// restart that follows compaction.
	Source string `json:"source"`

	// StopHookActive is true when this Stop hook is firing because a previous
	// Stop hook blocked. It is the only thing standing between a blocking Stop
	// hook and an infinite loop.
	StopHookActive bool `json:"stop_hook_active"`

	ToolName string `json:"tool_name"`
	// Claude nests tool arguments; Cursor's beforeShellExecution puts the
	// command at the top level.
	ToolInput struct {
		Command      string `json:"command"`
		FilePath     string `json:"file_path"`
		NotebookPath string `json:"notebook_path"`
	} `json:"tool_input"`
	Command  string `json:"command"`
	FilePath string `json:"file_path"`

	// ToolResponse stays raw: its shape differs per tool and per host version,
	// and a shape this package does not recognise must not be an error.
	ToolResponse json.RawMessage `json:"tool_response"`
}

// shellCommand returns whichever field the host filled in.
func (p payload) shellCommand() string {
	if p.ToolInput.Command != "" {
		return p.ToolInput.Command
	}
	return p.Command
}

// text returns the submitted prompt from whichever field carried it.
func (p payload) text() string {
	if p.UserPrompt != "" {
		return p.UserPrompt
	}
	return p.Prompt
}

// writeTarget returns the file a file-writing tool is aimed at.
func (p payload) writeTarget() string {
	for _, candidate := range []string{p.ToolInput.FilePath, p.ToolInput.NotebookPath, p.FilePath} {
		if candidate != "" {
			return candidate
		}
	}
	return ""
}

// Run handles one hook invocation and writes any response to out.
func Run(req Request, out io.Writer) error {
	body := readPayload(req.Stdin)

	switch req.Event {
	case EventSessionStart:
		return sessionStart(req, body, out)
	case EventUserPromptSubmit:
		// Cursor's beforeSubmitPrompt cannot inject context: its output is
		// {continue, user_message} and only validates or blocks. Injecting
		// nothing is correct there; blocking the user to deliver a reminder
		// would be a different and worse behavior.
		if req.Client == ClientCursor {
			return nil
		}
		text := promptContext(req, body.text())
		if text == "" {
			return nil
		}
		return emitContext(out, req.Client, "UserPromptSubmit", text)
	case EventStop, EventSubagentStop:
		return stop(req, body, out)
	case EventPreToolUse:
		return gate(req, body, out)
	case EventPostToolUse:
		return journal(req, body)
	default:
		// Naming the events this build does handle turns the message into a
		// diagnosis. The original text said only that the event was unknown, which
		// reads as a configuration typo when the usual cause is a binary older
		// than the config that calls it.
		return fmt.Errorf("unknown hook event %q; this build handles %s. "+
			"A host config registering an event listed nowhere here is usually a "+
			"binary older than the config: reinstall it (cd runtime && make install) "+
			"and run `vibe-agent doctor`",
			req.Event, strings.Join(EventNames(), ", "))
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

// sessionStart tells a new session where the rules are, what the workspace
// already knows, and whether a run is in flight, so it resumes rather than
// starting over.
func sessionStart(req Request, body payload, out io.Writer) error {
	text := sessionContext(req)
	if req.Client == ClientCursor {
		return write(out, map[string]any{"additional_context": text})
	}

	specific := map[string]any{
		"hookEventName":     "SessionStart",
		"additionalContext": text,
	}
	// Compaction re-fires SessionStart in the middle of a session. Steering
	// there would hijack the conversation already in progress.
	if body.Source != "compact" {
		if steer := steerMessage(req); steer != "" {
			specific["initialUserMessage"] = steer
		}
	}
	return write(out, map[string]any{"hookSpecificOutput": specific})
}

func sessionContext(req Request) string {
	var lines []string
	lines = append(lines, "vibe-agent control plane is available.")

	if rules := presentFiles(req.WorkspaceRoot, "AGENTS.md", "CLAUDE.md", "CLAUDE.local.md"); len(rules) > 0 {
		lines = append(lines, "Workspace rules: "+strings.Join(rules, ", ")+". Read them before applying any toolkit default.")
	}
	lines = append(lines,
		"Source of truth, most authoritative first: repository code and config, git-backed project rules, current run state, retrieved memory, model assumptions.")

	if active := activeRuns(req.WorkspaceRoot); len(active) > 0 {
		lines = append(lines, "Active runs:")
		for _, run := range active {
			lines = append(lines, fmt.Sprintf(
				"  %s at node %s (%s, iteration %d/%d). Do not infer or manually advance workflow state; ask the runtime.",
				run.Slug, orDash(run.CurrentNode), run.Status, run.Iteration, run.MaxTransitions))
		}
	}

	// Retrieval happens here rather than behind a tool call, so what the
	// workspace already learned reaches the model whether or not it thinks to
	// ask. Passing no query returns the most recently updated memories.
	if recalled := recall(req.WorkspaceRoot, ""); recalled != "" {
		lines = append(lines, recalled)
	}
	return strings.Join(lines, "\n")
}

// steerMessage points a fresh session at the run already in flight.
//
// One unambiguous run only. With none there is nothing to resume, and with
// several the runtime would be choosing which goal the person came back for.
func steerMessage(req Request) string {
	active := activeRuns(req.WorkspaceRoot)
	if len(active) != 1 || active[0].Status != state.StatusRunning {
		return ""
	}
	run := active[0]
	return fmt.Sprintf(
		"Resume run %s (goal: %s). It is at node %s. Read the run state with the runtime before doing anything else, and do not restart it or advance it by inference.",
		run.Slug, run.Goal, orDash(run.CurrentNode))
}

// promptContext rides along with every prompt: the run the workspace is in the
// middle of, and the memories that match what was asked.
//
// This used to fire only when the prompt contained a progress-sounding keyword,
// which meant an ordinary question got neither. Whether context is needed is not
// something a substring match can answer.
func promptContext(req Request, prompt string) string {
	var lines []string

	if active := activeRuns(req.WorkspaceRoot); len(active) > 0 {
		for _, run := range active {
			line := fmt.Sprintf("Run %s is at node %s (%s).", run.Slug, orDash(run.CurrentNode), run.Status)
			if node, ok := nodeFor(req, run); ok && node.Description != "" {
				line += " " + node.Description
			}
			lines = append(lines, line)
		}
		lines = append(lines, "Follow the current node the runtime reports. Do not advance workflow state by inference.")
	}

	if recalled := recall(req.WorkspaceRoot, prompt); recalled != "" {
		lines = append(lines, recalled)
	}
	return strings.Join(lines, "\n")
}

// stop decides whether the turn may end.
//
// A run mid-graph with nothing recorded is the failure this refuses: the work
// happened, the evidence did not, and the next session starts from a manifest
// that never learned about it.
func stop(req Request, body payload, out io.Writer) error {
	runs := activeRuns(req.WorkspaceRoot)
	if len(runs) == 0 {
		return nil
	}

	// StopHookActive means a previous Stop hook already blocked and the model
	// has had its extra turn. Blocking again is how this becomes a loop.
	if !body.StopHookActive {
		if reason := blockReason(runs); reason != "" {
			if req.Client == ClientCursor {
				return write(out, map[string]any{"followup_message": reason})
			}
			return write(out, map[string]any{"decision": "block", "reason": reason})
		}
	}

	// Cursor's stop hook has one field, followup_message, and using it is the
	// blocking behavior. With nothing to block on there is nothing to say.
	if req.Client == ClientCursor {
		return nil
	}
	text := runReminder(runs)
	if text == "" {
		return nil
	}
	return emitMessage(out, text)
}

// blockReason returns why the turn may not end, or "" when every active run is
// somewhere the model cannot move it from.
func blockReason(runs []*state.Run) string {
	var lines []string
	for _, run := range runs {
		if !advanceable(run) {
			continue
		}
		lines = append(lines, reminderLine(run))
	}
	if len(lines) == 0 {
		return ""
	}
	lines = append(lines,
		"Do not end the turn with a run mid-graph. Record the real result with vibe-agent checkpoint, or record a blocker if the step cannot pass. Model assertion is not evidence.")
	return strings.Join(lines, "\n")
}

// advanceable reports whether another turn could plausibly move this run.
//
// A run waiting on a person cannot be advanced by the model, and one past the
// blocker cap has already been told to stop trying. Blocking either would spin
// the session instead of finishing the work.
func advanceable(run *state.Run) bool {
	if run.Status != state.StatusRunning {
		return false
	}
	for _, blocker := range run.Blockers {
		if blocker.Attempts >= loop.MaxBlockerAttempts {
			return false
		}
	}
	return true
}

// runReminder is the advisory form, emitted when nothing is blockable.
func runReminder(runs []*state.Run) string {
	var lines []string
	for _, run := range runs {
		lines = append(lines, reminderLine(run))
	}
	return strings.Join(lines, "\n")
}

func reminderLine(run *state.Run) string {
	if len(run.Blockers) > 0 {
		blocker := run.Blockers[len(run.Blockers)-1]
		return fmt.Sprintf("Run %s is blocked at %s: %s (attempt %d).",
			run.Slug, blocker.Node, blocker.Reason, blocker.Attempts)
	}
	if run.Status == state.StatusAwaitingHuman {
		return fmt.Sprintf("Run %s is waiting on a human decision at node %s.",
			run.Slug, orDash(run.CurrentNode))
	}
	return fmt.Sprintf(
		"Run %s is still at node %s. Record evidence with vibe-agent checkpoint rather than assuming the step is done.",
		run.Slug, orDash(run.CurrentNode))
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
func emitContext(out io.Writer, client Client, event, text string) error {
	if text == "" {
		return nil
	}
	if client == ClientCursor {
		return write(out, map[string]any{"additional_context": text})
	}
	return write(out, map[string]any{
		"hookSpecificOutput": map[string]any{
			"hookEventName":     event,
			"additionalContext": text,
		},
	})
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
