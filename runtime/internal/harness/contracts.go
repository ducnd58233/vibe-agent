package harness

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
	Source        string
	WorkspaceRoot WorkspaceRoot
	Events        []EventContract
	// Gaps are the things this host does not provide. They matter as much as the
	// events: a gap recorded here is one nobody tries to wire a sixth time.
	Gaps []string
}

// hostContracts is the table. Order is the order the document renders in.
var hostContracts = []HostContract{claudeContract, cursorContract, codexContract, opencodeContract}

// HostContractFor returns a host's contract.
func HostContractFor(client Client) (HostContract, bool) {
	for _, contract := range hostContracts {
		if contract.Client == client {
			return contract, true
		}
	}
	return HostContract{}, false
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
	Client:     ClientClaude,
	ConfigPath: ".claude/settings.json",
	Source:     "https://code.claude.com/docs/en/hooks",
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
			Note: "This toolkit refuses through exit 2 and stderr here rather than the JSON shape; both are documented.",
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
	Client:     ClientCursor,
	ConfigPath: ".cursor/hooks.json",
	Source:     "https://cursor.com/docs/agent/hooks",
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
		"No per-prompt context injection. beforeSubmitPrompt can only validate or block, so the run's current node and matching memories never reach a Cursor session.",
		"No documented project-directory variable and no documented cwd for hook commands.",
	},
}

// cursorNeverObserved is the one reason every Cursor row carries.
const cursorNeverObserved = "No Cursor hook has been observed firing in this workspace. " +
	"cursor-agent 2026.08.11 failed a hook command of `true` before the hook process started, " +
	"so this row is read from the vendor page, not measured."

// codexContract is Codex.
//
// The envelopes here were established by experiment rather than inference,
// because the documentation and the binary disagreed twice. a8f2c64 records the
// first: Codex ignores exit 2 outright, running the command anyway while the
// hook exited 2 and printed its refusal, so the JSON shape is the only gate that
// works. The second is the missing failure event below.
var codexContract = HostContract{
	Client:     ClientCodex,
	ConfigPath: ".codex/hooks.json",
	Source:     "https://learn.chatgpt.com/docs/hooks",
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
			OutputKeys: nil, Wired: true,
			Verification: unverified("Codex's blocking shape for Stop has not been measured, so the runtime sends nothing here."),
		},
		{
			HostKey: "SubagentStop", Event: EventSubagentStop,
			OutputKeys: nil, Wired: true,
			Verification: unverified("Same as Stop."),
		},
	},
	Gaps: []string{
		"No failure event exists, and PostToolUse does not fire for a failed command despite the documentation saying it does. A failed command therefore cannot be journalled on Codex. This gap is the host's and cannot be wired shut.",
		"Exit code 2 does not refuse a tool call.",
	},
}

// opencodeContract is opencode.
//
// opencode exposes no shell-command hook surface at all. Its lifecycle is
// reachable only from a JS/TS plugin, which is why this host had nothing
// deterministic wired and policy reached it through commands and skills alone.
var opencodeContract = HostContract{
	Client:     ClientOpencode,
	ConfigPath: "opencode.json",
	Source:     "https://opencode.ai/docs/plugins/",
	WorkspaceRoot: WorkspaceRoot{
		Variable: "directory",
		Reliable: true,
		Note:     "A plugin receives the project directory in its PluginInput, so it can pass --workspace without guessing.",
	},
	Events: []EventContract{
		{
			HostKey: "event", Event: EventSessionStart,
			CanInject: false, Wired: false,
			Verification: unverified("No opencode plugin is committed yet."),
			Note:         "Session lifecycle arrives on the generic event stream rather than a named hook.",
		},
		{
			HostKey: "chat.message", Event: EventUserPromptSubmit,
			Wired:        false,
			Verification: unverified("No opencode plugin is committed yet."),
		},
		{
			HostKey: "tool.execute.before", Event: EventPreToolUse,
			Wired: false, CanRefuse: true,
			Verification: unverified("No opencode plugin is committed yet."),
		},
		{
			HostKey: "tool.execute.after", Event: EventPostToolUse,
			Wired:        false,
			Verification: unverified("No opencode plugin is committed yet."),
		},
		{
			HostKey: "permission.ask", Event: EventPreToolUse,
			OutputKeys: []string{"status"},
			CanRefuse:  true, Wired: false,
			Verification: unverified("No opencode plugin is committed yet."),
			Note:         "status accepts ask, deny or allow. This is the refusal path.",
		},
	},
	Gaps: []string{
		"No shell-command hook surface. Everything deterministic must go through a JS/TS plugin, so a workspace without one gets no journalling, no gate, and no injection.",
		"Registering an MCP server is not a substitute: the model decides whether to call a tool, and a control plane the model may skip is not deterministic.",
	},
}
