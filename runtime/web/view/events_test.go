package view

import (
	"os"
	"path/filepath"
	"strings"
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
	if rows[2].Client != "cursor" || rows[2].Tool != "bash" || rows[2].Command != "echo hi" {
		t.Fatalf("tool row = %+v", rows[2])
	}
	if rows[2].Type != session.TypeToolUse || rows[2].At.IsZero() {
		t.Fatalf("tool row type/at = %+v", rows[2])
	}
	if rows[1].HasUsage {
		t.Fatal("user prompt must not inherit model usage")
	}
	if rows[1].BodyHTML == "" {
		t.Fatal("user prose should render as markdown HTML")
	}
	if rows[2].BodyHTML != "" {
		t.Fatal("tool command should stay plain pre, not markdown")
	}
	if !strings.Contains(string(rows[3].BodyHTML), "<p>") {
		t.Fatalf("assistant prose should wrap as HTML, got %q", rows[3].BodyHTML)
	}
}

func TestPromoteUsageMovesStopCountsOntoAssistant(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, session.LogName)
	stamp := time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC)
	for _, rec := range []session.Record{
		{Type: session.TypePromptSubmit, Source: session.SourceHook, Body: "hi", At: stamp},
		{Type: session.TypeTranscriptMessage, Source: session.SourceTranscript, Role: "assistant", Body: "done", At: stamp},
		{Type: session.TypeStop, Source: session.SourcePrint, Event: "ComposerStop", Usage: &session.Usage{Input: 9, Output: 2, CacheRead: 4}, At: stamp},
	} {
		if _, err := session.Append(path, rec); err != nil {
			t.Fatal(err)
		}
	}
	events, err := session.Replay(path)
	if err != nil {
		t.Fatal(err)
	}
	rows := ProjectEvents(events)
	if len(rows) != 3 {
		t.Fatalf("rows = %d", len(rows))
	}
	if !rows[1].HasUsage || rows[1].Usage == nil || rows[1].Usage.Input != 9 || rows[1].Usage.Output != 2 || rows[1].Usage.CacheRead != 4 {
		t.Fatalf("assistant usage = %+v", rows[1])
	}
	if rows[2].HasUsage {
		t.Fatal("ComposerStop must not keep usage after it moves to the assistant row")
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

func TestFormatToolbarTokensCombinedWhenOnlyTotal(t *testing.T) {
	got := FormatToolbarTokens(UsageTotals{Reported: true, Total: 88})
	if got != "tokens 88" {
		t.Fatalf("got %q", got)
	}
}

func TestChatFoldClosedOnLongAssistantBody(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, session.LogName)
	long := strings.Repeat("word ", 120)
	short := "short reply"
	stamp := time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC)
	for _, rec := range []session.Record{
		{Type: session.TypePromptSubmit, Source: session.SourceHook, Body: short, At: stamp},
		{Type: session.TypeTranscriptMessage, Source: session.SourceTranscript, Role: "assistant", Body: long, At: stamp},
		{Type: session.TypeToolUse, Source: session.SourceHook, Tool: "Read", Command: strings.Repeat("x", 600), At: stamp},
	} {
		if _, err := session.Append(path, rec); err != nil {
			t.Fatal(err)
		}
	}
	events, err := session.Replay(path)
	if err != nil {
		t.Fatal(err)
	}
	rows := ProjectEvents(events)
	if len(rows) != 3 {
		t.Fatalf("rows = %d", len(rows))
	}
	if rows[0].FoldClosed {
		t.Fatal("short user prompt must stay expanded")
	}
	if rows[1].FoldClosed {
		t.Fatal("assistant Chat rows stay full height, no Show more")
	}
	if rows[2].FoldClosed {
		t.Fatal("tool rows are Trajectory cards, not Chat folds")
	}
}

func TestEventSummaryComposerStartAndSessionEnd(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, session.LogName)
	records := []session.Record{
		{Type: session.TypeSessionStart, Source: session.SourcePrint, Client: "cursor-agent", Event: "ComposerStart"},
		{Type: session.TypeStop, Source: session.SourcePrint, Client: "cursor-agent", Event: "SessionEnd"},
		{Type: session.TypeStop, Source: session.SourcePrint, Client: "cursor-agent", Event: "ComposerStop"},
	}
	var events []session.Event
	for _, rec := range records {
		rec.At = time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC)
		ev, err := session.Append(path, rec)
		if err != nil {
			t.Fatal(err)
		}
		events = append(events, ev)
	}
	rows := ProjectEvents(events)
	if len(rows) != 3 {
		t.Fatalf("rows = %d", len(rows))
	}
	if !strings.HasPrefix(rows[0].Summary, "Composer start") {
		t.Fatalf("start summary = %q", rows[0].Summary)
	}
	if rows[1].Summary != "SessionEnd" {
		t.Fatalf("session end summary = %q", rows[1].Summary)
	}
	if !strings.HasPrefix(rows[2].Summary, "Composer stop") {
		t.Fatalf("stop summary = %q", rows[2].Summary)
	}
	if ChatVisibleRole(rows[0].Role) || ChatVisibleRole(rows[1].Role) {
		t.Fatal("lifecycle rows belong on Trajectory, not Chat")
	}
}

