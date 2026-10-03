package harness

// The inbound half of the host adapters: every field a host's hook payload may
// carry, under whichever spelling it uses. Hosts disagree on names ("prompt" and
// "user_prompt", "error" and "error_message"), and reading only one spelling
// gives every host that uses the other an empty value that looks like success.
// Everything past this file reads the methods below and never a host's own key,
// so the rest of the package stays host-neutral.

import (
	"encoding/json"
	"io"
	"strings"
)

// payload is the union of the fields this package reads from either host.
type payload struct {
	// raw is the host's original bytes, kept for hooks this package delegates
	// to another process rather than handles itself.
	raw []byte

	// Claude Code sends user_prompt; older builds and Cursor send prompt.
	// Reading both keeps one adapter working across versions.
	Prompt     string `json:"prompt"`
	UserPrompt string `json:"user_prompt"`

	// Hosts name the conversation differently; sessionKey reads either.
	SessionID      string `json:"session_id"`
	ConversationID string `json:"conversation_id"`

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
// sessionKey identifies the conversation, or "" when the host did not say. The
// injection ledger is scoped by it so two conversations in one workspace do not
// suppress each other's context.
func (p payload) sessionKey() string {
	if p.SessionID != "" {
		return p.SessionID
	}
	return p.ConversationID
}

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
