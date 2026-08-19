package view

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
	"github.com/ducnd58233/vibe-agent/runtime/internal/session"
)

func TestEventsAfterForViewChatDemotesIntermediateAssistants(t *testing.T) {
	dir := t.TempDir()
	logPath := session.AmbientLogPath(dir)
	stamp := time.Date(2026, 8, 19, 10, 0, 0, 0, time.UTC)
	for i, rec := range []session.Record{
		{Type: session.TypePromptSubmit, Source: session.SourceHook, Client: "cursor", Body: "start", At: stamp},
		{Type: session.TypeTranscriptMessage, Source: session.SourceTranscript, Role: "assistant", Body: "step 1", At: stamp.Add(time.Second)},
		{Type: session.TypeTranscriptMessage, Source: session.SourceTranscript, Role: "assistant", Body: "step 2", At: stamp.Add(2 * time.Second)},
		{
			Type:    session.TypeTranscriptMessage,
			Source:  session.SourceTranscript,
			Role:    "assistant",
			Body:    "final",
			Usage:   &session.Usage{Input: 60, Output: 30, CacheRead: 0},
			At:      stamp.Add(3 * time.Second),
			Event:   "",
			Tool:    "",
			Command: "",
		},
	} {
		if _, err := session.Append(logPath, rec); err != nil {
			t.Fatalf("append record %d: %v", i, err)
		}
	}

	rows, err := EventsAfterForView(dir, "ambient", 0, "chat")
	if err != nil {
		t.Fatalf("EventsAfterForView: %v", err)
	}

	var (
		thinkingFold    bool
		thinkingSummary string
		thinkingBody    string
		finalHasUsage   bool
	)
	for _, row := range rows {
		if row.Role == "thinking" {
			thinkingFold = row.FoldClosed
			thinkingSummary = row.Summary
			thinkingBody = row.Body
		}
		if row.Body == "final" {
			finalHasUsage = row.HasUsage && row.Role == "assistant"
		}
	}

	if !strings.Contains(thinkingBody, "step 1") || !strings.Contains(thinkingBody, "step 2") {
		t.Fatalf("merged thinking should include intermediate steps, got %q", thinkingBody)
	}
	if thinkingSummary != "agent progress" {
		t.Fatalf("thinking summary = %q, want %q", thinkingSummary, "agent progress")
	}
	if !thinkingFold {
		t.Fatal("thinking should be fold-closed when demoted")
	}
	if !finalHasUsage {
		t.Fatal("final row should stay assistant with usage")
	}
}

func TestEventsAfterForViewTrajectoryKeepsAssistantProgress(t *testing.T) {
	dir := t.TempDir()
	logPath := session.AmbientLogPath(dir)
	stamp := time.Date(2026, 8, 19, 10, 0, 0, 0, time.UTC)
	for i, rec := range []session.Record{
		{Type: session.TypePromptSubmit, Source: session.SourceHook, Client: "cursor", Body: "start", At: stamp},
		{Type: session.TypeTranscriptMessage, Source: session.SourceTranscript, Role: "assistant", Body: "step 1", At: stamp.Add(time.Second)},
		{
			Type:    session.TypeTranscriptMessage,
			Source:  session.SourceTranscript,
			Role:    "assistant",
			Body:    "final",
			Usage:   &session.Usage{Input: 60, Output: 30, CacheRead: 0},
			At:      stamp.Add(2 * time.Second),
			Event:   "",
			Tool:    "",
			Command: "",
		},
	} {
		if _, err := session.Append(logPath, rec); err != nil {
			t.Fatalf("append record %d: %v", i, err)
		}
	}

	rows, err := EventsAfterForView(dir, "ambient", 0, "trajectory")
	if err != nil {
		t.Fatalf("EventsAfterForView: %v", err)
	}

	var step1Role string
	for _, row := range rows {
		if row.Body == "step 1" {
			step1Role = row.Role
			break
		}
	}
	if step1Role != "assistant" {
		t.Fatalf("step 1 role = %q, want %q", step1Role, "assistant")
	}
}