func TestChatRowsIncludesThinkingBeforeAssistant(t *testing.T) {
	rows := []EventRow{
		{Role: "thinking", Body: "I will read the file"},
		{Role: "assistant", Body: "done"},
		{Role: "user", Body: "go"},
	}
	chat := ChatRows(rows)
	if len(chat) != 3 || chat[0].Role != "thinking" || chat[1].Role != "assistant" || chat[2].Role != "user" {
		t.Fatalf("chat = %+v", chat)
	}
	if !ChatVisibleRole("thinking") {
		t.Fatal("thinking belongs on Chat, collapsed above the assistant reply")
	}
}

func TestThinkingChatRowStartsFolded(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, session.LogName)
	rec := session.Record{
		Type:   session.TypeTranscriptMessage,
		Source: session.SourcePrint,
		Role:   "thinking",
		Body:   "planning the edit",
		At:     time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC),
	}
	if _, err := session.Append(path, rec); err != nil {
		t.Fatal(err)
	}
	events, err := session.Replay(path)
	if err != nil {
		t.Fatal(err)
	}
	rows := ProjectEvents(events)
	if len(rows) != 1 || rows[0].Role != "thinking" || !rows[0].FoldClosed {
		t.Fatalf("rows = %+v", rows)
	}
}

func TestChatRowsKeepsHostQuestionOnTrajectory(t *testing.T) {
	rows := []EventRow{
		{Role: "tool", Body: "Read"},
		{Role: "question", Body: "Approve the spec?"},
		{Role: "user", Body: "yes"},
		{Role: "context", Body: "ComposerStart"},
	}
	chat := ChatRows(rows)
	if len(chat) != 1 || chat[0].Role != "user" {
		t.Fatalf("chat = %+v", chat)
	}
	if ChatVisibleRole("question") || ChatVisibleRole("context") || ChatVisibleRole("system") || ChatVisibleRole("tool") || ChatVisibleRole("hook") || ChatVisibleRole("graph") {
		t.Fatal("host questions and trace roles belong on Trajectory, not Chat")
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

func TestProjectPreToolUsesHookRoleAndDropsDuplicateBody(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, session.LogName)
	stamp := time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC)
	if _, err := session.Append(path, session.Record{
		Type:   session.TypePreTool,
		Source: session.SourceHook,
		Client: "cursor",
		Event:  "pre-tool-use",
		At:     stamp,
	}); err != nil {
		t.Fatal(err)
	}
	events, err := session.Replay(path)
	if err != nil {
		t.Fatal(err)
	}
	rows := ProjectEvents(events)
	if len(rows) != 1 {
		t.Fatalf("rows = %+v", rows)
	}
	row := rows[0]
	if row.Role != "hook" {
		t.Fatalf("role = %q, want hook so the gutter is not tool", row.Role)
	}
	if row.Kind != session.FilterHook {
		t.Fatalf("kind = %q, want hook", row.Kind)
	}
	if row.Summary != "PreToolUse" {
		t.Fatalf("summary = %q, want PreToolUse not the raw event slug", row.Summary)
	}
	if strings.TrimSpace(row.Body) != "" {
		t.Fatalf("body = %q, should not repeat the title", row.Body)
	}
}

func TestProjectPreToolShowsToolAndCommand(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, session.LogName)
	if _, err := session.Append(path, session.Record{
		Type:    session.TypePreTool,
		Source:  session.SourceHook,
		Client:  "cursor",
		Event:   "pre-tool-use",
		Tool:    "Shell",
		Command: "ls",
		At:      time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatal(err)
	}
	events, err := session.Replay(path)
	if err != nil {
		t.Fatal(err)
	}
	row := ProjectEvents(events)[0]
	if row.Role != "hook" || row.Summary != "Shell" || row.Body != "ls" {
		t.Fatalf("row = %+v", row)
	}
}

func TestProjectEmptyToolUseOmitsBody(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, session.LogName)
	if _, err := session.Append(path, session.Record{
		Type:   session.TypeToolUse,
		Source: session.SourceHook,
		Client: "cursor",
		At:     time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatal(err)
	}
	events, err := session.Replay(path)
	if err != nil {
		t.Fatal(err)
	}
	row := ProjectEvents(events)[0]
	if row.Role != "tool" || row.Kind != session.FilterTool {
		t.Fatalf("row = %+v", row)
	}
	if row.Summary != "ToolUse" {
		t.Fatalf("summary = %q", row.Summary)
	}
	if strings.TrimSpace(row.Body) != "" {
		t.Fatalf("empty tool_use must not invent a body, got %q", row.Body)
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
