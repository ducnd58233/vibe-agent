package harness

import "slices"

// What each host's hook API actually is, as data.
//
// This table exists because the same class of defect was fixed five times and
// never once written down. dd79c7f corrected Cursor's matchers, e8d92b4 refused
// a host name this build could not answer, d4c0b0c re-synced the docs with
// reality, a8f2c64 established Codex's envelopes by measurement, d2827a4 found
// that the failure half of a tool call had never been recorded. Every one of
// them added a doctor check scoped to its own instance, so the next defect
// landed in a cell no check covered.
//
// A document would have repeated that. All five fixes edited documentation too.
// What was missing is a single source both the checks and the prose read, so
// that correcting one corrects the other, and so a host's contract is a thing
// the build knows rather than a thing a person remembers.
//
// Two rules govern edits here:
//
//  1. Every host carries a Source URL, and a claim not supported by it does not
//     belong in a row.
//  2. Verified is set from observation, never from reading. A row describing
//     what the vendor documents but nobody has seen fire stays Unverified, and
//     says why. Cursor's rows are the reason this field exists.

// Verification records whether a contract row was observed or only read.
type Verification struct {
	// Verified is true only when a hook was watched firing with this shape.
	Verified bool
	// Why explains an unverified row. Empty on a verified one.
	Why string
}

// observed marks a row someone watched fire.
func observed() Verification { return Verification{Verified: true} }

// unverified marks a row taken from documentation alone.
func unverified(why string) Verification { return Verification{Why: why} }

// WorkspaceRoot names how a host's hook command learns which directory it is
// working on.
//
// It is a first-class field because getting it wrong is silent. A hook that
// resolves the workspace from an undocumented cwd still runs, still exits 0, and
// reads every piece of state from the wrong place, so the control plane reports
// no runs and no memory while appearing perfectly wired.
type WorkspaceRoot struct {
	// Variable is the host's own substitution, if it publishes one.
	Variable string
	// Reliable is true when the host documents a value the config can depend on.
	// When false, the hook command must pass an explicit --workspace.
	Reliable bool
	// Note explains the mechanism.
	Note string
}

// ToolVocabulary names a host's tools by what they do. The guards act on what a
// tool does; hosts disagree on what it is called, and a name missing from here
// is a tool whose writes no guard ever scans.
type ToolVocabulary struct {
	// Writes put text into a file the guards should read afterwards.
	Writes []string
	// Fetches retrieve a URL in the payload shape the WebFetch cache script reads.
	Fetches []string
}

// sharedEditTools are the edit tools of the hosts that share one tool family.
var sharedEditTools = []string{"Edit", "Write", "NotebookEdit", "MultiEdit"}

// sharedFetchTools is that family's fetch tool.
var sharedFetchTools = []string{"WebFetch"}

// EventContract is one lifecycle event on one host.
type EventContract struct {
	// HostKey is the exact key the host's config file uses. Case matters: this
	// is compared against the JSON keys in a real config.
	HostKey string
	// Event is the vendor-neutral event it maps to, or "" for one this toolkit
	// does not wire.
	Event Event
	// OutputKeys are the exact field names the host reads back, casing included.
	// Cursor reads agent_message and discards agentMessage without complaint,
	// which is the defect this field exists to make checkable.
	OutputKeys []string
	// CanInject is true when the event can add text to the model's context.
	CanInject bool
	// CanRefuse is true when the event can stop the thing it fires for.
	CanRefuse bool
	// Wired is true when this toolkit registers the event.
	Wired bool
	Verification
	// Note carries anything a reader needs that the fields cannot say.
	Note string
}

// HostContract is one host's hook API.
type HostContract struct {
	Client Client
	// ConfigPath is where the host reads hook wiring from, relative to the
	// workspace root.
	ConfigPath string
	// Source is the vendor documentation this row set was read from.
	Source string
	// AltConfigPaths are further files the host reads hook wiring from.
	AltConfigPaths []string
	// SplitsToolOutcome is true where the host reports a failed tool call as its
	// own event, so a config wiring only the success half records the wrong half
	// rather than less.
	//
	// Claude and Cursor both split: each fires exactly one of PostToolUse and
	// PostToolUseFailure per call, so wiring one alone is a defect.
	//
	// Codex is false for a different and worse reason. Its documentation says
	// PostToolUse fires "including when commands exit with a non-zero status",
	// and codex-cli 0.147.0 does not: a failing command produces PreToolUse and
	// then nothing, measured twice, while a passing one in the same session
	// produced both. Codex publishes no failure event either, so there is no
	// second hook to ask for - the gap is the host's and cannot be wired shut.
	// Reporting it would only tell someone to add a hook that does not exist.
	//
	// opencode exposes tool lifecycle through JS/TS plugins rather than shell
	// commands, so it registers no events here at all.
	SplitsToolOutcome bool
	// HonorsHandlerIf is true when a hook handler may carry an "if" filter that
	// the host evaluates. For every other host the field is a silent no-op.
	HonorsHandlerIf bool
	// Dialect is how the hooks' answers are spelled for this host.
	Dialect Dialect
	// Tools is what the host calls its tools.
	Tools         ToolVocabulary
	WorkspaceRoot WorkspaceRoot
	Events        []EventContract
	// Gaps are the things this host does not provide. They matter as much as the
	// events: a gap recorded here is one nobody tries to wire a sixth time.
	Gaps []string
}

