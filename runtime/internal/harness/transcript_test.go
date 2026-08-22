package harness

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ducnd58233/vibe-agent/runtime/internal/session"
)

const transcriptTestSecret = "sk-0123456789abcdef0123456789ab"

func writeTranscriptFile(t *testing.T, lines ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "transcript.jsonl")
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestTranscriptProjectsUserAndAssistant(t *testing.T) {
	root := workspaceWithRun(t)
	path := writeTranscriptFile(t,
		`{"type":"user","message":{"content":[{"type":"text","text":"hello there"}]}}`,
		`{"type":"assistant","message":{"content":[{"type":"text","text":"hi back"}]}}`,
	)
	invoke(t, Request{
		Event: EventStop, Client: ClientClaude, WorkspaceRoot: root,
		Stdin: strings.NewReader(`{"transcript_path":` + jsonString(path) + `}`),
	})

	events := sessionLog(t, root)
	var transcript []session.Event
	for _, ev := range events {
		if ev.Source == session.SourceTranscript {
			transcript = append(transcript, ev)
		}
	}
	if len(transcript) != 2 {
		t.Fatalf("transcript events = %+v", transcript)
	}
	if transcript[0].Type != session.TypeMessage || transcript[1].Type != session.TypeMessage {
		t.Fatalf("types = %+v", transcript)
	}
}

func TestTranscriptRedactsSecrets(t *testing.T) {
	root := workspaceWithRun(t)
	path := writeTranscriptFile(t,
		`{"type":"user","message":{"content":[{"type":"text","text":"use `+transcriptTestSecret+`"}]}}`,
	)
	invoke(t, Request{
		Event: EventStop, Client: ClientClaude, WorkspaceRoot: root,
		Stdin: strings.NewReader(`{"transcript_path":` + jsonString(path) + `}`),
	})
	raw, err := os.ReadFile(filepath.Clean(session.LogPath(root, "demo")))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), transcriptTestSecret) {
		t.Fatalf("secret leaked: %s", raw)
	}
}

func TestUnfamiliarTranscriptShapeAppendsNothing(t *testing.T) {
	root := workspaceWithRun(t)
	path := writeTranscriptFile(t, "not json", `{"unfamiliar":true}`)
	invoke(t, Request{
		Event: EventStop, Client: ClientClaude, WorkspaceRoot: root,
		Stdin: strings.NewReader(`{"transcript_path":` + jsonString(path) + `}`),
	})
	events := sessionLog(t, root)
	for _, ev := range events {
		if ev.Source == session.SourceTranscript {
			t.Fatalf("unexpected transcript event: %+v", ev)
		}
	}
}

func TestTranscriptUsageCopiedWhenPresent(t *testing.T) {
	root := workspaceWithRun(t)
	path := writeTranscriptFile(t,
		`{"type":"assistant","message":{"content":[{"type":"text","text":"done"}],"usage":{"input_tokens":120,"output_tokens":45,"cache_read_input_tokens":10}}}`,
	)
	invoke(t, Request{
		Event: EventStop, Client: ClientClaude, WorkspaceRoot: root,
		Stdin: strings.NewReader(`{"transcript_path":` + jsonString(path) + `}`),
	})
	events := sessionLog(t, root)
	if len(events) != 2 {
		t.Fatalf("events = %+v", events)
	}
	var body session.Payload
	if err := json.Unmarshal(events[1].Payload, &body); err != nil {
		t.Fatal(err)
	}
	if body.Usage == nil || body.Usage.Input != 120 || body.Usage.Output != 45 || body.Usage.CacheRead != 10 {
		t.Fatalf("usage = %+v", body.Usage)
	}
}

