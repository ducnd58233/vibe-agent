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
	"sort"
	"strings"

	"github.com/ducnd58233/vibe-agent/runtime/internal/graph"
	"github.com/ducnd58233/vibe-agent/runtime/internal/loop"
	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/observability"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/workspace"
)

// Client is the host whose hook is firing. Payload shapes differ between them.
type Client string

const (
	ClientClaude Client = "claude"
	ClientCursor Client = "cursor"
	// ClientCodex is Claude's envelope everywhere but the gate.
	//
	// Measured against codex-cli 0.147.0 rather than read from documentation,
	// because the one place the two disagree is the one that refuses an
	// irreversible action, and a guard that only looks connected is worse than
	// none. Codex reads hookSpecificOutput.additionalContext, {"decision":
	// "block"} on Stop, and tool_name / tool_input.command / tool_response with
	// Claude's spelling. It ignores exit 2.
	ClientCodex Client = "codex"

	// ClientOpencode is opencode, answered through the plugin at
	// .opencode/plugin/vibe-agent.js.
	//
	// opencode publishes no shell-command hook surface, so unlike the other
	// three this host cannot be wired from a config file at all: its lifecycle
	// is reachable only from a JS/TS plugin. Registering an MCP server is not a
	// substitute, because the model decides whether to call a tool and a control
	// plane the model may skip is not deterministic.
	//
	// Its envelope is this package's choice rather than a vendor's, since the
	// plugin on the other side is in this repository. Flat and snake_case, which
	// is the shape a small JS reader wants; it is written down in the contract
	// like every other host's so the two sides have one source rather than two
	// habits.
	ClientOpencode Client = "opencode"

	// ClientAntigravity is Google Antigravity. PreToolUse answers with decision
	// and reason; PreInvocation maps to user-prompt-submit via injectSteps.
	ClientAntigravity Client = "antigravity"

	// ClientKimi is Kimi Code CLI. Only PreToolUse, PostToolUse, and Stop are
	// published; the refusal envelope matches Codex because Kimi documents no
	// stdout schema.
	ClientKimi Client = "kimi"

	// ClientMuse is Muse Code. The wire shape matches Claude's hookSpecificOutput
	// per vendor-adjacent measurement; project hooks live at .muse/hooks.json and
	// require muse hooks trust before they run.
	ClientMuse Client = "muse"
)

// Clients is every host this build has an envelope for.
//
// One list, for the reason Events gives, and with a sharper failure mode. The
// envelopes differ per host and Claude's is the fallback, so an unrecognised
// name - a typo, or a host wired into a config before the binary learned it -
// used to be answered in Claude's shape. Every other host ignores that shape,
// which leaves a hook that is registered, fires on every tool call, and delivers
// nothing. Silence is the one failure this control plane cannot see.
func Clients() []Client {
	return []Client{
		ClientClaude, ClientCursor, ClientCodex, ClientOpencode,
		ClientAntigravity, ClientKimi, ClientMuse,
	}
}

// KnownClient reports whether this build can answer a host.
func KnownClient(client Client) bool {
	for _, known := range Clients() {
		if known == client {
			return true
		}
	}
	return false
}

// ClientNames is Clients as strings, for messages and flag help.
func ClientNames() []string {
	names := make([]string, 0, len(Clients()))
	for _, client := range Clients() {
		names = append(names, string(client))
	}
	return names
}

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
	// EventPostToolUseFailure records a tool call the host reported as failed.
	//
	// It is a separate event because Claude Code fires exactly one of the two,
	// and PostToolUse is the success half. Registering only that one is what
	// made this control plane blind to every failing command: the journal's
	// entire purpose is to remember what broke, and the break was the one
	// outcome that never arrived.
	//
	// The event name is itself the evidence. A host that routes a call here has
	// observed the failure, which is the same provenance as an exit code and a
	// world away from reading "error" out of result text.
	EventPostToolUseFailure Event = "post-tool-use-failure"
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
		EventPostToolUseFailure,
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
	Log           observability.Logger
}