// hostContracts is the table. Order is the order the document renders in.
var hostContracts = []HostContract{
	claudeContract, cursorContract, codexContract, opencodeContract,
	antigravityContract, kimiContract, museContract,
}

// HostContractFor returns a host's contract.
func HostContractFor(client Client) (HostContract, bool) {
	for _, contract := range hostContracts {
		if contract.Client == client {
			return contract, true
		}
	}
	return HostContract{}, false
}

// isFileWrite reports whether a host's tool call put text into a file.
//
// One binary answers every PostToolUse, so a read, which also carries a file
// path, would otherwise be scanned as though it wrote the file it opened.
//
// An empty name passes. A host whose edit event is already edit-only sends no
// tool name, and refusing it would silence every guard on that host.
func isFileWrite(client Client, tool string) bool {
	if tool == "" {
		return true
	}
	return slices.Contains(toolsFor(client).Writes, tool)
}

// isFetch reports whether a host's tool call is the fetch the WebFetch cache
// script understands.
func isFetch(client Client, tool string) bool {
	return slices.Contains(toolsFor(client).Fetches, tool)
}

// toolsFor returns a host's vocabulary, or the shared family's for a host this
// build has no row for, the same default the zero Dialect gives.
func toolsFor(client Client) ToolVocabulary {
	if contract, ok := HostContractFor(client); ok {
		return contract.Tools
	}
	return ToolVocabulary{Writes: sharedEditTools, Fetches: sharedFetchTools}
}

// HostContracts returns every contract, for the document generator and the
// doctor checks that read it.
func HostContracts() []HostContract { return hostContracts }

// HostKeys returns every event key a host publishes, wired or not.
//
// doctor compares a config's JSON keys against this. A key absent from it is a
// hook that will never fire, which today reads as correct wiring.
func (h HostContract) HostKeys() []string {
	keys := make([]string, 0, len(h.Events))
	for _, event := range h.Events {
		keys = append(keys, event.HostKey)
	}
	return keys
}

// EventFor returns the contract for one host-side key.
func (h HostContract) EventFor(hostKey string) (EventContract, bool) {
	for _, event := range h.Events {
		if event.HostKey == hostKey {
			return event, true
		}
	}
	return EventContract{}, false
}

