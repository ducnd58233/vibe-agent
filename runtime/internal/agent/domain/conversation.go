package domain

import (
	"context"
	"encoding/json"
)

// The shapes here follow the Messages API contract rather than being invented:
// a turn ends for one of five reasons, tool results all travel in a single user
// message, and the assistant's own message joins the history before its tool
// calls are executed. Getting any of those wrong produces a loop that works on
// the happy path and misbehaves the moment a model does something normal.
//
// They stay provider-neutral all the same. This package names what a turn is;
// an adapter maps it to one vendor's wire format, and the loop above never
// learns which vendor answered.

// Role is who a message came from.
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	// RoleSystem is an operator instruction placed in the message list rather
	// than in a system field, which some models accept mid-conversation and
	// others do not. An adapter that cannot send one says so.
	RoleSystem Role = "system"
)

// StopReason is why a turn ended.
//
// Five, because that is what the API returns. A loop that only knows end_turn
// and tool_use treats the other three as "done", which turns a truncated
// answer, a paused server tool, and a safety refusal into the same silent
// success.
type StopReason string

const (
	// StopEndTurn is the model finishing normally.
	StopEndTurn StopReason = "end_turn"
	// StopMaxTokens is the response hitting its ceiling mid-thought. The text
	// is real but incomplete, which is not the same as an answer.
	StopMaxTokens StopReason = "max_tokens"
	// StopToolUse is the model asking for tools. The only reason that continues
	// a loop.
	StopToolUse StopReason = "tool_use"
	// StopPauseTurn is a long-running server-side tool yielding. The turn is
	// unfinished: send the conversation back unchanged to continue it, without
	// adding a user message.
	StopPauseTurn StopReason = "pause_turn"
	// StopRefusal is a safety classifier declining. It arrives as a success at
	// the transport level, so content must not be read before the reason is
	// checked.
	StopRefusal StopReason = "refusal"
)

// Continues reports whether this reason means the loop has more to do.
func (s StopReason) Continues() bool {
	return s == StopToolUse || s == StopPauseTurn
}

// ToolCall is the model asking for one tool.
type ToolCall struct {
	// ID pairs the call with its result. An adapter must echo it back
	// unchanged; nothing else links the two.
	ID    string
	Name  string
	Input json.RawMessage
}

// ToolResult is what a tool produced.
type ToolResult struct {
	// CallID is the ToolCall.ID this answers.
	CallID string
	// Content is the result as the model will read it.
	Content string
	// IsError marks a failure. A failed tool still returns a result: dropping
	// it leaves a call unanswered, which is a malformed conversation rather
	// than a quiet skip.
	IsError bool
}

// Message is one turn in the conversation.
type Message struct {
	Role Role
	Text string
	// ToolCalls belong to an assistant message, ToolResults to a user one.
	// Every result from one assistant turn travels in a single message:
	// splitting them teaches the model to stop asking for tools in parallel.
	ToolCalls   []ToolCall
	ToolResults []ToolResult
}

// ToolSpec is a tool offered to the model.
type ToolSpec struct {
	Name        string
	Description string
	// Schema is the JSON Schema for the input.
	Schema json.RawMessage
}

// Conversation is everything a transport needs to produce the next reply.
type Conversation struct {
	System   string
	Messages []Message
	Tools    []ToolSpec
}

// Usage is what a reply cost, as the provider reported it.
type Usage struct {
	Input     int
	Output    int
	CacheRead int
}

// Billable is what counts against a token budget. Cache reads are excluded
// because a cache hit is the cost being avoided rather than paid.
func (u Usage) Billable() int { return u.Input + u.Output }

// Add accumulates another reply's usage.
func (u *Usage) Add(other Usage) {
	u.Input += other.Input
	u.Output += other.Output
	u.CacheRead += other.CacheRead
}

// Reply is one model response.
type Reply struct {
	Message    Message
	StopReason StopReason
	// RefusalCategory is set only when StopReason is StopRefusal. Every other
	// reason leaves it empty, matching the API, where the detail field is null
	// for anything but a refusal.
	RefusalCategory string
	Usage           Usage
}

// Transport is one model call. It has no memory: the whole conversation goes
// out every time, which is what makes a run resumable from state alone.
type Transport interface {
	Name() string
	Send(ctx context.Context, conversation Conversation) (Reply, error)
}

// ToolDispatcher executes the tools a model asked for.
//
// It returns a result rather than an error for a failed tool, because a failure
// is something the model needs to read and react to, not something that ends
// the loop.
type ToolDispatcher interface {
	Dispatch(ctx context.Context, call ToolCall) ToolResult
}