// payload is the union of the fields this package reads from either host.
type payload struct {
	// raw is the host's original bytes, kept for hooks this package delegates
	// to another process rather than handles itself.
	raw []byte

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

		// Content is what Write sends; NewString is what Edit sends. The
		// credential gate needs the text going in, not only its destination:
		// a key reaching a file is the event, and the path says nothing about
		// it.
		Content   string `json:"content"`
		NewString string `json:"new_string"`
		// OldString is what Edit is replacing. The suppression gate needs both
		// halves: a rule it already had is not a rule it just added, and
		// without the before there is no way to tell a move from an addition.
		OldString string `json:"old_string"`
	} `json:"tool_input"`
	Command  string `json:"command"`
	FilePath string `json:"file_path"`

	// ToolResponse stays raw: its shape differs per tool and per host version,
	// and a shape this package does not recognise must not be an error.
	ToolResponse json.RawMessage `json:"tool_response"`

	// Error is what a failing tool printed. Claude Code sends no tool_response
	// at all on PostToolUseFailure, so this is the only account of what went
	// wrong, and it is where the exit code appears: "Exit code 2\nundefined: Foo".
	//
	// The number is deliberately not parsed out of it. A field this package can
	// read is evidence; a number recovered from a sentence is a guess that would
	// stay confident after the host reworded it. Quoting the line keeps the
	// figure legible to a human without anyone claiming to have measured it.
	Error string `json:"error"`

	// ErrorMessage is Cursor's name for the same text. Reading only Claude's
	// spelling would give every Cursor failure an empty detail line, which is
	// the whole defect the failure event exists to fix, reintroduced one field
	// deeper. Ref: https://cursor.com/docs/agent/hooks
	ErrorMessage string `json:"error_message"`

	// FailureType is Cursor's category for a failure: "error", "timeout", or
	// "permission_denied".
	FailureType string `json:"failure_type"`

	// IsInterrupt is Claude's top-level cancellation flag. Cursor puts the same
	// meaning inside the response as "interrupted", so both are read.
	IsInterrupt bool `json:"is_interrupt"`

	// LastAssistantMessage is Claude Stop stdin when the host includes the last
	// assistant turn. When present it is projected as one redacted assistant row.
	LastAssistantMessage string `json:"last_assistant_message"`
}

// failurePermissionDenied is the failure_type Cursor reports when a tool call
// was refused rather than attempted.
const failurePermissionDenied = "permission_denied"

// failureText returns the account of what went wrong, from whichever field the
// host filled in.
func (p payload) failureText() string {
	if p.Error != "" {
		return p.Error
	}
	return p.ErrorMessage
}

