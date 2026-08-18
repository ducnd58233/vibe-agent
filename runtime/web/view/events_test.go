package view

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/session"
)

func TestProjectEventsMixedRoles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, session.LogName)
	records := []session.Record{
		{Type: session.TypeSessionStart, Source: session.SourceHook, Client: "cursor", Event: "SessionStart"},
		{Type: session.TypePromptSubmit, Source: session.SourceHook, Client: "cursor", Body: "hello"},
		{Type: session.TypeToolUse, Source: session.SourceHook, Client: "cursor", Tool: "bash", Command: "echo hi"},
		{Type: session.TypeTranscriptMessage, Source: session.SourceTranscript, Role: "assistant", Body: "done", Usage: &session.Usage{Input: 10, Output: 5}},
	}
	events := make([]session.Event, 0, len(records))
	for _, rec := range records {
		rec.At = time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC)
		ev, err := session.Append(path, rec)
		if err != nil {
			t.Fatal(err)
		}
		events = append(events, ev)
	}
	rows := ProjectEvents(events)
	if len(rows) != 4 {
		t.Fatalf("rows = %d", len(rows))
	}
	wantRoles := []string{"system", "user", "tool", "assistant"}
	for i, want := range wantRoles {
		if rows[i].Role != want {
			t.Fatalf("row %d role = %q, want %q", i, rows[i].Role, want)
		}
	}
	if !rows[3].HasUsage {
		t.Fatal("expected usage on assistant row")
	}
}

func TestSumUsageAggregatesReportedRows(t *testing.T) {
	rows := []EventRow{
		{HasUsage: true, Usage: &session.Usage{Input: 100, Output: 40, CacheRead: 10}},
		{HasUsage: true, Usage: &session.Usage{Input: 20, Output: 6}},
		{HasUsage: false},
	}
	totals := SumUsage(rows)
	if !totals.Reported || totals.Input != 120 || totals.Output != 46 || totals.CacheRead != 10 {
		t.Fatalf("totals = %+v", totals)
	}
	if got := FormatToolbarTokens(totals); got == "not reported" {
		t.Fatal("expected reported toolbar text")
	}
}

func TestChatHasProseFalseForHookOnly(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, session.LogName)
	for _, rec := range []session.Record{
		{Type: session.TypeSessionStart, Source: session.SourceHook, Event: "SessionStart"},
		{Type: session.TypeToolUse, Source: session.SourceHook, Tool: "Read"},
	} {
		rec.At = time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC)
		if _, err := session.Append(path, rec); err != nil {
			t.Fatal(err)
		}
	}
	events, err := session.Replay(path)
	if err != nil {
		t.Fatal(err)
	}
	if ChatHasProse(ProjectEvents(events)) {
		t.Fatal("chat should be empty for hook/tool only log")
	}
}

func TestHostGapFromPayload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, session.LogName)
	raw := `{"sequence":1,"type":"tool_use","source":"hook","payload":{"source":"hook","client":"codex","tool":"bash","hostGap":true},"at":"2026-08-18T10:00:00Z"}` + "\n"
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	events, err := session.Replay(path)
	if err != nil {
		t.Fatal(err)
	}
	rows := ProjectEvents(events)
	if len(rows) != 1 || !rows[0].HostGap {
		t.Fatalf("host gap row = %+v", rows)
	}
}

func TestKindOrderIncludesSkill(t *testing.T) {
	if len(KindOrder) < 5 {
		t.Fatal("expected all filter kinds in order")
	}
	found := false
	for _, kind := range KindOrder {
		if kind == session.FilterSkill {
			found = true
		}
	}
	if !found {
		t.Fatal("skill kind missing from order")
	}
}