// claudeContract is Claude Code.
//
// Only the events this toolkit could wire are listed. Claude publishes roughly
// thirty, and copying all of them would make this a worse copy of the vendor's
// table; what earns a row here is an event the toolkit wires or deliberately
// declines to.
var claudeContract = HostContract{
	Client:            "claude",
	Tools:             ToolVocabulary{Writes: sharedEditTools, Fetches: sharedFetchTools},
	HonorsHandlerIf:   true,
	SplitsToolOutcome: true,
	Dialect:           Dialect{PromptInjection: true, StopAdvisory: true, SteersSessionStart: true},
	ConfigPath:        ".claude/settings.json",
	Source:            "https://code.claude.com/docs/en/hooks",
	WorkspaceRoot: WorkspaceRoot{
		Variable: "${CLAUDE_PROJECT_DIR}",
		Reliable: true,
		Note:     "Published for every hook command, so a config can pass --workspace ${CLAUDE_PROJECT_DIR} rather than trusting cwd.",
	},
	Events: []EventContract{
		{
			HostKey: "SessionStart", Event: EventSessionStart,
			OutputKeys: []string{"hookSpecificOutput.hookEventName", "hookSpecificOutput.additionalContext", "systemMessage"},
			CanInject:  true, Wired: true, Verification: observed(),
			Note: "Re-fires after compaction, which is why steering is suppressed when source is compact.",
		},
		{
			HostKey: "UserPromptSubmit", Event: EventUserPromptSubmit,
			OutputKeys: []string{"hookSpecificOutput.hookEventName", "hookSpecificOutput.additionalContext"},
			CanInject:  true, CanRefuse: true, Wired: true, Verification: observed(),
			Note: "The only per-prompt injection point. No matcher support.",
		},
		{
			HostKey: "PreToolUse", Event: EventPreToolUse,
			OutputKeys: []string{"hookSpecificOutput.permissionDecision", "hookSpecificOutput.permissionDecisionReason"},
			CanRefuse:  true, Wired: true, Verification: observed(),
			Note: "This toolkit refuses through exit 2 and stderr here rather than the JSON shape; both are documented. " +
				"The matcher also names mcp__.*, which the vendor page documents as a regular expression over mcp__<server>__<tool>, so the outward-action danger category sees MCP calls.",
		},
		{
			HostKey: "PostToolUse", Event: EventPostToolUse,
			OutputKeys: []string{"systemMessage"}, Wired: true, Verification: observed(),
			Note: "Success half only. Fires exactly one of this and PostToolUseFailure per call.",
		},
		{
			HostKey: "PostToolUseFailure", Event: EventPostToolUseFailure,
			OutputKeys: []string{"systemMessage"}, Wired: true, Verification: observed(),
			Note: "Failure half. Carries no tool_response; what the tool printed is in error.",
		},
		{
			HostKey: "Stop", Event: EventStop,
			OutputKeys: []string{"decision", "reason"},
			CanRefuse:  true, Wired: true, Verification: observed(),
			Note: "Blocks at most once per turn, guarded by stop_hook_active. The top-level {decision: block, reason} " +
				"shape is honoured: measured on 2026-08-15 against Claude Code 2.1.229 with a run at node build, " +
				"where the hook refused the turn and the reason arrived verbatim in the next one. Worth knowing " +
				"because the vendor table documents this event as reading hookSpecificOutput.decision with " +
				"allow/deny, and rewriting the working shape to match that page would have broken a hook that works.",
		},
		{
			HostKey: "SubagentStop", Event: EventSubagentStop,
			OutputKeys: []string{"decision", "reason"},
			CanRefuse:  true, Wired: true,
			Verification: unverified("Stop was measured honouring the top-level shape and this event was not. " +
				"They share an implementation, which is a reason to expect the same result and not a substitute " +
				"for seeing it: the whole point of this column is that expectation and observation are different columns."),
			Note: "The only event that sees a subagent transcript, so the grounding check runs here.",
		},
	},
	Gaps: []string{
		"No event reports a tool call's outcome in one place: the success and failure halves are separate events, and wiring only one records the wrong half rather than less.",
	},
}

