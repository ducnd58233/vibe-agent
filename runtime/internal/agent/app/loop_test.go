package app

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	agent "github.com/ducnd58233/vibe-agent/runtime/internal/agent/domain"
	"github.com/ducnd58233/vibe-agent/runtime/internal/testutil"
)

func ask(text string) agent.Conversation {
	return agent.Conversation{Messages: []agent.Message{{Role: agent.RoleUser, Text: text}}}
}

func toolUse(id, name string) agent.Reply {
	return agent.Reply{
		Message: agent.Message{
			Role:      agent.RoleAssistant,
			ToolCalls: []agent.ToolCall{{ID: id, Name: name}},
		},
		StopReason: agent.StopToolUse,
	}
}

func finished(text string) agent.Reply {
	return agent.Reply{
		Message:    agent.Message{Role: agent.RoleAssistant, Text: text},
		StopReason: agent.StopEndTurn,
	}
}

// No host binary, no network, no key. The inner loop is the one part of this
// runtime that would otherwise be untestable in CI.
func TestALoopFinishesOnTheFirstReply(t *testing.T) {
	transport := &testutil.ScriptedTransport{Replies: []agent.Reply{finished("done")}}
	loop := &Loop{Transport: transport}

	outcome, err := loop.Run(context.Background(), ask("hello"))
	if err != nil {
		t.Fatal(err)
	}
	if !outcome.Done() {
		t.Errorf("outcome = %+v, want a finished run", outcome)
	}
	if outcome.Turns != 1 || transport.Calls() != 1 {
		t.Errorf("turns = %d, calls = %d, want 1 and 1", outcome.Turns, transport.Calls())
	}
}

// Every result from one turn travels in a single user message. Splitting them
// teaches the model to stop asking for tools in parallel.
func TestEveryToolResultFromOneTurnTravelsInOneMessage(t *testing.T) {
	parallel := agent.Reply{
		Message: agent.Message{
			Role: agent.RoleAssistant,
			ToolCalls: []agent.ToolCall{
				{ID: "a", Name: "read"},
				{ID: "b", Name: "grep"},
			},
		},
		StopReason: agent.StopToolUse,
	}
	transport := &testutil.ScriptedTransport{Replies: []agent.Reply{parallel, finished("done")}}
	tools := &testutil.EchoDispatcher{}
	loop := &Loop{Transport: transport, Tools: tools}

	outcome, err := loop.Run(context.Background(), ask("look"))
	if err != nil {
		t.Fatal(err)
	}
	if tools.Asked() != 2 {
		t.Fatalf("dispatched %d calls, want 2", tools.Asked())
	}

	var resultMessages int
	for _, message := range outcome.Messages {
		if len(message.ToolResults) > 0 {
			resultMessages++
			if len(message.ToolResults) != 2 {
				t.Errorf("a result message carried %d results, want both", len(message.ToolResults))
			}
		}
	}
	if resultMessages != 1 {
		t.Errorf("results were split across %d messages, want 1", resultMessages)
	}
}

// A tool result that referred to a call the history did not contain would be a
// malformed conversation.
func TestTheAssistantTurnIsInHistoryBeforeItsResults(t *testing.T) {
	transport := &testutil.ScriptedTransport{
		Replies: []agent.Reply{toolUse("a", "read"), finished("done")},
	}
	loop := &Loop{Transport: transport, Tools: &testutil.EchoDispatcher{}}

	if _, err := loop.Run(context.Background(), ask("look")); err != nil {
		t.Fatal(err)
	}

	// The second call is the one carrying the results, so its history is where
	// the ordering shows.
	sent := transport.Sent[1].Messages
	var assistantAt, resultAt = -1, -1
	for i, message := range sent {
		if len(message.ToolCalls) > 0 {
			assistantAt = i
		}
		if len(message.ToolResults) > 0 {
			resultAt = i
		}
	}
	if assistantAt < 0 || resultAt < 0 {
		t.Fatalf("history is missing a half: %+v", sent)
	}
	if assistantAt > resultAt {
		t.Errorf("results at %d precede the call at %d", resultAt, assistantAt)
	}
}

