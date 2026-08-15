package harness

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/ducnd58233/vibe-agent/runtime/internal/memory"
	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
)

// The payloads here were captured from Claude Code 2.1.229, not written from the
// documentation. Three properties of the real shape are what this package got
// wrong, and only the first is visible in the docs:
//
//   - a failing tool call arrives as PostToolUseFailure. PostToolUse fires on
//     success only, so the failure the journal exists to record never reached it.
//   - the failure payload carries no tool_response at all. What the tool printed
//     is in "error", and reading only tool_response left every failure with no
//     detail.
//   - the interruption flag is "is_interrupt" at the top level, not "interrupted"
//     inside the response.
//
// tool_response on success is a bare JSON string, so unmarshalling it into a
// struct with an exit_code field yields nothing. No Claude payload carries an
// exit code in any field.
//
// Ref: https://code.claude.com/docs/en/hooks
const (
	claudeFailurePayload = `{
		"hook_event_name": "PostToolUseFailure",
		"tool_name": "Bash",
		"tool_input": {"command": "go build ./..."},
		"tool_use_id": "toolu_01Uz19Ck61bCqAir3ENcLnki",
		"error": "Exit code 2\nundefined: Foo",
		"is_interrupt": false,
		"duration_ms": 122
	}`

	claudeSuccessPayload = `{
		"hook_event_name": "PostToolUse",
		"tool_name": "Bash",
		"tool_input": {"command": "go test ./..."},
		"tool_response": "ok\texample\t0.2s"
	}`
)

// events reads back the run's log so a test can assert on what was recorded
// rather than on what the code was asked to record.
func events(t *testing.T, root string) []state.Event {
	t.Helper()
	raw, err := os.ReadFile(state.EventLogPath(root, "demo"))
	if err != nil {
		return nil
	}
	var log []state.Event
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		if line == "" {
			continue
		}
		var event state.Event
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatalf("event log holds a line that is not an event: %s", line)
		}
		log = append(log, event)
	}
	return log
}

func memories(t *testing.T, root string) []memory.Record {
	t.Helper()
	if _, err := os.Stat(memory.DBPath(root)); err != nil {
		return nil
	}
	store, err := memory.OpenAt(t.Context(), memory.DBPath(root))
	if err != nil {
		t.Fatalf("OpenAt: %v", err)
	}
	defer func() { _ = store.Close() }()
	records, err := store.List(t.Context(), root)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	return records
}

func TestPostToolUseFailureIsHandled(t *testing.T) {
	if !Handles(EventPostToolUseFailure) {
		t.Fatalf("this build does not handle %s, so every failing tool call is dropped",
			EventPostToolUseFailure)
	}
}

// A failed command is the whole reason the journal exists. It has to survive
// the trip with no exit code anywhere in the payload.
func TestFailingCommandIsJournalledAndRemembered(t *testing.T) {
	root := workspaceWithRun(t)

	invoke(t, Request{
		Event:         EventPostToolUseFailure,
		Client:        ClientClaude,
		WorkspaceRoot: root,
		Stdin:         strings.NewReader(claudeFailurePayload),
	})

	log := events(t, root)
	if len(log) != 1 {
		t.Fatalf("want one journal entry, got %d", len(log))
	}
	var recorded toolUse
	if err := json.Unmarshal(log[0].Payload, &recorded); err != nil {
		t.Fatalf("journal payload: %v", err)
	}
	if !recorded.Failed {
		t.Errorf("a PostToolUseFailure entry is not marked failed: %+v", recorded)
	}
	if recorded.Command != "go build ./..." {
		t.Errorf("command = %q, want the command from tool_input", recorded.Command)
	}

	stored := memories(t, root)
	if len(stored) != 1 {
		t.Fatalf("want one memory from a failed command, got %d", len(stored))
	}
	if stored[0].Status != memory.StatusConfirmed {
		t.Errorf("status = %s, want confirmed: a proposed memory is never retrieved",
			stored[0].Status)
	}
	if !strings.Contains(stored[0].Content, "go build ./...") {
		t.Errorf("memory does not name the command: %q", stored[0].Content)
	}
	// The host's own output is the evidence. Without it the memory says a
	// command failed and cannot say how.
	joined := strings.Join(stored[0].Evidence, " ")
	if !strings.Contains(joined, "undefined: Foo") {
		t.Errorf("evidence drops the tool output: %v", stored[0].Evidence)
	}
	// Claude puts the exit code in that text and in no field. Quoting the host
	// keeps the number available to whoever reads the memory without this
	// package claiming to have parsed one.
	if !strings.Contains(joined, "Exit code 2") {
		t.Errorf("evidence drops the host's own exit line: %v", stored[0].Evidence)
	}
}