// cursorContract is Cursor.
//
// Every row here is Unverified, and that is the finding rather than an omission.
// .ai-agents/hooks/README.md records the attempt: cursor-agent 2026.08.11 failed
// even a hook command of `true` before the hook process started. So this host's
// wiring was written from the vendor page and has never been watched running,
// which is precisely the condition that produced the defects below.
var cursorContract = HostContract{
	Client:            "cursor",
	Tools:             ToolVocabulary{Writes: sharedEditTools, Fetches: sharedFetchTools},
	SplitsToolOutcome: true,
	Dialect:           Dialect{Context: ContextFlat, Refusal: RefusePermission, StopBlock: StopFollowup, PostTool: PostToolFlat, ToolUseNodeReminder: true},
	ConfigPath:        ".cursor/hooks.json",
	Source:            "https://cursor.com/docs/agent/hooks",
	WorkspaceRoot: WorkspaceRoot{
		Reliable: false,
		Note: "Cursor publishes no project-directory variable for hook commands and does not document the cwd they run in. " +
			"A hook here must pass an explicit --workspace, and a relative script path is equally unsafe.",
	},
	Events: []EventContract{
		{
			HostKey: "sessionStart", Event: EventSessionStart,
			OutputKeys: []string{"additional_context", "env"},
			CanInject:  true, Wired: true,
			Verification: unverified(cursorNeverObserved),
			Note:         "snake_case, unlike Claude's nested camelCase.",
		},
		{
			HostKey: "beforeSubmitPrompt", Event: EventUserPromptSubmit,
			OutputKeys: []string{"continue", "user_message"},
			CanRefuse:  true, Wired: false,
			Verification: unverified(cursorNeverObserved),
			Note: "Cannot inject context: it validates or blocks the prompt and nothing else. " +
				"The runtime therefore returns no prompt-time context for Cursor, so Cursor gets no " +
				"per-prompt node or memory injection at all.",
		},
		{
			HostKey: "beforeShellExecution", Event: EventPreToolUse,
			OutputKeys: []string{"permission", "user_message", "agent_message"},
			CanRefuse:  true, Wired: true,
			Verification: unverified(cursorNeverObserved),
			Note:         "permission accepts allow, deny or ask.",
		},
		{
			HostKey: "preToolUse", Event: EventPreToolUse,
			OutputKeys: []string{"permission", "user_message", "agent_message", "updated_input"},
			CanRefuse:  true, Wired: true,
			Verification: unverified(cursorNeverObserved),
			Note:         "permission accepts allow or deny only. Unlike beforeShellExecution, there is no ask.",
		},
		{
			HostKey: "postToolUse", Event: EventPostToolUse,
			OutputKeys: []string{"additional_context", "updated_mcp_tool_output"},
			CanInject:  true, Wired: true,
			Verification: unverified(cursorNeverObserved),
			Note: "The only injection point Cursor has, so the run's current node is delivered here " +
				"rather than at prompt time. Once per node change, not once per tool call: a run sitting " +
				"at one node for twenty edits has nothing new to report, and repeating it teaches a reader " +
				"to skip the line that matters when it does change.",
		},
		{
			HostKey: "postToolUseFailure", Event: EventPostToolUseFailure,
			OutputKeys: nil, Wired: true,
			Verification: unverified(cursorNeverObserved),
			Note: "No output fields are supported. Names the failure text error_message, the category " +
				"failure_type, and reports an exit code where Claude does not.",
		},
		{
			HostKey: "subagentStop", Event: EventSubagentStop,
			OutputKeys: []string{"followup_message"},
			CanRefuse:  true, Wired: true,
			Verification: unverified(cursorNeverObserved),
		},
		{
			HostKey: "stop", Event: EventStop,
			OutputKeys: []string{"followup_message"},
			CanRefuse:  true, Wired: true,
			Verification: unverified(cursorNeverObserved),
			Note:         "Sending followup_message is the blocking behaviour; there is no decision field.",
		},
	},
	Gaps: []string{
		"cursor-agent 2026.08.11 does not read this file. Measured: it ran a hook out of .claude/settings.json, so --client cursor never reached the runtime and the reply would have been built in Claude's shape. Whether the Cursor editor reads it is untested.",
		"cursor-agent wraps a hook command in PowerShell and executes it with a POSIX shell on Windows, so the hook process never starts. A defect in the host; nothing in this repository can wire around it.",
		"No per-prompt context injection. beforeSubmitPrompt can only validate or block. The run's current node is delivered on postToolUse instead, which is a partial substitute: it arrives after a tool call rather than before a prompt, and a session that runs no tools never sees it.",
		"No per-prompt memory retrieval. Memories reach a Cursor session at session start and not again, so one that runs for hours works from what was true when it opened.",
		"No documented project-directory variable and no documented cwd for hook commands.",
		"MCP tool calls are not wired to the gate. The outward-action danger category matches an MCP tool name, but this config does not register beforeMCPExecution and the payload that event sends has not been read, so on Cursor an MCP send is not refused on an auto run.",
	},
}

// cursorNeverObserved is the one reason every Cursor row carries.
//
// Updated from "the CLI would not start a hook" to what was actually seen.
// cursor-agent 2026.08.11 now completes ordinary turns, and forcing a shell
// call showed it running a hook out of .claude/settings.json rather than this
// file: the command carried --workspace ${CLAUDE_PROJECT_DIR} and no --client
// cursor, and that string exists in exactly one config here. It then failed
// before vibe-agent started, because the wrapper is PowerShell and the shell
// executing it is bash: "eval: syntax error near unexpected token `&'".
//
// So the rows below are still unmeasured, for a sharper reason than before. The
// CLI does not read them, and the editor was not tested.
const cursorNeverObserved = "No Cursor hook has been observed firing from this config. " +
	"cursor-agent 2026.08.11 was measured running .claude/settings.json instead of .cursor/hooks.json, " +
	"and failing before the hook process started because it wraps the command in PowerShell and executes " +
	"it with bash. The Cursor editor was not tested and may differ. See " +
	"tmp/control-plane-activation/runtime/host-measurements.md."