// declined reports a call the person stopped rather than one the code got wrong.
//
// A cancellation and a denied permission are the same event wearing two names:
// in both the tool never ran, and remembering them would fill the store with a
// record of the user saying no.
func (p payload) declined() bool {
	return p.IsInterrupt || p.FailureType == failurePermissionDenied
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

// writtenText returns every piece of text this tool call would put somewhere:
// the body of a write, the replacement half of an edit, and the shell command
// itself, since a heredoc writes a file without any tool_input at all.
func (p payload) writtenText() string {
	parts := make([]string, 0, 3)
	for _, candidate := range []string{p.ToolInput.Content, p.ToolInput.NewString, p.shellCommand()} {
		if candidate != "" {
			parts = append(parts, candidate)
		}
	}
	return strings.Join(parts, "\n")
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
	if req.Log != nil {
		req.Log.Debug("hook invoke", "event", req.Event, "client", req.Client)
	}
	body := readPayload(req.Stdin)

	switch req.Event {
	case EventSessionStart:
		recordSessionStart(req)
		return sessionStart(req, body, out)
	case EventUserPromptSubmit:
		recordPromptSubmit(req, body)
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
	case EventStop:
		recordStop(req, body, false)
		return stop(req, body, out, "")
	case EventSubagentStop:
		recordStop(req, body, true)
		// A subagent's transcript is the one place the grounding rule can be
		// checked rather than restated, and this is the only event that sees it.
		return stop(req, body, out, groundingReport(body))
	case EventPreToolUse:
		recordPreToolUse(req, body)
		return gate(req, body, out)
	case EventPostToolUse:
		if err := postToolUse(req, body, out, false); err != nil {
			return err
		}
		// The write half cannot refuse: the fetch already happened. Its exit
		// status is the script's own business.
		_ = sddCache(req, body, "sdd-cache-post.py")
		return nil
	case EventPostToolUseFailure:
		return postToolUse(req, body, out, true)
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
	// Kept so a delegated hook can be handed exactly what the host sent. Re-
	// encoding the parsed struct would forward this package's view of the
	// payload rather than the host's, and drop every field it does not read.
	body.raw = raw
	body.enrichFromRaw()
	return body
}

// sessionStart tells a new session where the rules are, what the workspace
// already knows, and whether a run is in flight, so it resumes rather than
// starting over.
func sessionStart(req Request, body payload, out io.Writer) error {
	text := sessionContext(req)
	// The flat-envelope hosts. This branch named only Cursor and answered
	// opencode in Claude's nested shape, which opencode's plugin does not read:
	// the hook fired, the reply was discarded, and nothing reported it. The bug
	// survived a test that checked emitContext directly, because this function
	// does not call it.
	if req.Client == ClientCursor || req.Client == ClientOpencode {
		return write(out, map[string]any{"additional_context": text})
	}
	if req.Client == ClientAntigravity {
		return write(out, map[string]any{
			"injectSteps": []map[string]any{{"ephemeralMessage": text}},
		})
	}

	specific := map[string]any{
		"hookEventName":     "SessionStart",
		"additionalContext": text,
	}
	// Compaction re-fires SessionStart in the middle of a session. Steering
	// there would hijack the conversation already in progress.
	//
	// Claude only. Codex reads the same envelope but rejects fields it does not
	// know, and this one was never measured against it. A rejected response
	// would cost the retrieved memory as well as the steer, so what is
	// unverified stays out rather than endangering what is verified.
	if req.Client == ClientClaude && body.Source != "compact" {
		if steer := steerMessage(req); steer != "" {
			specific["initialUserMessage"] = steer
		}
	}
	return write(out, map[string]any{"hookSpecificOutput": specific})
}

func sessionContext(req Request) string {
	var lines []string
	lines = append(lines, "vibe-agent control plane is available.")

	if rules := workspace.PresentBasenames(req.WorkspaceRoot, "AGENTS.md", "CLAUDE.md", "CLAUDE.local.md"); len(rules) > 0 {
		lines = append(lines, "Workspace rules: "+strings.Join(rules, ", ")+". Read them before applying any toolkit default.")
	}
	lines = append(lines,
		"Source of truth, most authoritative first: repository code and config, git-backed project rules, current run state, retrieved memory, model assumptions.")

	if line := metaSkillLine(req.ToolkitRoot); line != "" {
		lines = append(lines, line)
	}

	if active := activeRuns(req.WorkspaceRoot); len(active) > 0 {
		lines = append(lines, "Active runs:")
		for _, run := range active {
			line := fmt.Sprintf(
				"  %s at node %s (%s, iteration %d/%d). Do not infer or manually advance workflow state; ask the runtime.",
				run.Slug, orNotEntered(run.CurrentNode), run.Status, run.Iteration, run.MaxTransitions)
			if run.Flags["auto"] {
				if hint := autoRunHint(run); hint != "" {
					line += " " + hint
				}
			}
			lines = append(lines, line)
		}
	} else {
		// Name the state rather than leave it to be inferred from silence.
		// Silence is what this looked like before: hooks fired, said nothing,
		// and the reasonable reading was that the control plane was broken
		// rather than that nothing had asked it to track anything.
		lines = append(lines,
			"No active run. Tool use is journalled to the workspace, and gates that need a run "+
				"(stop, the pre-tool refusals) stay off until one starts. `vibe-agent run start` begins one.")
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
	msg := fmt.Sprintf(
		"Resume run %s (goal: %s). It is at node %s. Read the run state with the runtime before doing anything else, and do not restart it or advance it by inference.",
		run.Slug, run.Goal, orNotEntered(run.CurrentNode))
	if run.Flags["auto"] {
		if hint := autoRunHint(run); hint != "" {
			msg += " " + hint
		}
	}
	return msg
}

// autoRunHint tells a host session not to stop mid-pipeline on an auto run.
func autoRunHint(run *state.Run) string {
	switch run.GraphID {
	case "researcher-delivery":
		return "Auto research: continue through hypothesis, experiment_design, experiment_run, findings, and writeup without asking the human; call vibe_checkpoint after each artifact."
	case "goal-delivery":
		return "Auto delivery: continue every node until status is done; call vibe_checkpoint after artifacts."
	default:
		if run.Flags["auto"] {
			return "Auto mode: continue until status is done without asking the human except when a gate document leaves items open."
		}
	}
	return ""
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
			line := fmt.Sprintf("Run %s is at node %s (%s).", run.Slug, orNotEntered(run.CurrentNode), run.Status)
			if node, ok := nodeFor(req, run); ok && node.Description != "" {
				line += " " + node.Description
			}
			lines = append(lines, line)
		}
		lines = append(lines, "Follow the current node the runtime reports. Do not advance workflow state by inference.")
	}

	if reminder := authoringContext(prompt); reminder != "" {
		lines = append(lines, reminder)
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
// stop ends a turn, or refuses to.
//
// extra is an advisory the caller has already worked out, and it is why the
// early return on "no active run" is gone: the subagent grounding check reads a
// transcript, which is a fact about the turn rather than about run state, and a
// workspace with no run in flight still deserves the answer. With extra empty
// the behavior is what it was.
func stop(req Request, body payload, out io.Writer, extra string) error {
	runs := activeRuns(req.WorkspaceRoot)

	// StopHookActive means a previous Stop hook already blocked and the model
	// has had its extra turn. Blocking again is how this becomes a loop.
	if len(runs) > 0 && !body.StopHookActive {
		if reason := blockReason(runs); reason != "" {
			switch req.Client {
			case ClientCursor:
				return write(out, map[string]any{"followup_message": reason})
			case ClientAntigravity:
				return write(out, map[string]any{"decision": "continue", "reason": reason})
			case ClientOpencode:
				// opencode exposes no end-of-turn hook, so nothing here can
				// refuse a turn. Emitting Claude's shape would be a reply no
				// reader parses, which is the silent divergence this package
				// keeps finding; saying nothing is the honest answer.
				return nil
			}
			return write(out, map[string]any{"decision": "block", "reason": reason})
		}
	}

	// The advisory line is Claude's systemMessage and nothing else's. Cursor's
	// stop hook has one field, followup_message, and using it is the blocking
	// behavior; Codex's blocking shape is measured but this one is not. With
	// nothing to block on, saying nothing is the correct output for both.
	if req.Client != ClientClaude {
		return nil
	}

	var parts []string
	if len(runs) > 0 {
		if text := runReminder(runs); text != "" {
			parts = append(parts, text)
		}
	}
	if extra != "" {
		parts = append(parts, extra)
	}
	if len(parts) == 0 {
		return nil
	}
	return emitMessage(out, strings.Join(parts, "\n\n"))
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

// advanceable reports whether another turn could plausibly move this state.
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
		if blocker.Node != run.CurrentNode {
			return fmt.Sprintf(
				"Run %s is still at node %s. Record evidence with vibe-agent checkpoint rather than assuming the step is done.",
				run.Slug, orNotEntered(run.CurrentNode))
		}
		return fmt.Sprintf("Run %s is blocked at %s: %s (attempt %d).",
			run.Slug, blocker.Node, blocker.Reason, blocker.Attempts)
	}
	if run.Status == state.StatusAwaitingHuman {
		return fmt.Sprintf("Run %s is waiting on a human decision at node %s.",
			run.Slug, orNotEntered(run.CurrentNode))
	}
	return fmt.Sprintf(
		"Run %s is still at node %s. Record evidence with vibe-agent checkpoint rather than assuming the step is done.",
		run.Slug, orNotEntered(run.CurrentNode))
}

func nodeFor(req Request, run *state.Run) (graph.Node, bool) {
	loaded, err := graph.LoadByID(graph.DefaultDir(req.ToolkitRoot), run.GraphID)
	if err != nil {
		return graph.Node{}, false
	}
	return loaded.Node(run.CurrentNode)
}

// activeRuns finds manifests under .agent-state/runs/ that have not finished.
// or invalid manifest is skipped rather than reported: a hook is not the place
// to fail a session over a stale file.
func activeRuns(workspaceRoot string) []*state.Run {
	slugs, err := state.List(workspaceRoot)
	if err != nil {
		return nil
	}
	var runs []*state.Run
	for _, slug := range slugs {
		run, err := state.Load(state.ManifestPath(workspaceRoot, slug))
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
	// Cursor and opencode both read a flat additional_context, for different
	// reasons: Cursor because its vendor documents that field, opencode because
	// the plugin reading it is in this repository and was written to this shape.
	if client == ClientCursor || client == ClientOpencode {
		return write(out, map[string]any{"additional_context": text})
	}
	if client == ClientAntigravity {
		return write(out, map[string]any{
			"injectSteps": []map[string]any{{"ephemeralMessage": text}},
		})
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

// orNotEntered fills a blank with the words a reader needs, not a dash.
//
// It was called orNotEntered, the same name cmd/common.go uses for a function that
// really does return a dash. One name over two behaviours is worse than two
// copies of one: both compile, both are used, and the difference only shows in
// rendered text, so a reader who learns it in one file is quietly wrong in the
// other.
func orNotEntered(value string) string {
	if value == "" {
		return "(not entered)"
	}
	return value
}