// An interrupted call is one the human stopped, not one the code got wrong.
// Remembering it would fill the store with the user's own cancellations.
func TestInterruptedCallIsNotRemembered(t *testing.T) {
	root := workspaceWithRun(t)

	invoke(t, Request{
		Event:         EventPostToolUseFailure,
		Client:        ClientClaude,
		WorkspaceRoot: root,
		Stdin: strings.NewReader(`{
			"tool_name": "Bash",
			"tool_input": {"command": "sleep 600"},
			"error": "The user doesn't want to proceed with this tool use.",
			"is_interrupt": true
		}`),
	})

	if stored := memories(t, root); len(stored) != 0 {
		t.Errorf("an interrupted call produced %d memories: %+v", len(stored), stored)
	}
}

// Cursor names the failure text error_message where Claude names it error, and
// it was the only host whose failure event went unwired. Wiring it while reading
// only Claude's spelling would record every Cursor failure with no account of
// what broke - the same emptiness, one field deeper.
//
// Ref: https://cursor.com/docs/agent/hooks
func TestCursorFailureTextIsKept(t *testing.T) {
	root := workspaceWithRun(t)

	invoke(t, Request{
		Event:         EventPostToolUseFailure,
		Client:        ClientCursor,
		WorkspaceRoot: root,
		Stdin: strings.NewReader(`{
			"hook_event_name": "postToolUseFailure",
			"tool_name": "Bash",
			"tool_input": {"command": "npm test"},
			"error_message": "1 test failed: expected 200, got 500",
			"failure_type": "error",
			"duration": 1200,
			"is_interrupt": false
		}`),
	})

	stored := memories(t, root)
	if len(stored) != 1 {
		t.Fatalf("want one memory from a failed Cursor command, got %d", len(stored))
	}
	joined := strings.Join(stored[0].Evidence, " ")
	if !strings.Contains(joined, "expected 200, got 500") {
		t.Errorf("evidence drops Cursor's error_message: %v", stored[0].Evidence)
	}
}

// A denied tool call and an interrupted one are the same event under two names:
// the tool never ran, and the person said no. Remembering it would fill the
// store with a record of the user declining rather than of anything breaking.
func TestADeniedCallIsNotRemembered(t *testing.T) {
	root := workspaceWithRun(t)

	invoke(t, Request{
		Event:         EventPostToolUseFailure,
		Client:        ClientCursor,
		WorkspaceRoot: root,
		Stdin: strings.NewReader(`{
			"tool_name": "Bash",
			"tool_input": {"command": "rm -rf build"},
			"error_message": "The user denied this command.",
			"failure_type": "permission_denied"
		}`),
	})

	if stored := memories(t, root); len(stored) != 0 {
		t.Errorf("a denied call produced %d memories: %+v", len(stored), stored)
	}
}

// The store this guard was written for held exactly one memory: a three-line
// shell probe typed while debugging, promoted to "fails in this workspace" and
// then read back into every prompt for a week. Debugging is mostly failing
// commands by design, so the journal has to keep recording them while the
// memory stops claiming each one is a fact about the repository.
func TestAnAdHocProbeIsJournalledButNotRemembered(t *testing.T) {
	root := workspaceWithRun(t)

	invoke(t, Request{
		Event:         EventPostToolUseFailure,
		Client:        ClientClaude,
		WorkspaceRoot: root,
		Stdin: strings.NewReader(`{
			"tool_name": "Bash",
			"tool_input": {"command": "V=\"/c/versions/2026.08.11\"\nF=$(grep -rl \"beforeShellExecution\" \"$V\" | head -3)\nfor f in $F; do echo \"--- $f\"; done"},
			"error": "grep: no such file or directory"
		}`),
	})

	if log := events(t, root); len(log) != 1 {
		t.Fatalf("the log still has to record what ran; got %d entries", len(log))
	}
	if stored := memories(t, root); len(stored) != 0 {
		t.Errorf("a multi-line probe produced %d memories: %+v", len(stored), stored)
	}
}