// opencodePluginUnmeasured is the reason every opencode row is unverified.
//
// The plugin is committed and the wiring is real; what has not happened is
// someone watching a hook fire. Marking these verified because the code exists
// would be the Cursor mistake with the authorship reversed: reading an
// implementation instead of a vendor page, and calling either one an
// observation.
const opencodePluginUnmeasured = "The plugin is loaded and no hook in it has been observed firing. " +
	"opencode 1.14.39 resolves it: `opencode debug config` reports plugin_origins with scope local from " +
	".opencode, which confirms both the directory and that no opencode.json entry is needed. Loading and " +
	"firing are different claims, and only the first was watched."

// codexContract is Codex.
//
// The envelopes here were established by experiment rather than inference,
// because the documentation and the binary disagreed twice. a8f2c64 records the
// first: Codex ignores exit 2 outright, running the command anyway while the
// hook exited 2 and printed its refusal, so the JSON shape is the only gate that
// works. The second is the missing failure event below.
//
// Measured against codex-cli 0.147.0: it reads hookSpecificOutput.additionalContext,
// {"decision": "block"} on Stop, and tool_name / tool_input.command /
// tool_response in the shared family's spelling.
var codexContract = HostContract{
	Client:         "codex",
	Tools:          ToolVocabulary{Writes: sharedEditTools, Fetches: sharedFetchTools},
	Dialect:        Dialect{Refusal: RefuseHookSpecific, PromptInjection: true},
	ConfigPath:     ".codex/hooks.json",
	AltConfigPaths: []string{".codex/config.toml"},
	Source:         "https://learn.chatgpt.com/docs/hooks",
	WorkspaceRoot: WorkspaceRoot{
		Reliable: false,
		Note: "Hook commands run with the session's cwd and Codex publishes no project-directory variable for them. " +
			"The documented workaround is git root resolution; this toolkit passes an explicit --workspace instead.",
	},
	Events: []EventContract{
		{
			HostKey: "SessionStart", Event: EventSessionStart,
			OutputKeys: []string{"hookSpecificOutput.hookEventName", "hookSpecificOutput.additionalContext"},
			CanInject:  true, Wired: true, Verification: observed(),
		},
		{
			HostKey: "UserPromptSubmit", Event: EventUserPromptSubmit,
			OutputKeys: []string{"hookSpecificOutput.hookEventName", "hookSpecificOutput.additionalContext"},
			CanInject:  true, Wired: true, Verification: observed(),
		},
		{
			HostKey: "PreToolUse", Event: EventPreToolUse,
			OutputKeys: []string{"hookSpecificOutput.hookEventName", "hookSpecificOutput.permissionDecision", "hookSpecificOutput.permissionDecisionReason"},
			CanRefuse:  true, Wired: true, Verification: observed(),
			Note: "Exit 2 is ignored by this host: it was measured running the command anyway while the hook " +
				"exited 2 and printed the refusal. The JSON shape is the only gate that holds.",
		},
		{
			HostKey: "PostToolUse", Event: EventPostToolUse,
			OutputKeys: []string{"hookSpecificOutput.hookEventName", "decision"},
			Wired:      true, Verification: observed(),
			Note: "Documented as firing \"including when commands exit with a non-zero status\", and measured twice " +
				"not doing so: a failing command produces PreToolUse and then nothing.",
		},
		{
			HostKey: "Stop", Event: EventStop,
			OutputKeys: []string{"decision", "reason"}, CanRefuse: true, Wired: true,
			Verification: unverified("The runtime sends {decision: block, reason} here, the shape Codex was recorded reading on Stop " +
				"when its envelopes were measured against codex-cli 0.147.0; this row itself has not been observed separately."),
		},
		{
			HostKey: "SubagentStop", Event: EventSubagentStop,
			OutputKeys: []string{"decision", "reason"}, CanRefuse: true, Wired: true,
			Verification: unverified("Same as Stop."),
		},
	},
	Gaps: []string{
		"No failure event exists, and PostToolUse does not fire for a failed command despite the documentation saying it does. A failed command therefore cannot be journalled on Codex. This gap is the host's and cannot be wired shut.",
		"Exit code 2 does not refuse a tool call.",
		"Whether PreToolUse fires for an MCP tool call, and under what tool_name, has not been observed. The outward-action danger category matches mcp__<server>__<tool> names, so until that is measured it is not known to refuse an MCP send on Codex.",
	},
}

