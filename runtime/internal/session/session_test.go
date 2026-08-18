package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const testSecret = "sk-0123456789abcdef0123456789ab"

func TestAppendRedactsPromptSecret(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, LogName)

	body := "Please deploy with key " + testSecret + " tonight"
	_, err := Append(path, Record{
		Type:   TypePromptSubmit,
		Source: SourceHook,
		Client: "claude",
		Body:   body,
		At:     time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if strings.Contains(text, testSecret) {
		t.Fatalf("log still contains secret: %s", text)
	}
	if !strings.Contains(text, redactedMarker) || !strings.Contains(text, "credential") {
		t.Fatalf("expected redaction markers in: %s", text)
	}
}

func TestAppendRedactsToolCommand(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, LogName)

	command := "curl -H 'Authorization: Bearer " + testSecret + "' https://example.com"
	_, err := Append(path, Record{
		Type:    TypeToolUse,
		Source:  SourceHook,
		Client:  "cursor",
		Tool:    "bash",
		Command: command,
		At:      time.Date(2026, 8, 18, 10, 1, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), testSecret) {
		t.Fatalf("command secret leaked: %s", raw)
	}
}

func TestAppendRejectsUnknownType(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, LogName)
	_, err := Append(path, Record{Type: "unknown_kind", Source: SourceHook})
	if err == nil {
		t.Fatal("expected error for unknown type")
	}
}

func TestReplayOrdersBySequenceAndSkipsUnknown(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, LogName)

	first, err := Append(path, Record{Type: TypeSessionStart, Source: SourceHook, Client: "claude", Event: "SessionStart"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := Append(path, Record{Type: TypePromptSubmit, Source: SourceHook, Client: "claude", Body: "hello"})
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(filepath.Clean(path), os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(`{"sequence":99,"type":"not_a_session_type","at":"2026-08-18T10:00:00Z"}` + "\n"); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	events, err := Replay(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("events = %d, want 2", len(events))
	}
	if events[0].Sequence != first.Sequence || events[1].Sequence != second.Sequence {
		t.Fatalf("order = %+v", events)
	}
}

func TestKindMapping(t *testing.T) {
	cases := []struct {
		name string
		rec  Record
		want FilterKind
	}{
		{"session start hook", Record{Type: TypeSessionStart, Source: SourceHook}, FilterHook},
		{"bash tool", Record{Type: TypeToolUse, Source: SourceHook, Tool: "bash"}, FilterTool},
		{"skill tool", Record{Type: TypeToolUse, Source: SourceHook, Tool: "Skill"}, FilterSkill},
		{"transcript", Record{Type: TypeTranscriptMessage, Source: SourceTranscript}, FilterTranscript},
	}
	dir := t.TempDir()
	path := filepath.Join(dir, LogName)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stored, err := Append(path, tc.rec)
			if err != nil {
				t.Fatal(err)
			}
			if got := Kind(stored); got != tc.want {
				t.Fatalf("Kind = %q, want %q", got, tc.want)
			}
		})
	}
}
