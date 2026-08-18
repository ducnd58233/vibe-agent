package harness

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ducnd58233/vibe-agent/runtime/internal/session"
)

const hookTestSecret = "sk-0123456789abcdef0123456789ab"

func sessionLog(t *testing.T, root string) []session.Event {
	t.Helper()
	path := session.LogPath(root, "demo")
	events, err := session.Replay(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("Replay: %v", err)
	}
	return events
}

func ambientSessionLog(t *testing.T, root string) []session.Event {
	t.Helper()
	path := session.AmbientLogPath(root)
	events, err := session.Replay(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("Replay ambient: %v", err)
	}
	return events
}

func TestSessionStartWritesSessionLog(t *testing.T) {
	root := workspaceWithRun(t)
	invoke(t, Request{
		Event: EventSessionStart, Client: ClientClaude, WorkspaceRoot: root,
	})
	events := sessionLog(t, root)
	if len(events) != 1 || events[0].Type != session.TypeSessionStart {
		t.Fatalf("events = %+v", events)
	}
}

func TestCursorPromptSubmitRecordsSessionWithoutOutput(t *testing.T) {
	root := workspaceWithRun(t)
	output := invoke(t, Request{
		Event: EventUserPromptSubmit, Client: ClientCursor,
		WorkspaceRoot: root,
		Stdin:         strings.NewReader(`{"prompt":"status check"}`),
	})
	if output != "" {
		t.Fatalf("unexpected output: %s", output)
	}
	events := sessionLog(t, root)
	if len(events) != 1 || events[0].Type != session.TypePromptSubmit {
		t.Fatalf("events = %+v", events)
	}
}

func TestPreToolUseRecordsSession(t *testing.T) {
	root := workspaceWithRun(t)
	invoke(t, Request{
		Event: EventPreToolUse, Client: ClientClaude, WorkspaceRoot: root,
		Stdin: strings.NewReader(`{"tool_name":"Bash","tool_input":{"command":"echo hi"}}`),
	})
	events := sessionLog(t, root)
	if len(events) != 1 || events[0].Type != session.TypePreTool {
		t.Fatalf("events = %+v", events)
	}
	got := sessionPayload(t, events[0])
	if got.Tool != "Bash" || got.Command != "echo hi" {
		t.Fatalf("payload = %+v", got)
	}
}

func TestCursorBeforeShellRecordsCommand(t *testing.T) {
	root := workspaceWithRun(t)
	invoke(t, Request{
		Event: EventPreToolUse, Client: ClientCursor, WorkspaceRoot: root,
		Stdin: strings.NewReader(`{"command":"git status","cwd":"/project"}`),
	})
	events := sessionLog(t, root)
	if len(events) != 1 || events[0].Type != session.TypePreTool {
		t.Fatalf("events = %+v", events)
	}
	got := sessionPayload(t, events[0])
	if got.Tool != "Shell" || got.Command != "git status" {
		t.Fatalf("payload = %+v, want Shell / git status", got)
	}
}

func TestCursorStringToolInputRecordsCommand(t *testing.T) {
	root := workspaceWithRun(t)
	invoke(t, Request{
		Event: EventPostToolUse, Client: ClientCursor, WorkspaceRoot: root,
		Stdin: strings.NewReader(`{"tool_name":"Shell","tool_input":"{\"command\":\"npm test\"}"}`),
	})
	events := sessionLog(t, root)
	if len(events) != 1 || events[0].Type != session.TypeToolUse {
		t.Fatalf("events = %+v", events)
	}
	got := sessionPayload(t, events[0])
	if got.Tool != "Shell" || got.Command != "npm test" {
		t.Fatalf("payload = %+v, want Shell / npm test", got)
	}
}

func sessionPayload(t *testing.T, ev session.Event) session.Payload {
	t.Helper()
	var body session.Payload
	if err := json.Unmarshal(ev.Payload, &body); err != nil {
		t.Fatalf("payload: %v", err)
	}
	return body
}

func TestPostToolUseRecordsSessionWithoutSecret(t *testing.T) {
	root := workspaceWithRun(t)
	cmd := "curl -H 'Authorization: Bearer " + hookTestSecret + "'"
	invoke(t, Request{
		Event: EventPostToolUse, Client: ClientClaude, WorkspaceRoot: root,
		Stdin: strings.NewReader(`{"tool_name":"Bash","tool_input":{"command":"` + cmd + `"}}`),
	})
	path := session.LogPath(root, "demo")
	raw, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if strings.Contains(text, hookTestSecret) {
		t.Fatalf("secret leaked into session log: %s", text)
	}
	events := sessionLog(t, root)
	if len(events) != 1 || events[0].Type != session.TypeToolUse {
		t.Fatalf("events = %+v", events)
	}
}

func TestStopRecordsAssistantMessageWhenPresent(t *testing.T) {
	root := workspaceWithRun(t)
	msg := "Done. Key was " + hookTestSecret
	invoke(t, Request{
		Event: EventStop, Client: ClientClaude, WorkspaceRoot: root,
		Stdin: strings.NewReader(`{"last_assistant_message":` + jsonString(msg) + `}`),
	})
	events := sessionLog(t, root)
	if len(events) != 2 {
		t.Fatalf("events = %+v", events)
	}
	if events[0].Type != session.TypeStop {
		t.Fatalf("first event = %+v", events[0])
	}
	if events[1].Type != session.TypeTranscriptMessage {
		t.Fatalf("second event = %+v", events[1])
	}
	path := session.LogPath(root, "demo")
	raw, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), hookTestSecret) {
		t.Fatalf("assistant secret leaked: %s", raw)
	}
}

func TestStopWithoutAssistantMessageDoesNotInventOne(t *testing.T) {
	root := workspaceWithRun(t)
	invoke(t, Request{
		Event: EventStop, Client: ClientClaude, WorkspaceRoot: root,
	})
	events := sessionLog(t, root)
	if len(events) != 1 || events[0].Type != session.TypeStop {
		t.Fatalf("events = %+v", events)
	}
}

func TestSessionLogUsesAmbientWhenNoRun(t *testing.T) {
	root := t.TempDir()
	invoke(t, Request{
		Event: EventSessionStart, Client: ClientClaude, WorkspaceRoot: root,
	})
	events := ambientSessionLog(t, root)
	if len(events) != 1 {
		t.Fatalf("ambient events = %+v", events)
	}
}

func TestCodexFailureGapIsDocumented(t *testing.T) {
	contract, ok := HostContractFor(ClientCodex)
	if !ok {
		t.Fatal("Codex contract not found")
	}
	for _, gap := range contract.Gaps {
		if strings.Contains(gap, "cannot be journalled on Codex") {
			return
		}
	}
	t.Fatalf("Codex contract should document the failure gap: %v", contract.Gaps)
}

func jsonString(text string) string {
	escaped := strings.ReplaceAll(text, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	return `"` + escaped + `"`
}