// opencodeContract is opencode.
//
// opencode exposes no shell-command hook surface at all. Its lifecycle is
// reachable only from a JS/TS plugin, which is why this host had nothing
// deterministic wired and policy reached it through commands and skills alone.
// Registering an MCP server is not a substitute: the model decides whether to
// call a tool, and a control plane the model may skip is not deterministic.
//
// The plugin is .opencode/plugin/vibe-agent.js, in this repository, so the
// envelope is this toolkit's choice rather than a vendor's: flat and snake_case,
// the shape a small JS reader wants, recorded here like every other host's so
// the two sides have one source.
var opencodeContract = HostContract{
	Client:     "opencode",
	Tools:      ToolVocabulary{Writes: []string{"edit", "write", "patch", "multiedit"}},
	Dialect:    Dialect{Context: ContextFlat, Refusal: RefusePermissionReason, StopBlock: StopNone, PromptInjection: true},
	ConfigPath: "opencode.json",
	Source:     "https://opencode.ai/docs/plugins/",
	WorkspaceRoot: WorkspaceRoot{
		Variable: "directory",
		Reliable: true,
		Note: "A plugin receives the project directory in its PluginInput, so it passes --workspace without guessing. " +
			"The plugin itself lives at .opencode/plugin/, singular, which is loaded automatically; the `plugin` array " +
			"in opencode.json is for npm packages and registering a local file there is not how it is picked up. " +
			"The directory name was measured out of the opencode 1.14.39 binary, which contains \".opencode/plugin\", " +
			"because the documentation page says \"plugins\" and a plugin in the wrong directory is never loaded at all.",
	},
	Events: []EventContract{
		{
			HostKey: "experimental.chat.system.transform", Event: EventSessionStart,
			OutputKeys: []string{"additional_context"},
			CanInject:  true, Wired: true,
			Verification: unverified(opencodePluginUnmeasured),
			Note: "The injection point, and the only one that reaches every request. Named experimental " +
				"by the vendor, so it is the part of this plugin most likely to need rewriting; the gate " +
				"and the journal below use stable hooks and do not depend on it.",
		},
		{
			HostKey: "chat.message", Event: EventUserPromptSubmit,
			Wired:        false,
			Verification: unverified(opencodePluginUnmeasured),
			Note: "Fires per prompt but its output is the message and its parts, so injecting means " +
				"editing what the person typed. The system transform above says the same thing without that.",
		},
		{
			HostKey: "tool.execute.before", Event: EventPreToolUse,
			Wired: false, CanRefuse: false,
			Verification: unverified(opencodePluginUnmeasured),
			Note: "Deliberately unwired. Refusing here means throwing, which the model reads as a broken " +
				"tool rather than as a decision about one, and consulting the gate twice per call would " +
				"double the cost to say the same thing. permission.ask carries the verdict.",
		},
		{
			HostKey: "tool.execute.after", Event: EventPostToolUse,
			Wired:        true,
			Verification: unverified(opencodePluginUnmeasured),
			Note:         "The journal. Its output carries title, output and metadata, and no exit status.",
		},
		{
			HostKey: "permission.ask", Event: EventPreToolUse,
			OutputKeys: []string{"permission", "reason"},
			CanRefuse:  true, Wired: true,
			Verification: unverified(opencodePluginUnmeasured),
			Note: "The refusal path. opencode's own field is status, accepting ask, deny or allow; the " +
				"plugin maps this envelope's permission onto it, so the runtime speaks one shape and the " +
				"plugin owns the translation.",
		},
	},
	Gaps: []string{
		"No shell-command hook surface. Everything deterministic must go through a JS/TS plugin, so a workspace without one gets no journalling, no gate, and no injection.",
		"Registering an MCP server is not a substitute: the model decides whether to call a tool, and a control plane the model may skip is not deterministic.",
		"No failure event and no exit status on tool.execute.after, so a failed command cannot be told from a successful one. The same gap as Codex, reached by a different route.",
		"No end-of-turn hook, so nothing can refuse to end a turn with a run mid-graph the way stop does on Claude and Cursor.",
		"permission.ask reports a permission type rather than a tool name, and how an MCP tool appears there has not been observed. The outward-action danger category matches mcp__<server>__<tool> names, so it is not known to refuse an MCP send on opencode.",
	},
}

const antigravityNeverObserved = "No Antigravity hook has been observed firing from this config. " +
	"The envelope matches https://antigravity.google/docs/hooks and nobody here has run the binary."

const kimiNeverObserved = "No Kimi hook has been observed firing from this config. The shapes are the vendor's own " +
	"documentation (MoonshotAI/kimi-code docs/en/customization/hooks.md), read rather than measured."