// A failed tool still returns a result. Dropping it leaves a call unanswered.
func TestAFailedToolStillAnswersItsCall(t *testing.T) {
	transport := &testutil.ScriptedTransport{
		Replies: []agent.Reply{toolUse("a", "bash"), finished("recovered")},
	}
	tools := &testutil.EchoDispatcher{Body: "exit 1", FailFor: map[string]bool{"bash": true}}
	loop := &Loop{Transport: transport, Tools: tools}

	outcome, err := loop.Run(context.Background(), ask("run it"))
	if err != nil {
		t.Fatalf("a failing tool ended the loop: %v", err)
	}
	if !outcome.Done() {
		t.Errorf("outcome = %+v, want the model to have finished", outcome)
	}

	found := false
	for _, message := range outcome.Messages {
		for _, result := range message.ToolResults {
			if result.CallID == "a" && result.IsError {
				found = true
			}
		}
	}
	if !found {
		t.Error("the failure never reached the model as a result")
	}
}

// A paused turn is unfinished rather than answered: send it back unchanged.
func TestAPausedTurnResendsWithoutAddingAMessage(t *testing.T) {
	paused := agent.Reply{StopReason: agent.StopPauseTurn}
	transport := &testutil.ScriptedTransport{Replies: []agent.Reply{paused, finished("done")}}
	loop := &Loop{Transport: transport}

	outcome, err := loop.Run(context.Background(), ask("search"))
	if err != nil {
		t.Fatal(err)
	}
	if transport.Calls() != 2 {
		t.Fatalf("calls = %d, want the pause to have continued the turn", transport.Calls())
	}
	// One user message in, one assistant message out. The pause added nothing.
	if len(outcome.Messages) != 2 {
		t.Errorf("messages = %d, want 2; the pause left a turn behind", len(outcome.Messages))
	}
}

// A refusal arrives as a success at the transport level, so the reason has to
// be checked before the content is read.
func TestARefusalStopsAndKeepsItsCategory(t *testing.T) {
	refusal := agent.Reply{
		Message:         agent.Message{Role: agent.RoleAssistant},
		StopReason:      agent.StopRefusal,
		RefusalCategory: "cyber",
	}
	transport := &testutil.ScriptedTransport{Replies: []agent.Reply{refusal}}
	loop := &Loop{Transport: transport}

	outcome, err := loop.Run(context.Background(), ask("do something"))
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Done() {
		t.Error("a refusal was reported as a finished run")
	}
	if outcome.Stop != agent.StopRefusal || outcome.RefusalCategory != "cyber" {
		t.Errorf("stop = %q, category = %q", outcome.Stop, outcome.RefusalCategory)
	}
}

// Hitting the ceiling mid-thought is not the same as an answer.
func TestATruncatedReplyIsNotAFinishedRun(t *testing.T) {
	truncated := agent.Reply{
		Message:    agent.Message{Role: agent.RoleAssistant, Text: "half an ans"},
		StopReason: agent.StopMaxTokens,
	}
	transport := &testutil.ScriptedTransport{Replies: []agent.Reply{truncated}}
	loop := &Loop{Transport: transport}

	outcome, err := loop.Run(context.Background(), ask("write an essay"))
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Done() {
		t.Error("a truncated reply counted as done")
	}
	if transport.Calls() != 1 {
		t.Errorf("calls = %d; max_tokens should not continue a loop", transport.Calls())
	}
}

