// Package harness adapts the control plane to each host's lifecycle hooks.
//
// Hooks are the deterministic surface: a host always fires them, unlike an MCP
// tool call, which the model decides to make. That is why the
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
	"strings"

	"github.com/ducnd58233/vibe-agent/runtime/internal/graph"
	"github.com/ducnd58233/vibe-agent/runtime/internal/hosts"
	"github.com/ducnd58233/vibe-agent/runtime/internal/loop"
	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/observability"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/workspace"
)

// Client names the host whose hook is firing: one row of the contract table.
// It is data, not a set of identifiers; what differs between hosts is read from
// that row (HostContract.Dialect, HostContract.Tools), never branched on here.
type Client string

// Clients is every host this build has an envelope for.
//
// Derived from the contract table, so a host cannot have a contract and be
// unanswerable, or the reverse. One list, for the reason Events gives, and with
// a sharper failure mode. The
// envelopes differ per host and the default host's is the fallback, so an unrecognised
// name - a typo, or a host wired into a config before the binary learned it -
// used to be answered in the default host's shape. Every other host ignores it,
// which leaves a hook that is registered, fires on every tool call, and delivers
// nothing. Silence is the one failure this control plane cannot see.
func Clients() []Client {
	clients := make([]Client, 0, len(hostContracts))
	for _, contract := range hostContracts {
		clients = append(clients, contract.Client)
	}
	return clients
}

// DefaultClient answers when a hook command names no host. Configs written
// before --client existed omit it, which is why a default exists at all; a name
// this build does not know is still refused, never defaulted.
const DefaultClient Client = "claude"

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
	// EventPreToolUse is the one event that can refuse a tool call. Each host
	// spells it its own way; the contract table maps the spellings.
	EventPreToolUse Event = "pre-tool-use"
	// EventPostToolUse records what a tool actually did. It never refuses:
	// the work already happened by the time it fires.
	EventPostToolUse Event = "post-tool-use"
	// EventPostToolUseFailure records a tool call the host reported as failed.
	//
	// It is a separate event because some hosts fire exactly one of the two,
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
		// A host whose prompt event can only validate or block cannot be told
		// anything here. Injecting nothing is correct; blocking the user to
		// deliver a reminder would be a different and worse behavior.
		if !dialectFor(req.Client).PromptInjection {
			return nil
		}
		text := promptContext(req, body)
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

// sessionStart tells a new session where the rules are, what the workspace
// already knows, and whether a run is in flight, so it resumes rather than
// starting over.
func sessionStart(req Request, body payload, out io.Writer) error {
	// A new, resumed, cleared or compacted session has lost whatever the last
	// one was told, so its ledger starts over and records what this one is told.
	ledger := resetInjectLedger(req, body)
	text := sessionContextWith(req, ledger)
	ledger.save(req)
	dialect := dialectFor(req.Client)
	if dialect.Context == ContextPlain {
		return writeContext(out, dialect, "SessionStart", text)
	}
	envelope := contextBody(dialect, "SessionStart", text)

	// Compaction re-fires SessionStart in the middle of a session. Steering
	// there would hijack the conversation already in progress.
	//
	// Only a host whose contract says it reads the field. A host that rejects
	// fields it does not know would cost the retrieved memory as well as the
	// steer, so what is unverified stays out rather than endangering what is
	// verified.
	if dialect.SteersSessionStart && body.Source != "compact" {
		if specific, ok := envelope["hookSpecificOutput"].(map[string]any); ok {
			if steer := steerMessage(req); steer != "" {
				specific["initialUserMessage"] = steer
			}
		}
	}
	return write(out, envelope)
}