func TestEventsAfterReturnsNewRowsOnly(t *testing.T) {
	root := t.TempDir()
	slug := "tail-test"
	if err := os.MkdirAll(filepath.Join(root, "tmp", slug), 0o750); err != nil {
		t.Fatal(err)
	}
	path := session.LogPath(root, slug)
	stamp := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	first, err := session.Append(path, session.Record{
		Type: session.TypePromptSubmit, Source: session.SourceHook, Client: "claude", Body: "first", At: stamp,
	})
	if err != nil {
		t.Fatal(err)
	}
	all, err := EventsAfter(root, slug, 0)
	if err != nil || len(all) != 1 {
		t.Fatalf("all rows = %d err = %v", len(all), err)
	}
	none, err := EventsAfter(root, slug, first.Sequence)
	if err != nil || len(none) != 0 {
		t.Fatalf("after first = %d err = %v", len(none), err)
	}
	second, err := session.Append(path, session.Record{
		Type: session.TypePromptSubmit, Source: session.SourceHook, Client: "claude", Body: "second", At: stamp,
	})
	if err != nil {
		t.Fatal(err)
	}
	tail, err := EventsAfter(root, slug, first.Sequence)
	if err != nil {
		t.Fatal(err)
	}
	if len(tail) != 1 || tail[0].Seq != second.Sequence {
		t.Fatalf("tail = %+v want seq %d", tail, second.Sequence)
	}
}

func TestLastSequenceEmptyLog(t *testing.T) {
	root := t.TempDir()
	if LastSequence(root, "missing") != 0 {
		t.Fatal("expected zero for missing log")
	}
}

func TestTrajectoryRowsMergesGraphTransitions(t *testing.T) {
	root := t.TempDir()
	slug := "graph-tail"
	if err := os.MkdirAll(filepath.Join(root, "tmp", slug), 0o750); err != nil {
		t.Fatal(err)
	}
	stamp := time.Date(2026, 8, 19, 3, 0, 0, 0, time.UTC)
	if _, err := state.AppendEvent(state.EventLogPath(root, slug), state.Event{
		Type: "run_started", Node: "intake", At: stamp, Payload: []byte(`{"goal":"graph on trajectory"}`),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := state.AppendEvent(state.EventLogPath(root, slug), state.Event{
		Type: "tool_use", Node: "intake", At: stamp.Add(time.Second), Payload: []byte(`{"tool":"Bash","command":"true"}`),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := state.AppendEvent(state.EventLogPath(root, slug), state.Event{
		Type: "transition", Node: "research", At: stamp.Add(2 * time.Second),
		Payload: []byte(`{"from":"intake","to":"research","via":"goal_clear=true"}`),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := session.Append(session.LogPath(root, slug), session.Record{
		Type: session.TypePromptSubmit, Source: session.SourceHook, Client: "cursor", Body: "fix graph rows", At: stamp.Add(time.Second),
	}); err != nil {
		t.Fatal(err)
	}
	rows, err := TrajectoryRows(root, slug)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Fatalf("rows = %d want 3", len(rows))
	}
	if rows[0].Kind != session.FilterGraph || rows[0].Seq != 1 || rows[0].Summary != "Run started" {
		t.Fatalf("first = %+v", rows[0])
	}
	if rows[1].Role != "user" || rows[1].Seq != 2 {
		t.Fatalf("prompt = %+v", rows[1])
	}
	if rows[2].Kind != session.FilterGraph || rows[2].Seq != 3 || rows[2].Summary != "research" {
		t.Fatalf("transition = %+v", rows[2])
	}
	tail, err := EventsAfter(root, slug, 1)
	if err != nil || len(tail) != 2 {
		t.Fatalf("after 1 = %d err = %v", len(tail), err)
	}
}