func TestEachBudgetStopsTheLoopAndNamesItself(t *testing.T) {
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	looping := []agent.Reply{
		toolUse("a", "read"), toolUse("b", "read"), toolUse("c", "read"), finished("done"),
	}

	for _, testCase := range []struct {
		name   string
		budget Budget
		now    time.Time
		want   string
	}{
		{"turns", Budget{MaxTurns: 2}, at, "turns"},
		{"tokens", Budget{MaxTokens: 10}, at, "tokens"},
		{"wallclock", Budget{Deadline: at.Add(-time.Second)}, at, "wallclock"},
	} {
		replies := make([]agent.Reply, len(looping))
		copy(replies, looping)
		for i := range replies {
			replies[i].Usage = agent.Usage{Input: 100, Output: 100}
		}
		transport := &testutil.ScriptedTransport{Replies: replies}
		loop := &Loop{
			Transport: transport,
			Tools:     &testutil.EchoDispatcher{},
			Budget:    testCase.budget,
			Now:       func() time.Time { return testCase.now },
		}

		outcome, err := loop.Run(context.Background(), ask("go"))
		if err != nil {
			t.Fatalf("%s: %v", testCase.name, err)
		}
		if outcome.StoppedBy != testCase.want {
			t.Errorf("%s: stoppedBy = %q, want %q", testCase.name, outcome.StoppedBy, testCase.want)
		}
		if outcome.Done() {
			t.Errorf("%s: a budget stop reported as done", testCase.name)
		}
	}
}

// Cache reads are the cost avoided rather than paid, so they must not consume a
// token budget.
func TestACacheReadDoesNotSpendTheTokenBudget(t *testing.T) {
	reply := finished("done")
	reply.Usage = agent.Usage{Input: 10, Output: 10, CacheRead: 100_000}
	transport := &testutil.ScriptedTransport{Replies: []agent.Reply{reply}}
	loop := &Loop{Transport: transport, Budget: Budget{MaxTokens: 1000}}

	outcome, err := loop.Run(context.Background(), ask("hello"))
	if err != nil {
		t.Fatal(err)
	}
	if outcome.StoppedBy != "" {
		t.Errorf("stoppedBy = %q; a cache read spent the budget", outcome.StoppedBy)
	}
	if outcome.Usage.CacheRead != 100_000 {
		t.Errorf("cache reads were not recorded: %+v", outcome.Usage)
	}
}

func TestAToolStopWithNoCallsIsAnError(t *testing.T) {
	empty := agent.Reply{Message: agent.Message{Role: agent.RoleAssistant}, StopReason: agent.StopToolUse}
	loop := &Loop{
		Transport: &testutil.ScriptedTransport{Replies: []agent.Reply{empty}},
		Tools:     &testutil.EchoDispatcher{},
	}

	_, err := loop.Run(context.Background(), ask("go"))
	if err == nil {
		t.Fatal("a tool stop with no tools was accepted")
	}
	if !strings.Contains(err.Error(), "no tools") {
		t.Errorf("error = %q", err)
	}
}

func TestAToolCallWithNoDispatcherIsAnError(t *testing.T) {
	loop := &Loop{Transport: &testutil.ScriptedTransport{Replies: []agent.Reply{toolUse("a", "read")}}}

	if _, err := loop.Run(context.Background(), ask("go")); err == nil {
		t.Fatal("a tool call ran with no dispatcher")
	}
}

func TestALoopWithNoTransportIsAnError(t *testing.T) {
	if _, err := (&Loop{}).Run(context.Background(), ask("go")); err == nil {
		t.Fatal("a loop ran with no transport")
	}
}

// A transport failure names which transport failed, because a run with two of
// them otherwise reports an error nobody can place.
func TestATransportFailureNamesTheTransport(t *testing.T) {
	transport := &testutil.ScriptedTransport{Err: errors.New("connection reset")}
	loop := &Loop{Transport: transport}

	_, err := loop.Run(context.Background(), ask("go"))
	if err == nil {
		t.Fatal("a failing transport produced no error")
	}
	if !strings.Contains(err.Error(), "scripted") {
		t.Errorf("error = %q, want it to name the transport", err)
	}
}
