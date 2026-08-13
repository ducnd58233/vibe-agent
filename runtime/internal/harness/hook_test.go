package harness

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
)

const toolkitRoot = "../../.."

func at() time.Time { return time.Date(2026, 7, 29, 10, 0, 0, 0, time.UTC) }

func workspaceWithRun(t *testing.T, mutate ...func(*state.Run)) string {
	t.Helper()
	root := t.TempDir()
	run, err := state.NewRun("demo", "prove hooks work", "goal-delivery", 50, at())
	if err != nil {
		t.Fatalf("NewRun: %v", err)
	}
	run.CurrentNode = "test"
	for _, apply := range mutate {
		apply(run)
	}
	if err := state.Save(state.ManifestPath(root, "demo"), run); err != nil {
		t.Fatalf("Save: %v", err)
	}
	return root
}

func invoke(t *testing.T, req Request) string {
	t.Helper()
	var out bytes.Buffer
	if req.Stdin == nil {
		req.Stdin = strings.NewReader("{}")
	}
	if req.ToolkitRoot == "" {
		req.ToolkitRoot = toolkitRoot
	}
	if err := Run(req, &out); err != nil {
		t.Fatalf("Run(%s): %v", req.Event, err)
	}
	return out.String()
}

func TestSessionStartReportsTheSourceOfTruthOrder(t *testing.T) {
	output := invoke(t, Request{
		Event: EventSessionStart, Client: ClientClaude, WorkspaceRoot: t.TempDir(),
	})
	if !strings.Contains(output, "repository code and config") {
		t.Errorf("session start does not state the source-of-truth order: %s", output)
	}
}

func TestSessionStartSurfacesAnActiveRun(t *testing.T) {
	output := invoke(t, Request{
		Event: EventSessionStart, Client: ClientClaude, WorkspaceRoot: workspaceWithRun(t),
	})
	if !strings.Contains(output, "demo") || !strings.Contains(output, "test") {
		t.Errorf("an active run was not surfaced: %s", output)
	}
	if !strings.Contains(output, "Do not infer or manually advance") {
		t.Error("session start does not warn against advancing state by inference")
	}
}

func TestClaudeSessionStartUsesTheAdditionalContextField(t *testing.T) {
	output := invoke(t, Request{
		Event: EventSessionStart, Client: ClientClaude, WorkspaceRoot: t.TempDir(),
	})
	var body map[string]any
	if err := json.Unmarshal([]byte(output), &body); err != nil {
		t.Fatalf("output is not JSON: %v: %s", err, output)
	}
	specific, ok := body["hookSpecificOutput"].(map[string]any)
	if !ok {
		t.Fatalf("no hookSpecificOutput: %s", output)
	}
	if specific["additionalContext"] == nil {
		t.Error("Claude session start does not inject additionalContext")
	}
}

// Cursor's beforeSubmitPrompt returns {continue, user_message} and cannot add
// context. Emitting nothing is the honest behavior; blocking the user to
// deliver a reminder would be worse than staying quiet.
func TestCursorPromptSubmitEmitsNothing(t *testing.T) {
	output := invoke(t, Request{
		Event: EventUserPromptSubmit, Client: ClientCursor,
		WorkspaceRoot: workspaceWithRun(t),
		Stdin:         strings.NewReader(`{"prompt":"what is the goal status"}`),
	})
	if output != "" {
		t.Errorf("Cursor prompt submit produced output it cannot deliver: %s", output)
	}
}

func TestClaudePromptSubmitSurfacesTheCurrentNode(t *testing.T) {
	output := invoke(t, Request{
		Event: EventUserPromptSubmit, Client: ClientClaude,
		WorkspaceRoot: workspaceWithRun(t),
		Stdin:         strings.NewReader(`{"prompt":"what is the goal status"}`),
	})
	if !strings.Contains(output, "demo") {
		t.Errorf("prompt submit did not surface the run: %s", output)
	}
}

// Nothing to say is still the right answer when there is nothing to say. What
// changed is that a prompt not sounding like a status question is no longer
// treated as one of those cases; see TestPromptSubmitSurfacesTheRunOnAnyPrompt.
func TestPromptSubmitStaysQuietWithNothingToReport(t *testing.T) {
	output := invoke(t, Request{
		Event: EventUserPromptSubmit, Client: ClientClaude,
		WorkspaceRoot: t.TempDir(),
		Stdin:         strings.NewReader(`{"prompt":"explain this regex"}`),
	})
	if output != "" {
		t.Errorf("prompt submit spoke with no run and no memory: %s", output)
	}
}