const museNeverObserved = "No Muse hook has been observed firing from this config. Payload keys and the deny " +
	"behaviour come from an independent live measurement (pinta-ai/pinta-musecode README), not from this repository."

// antigravityContract is Google Antigravity.
var antigravityContract = HostContract{
	Client:     "antigravity",
	Tools:      ToolVocabulary{Writes: []string{"write_to_file", "replace_file_content", "multi_replace_file_content"}},
	Dialect:    Dialect{Context: ContextSteps, Refusal: RefuseDecision, StopBlock: StopContinue, PromptInjection: true},
	ConfigPath: ".agents/hooks.json",
	Source:     "https://antigravity.google/docs/hooks",
	WorkspaceRoot: WorkspaceRoot{
		Reliable: false,
		Note: "Hook commands receive workspacePaths on stdin as an array. This toolkit passes an explicit --workspace " +
			"from the config rather than picking one path from the array.",
	},
	Events: []EventContract{
		{
			HostKey: "PreInvocation", Event: EventUserPromptSubmit,
			OutputKeys: []string{"injectSteps"},
			CanInject:  true, Wired: true,
			Verification: unverified(antigravityNeverObserved),
			Note:         "Antigravity has no SessionStart event; run context rides on PreInvocation via injectSteps.ephemeralMessage.",
		},
		{
			HostKey: "PreToolUse", Event: EventPreToolUse,
			OutputKeys: []string{"decision", "reason", "permissionOverrides"},
			CanRefuse:  true, Wired: true,
			Verification: unverified(antigravityNeverObserved),
			Note:         "Shell commands arrive as toolCall.args.CommandLine on run_command.",
		},
		{
			HostKey: "PostToolUse", Event: EventPostToolUse,
			OutputKeys: nil, Wired: true,
			Verification: unverified(antigravityNeverObserved),
			Note:         "Failures share this event via a top-level error string; there is no separate failure hook.",
		},
		{
			HostKey: "Stop", Event: EventStop,
			OutputKeys: []string{"decision", "reason"},
			CanRefuse:  true, Wired: true,
			Verification: unverified(antigravityNeverObserved),
			Note:         "Set decision to continue to keep the loop running; any other value allows the stop.",
		},
	},
	Gaps: []string{
		"No SessionStart event; first-turn steering uses PreInvocation instead.",
		"No PostToolUseFailure event; journal a failed tool from PostToolUse when error is non-empty.",
		"workspacePaths is an array; a hook must not assume a single checkout root from stdin alone.",
		"Stdin is camelCase (conversationId, transcriptPath, toolCall.name/args); the payload adapter maps it.",
		"User-level hooks also load from ~/.gemini/config/hooks.json.",
	},
}

// kimiContract is Kimi Code CLI, the successor to kimi-cli.
//
// Read from the vendor's documentation rather than measured. Three facts set its
// dialect apart from the hosts it resembles. Whatever a UserPromptSubmit hook
// prints is appended to the context as text, so the envelope is the text
// itself. Blocking is exit 2 with the reason on stderr, the only channel Stop
// documents; PreToolUse also takes a JSON deny, and both are sent. SessionStart
// is observation-only, so what it prints is discarded.
var kimiContract = HostContract{
	Client:            "kimi",
	Tools:             ToolVocabulary{Writes: append([]string{"WriteFile", "EditFile", "StrReplaceFile"}, sharedEditTools...)},
	SplitsToolOutcome: true,
	Dialect: Dialect{
		Context: ContextPlain, Refusal: RefuseHookSpecific, RefusalExits: true, StopBlock: StopExit,
		PromptInjection: true,
	},
	ConfigPath: ".kimi-code/hooks.toml",
	Source:     "https://moonshotai.github.io/kimi-code/en/customization/hooks.html",
	WorkspaceRoot: WorkspaceRoot{
		Reliable: true,
		Note: "The vendor documents a hook command's working directory as the session's project directory. Hooks are read " +
			"only from the user's ~/.kimi-code/config.toml ($KIMI_CODE_HOME); the workspace file is a copy-ready snippet.",
	},
	Events: []EventContract{
		{
			HostKey: "SessionStart", Event: EventSessionStart,
			Wired: true, Verification: unverified(kimiNeverObserved),
			Note: "Observation-only: stdout is discarded, so the session context arrives with the first prompt instead.",
		},
		{
			HostKey: "UserPromptSubmit", Event: EventUserPromptSubmit,
			OutputKeys: []string{"stdout"},
			CanInject:  true, CanRefuse: true, Wired: true,
			Verification: unverified(kimiNeverObserved),
			Note:         "Text printed with exit 0 is appended to the context; the field is the text itself.",
		},
		{
			HostKey: "PreToolUse", Event: EventPreToolUse,
			OutputKeys: []string{"hookSpecificOutput.hookEventName", "hookSpecificOutput.permissionDecision", "hookSpecificOutput.permissionDecisionReason"},
			CanRefuse:  true, Wired: true,
			Verification: unverified(kimiNeverObserved),
			Note: "The documented deny carries permissionDecision and its reason; hookEventName is sent beside them, as for the " +
				"hosts that require it. Exit 2 with the reason on stderr also blocks; both are sent.",
		},
		{
			HostKey: "PostToolUse", Event: EventPostToolUse,
			Wired: true, Verification: unverified(kimiNeverObserved),
		},
		{
			HostKey: "PostToolUseFailure", Event: EventPostToolUseFailure,
			Wired: true, Verification: unverified(kimiNeverObserved),
			Note: "Fires after a tool fails or is blocked.",
		},
		{
			HostKey: "Stop", Event: EventStop,
			CanRefuse: true, Wired: true,
			Verification: unverified(kimiNeverObserved),
			Note:         "Blocked by exit 2; stderr is appended so the model continues.",
		},
	},
	Gaps: []string{
		"Hooks are user-level only: ~/.kimi-code/config.toml. A repository cannot wire them; someone merges the snippet.",
		"[[hooks]] accepts exactly event, matcher, command, and timeout; any other key stops the config from loading.",
		"Fail-open: a non-2 exit, a crash, or a timeout lets the action proceed.",
	},
}

