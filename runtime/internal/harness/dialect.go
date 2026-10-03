package harness

import "io"

// Dialect is how one host spells what the hooks say back, as data.
//
// The hooks decide what to tell a session and whether to refuse a call; hosts
// disagree only on the envelope those answers travel in. That disagreement used
// to live as a branch on the host's name at each place something was written,
// so adding a host meant finding every branch and a branch left out answered in
// the wrong shape without failing. Here each host states its spelling once, in
// its contract row, and one renderer per question reads it. Nothing outside the
// contract table names a host.
//
// The zero value is the nested shape, the one most hosts share.
type Dialect struct {
	// Context is how text added to the model's context is wrapped.
	Context ContextShape
	// Refusal is how a refused tool call is reported.
	Refusal RefusalShape
	// StopBlock is how a refusal to end the turn is reported.
	StopBlock StopShape
	// PostTool is how advice after a completed tool call is reported.
	PostTool PostToolShape
	// RefusalExits also exits with the blocking status after writing the
	// refusal body. For a host measured to fail open on everything but that
	// status, the JSON alone can be ignored without a word, so both channels
	// carry the same refusal.
	RefusalExits bool

	// PromptInjection is true when the prompt-submit event can add context. A
	// host that cannot is sent nothing there: blocking the person to deliver a
	// reminder would be a worse behaviour than saying nothing.
	PromptInjection bool
	// StopAdvisory is true when a stop event with nothing to block may still
	// carry a message for the person.
	StopAdvisory bool
	// SteersSessionStart is true when a fresh session may be handed an initial
	// user message pointing at the run in flight.
	SteersSessionStart bool
	// ToolUseNodeReminder is true when the run's current node is delivered after
	// a tool call, because nothing earlier in the turn can carry it.
	ToolUseNodeReminder bool
}

// ContextShape names an envelope for added context.
type ContextShape string

const (
	// ContextNested is hookSpecificOutput.additionalContext, and the default.
	ContextNested ContextShape = ""
	// ContextFlat is a top-level additional_context field.
	ContextFlat ContextShape = "flat"
	// ContextSteps is injectSteps carrying an ephemeral message.
	ContextSteps ContextShape = "steps"
	// ContextPlain is the text itself on stdout, for a host that appends
	// whatever a hook prints to the model's context. A JSON envelope there
	// would arrive as literal JSON.
	ContextPlain ContextShape = "plain"
)

// RefusalShape names how a PreToolUse refusal reaches the host.
type RefusalShape string

const (
	// RefuseExit surfaces the refusal as a BlockError, which main turns into the
	// host's blocking exit code.
	RefuseExit RefusalShape = ""
	// RefuseHookSpecific is hookSpecificOutput.permissionDecision "deny", for
	// hosts that ignore the exit code.
	RefuseHookSpecific RefusalShape = "hook-specific"
	// RefusePermission is {permission: deny} with a message for the agent and one
	// for the person.
	RefusePermission RefusalShape = "permission"
	// RefusePermissionReason is {permission: deny, reason}.
	RefusePermissionReason RefusalShape = "permission-reason"
	// RefuseDecision is {decision: deny, reason}.
	RefuseDecision RefusalShape = "decision"
)

// StopShape names how a refusal to end the turn reaches the host.
type StopShape string

const (
	// StopDecisionBlock is {decision: block, reason}, and the default.
	StopDecisionBlock StopShape = ""
	// StopFollowup is {followup_message}, the host's only stop output.
	StopFollowup StopShape = "followup"
	// StopContinue is {decision: continue, reason}.
	StopContinue StopShape = "continue"
	// StopNone means the host has no end-of-turn hook, so nothing can be refused
	// and saying nothing is the honest answer.
	StopNone StopShape = "none"
	// StopExit refuses through the blocking exit status, with the reason on
	// stderr, for a host whose stop hook reads nothing from stdout.
	StopExit StopShape = "exit"
)

// PostToolShape names how advice after a tool call reaches the host.
type PostToolShape string

const (
	// PostToolBoth sends systemMessage for the person and additionalContext for
	// the model; whichever the host does not read is ignored, and a warning only
	// the person can see is one the agent repeats on the next edit.
	PostToolBoth PostToolShape = ""
	// PostToolFlat sends the single additional_context field.
	PostToolFlat PostToolShape = "flat"
)

// refusalPersonNote is what the person is told when a host reports a refusal
// with a separate message for them.
const refusalPersonNote = "vibe-agent blocked a command that would bypass the delivery graph."

// dialectFor returns a host's spelling. A host this build has no contract for
// gets the zero Dialect; KnownClient is what keeps one from getting that far.
func dialectFor(client Client) Dialect {
	contract, _ := HostContractFor(client)
	return contract.Dialect
}

// contextBody is the envelope that adds text to the model's context.
func contextBody(d Dialect, event, text string) map[string]any {
	switch d.Context {
	case ContextFlat:
		return map[string]any{"additional_context": text}
	case ContextSteps:
		return map[string]any{"injectSteps": []map[string]any{{"ephemeralMessage": text}}}
	case ContextPlain:
		return nil
	default:
		return map[string]any{"hookSpecificOutput": map[string]any{
			"hookEventName":     event,
			"additionalContext": text,
		}}
	}
}

// refusalBody is the envelope reporting a refused tool call, or nil for a host
// that reads the refusal from the exit code instead.
func refusalBody(d Dialect, reason string) map[string]any {
	switch d.Refusal {
	case RefusePermission:
		return map[string]any{
			"permission":    "deny",
			"agent_message": reason,
			"user_message":  refusalPersonNote,
		}
	case RefusePermissionReason:
		return map[string]any{"permission": "deny", "reason": reason}
	case RefuseHookSpecific:
		return map[string]any{"hookSpecificOutput": map[string]any{
			"hookEventName":            "PreToolUse",
			"permissionDecision":       "deny",
			"permissionDecisionReason": reason,
		}}
	case RefuseDecision:
		return map[string]any{"decision": "deny", "reason": reason}
	default:
		return nil
	}
}

// stopBody is the envelope refusing to end the turn, or nil for a host with no
// end-of-turn hook.
func stopBody(d Dialect, reason string) map[string]any {
	switch d.StopBlock {
	case StopFollowup:
		return map[string]any{"followup_message": reason}
	case StopContinue:
		return map[string]any{"decision": "continue", "reason": reason}
	case StopNone, StopExit:
		return nil
	default:
		return map[string]any{"decision": "block", "reason": reason}
	}
}

// postToolBody is the envelope carrying advice after a completed tool call.
func postToolBody(d Dialect, text string) map[string]any {
	if d.PostTool == PostToolFlat {
		return map[string]any{"additional_context": text}
	}
	body := contextBody(Dialect{}, "PostToolUse", text)
	body["systemMessage"] = text
	return body
}

// writeContext writes text added to the model's context in the host's shape.
func writeContext(out io.Writer, d Dialect, event, text string) error {
	if d.Context == ContextPlain {
		_, err := io.WriteString(out, text+"\n")
		return err
	}
	return write(out, contextBody(d, event, text))
}

func writeBody(out io.Writer, body map[string]any) error {
	if body == nil {
		return nil
	}
	return write(out, body)
}
