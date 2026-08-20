// Package app runs the inner loop: call the model, dispatch the tools it asked
// for, feed the results back, repeat until the turn ends or a budget does.
//
// This is the loop the runtime deliberately did not own for most of its life.
// It owns one now, bounded on purpose: it runs mechanical steps headlessly, and
// it does not replace a coding agent's own session. The outer loop is unchanged
// above it, still deciding which node runs next on recorded evidence.
package app

import (
	"context"
	"fmt"
	"time"

	agent "github.com/ducnd58233/vibe-agent/runtime/internal/agent/domain"
)

// Budget bounds one run of the loop.
//
// Three limits for the same reason run state carries three: turns bound how
// many times the model may be asked, tokens bound what it may cost, and a
// deadline bounds how long it may hold anything open. Zero is no limit on each,
// so a caller opts into the ones it cares about.
type Budget struct {
	MaxTurns  int
	MaxTokens int
	Deadline  time.Time
}

// DefaultMaxTurns bounds a budget that bounds nothing.
//
// A zero-value Budget reads as "no limits", and the one stop reason that
// continues a turn without adding a message is pause_turn, so a transport that
// keeps pausing would spin forever on a struct someone left empty. A loop with
// no ceiling is the failure mode that runs all night, and defaulting is
// cheaper than a knob nobody remembers to set.
const DefaultMaxTurns = 50

// withDefaults fills in the ceiling that stops an unbounded loop.
func (b Budget) withDefaults() Budget {
	if b.MaxTurns == 0 && b.MaxTokens == 0 && b.Deadline.IsZero() {
		b.MaxTurns = DefaultMaxTurns
	}
	return b
}

func (b Budget) exceeded(turns int, usage agent.Usage, now time.Time) string {
	switch {
	case b.MaxTurns > 0 && turns >= b.MaxTurns:
		return "turns"
	case b.MaxTokens > 0 && usage.Billable() >= b.MaxTokens:
		return "tokens"
	case !b.Deadline.IsZero() && now.After(b.Deadline):
		return "wallclock"
	default:
		return ""
	}
}

// Outcome is what a run of the loop produced.
type Outcome struct {
	// Stop is the last reason the model gave. Empty when a budget ended the run
	// before any reply arrived.
	Stop agent.StopReason
	// StoppedBy names the budget that ended the run, or "" when the model did.
	StoppedBy string
	// RefusalCategory carries the reason a refusal gave, when Stop is a refusal.
	RefusalCategory string
	Turns           int
	Usage           agent.Usage
	// Messages is the conversation as it ended, including the assistant turns
	// and the tool results fed back. A caller that wants to resume sends it
	// again; a caller that wants to audit reads it.
	Messages []agent.Message
}

// Done reports whether the model finished rather than a budget stopping it.
func (o Outcome) Done() bool { return o.StoppedBy == "" && o.Stop == agent.StopEndTurn }

// Loop drives a conversation to its end.
type Loop struct {
	Transport agent.Transport
	Tools     agent.ToolDispatcher
	Budget    Budget
	// Now is injectable so a deadline can be tested without waiting for one.
	Now func() time.Time
}

func (l *Loop) now() time.Time {
	if l.Now == nil {
		return time.Now()
	}
	return l.Now()
}

// Run drives the conversation until the model stops or a budget does.
//
// The order inside the loop is not arbitrary. The assistant's message joins the
// history before its tool calls are executed, because a tool result that
// referred to a call the history did not contain would be a malformed
// conversation. Every result from one turn goes back in a single user message,
// because splitting them teaches the model to stop asking for tools in
// parallel.
func (l *Loop) Run(ctx context.Context, conversation agent.Conversation) (Outcome, error) {
	if l.Transport == nil {
		return Outcome{}, fmt.Errorf("loop has no transport")
	}

	budget := l.Budget.withDefaults()
	outcome := Outcome{Messages: append([]agent.Message{}, conversation.Messages...)}
	for {
		if stoppedBy := budget.exceeded(outcome.Turns, outcome.Usage, l.now()); stoppedBy != "" {
			outcome.StoppedBy = stoppedBy
			return outcome, nil
		}

		conversation.Messages = outcome.Messages
		reply, err := l.Transport.Send(ctx, conversation)
		outcome.Turns++
		outcome.Usage.Add(reply.Usage)
		if err != nil {
			return outcome, fmt.Errorf("%s: %w", l.Transport.Name(), err)
		}

		outcome.Stop = reply.StopReason
		outcome.RefusalCategory = reply.RefusalCategory

		// A paused turn is unfinished rather than answered. Send the
		// conversation back unchanged, without adding a user message, and
		// without recording an assistant turn that only exists to say it
		// stopped.
		if reply.StopReason == agent.StopPauseTurn {
			continue
		}

		outcome.Messages = append(outcome.Messages, reply.Message)
		if !reply.StopReason.Continues() {
			return outcome, nil
		}

		results, err := l.dispatch(ctx, reply.Message.ToolCalls)
		if err != nil {
			return outcome, err
		}
		outcome.Messages = append(outcome.Messages, agent.Message{
			Role:        agent.RoleUser,
			ToolResults: results,
		})
	}
}

// dispatch runs every call from one turn and returns every result.
//
// A turn that asked for tools and got none back would leave the model waiting
// on answers that are not coming, so an empty call list on a tool_use stop is
// an error rather than an empty user message.
func (l *Loop) dispatch(ctx context.Context, calls []agent.ToolCall) ([]agent.ToolResult, error) {
	if len(calls) == 0 {
		return nil, fmt.Errorf("the model stopped for tool use and asked for no tools")
	}
	if l.Tools == nil {
		return nil, fmt.Errorf("the model asked for %d tool(s) and the loop has no dispatcher", len(calls))
	}

	results := make([]agent.ToolResult, 0, len(calls))
	for _, call := range calls {
		results = append(results, l.Tools.Dispatch(ctx, call))
	}
	return results, nil
}