// sessionContextWith builds the session-start text. A non-nil ledger records the
// memories it includes so the first prompt does not repeat them.
func sessionContextWith(req Request, ledger *injectLedger) string {
	var lines []string
	lines = append(lines, "vibe-agent control plane is available.")

	if rules := workspace.PresentBasenames(req.WorkspaceRoot, hosts.RulesFiles()...); len(rules) > 0 {
		lines = append(lines, "Workspace rules: "+strings.Join(rules, ", ")+". Read them before applying any toolkit default.")
	}
	lines = append(lines,
		"Source of truth, most authoritative first: repository code and config, git-backed project rules, current run state, retrieved memory, model assumptions.")

	if line := metaSkillLine(req.ToolkitRoot); line != "" {
		lines = append(lines, line)
	}

	if active := state.Active(req.WorkspaceRoot); len(active) > 0 {
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
	if recalled := recall(req.WorkspaceRoot, "", ledger); recalled != "" {
		lines = append(lines, recalled)
	}
	return strings.Join(lines, "\n")
}

// steerMessage points a fresh session at the run already in flight.
//
// One unambiguous run only. With none there is nothing to resume, and with
// several the runtime would be choosing which goal the person came back for.
func steerMessage(req Request) string {
	active := state.Active(req.WorkspaceRoot)
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
func promptContext(req Request, body payload) string {
	prompt := body.text()
	ledger := loadInjectLedger(req, body)
	ledger.Prompt++

	var lines []string
	emit := func(line string) {
		if ledger.due(line) {
			lines = append(lines, line)
		}
	}

	if active := state.Active(req.WorkspaceRoot); len(active) > 0 {
		for _, run := range active {
			line := fmt.Sprintf("Run %s is at node %s (%s).", run.Slug, orNotEntered(run.CurrentNode), run.Status)
			if node, ok := nodeFor(req, run); ok && node.Description != "" {
				line += " " + node.Description
			}
			emit(line)
		}
		emit("Follow the current node the runtime reports. Do not advance workflow state by inference.")
	}

	if reminder := authoringContext(prompt); reminder != "" {
		emit(reminder)
	}

	if recalled := recall(req.WorkspaceRoot, prompt, ledger); recalled != "" {
		lines = append(lines, recalled)
	}
	ledger.save(req)
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
	runs := state.Active(req.WorkspaceRoot)

	// Every other run gets the one-time exemption below; a run parked at its
	// own graph's research/experiment loop does not, unless something was
	// actually recorded since the prior block. Without this, the exemption is
	// exactly the gap that lets an agent give up on that loop: receive the
	// nudge once, try to stop again without calling checkpoint or verify, and
	// succeed on the second attempt.
	if len(runs) > 0 && body.StopHookActive {
		if reason := researchLoopBlockReason(runs); reason != "" {
			return writeBlockDecision(out, req.Client, reason)
		}
	}

	// StopHookActive means a previous Stop hook already blocked and the model
	// has had its extra turn. Blocking again is how this becomes a loop.
	if len(runs) > 0 && !body.StopHookActive {
		if reason := blockReason(runs); reason != "" {
			recordStopNotice(req, runs)
			return writeBlockDecision(out, req.Client, reason)
		}
	}

	// The advisory line is a systemMessage, which only some hosts read. Where a
	// stop hook's one field is the blocking behavior, or the shape was never
	// measured, saying nothing is the correct output.
	if !dialectFor(req.Client).StopAdvisory {
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

// writeBlockDecision emits the host's shape for refusing to end the turn.
// Shared by the two callers that decide separately whether to refuse. A host
// with no end-of-turn hook gets nothing: a reply no reader parses is the silent
// divergence this package keeps finding.
func writeBlockDecision(out io.Writer, client Client, reason string) error {
	dialect := dialectFor(client)
	if dialect.StopBlock == StopExit {
		return &BlockError{Reason: reason}
	}
	return writeBody(out, stopBody(dialect, reason))
}

// researchLoopNodes names each graph's own experiment retry cycle, where a
// missed threshold routes back automatically rather than needing a human or a
// blocker (see AGENTS.md "Blocker vs. retry"). Keyed by graph id, not one flat
// node list, so a future graph reusing one of these node names for something
// unrelated does not inherit this exemption narrowing by accident.
var researchLoopNodes = map[string]map[string]bool{
	"goal-delivery": {
		"experiment_run": true, "experiment_monitor": true,
		"results_eval": true, "auto_research": true,
	},
	"researcher-delivery": {
		"experiment_run": true, "experiment_monitor": true,
		"results_eval": true, "hypothesis": true, "experiment_design": true,
	},
}

func inResearchLoop(run *state.Run) bool {
	return InResearchLoop(run.GraphID, run.CurrentNode)
}

// InResearchLoop reports whether a node is one of a graph's own research or
// experiment retry cycle nodes - the same set the Stop hook narrows its
// one-time stop exemption for. Exported so other packages (doctor's idle-run
// advice, for one) read the one list this file already maintains instead of
// keeping a second copy that can drift from it.
func InResearchLoop(graphID, node string) bool {
	return researchLoopNodes[graphID][node]
}

// stuckSinceLastNotice reports whether run is a research-loop run this hook
// already blocked once, with nothing recorded since. Shared by
// researchLoopBlockReason (decides whether to refuse again) and
// recordStopNotice (decides which runs to stamp) so the two conditions cannot
// drift apart.
func stuckSinceLastNotice(run *state.Run) bool {
	if !advanceable(run) || !inResearchLoop(run) {
		return false
	}
	return run.StopNoticeAt != nil && !run.UpdatedAt.After(*run.StopNoticeAt)
}

// filteredBlockReason builds the standard block-reason shape - one line per
// selected run, then the shared instruction - for whichever subset of runs
// include picks. blockReason and researchLoopBlockReason differ only in that
// predicate.
func filteredBlockReason(runs []*state.Run, include func(*state.Run) bool) string {
	var lines []string
	for _, run := range runs {
		if !include(run) {
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

// researchLoopBlockReason refuses a second consecutive stop attempt for a run
// parked at one of its own graph's research/experiment loop nodes, when
// nothing has been recorded since the last time this hook blocked it.
func researchLoopBlockReason(runs []*state.Run) string {
	return filteredBlockReason(runs, stuckSinceLastNotice)
}

// recordStopNotice persists run.UpdatedAt as of this block, for every loop-
// node run this call is about to block, so a later stop call - possibly with
// the host's retry-after-block signal set - can tell whether anything
// happened in between. A write failure is logged, not surfaced: a hook that
// fails a session because it could not write this bookkeeping field would be
// a worse failure than not narrowing the exemption this one time.
func recordStopNotice(req Request, runs []*state.Run) {
	for _, run := range runs {
		if !advanceable(run) || !inResearchLoop(run) {
			continue
		}
		notice := run.UpdatedAt
		run.StopNoticeAt = &notice
		if err := state.Save(state.ManifestPath(req.WorkspaceRoot, run.Slug), run); err != nil && req.Log != nil {
			req.Log.Debug("stop notice not recorded", "slug", run.Slug, "error", err.Error())
		}
	}
}

// blockReason returns why the turn may not end, or "" when every active run is
// somewhere the model cannot move it from.
func blockReason(runs []*state.Run) string {
	return filteredBlockReason(runs, advanceable)
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

// emitContext writes the host's envelope for text added to the model's context.
func emitContext(out io.Writer, client Client, event, text string) error {
	if text == "" {
		return nil
	}
	return writeContext(out, dialectFor(client), event, text)
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
