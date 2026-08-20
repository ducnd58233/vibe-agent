package testutil

import (
	"context"
	"fmt"
	"sync"

	agent "github.com/ducnd58233/vibe-agent/runtime/internal/agent/domain"
)

// ScriptedTransport answers model calls from a fixed list, in order.
//
// The inner loop is the one part of this runtime that would otherwise need a
// network and an API key to test at all, which in practice means not being
// tested. A scripted transport makes the loop's behavior, rather than a
// provider's availability, the thing under test.
type ScriptedTransport struct {
	// Replies are returned in order. Running past the end is an error rather
	// than a repeat: a loop that asked more times than the script described has
	// done something the test did not intend.
	Replies []agent.Reply
	// Err, when set, is returned instead of the next reply.
	Err error

	mu   sync.Mutex
	Sent []agent.Conversation
}

func (t *ScriptedTransport) Name() string { return "scripted" }

func (t *ScriptedTransport) Send(_ context.Context, conversation agent.Conversation) (agent.Reply, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	// The conversation is copied because the loop reuses its slice, and a test
	// that asserted on it later would otherwise see the final state every time.
	snapshot := conversation
	snapshot.Messages = append([]agent.Message{}, conversation.Messages...)
	t.Sent = append(t.Sent, snapshot)

	if t.Err != nil {
		return agent.Reply{}, t.Err
	}
	index := len(t.Sent) - 1
	if index >= len(t.Replies) {
		return agent.Reply{}, fmt.Errorf("scripted transport has %d replies and was asked %d times",
			len(t.Replies), len(t.Sent))
	}
	return t.Replies[index], nil
}

// Calls reports how many times the transport was asked.
func (t *ScriptedTransport) Calls() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.Sent)
}

// EchoDispatcher answers every tool call with a fixed body, recording what it
// was asked. Failing is opt-in per tool name.
type EchoDispatcher struct {
	Body string
	// FailFor names the tools that should come back as errors, so a test can
	// prove a failed tool still returns a result rather than ending the loop.
	FailFor map[string]bool

	mu    sync.Mutex
	Calls []agent.ToolCall
}

func (d *EchoDispatcher) Dispatch(_ context.Context, call agent.ToolCall) agent.ToolResult {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.Calls = append(d.Calls, call)
	body := d.Body
	if body == "" {
		body = "ok"
	}
	return agent.ToolResult{
		CallID:  call.ID,
		Content: body,
		IsError: d.FailFor[call.Name],
	}
}

// Asked reports how many tool calls the dispatcher received.
func (d *EchoDispatcher) Asked() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.Calls)
}