// museContract is Muse Code.
//
// The payload is the shared family's (session_id, prompt, tool_name, tool_input,
// stop_hook_active). What makes its dialect different was measured live by an
// independent adapter: the host fails open on every response except exit 2, and
// a JSON deny in the wrong shape is ignored without a word. So refusals travel
// on both channels.
var museContract = HostContract{
	Client:            "muse",
	Tools:             ToolVocabulary{Writes: sharedEditTools, Fetches: sharedFetchTools},
	SplitsToolOutcome: true,
	Dialect:           Dialect{Refusal: RefuseHookSpecific, RefusalExits: true, PromptInjection: true},
	ConfigPath:        ".muse/hooks.json",
	Source:            "https://dev.meta.ai/docs/muse-code/extending",
	WorkspaceRoot: WorkspaceRoot{
		Reliable: false,
		Note:     "Project hooks at .muse/hooks.json run only after the project folder is trusted. Pass --workspace explicitly.",
	},
	Events: []EventContract{
		{
			HostKey: "SessionStart", Event: EventSessionStart,
			OutputKeys: []string{"hookSpecificOutput.hookEventName", "hookSpecificOutput.additionalContext"},
			CanInject:  true, Wired: true,
			Verification: unverified(museNeverObserved),
		},
		{
			HostKey: "UserPromptSubmit", Event: EventUserPromptSubmit,
			OutputKeys: []string{"hookSpecificOutput.hookEventName", "hookSpecificOutput.additionalContext"},
			CanInject:  true, Wired: true,
			Verification: unverified(museNeverObserved),
		},
		{
			HostKey: "PreToolUse", Event: EventPreToolUse,
			OutputKeys: []string{"hookSpecificOutput.hookEventName", "hookSpecificOutput.permissionDecision", "hookSpecificOutput.permissionDecisionReason"},
			CanRefuse:  true, Wired: true,
			Verification: unverified(museNeverObserved),
			Note:         "The binary's validation requires hookEventName to match the firing event; exit 2 is sent as well.",
		},
		{
			HostKey: "PostToolUse", Event: EventPostToolUse,
			Wired: true, Verification: unverified(museNeverObserved),
		},
		{
			HostKey: "PostToolUseFailure", Event: EventPostToolUseFailure,
			Wired: true, Verification: unverified(museNeverObserved),
		},
		{
			HostKey: "Stop", Event: EventStop,
			OutputKeys: []string{"decision", "reason"},
			CanRefuse:  true, Wired: true,
			Verification: unverified(museNeverObserved),
			Note:         "{\"decision\":\"block\"} was measured blocking a prompt; Stop carries stop_hook_active.",
		},
	},
	Gaps: []string{
		"Project hooks need the folder trusted first; the link script installs config only.",
		"Fails open on everything except exit 2, so a JSON-only refusal can be ignored silently.",
		"Treats .muse, .git and .agents as read-only inside the workspace.",
	},
}