// Length is the other half of the same signal. A command assembled to answer one
// question runs once in that shape, so a memory of it can never match anything
// that comes back.
func TestAOneOffPipelineIsNotRemembered(t *testing.T) {
	root := workspaceWithRun(t)

	command := "docker compose -f docker-compose.test.yml run --rm api pytest -q " +
		strings.Repeat("tests/integration/test_one_specific_case.py ", 2)
	if len(command) <= memorableCommandLimit {
		t.Fatalf("fixture is %d chars, too short to reach the guard", len(command))
	}

	invoke(t, Request{
		Event:         EventPostToolUseFailure,
		Client:        ClientClaude,
		WorkspaceRoot: root,
		Stdin: strings.NewReader(fmt.Sprintf(
			`{"tool_name": "Bash", "tool_input": {"command": %q}, "error": "2 failed"}`, command)),
	})

	if log := events(t, root); len(log) != 1 {
		t.Fatalf("the log still has to record what ran; got %d entries", len(log))
	}
	if stored := memories(t, root); len(stored) != 0 {
		t.Errorf("a one-off pipeline produced %d memories: %+v", len(stored), stored)
	}
}

// The guard has to leave real project commands alone. This one is long enough to
// look unusual and short enough to run again tomorrow, which is the case a
// tighter limit would break.
func TestARealProjectCommandIsStillRemembered(t *testing.T) {
	root := workspaceWithRun(t)

	command := "go test ./internal/harness/... -run TestFailingCommand -count=1"

	invoke(t, Request{
		Event:         EventPostToolUseFailure,
		Client:        ClientClaude,
		WorkspaceRoot: root,
		Stdin: strings.NewReader(fmt.Sprintf(
			`{"tool_name": "Bash", "tool_input": {"command": %q}, "error": "FAIL"}`, command)),
	})

	stored := memories(t, root)
	if len(stored) != 1 {
		t.Fatalf("want one memory from a real command failure, got %d", len(stored))
	}
	if !strings.Contains(stored[0].Content, command) {
		t.Errorf("memory does not name the command: %q", stored[0].Content)
	}
}

// A success is worth logging and not worth remembering. Proposing a memory for
// every green command would bury the failures that matter.
func TestSucceedingCommandIsJournalledButNotRemembered(t *testing.T) {
	root := workspaceWithRun(t)

	invoke(t, Request{
		Event:         EventPostToolUse,
		Client:        ClientClaude,
		WorkspaceRoot: root,
		Stdin:         strings.NewReader(claudeSuccessPayload),
	})

	if log := events(t, root); len(log) != 1 {
		t.Fatalf("want one journal entry, got %d", len(log))
	}
	if stored := memories(t, root); len(stored) != 0 {
		t.Errorf("a successful command produced %d memories", len(stored))
	}
}

// The quiet path stays quiet. Without a run there is nothing to journal against,
// and seeding a database in a workspace nobody started a run in would litter.
func TestFailureWithoutARunWritesNothing(t *testing.T) {
	root := t.TempDir()

	invoke(t, Request{
		Event:         EventPostToolUseFailure,
		Client:        ClientClaude,
		WorkspaceRoot: root,
		Stdin:         strings.NewReader(claudeFailurePayload),
	})

	if _, err := os.Stat(memory.DBPath(root)); err == nil {
		t.Errorf("a workspace with no run got a memory database at %s", memory.DBPath(root))
	}
}

// Cursor reports an exit code where Claude does not. When one arrives it belongs
// in the record, because "exits 2" is more use later than "failed".
func TestExitCodeIsKeptWhenTheHostReportsOne(t *testing.T) {
	root := workspaceWithRun(t)

	invoke(t, Request{
		Event:         EventPostToolUseFailure,
		Client:        ClientCursor,
		WorkspaceRoot: root,
		Stdin: strings.NewReader(`{
			"tool_name": "Bash",
			"command": "pytest",
			"tool_response": {"exit_code": 1, "stderr": "2 failed"}
		}`),
	})

	log := events(t, root)
	if len(log) != 1 {
		t.Fatalf("want one journal entry, got %d", len(log))
	}
	var recorded toolUse
	if err := json.Unmarshal(log[0].Payload, &recorded); err != nil {
		t.Fatalf("journal payload: %v", err)
	}
	if recorded.ExitCode == nil || *recorded.ExitCode != 1 {
		t.Errorf("exit code lost: %+v", recorded)
	}
	stored := memories(t, root)
	if len(stored) != 1 {
		t.Fatalf("want one memory, got %d", len(stored))
	}
	if !strings.Contains(stored[0].Content, "exits 1") {
		t.Errorf("content = %q, want it to name the exit code", stored[0].Content)
	}
}
