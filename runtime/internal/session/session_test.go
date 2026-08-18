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
	if _, err := Append(path, Record{Type: TypeTranscriptMessage, Source: SourcePrint, Client: "cursor-agent", Role: "assistant", Body: "ok"}); err != nil {
		t.Fatalf("print source: %v", err)
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
	if len(events) != 3 {
		t.Fatalf("events = %d, want 3", len(events))
	}
	if events[0].Sequence != first.Sequence || events[1].Sequence != second.Sequence {
		t.Fatalf("order = %+v", events)
	}
}

func TestComposePrefixRedactsAndCaps(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, LogName)
	secret := testSecret
	if _, err := Append(path, Record{Type: TypePromptSubmit, Source: SourceHook, Client: "claude", Body: "deploy " + secret}); err != nil {
		t.Fatal(err)
	}
	if _, err := Append(path, Record{Type: TypeTranscriptMessage, Source: SourcePrint, Client: "claude", Role: "assistant", Body: "ok"}); err != nil {
		t.Fatal(err)
	}
	if _, err := Append(path, Record{Type: TypePromptSubmit, Source: SourceHook, Client: "claude", Body: "second"}); err != nil {
		t.Fatal(err)
	}
	if _, err := Append(path, Record{Type: TypeTranscriptMessage, Source: SourcePrint, Client: "claude", Role: "thinking", Body: "hidden"}); err != nil {
		t.Fatal(err)
	}
	events, err := Replay(path)
	if err != nil {
		t.Fatal(err)
	}
	prefix := ComposePrefix(events, 8, DefaultReplayBytes)
	if strings.Contains(prefix, secret) {
		t.Fatal("secret leaked into prefix")
	}
	if !strings.Contains(prefix, "User:") || !strings.Contains(prefix, "Assistant: ok") {
		t.Fatalf("prefix = %q", prefix)
	}
	if strings.Contains(prefix, "hidden") {
		t.Fatal("thinking must not enter the replay prefix")
	}
	capped := ComposePrefix(events, 1, DefaultReplayBytes)
	if strings.Contains(capped, "deploy") {
		t.Fatalf("oldest turn should drop: %q", capped)
	}
	if !strings.Contains(capped, "second") {
		t.Fatalf("newest turn missing: %q", capped)
	}
	tiny := ComposePrefix(events, 8, 8)
	if tiny == prefix {
		t.Fatal("byte cap should drop oldest lines")
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