func TestStopRemindsAboutAnUnfinishedRun(t *testing.T) {
	output := invoke(t, Request{
		Event: EventStop, Client: ClientClaude, WorkspaceRoot: workspaceWithRun(t),
	})
	if !strings.Contains(output, "Record evidence") {
		t.Errorf("stop did not remind about recording evidence: %s", output)
	}
}

func TestStopReportsABlockerWhenThereIsOne(t *testing.T) {
	root := workspaceWithRun(t, func(run *state.Run) {
		run.Blockers = []state.Blocker{{
			Node: "test", Reason: "flaky integration suite", Attempts: 2, At: at(),
		}}
	})
	output := invoke(t, Request{Event: EventStop, Client: ClientClaude, WorkspaceRoot: root})
	if !strings.Contains(output, "flaky integration suite") {
		t.Errorf("stop did not report the blocker: %s", output)
	}
}

// A control plane that can wedge a session is worse than one that stays quiet.
func TestHooksStayQuietWithNoRunAndNoWorkspace(t *testing.T) {
	for _, event := range []Event{EventStop, EventSubagentStop, EventUserPromptSubmit} {
		output := invoke(t, Request{
			Event: event, Client: ClientClaude, WorkspaceRoot: t.TempDir(),
		})
		if output != "" {
			t.Errorf("%s produced output with no active run: %s", event, output)
		}
	}
}

func TestHooksSurviveGarbageOnStdin(t *testing.T) {
	for _, event := range []Event{EventSessionStart, EventUserPromptSubmit, EventStop} {
		var out bytes.Buffer
		err := Run(Request{
			Event: event, Client: ClientClaude, WorkspaceRoot: t.TempDir(),
			ToolkitRoot: toolkitRoot, Stdin: strings.NewReader("not json at all"),
		}, &out)
		if err != nil {
			t.Errorf("%s failed on malformed stdin: %v", event, err)
		}
	}
}

func TestFinishedRunsAreNotReported(t *testing.T) {
	root := workspaceWithRun(t, func(run *state.Run) { run.Status = state.StatusDone })
	output := invoke(t, Request{Event: EventStop, Client: ClientClaude, WorkspaceRoot: root})
	if output != "" {
		t.Errorf("a finished run was reported as outstanding: %s", output)
	}
}

func TestUnknownEventIsAnError(t *testing.T) {
	var out bytes.Buffer
	if err := Run(Request{Event: "nonsense", WorkspaceRoot: t.TempDir()}, &out); err == nil {
		t.Error("an unknown hook event was accepted")
	}
}

// Events() is what doctor trusts when it says a hook is implemented, and Run's
// switch is what actually implements one. A constant added to the list and not to
// the switch would pass every wiring check and then fail at runtime, which is the
// shape of the bug this whole check exists for, one layer down.
func TestEveryAdvertisedEventIsActuallyHandled(t *testing.T) {
	for _, event := range Events() {
		var out bytes.Buffer
		err := Run(Request{
			Event:         event,
			WorkspaceRoot: t.TempDir(),
			Stdin:         strings.NewReader(`{"tool_name":"Bash","tool_input":{"command":"ls"}}`),
		}, &out)
		// A hook with nothing to say returns nil. What it must not do is refuse the
		// event, which is what an unhandled one does.
		if err != nil && strings.Contains(err.Error(), "unknown hook event") {
			t.Errorf("Events() advertises %q and Run does not handle it", event)
		}
	}
}

// The refusal has to name a cause. "unknown hook event" alone reads as a typo in
// the config, and the cause is almost always a binary older than the config
// calling it, which is a different fix.
func TestTheRefusalNamesWhatThisBuildHandles(t *testing.T) {
	var out bytes.Buffer
	err := Run(Request{Event: "post-tool-usage", WorkspaceRoot: t.TempDir()}, &out)
	if err == nil {
		t.Fatal("a misspelled event was accepted")
	}
	for _, want := range []string{"post-tool-use", "make install"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not mention %q, so it does not point at the fix: %v", want, err)
		}
	}
}