func TestTranscriptSkipsDuplicateAssistantFromStop(t *testing.T) {
	root := workspaceWithRun(t)
	assistant := "same assistant text"
	path := writeTranscriptFile(t,
		`{"type":"assistant","message":{"content":[{"type":"text","text":"`+assistant+`"}]}}`,
	)
	invoke(t, Request{
		Event: EventStop, Client: ClientClaude, WorkspaceRoot: root,
		Stdin: strings.NewReader(`{"transcript_path":` + jsonString(path) + `,"last_assistant_message":` + jsonString(assistant) + `}`),
	})
	events := sessionLog(t, root)
	transcriptCount := 0
	for _, ev := range events {
		if ev.Source == session.SourceTranscript && ev.Type == session.TypeMessage {
			transcriptCount++
		}
	}
	if transcriptCount != 0 {
		t.Fatalf("expected no duplicate transcript assistant, got %d events: %+v", transcriptCount, events)
	}
	if len(events) != 2 {
		t.Fatalf("expected stop + hook assistant, got %+v", events)
	}
}

func TestTranscriptCommandInjectionProjectsAsThinking(t *testing.T) {
	root := workspaceWithRun(t)
	path := writeTranscriptFile(t,
		`{"type":"user","message":{"content":[{"type":"text","text":"<command-message>goal</command-message>\n<command-name>/goal</command-name>"}]}}`,
		`{"type":"user","message":{"content":[{"type":"text","text":"Drive one objective end to end.\n\n<context>\n\nFollow the skill.\n</context>\n\n## Inputs"}]}}`,
	)
	invoke(t, Request{
		Event: EventStop, Client: ClientClaude, WorkspaceRoot: root,
		Stdin: strings.NewReader(`{"transcript_path":` + jsonString(path) + `}`),
	})
	var thinking int
	for _, ev := range sessionLog(t, root) {
		if ev.Source != session.SourceTranscript {
			continue
		}
		var body session.Payload
		if err := json.Unmarshal(ev.Payload, &body); err != nil {
			t.Fatal(err)
		}
		if body.Role == "thinking" {
			thinking++
		}
		if body.Role == "user" {
			t.Fatalf("command injection must not stay user, got %+v", body)
		}
	}
	if thinking != 2 {
		t.Fatalf("thinking rows = %d, want 2", thinking)
	}
}

func TestStopCopiesUsageFromSkippedTranscriptAssistant(t *testing.T) {
	root := workspaceWithRun(t)
	assistant := "same assistant text"
	path := writeTranscriptFile(t,
		`{"type":"assistant","message":{"content":[{"type":"text","text":"`+assistant+`"}],"usage":{"input_tokens":120,"output_tokens":45,"cache_read_input_tokens":10}}}`,
	)
	invoke(t, Request{
		Event: EventStop, Client: ClientClaude, WorkspaceRoot: root,
		Stdin: strings.NewReader(`{"transcript_path":` + jsonString(path) + `,"last_assistant_message":` + jsonString(assistant) + `}`),
	})
	var body session.Payload
	for _, ev := range sessionLog(t, root) {
		if ev.Type != session.TypeMessage || ev.Source != session.SourceHook {
			continue
		}
		if err := json.Unmarshal(ev.Payload, &body); err != nil {
			t.Fatal(err)
		}
	}
	if body.Usage == nil || body.Usage.Input != 120 || body.Usage.Output != 45 || body.Usage.CacheRead != 10 {
		t.Fatalf("usage = %+v", body.Usage)
	}
}

func TestTranscriptMissingUsageStaysUnset(t *testing.T) {
	root := workspaceWithRun(t)
	path := writeTranscriptFile(t,
		`{"type":"assistant","message":{"content":[{"type":"text","text":"no usage here"}]}}`,
	)
	invoke(t, Request{
		Event: EventStop, Client: ClientClaude, WorkspaceRoot: root,
		Stdin: strings.NewReader(`{"transcript_path":` + jsonString(path) + `}`),
	})
	var body session.Payload
	for _, ev := range sessionLog(t, root) {
		if ev.Source != session.SourceTranscript {
			continue
		}
		if err := json.Unmarshal(ev.Payload, &body); err != nil {
			t.Fatal(err)
		}
		if body.Usage != nil {
			t.Fatalf("usage should be unset, got %+v", body.Usage)
		}
	}
}
