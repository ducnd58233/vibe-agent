package harness

// The gate was reachable from one place: a hook reading a host's JSON on
// stdin. That was every caller there was, until the runtime grew a loop that
// dispatches tools itself.
//
// A second caller could have carried a second copy of the rules. It does not.
// This file is the seam, and it is deliberately thin: it builds the same
// payload shape the hook parses and hands it to the same verdict function, so
// there is one set of refusals and one place to change them. Two permission
// paths means one of them goes stale, and the stale one is always the one in
// use when it matters.

// ToolCall is a tool call in the shape the gate needs to judge it.
//
// Not the agent package's ToolCall: that one carries a JSON blob whose keys
// differ per tool. Naming the fields the gate actually reads keeps the caller
// responsible for saying what it is about to do, rather than the gate guessing
// from an untyped map.
type ToolCall struct {
	// Name is the tool, as a host would report it: Bash, Write, Edit, Read.
	Name string
	// Command is the shell command, for tools that run one.
	Command string
	// FilePath is the file a tool writes or edits.
	FilePath string
	// Content is the text going in. The credential gate needs it: a key
	// reaching a file is the event, and the path says nothing about it.
	Content string
}

// Verdict answers whether a tool call may run, by exactly the rules
// pre-tool-use enforces. A nil result means the call may proceed.
func Verdict(workspaceRoot string, call ToolCall) *BlockError {
	return verdict(Request{WorkspaceRoot: workspaceRoot}, call.payload())
}

// payload renders a call in the shape the hook half parses from stdin, so both
// entry points reach verdict with the same input.
func (c ToolCall) payload() payload {
	var body payload
	body.ToolName = c.Name
	body.ToolInput.Command = c.Command
	body.ToolInput.FilePath = c.FilePath
	body.ToolInput.Content = c.Content
	return body
}
