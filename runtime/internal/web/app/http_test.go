package app

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
	"github.com/ducnd58233/vibe-agent/runtime/internal/session"
)

const testSecret = "sk-0123456789abcdef0123456789ab"

func testToolkitRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func TestEmptyShellRendersRequiredTestIDs(t *testing.T) {
	root := t.TempDir()
	handler, err := NewHandlerWithPort(root, testToolkitRoot(t), 3080)
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, id := range []string{"app-shell", "rail", "trajectory-empty", "workspace-path", "new-session-open", "composer"} {
		if !strings.Contains(text, `data-testid="`+id+`"`) {
			t.Fatalf("missing test id %q in %s", id, text)
		}
	}
	if !strings.Contains(text, root) {
		t.Fatalf("workspace path missing from shell")
	}
	assertNewSessionForm(t, text)
}

func TestSessionPageRendersEventList(t *testing.T) {
	root, slug := writeFixtureSession(t)
	handler, err := NewHandlerWithPort(root, testToolkitRoot(t), 3080)
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/session/"+slug, nil)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, id := range []string{
		"event-list", "event-pipeline", "kind-filter-open", "kind-filter",
		"kind-filter-clear", "event-tokens", "token-usage", "inspector", "inspector-pane",
		"chat-empty", "chat-prompts", "chat-prompt",
		"graph-view", "settings-dialog", "composer", "new-session-form",
		"composer-catalog", "composer-preview", "composer-description", "composer-file-panel",
		"composer-file-dialog", "composer-attach-file", "composer-attach-folder",
		"composer-host-open", "composer-host-menu", "composer-model", "composer-model-open", "composer-model-menu", "composer-mode-agent", "host-busy", "thinking-bar",
	} {
		if !strings.Contains(body, `data-testid="`+id+`"`) {
			t.Fatalf("missing test id %q", id)
		}
	}
	if !strings.Contains(body, "<ol class=\"event-list\"") {
		t.Fatal("expected ol event-list")
	}
	if !strings.Contains(body, `class="session-card"`) {
		t.Fatal("expected session card, not a bare link")
	}
	if !strings.Contains(body, `class="session-title"`) {
		t.Fatal("expected session title")
	}
	if strings.Contains(body, testSecret) {
		t.Fatalf("secret leaked into HTML: %s", body)
	}
	if !strings.Contains(body, "host gap") {
		t.Fatal("expected host gap chip in fixture")
	}
	if strings.Contains(body, `data-pane="summary"`) {
		t.Fatal("inspector panes should be a select, not a tab row")
	}
	if !strings.Contains(body, "docs/fixture-session/SPEC.md") {
		t.Fatal("expected expanded human_gate prompt on chat")
	}
	if strings.Contains(body, "Confirm this goal") {
		t.Fatal("approve_spec must not reuse the intake confirm title")
	}
	if !strings.Contains(body, `class="graph-desc"`) {
		t.Fatal("graph nodes must show the workflow description, not only the id")
	}
	if !strings.Contains(body, "<article class=\"event") {
		t.Fatal("event rows must be articles")
	}
	if !strings.Contains(body, `class="tag tag-in">in 100`) || !strings.Contains(body, `class="tag tag-out">out 40`) {
		t.Fatal("expected toolbar token chips from session usage")
	}
	assertNewSessionForm(t, body)
	if !strings.Contains(body, `class="dock"`) || !strings.Contains(body, `data-testid="composer"`) {
		t.Fatal("expected composer dock on the session page")
	}
	if strings.Contains(body, `data-testid="composer-file-open"`) && strings.Contains(body, `aria-controls="composer-file-dialog"`) {
		t.Fatal("Attach must not open the workspace explorer dialog")
	}
	if !strings.Contains(body, `data-tip="Ask is the default`) {
		t.Fatal("Agent checkbox needs a tooltip explaining ask vs agent")
	}
}

func TestSettingsDialogPinsURLAndHostChips(t *testing.T) {
	root, slug := writeEmptySession(t)
	handler, err := NewHandlerWithPort(root, testToolkitRoot(t), 3080)
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/session/"+slug, nil)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `class="settings-row"`) {
		t.Fatal("expected settings-row so URL sits on the start edge and the bind on the end")
	}
	if !strings.Contains(body, `localStorage.getItem("vibe-theme")`) {
		t.Fatal("settings theme must boot from localStorage before CSS paint")
	}
	if !strings.Contains(body, `data-testid="settings-bind"`) {
		t.Fatal("expected bind address")
	}
	if !strings.Contains(body, `class="chip chip-pass"`) && !strings.Contains(body, `class="chip chip-fail"`) {
		t.Fatal("expected PATH status as a chip, not plain text")
	}
	if !strings.Contains(body, `class="host-name"`) {
		t.Fatal("expected a host-name cell that can wrap independently of the chip")
	}
}

func TestTrajectoryOmitsEmptyPreAndLabelsHook(t *testing.T) {
	root := t.TempDir()
	slug := "empty-tool-rows"
	if err := os.MkdirAll(filepath.Join(root, "tmp", slug), 0o750); err != nil {
		t.Fatal(err)
	}
	path := session.LogPath(root, slug)
	stamp := time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC)
	for _, rec := range []session.Record{
		{Type: session.TypePreTool, Source: session.SourceHook, Client: "cursor", Event: "pre-tool-use", At: stamp},
		{Type: session.TypeToolUse, Source: session.SourceHook, Client: "cursor", At: stamp},
		{Type: session.TypePreTool, Source: session.SourceHook, Client: "cursor", Event: "pre-tool-use", Tool: "Shell", Command: "ls", At: stamp},
	} {
		if _, err := session.Append(path, rec); err != nil {
			t.Fatal(err)
		}
	}
	handler, err := NewHandlerWithPort(root, testToolkitRoot(t), 3080)
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/session/"+slug, nil)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `data-role="hook"`) {
		t.Fatal("pre_tool must use data-role=hook")
	}
	if !strings.Contains(body, `class="role role-hook"`) {
		t.Fatal("gutter must use role-hook, not role-tool, for hooks")
	}
	if !strings.Contains(body, `data-role="tool"`) {
		t.Fatal("tool_use must keep data-role=tool")
	}
	if strings.Contains(body, "<pre></pre>") {
		t.Fatal("empty tool/hook rows must not render an empty pre")
	}
	if !strings.Contains(body, "<pre>ls</pre>") {
		t.Fatal("command body should render in pre")
	}
	if strings.Count(body, "pre-tool-use") > 0 && strings.Contains(body, `<p class="event-text">pre-tool-use</p>`) {
		t.Fatal("must not repeat pre-tool-use as the body")
	}
}

func TestTrajectoryShowsGraphTransitionsFromRunLog(t *testing.T) {
	root := t.TempDir()
	slug := "graph-on-trajectory"
	if err := os.MkdirAll(filepath.Join(root, "tmp", slug), 0o750); err != nil {
		t.Fatal(err)
	}
	stamp := time.Date(2026, 8, 19, 2, 10, 0, 0, time.UTC)
	run, err := state.NewRun(slug, "show graph on trajectory", "goal-delivery", 50, stamp)
	if err != nil {
		t.Fatal(err)
	}
	run.CurrentNode = "research"
	if err := state.Save(state.ManifestPath(root, slug), run); err != nil {
		t.Fatal(err)
	}
	if _, err := state.AppendEvent(state.EventLogPath(root, slug), state.Event{
		Type: "run_started", Node: "intake", At: stamp, Payload: []byte(`{"goal":"show graph on trajectory","graph":"goal-delivery"}`),
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
	handler, err := NewHandlerWithPort(root, testToolkitRoot(t), 3080)
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/session/"+slug, nil)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	if got := strings.Count(body, `data-kind="graph"`); got != 2 {
		t.Fatalf("graph rows = %d want 2, body snippet missing?", got)
	}
	if !strings.Contains(body, "show graph on trajectory") {
		t.Fatal("run_started body should be the goal, not only the node id")
	}
	if !strings.Contains(body, `data-role="graph"`) || !strings.Contains(body, `class="role role-graph"`) {
		t.Fatal("graph rows need gutter role-graph")
	}
	if !strings.Contains(body, `data-testid="graph-view"`) {
		t.Fatal("Graph tab map must still render")
	}
	if strings.Contains(body, `data-kind="tool"`) {
		t.Fatal("journal tool_use in events.ndjson must not become a Trajectory tool row")
	}
	if !strings.Contains(body, ">research</span>") && !strings.Contains(body, "research") {
		t.Fatal("transition target should appear")
	}
	chat := httptest.NewRecorder()
	chatReq := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/session/"+slug+"?view=chat", nil)
	handler.ServeHTTP(chat, chatReq)
	if chat.Code != http.StatusOK {
		t.Fatalf("chat status = %d", chat.Code)
	}
	chatBody := chat.Body.String()
	if !strings.Contains(chatBody, `is-chat-view`) {
		t.Fatal("chat view class missing")
	}
	emptyAt := strings.Index(chatBody, `data-testid="chat-empty"`)
	if emptyAt < 0 {
		t.Fatal("Chat should stay empty of host prose when only graph rows exist")
	}
	start := emptyAt - 80
	if start < 0 {
		start = 0
	}
	if strings.Contains(chatBody[start:emptyAt], "hidden") {
		t.Fatal("chat-empty must be visible when Trajectory only has graph rows")
	}
}

func TestSessionRendersMarkdownAndOmitsEmptyUserTokens(t *testing.T) {
	root := t.TempDir()
	slug := "md-session"
	if err := os.MkdirAll(filepath.Join(root, "tmp", slug), 0o750); err != nil {
		t.Fatal(err)
	}
	path := session.LogPath(root, slug)
	stamp := time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC)
	records := []session.Record{
		{Type: session.TypePromptSubmit, Source: session.SourceHook, Client: "cursor", Body: "say hello"},
		{Type: session.TypeToolUse, Source: session.SourceHook, Client: "cursor", Tool: "Read", Command: "README.md"},
		{Type: session.TypeTranscriptMessage, Source: session.SourceTranscript, Role: "assistant", Body: "# Hello\n\n- one\n\nhi <script>alert(1)</script>", Usage: &session.Usage{Input: 12, Output: 4}},
	}
	for _, rec := range records {
		rec.At = stamp
		if _, err := session.Append(path, rec); err != nil {
			t.Fatal(err)
		}
	}
	handler, err := NewHandlerWithPort(root, testToolkitRoot(t), 3080)
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/session/"+slug, nil)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	if strings.Contains(body, `in -`) || strings.Contains(body, `out -`) {
		t.Fatal("rows without usage must not paint empty in/out placeholders")
	}
	if !strings.Contains(body, `data-testid="event-md"`) {
		t.Fatal("expected markdown body on transcript rows")
	}
	if strings.Contains(body, "<script>alert(1)</script>") {
		t.Fatal("assistant HTML must not keep a raw script tag")
	}
	if !strings.Contains(body, "is-trajectory-view") {
		t.Fatal("trajectory view class missing")
	}
	if !strings.Contains(body, `class="event-md"`) {
		t.Fatal("expected event-md wrapper")
	}
	rec = httptest.NewRecorder()
	req = httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/session/"+slug+"?view=chat", nil)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("chat status = %d", rec.Code)
	}
	chat := rec.Body.String()
	if !strings.Contains(chat, "is-chat-view") {
		t.Fatal("chat view class missing")
	}
	if strings.Contains(chat, `let view = "\"chat\""`) || !strings.Contains(chat, `let view = "chat"`) {
		t.Fatal("Chat JS view must be the string chat, not a quoted leftover from printf q")
	}
	if !strings.Contains(chat, "<article class=\"event") {
		t.Fatal("chat rows must be articles so inspector click is not a nested button")
	}
	if !strings.Contains(chat, `data-testid="graph-view" hidden inert`) {
		t.Fatal("graph must be hidden and inert on Chat so the 19-node list is not in the thread")
	}
	if !strings.Contains(chat, `tag-in`) || !strings.Contains(chat, "in 12") {
		t.Fatal("assistant Chat must still show token in/out")
	}
	if !strings.Contains(chat, `localStorage.getItem("vibe-theme")`) {
		t.Fatal("theme must apply from localStorage before first paint")
	}
}

func TestChatRendersFoldedThinkingBeforeAssistant(t *testing.T) {
	root := t.TempDir()
	slug := "think-session"
	if err := os.MkdirAll(filepath.Join(root, "tmp", slug), 0o750); err != nil {
		t.Fatal(err)
	}
	path := session.LogPath(root, slug)
	stamp := time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC)
	records := []session.Record{
		{Type: session.TypePromptSubmit, Source: session.SourceHook, Client: "cursor", Body: "read the file"},
		{Type: session.TypeTranscriptMessage, Source: session.SourcePrint, Role: "thinking", Body: "I will open README first"},
		{Type: session.TypeTranscriptMessage, Source: session.SourceTranscript, Role: "assistant", Body: "Opened README."},
	}
	for _, rec := range records {
		rec.At = stamp
		if _, err := session.Append(path, rec); err != nil {
			t.Fatal(err)
		}
	}
	handler, err := NewHandlerWithPort(root, testToolkitRoot(t), 3080)
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/session/"+slug+"?view=chat", nil)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	thinkAt := strings.Index(body, `data-role="thinking"`)
	assistAt := strings.Index(body, `data-role="assistant"`)
	if thinkAt < 0 || assistAt < 0 || thinkAt > assistAt {
		t.Fatal("thinking row must sit in the DOM before the assistant reply")
	}
	if !strings.Contains(body, "Show thinking") {
		t.Fatal("thinking must start collapsed with a Show thinking control")
	}
	if !strings.Contains(body, `data-fold="1"`) {
		t.Fatal("thinking Chat rows start folded")
	}
}

func TestLongAssistantRowHasNoChatExpand(t *testing.T) {
	root := t.TempDir()
	slug := "fold-session"
	if err := os.MkdirAll(filepath.Join(root, "tmp", slug), 0o750); err != nil {
		t.Fatal(err)
	}
	path := session.LogPath(root, slug)
	stamp := time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC)
	rec := session.Record{
		Type:   session.TypeTranscriptMessage,
		Source: session.SourceTranscript,
		Role:   "assistant",
		Body:   strings.Repeat("paragraph text. ", 40),
		At:     stamp,
	}
	if _, err := session.Append(path, rec); err != nil {
		t.Fatal(err)
	}
	handler, err := NewHandlerWithPort(root, testToolkitRoot(t), 3080)
	if err != nil {
		t.Fatal(err)
	}
	recHTTP := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/session/"+slug+"?view=chat", nil)
	handler.ServeHTTP(recHTTP, req)
	if recHTTP.Code != http.StatusOK {
		t.Fatalf("status = %d", recHTTP.Code)
	}
	body := recHTTP.Body.String()
	if strings.Contains(body, `data-testid="event-expand"`) {
		t.Fatal("assistant Chat rows must not use Show more")
	}
	if strings.Contains(body, `data-fold="1"`) {
		t.Fatal("assistant Chat rows must not clamp")
	}
	if !strings.Contains(body, `data-testid="event-md"`) {
		t.Fatal("assistant body still renders as markdown")
	}
}

func TestChromeButtonsUseSvgNotGlyphs(t *testing.T) {
	root, slug := writeEmptySession(t)
	handler, err := NewHandlerWithPort(root, testToolkitRoot(t), 3080)
	if err != nil {
		t.Fatal(err)
	}
	pages := []string{"/", "/session/" + slug}
	for _, path := range pages {
		rec := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, path, nil)
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d body = %s", path, rec.Code, rec.Body.String())
		}
		body := rec.Body.String()
		for _, glyph := range []string{">+</button>", ">→</button>", ">Menu</button>", ">×</button>"} {
			if strings.Contains(body, glyph) {
				t.Fatalf("%s still uses text glyph %q", path, glyph)
			}
		}
		assertButtonHasSVG(t, body, `class="btn-send"`)
		assertButtonHasSVG(t, body, `id="sidebar-open"`)
		if path == "/" {
			continue
		}
		assertButtonHasSVG(t, body, `id="composer-file-open"`)
		assertButtonHasSVG(t, body, `id="inspector-close"`)
	}
}

func assertButtonHasSVG(t *testing.T, body, marker string) {
	t.Helper()
	i := strings.Index(body, marker)
	if i < 0 {
		t.Fatalf("missing %s", marker)
	}
	end := strings.Index(body[i:], "</button>")
	if end < 0 {
		t.Fatalf("no button end after %s", marker)
	}
	inner := body[i : i+end]
	if !strings.Contains(inner, "<svg") {
		t.Fatalf("%s button has no svg: %s", marker, inner)
	}
}

func assertNewSessionForm(t *testing.T, body string) {
	t.Helper()
	for _, want := range []string{
		`placeholder="kebab-case-slug"`,
		`placeholder="What should this run accomplish?"`,
		`class="dialog-actions"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("new session form missing %q", want)
		}
	}
}

func TestCheckpointAdvancesHumanGate(t *testing.T) {
	root, slug := writeFixtureSession(t)
	handler, err := NewHandlerWithPort(root, testToolkitRoot(t), 3080)
	if err != nil {
		t.Fatal(err)
	}
	form := strings.NewReader("check=spec_approved&verdict=passed")
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/session/"+slug+"/checkpoint", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "/session/"+slug) || !strings.Contains(loc, "view=chat") {
		t.Fatalf("location = %q", loc)
	}
	run, err := state.Load(state.ManifestPath(root, slug))
	if err != nil {
		t.Fatal(err)
	}
	if run.CurrentNode != "plan" {
		t.Fatalf("current = %q", run.CurrentNode)
	}
	check, ok := run.Checks["spec_approved"]
	if !ok || !check.Passed {
		t.Fatalf("check = %+v ok=%v", check, ok)
	}
	if check.Source != state.SourceHumanEvent {
		t.Fatalf("source = %q", check.Source)
	}
}

func TestCheckpointRejectsVerifierNode(t *testing.T) {
	root, slug := writeFixtureSession(t)
	run, err := state.Load(state.ManifestPath(root, slug))
	if err != nil {
		t.Fatal(err)
	}
	run.CurrentNode = "test"
	run.Status = state.StatusRunning
	if err := state.Save(state.ManifestPath(root, slug), run); err != nil {
		t.Fatal(err)
	}
	handler, err := NewHandlerWithPort(root, testToolkitRoot(t), 3080)
	if err != nil {
		t.Fatal(err)
	}
	form := strings.NewReader("check=unit&verdict=passed")
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/session/"+slug+"/checkpoint", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	loaded, err := state.Load(state.ManifestPath(root, slug))
	if err != nil {
		t.Fatal(err)
	}
	if loaded.CurrentNode != "test" {
		t.Fatalf("current moved to %q", loaded.CurrentNode)
	}
}

func TestSessionPageRedactedPromptAbsentFromHTML(t *testing.T) {
	root := t.TempDir()
	slug := "redact-test"
	manifestDir := filepath.Join(root, "tmp", slug)
	if err := os.MkdirAll(manifestDir, 0o750); err != nil {
		t.Fatal(err)
	}
	logPath := session.LogPath(root, slug)
	rec := session.Record{
		Type:   session.TypePromptSubmit,
		Source: session.SourceHook,
		Client: "claude",
		Body:   "deploy with key " + testSecret,
		At:     time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC),
	}
	if _, err := session.Append(logPath, rec); err != nil {
		t.Fatal(err)
	}
	run, err := state.NewRun(slug, "redact", "goal-delivery", 50, time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if err := state.Save(state.ManifestPath(root, slug), run); err != nil {
		t.Fatal(err)
	}

	handler, err := NewHandlerWithPort(root, testToolkitRoot(t), 3080)
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/session/"+slug, nil)
	handler.ServeHTTP(recorder, req)
	raw, err := io.ReadAll(recorder.Body)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if strings.Contains(text, testSecret) {
		t.Fatalf("secret in HTML: %s", text)
	}
	if !strings.Contains(text, "redacted") {
		t.Fatal("expected redacted chip")
	}
}

func TestShellCSSKeepsFilterMenuAndEmptyStateOnCanvas(t *testing.T) {
	root := t.TempDir()
	handler, err := NewHandlerWithPort(root, testToolkitRoot(t), 3080)
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/static/shell.css", nil)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	css := rec.Body.String()
	menu := cssBlock(css, ".kind-menu")
	if !strings.Contains(menu, "left: 0") {
		t.Fatalf("kind-menu must open from the filter left edge, got %q", menu)
	}
	if strings.Contains(menu, "right: 0") {
		t.Fatal("kind-menu right: 0 clips kind labels when Filter wraps to the start of the toolbar")
	}
	empty := cssBlock(css, ".empty")
	if !strings.Contains(empty, "text-align: center") {
		t.Fatalf("empty copy must be centered, got %q", empty)
	}
	if !strings.Contains(empty, "margin: auto") {
		t.Fatalf("empty copy must sit in the middle of the stream, got %q", empty)
	}
	stream := cssBlock(css, ".stream")
	if !strings.Contains(stream, "display: flex") || !strings.Contains(stream, "flex-direction: column") {
		t.Fatalf("stream must be a column so empty can center, got %q", stream)
	}
	preview := cssBlock(css, ".composer-preview")
	if !strings.Contains(preview, "pointer-events: none") {
		t.Fatalf("composer preview must not steal clicks from the message field, got %q", preview)
	}
	graph := cssBlock(css, ".graph-view")
	if !strings.Contains(graph, "overflow: auto") && !strings.Contains(graph, "overflow-y: auto") {
		t.Fatalf("graph-view must scroll its own nodes, got %q", graph)
	}
	field := cssBlock(css, ".composer-field-wrap .composer-field")
	if field == "" {
		field = cssBlock(css, ".composer .composer-field")
	}
	if !strings.Contains(field, "caret-color") {
		t.Fatalf("composer field must keep a visible caret when text is painted on the preview, got %q", field)
	}
	if strings.Contains(field, "color: transparent") {
		t.Fatalf("empty composer field must show typed text, got %q", field)
	}
	if !strings.Contains(css, ":has(.composer-preview:not(:empty)) .composer-field") {
		t.Fatal("slash and @ highlight may hide input text only after the preview has content")
	}
	wrapFocus := cssBlock(css, ".composer-field-wrap:focus-within")
	if !strings.Contains(wrapFocus, "outline:") {
		t.Fatalf("composer wrap must own the focus ring so chips are not clipped, got %q", wrapFocus)
	}
	fieldFocus := cssBlock(css, ".composer-field-wrap .composer-field:focus-visible")
	if !strings.Contains(fieldFocus, "outline: none") {
		t.Fatalf("inner field must not draw a second ring, got %q", fieldFocus)
	}
	previewAlign := cssBlock(css, ".composer-field-wrap .composer-preview")
	if !strings.Contains(previewAlign, "align-items: center") {
		t.Fatalf("slash and @ chips must sit on the field midline, got %q", previewAlign)
	}
	statTokens := cssBlock(css, ".stat .token-pair")
	if !strings.Contains(statTokens, "margin-inline-start: 0") {
		t.Fatalf("toolbar in/out/cache chips must stay in the Tokens cell, got %q", statTokens)
	}
	chatTokenShow := cssBlock(css, ".is-chat-view .token-pair")
	if !strings.Contains(chatTokenShow, "display: inline-flex") {
		t.Fatalf("chat must keep in/out/cache chips visible, got %q", chatTokenShow)
	}
	toolsRow := cssBlock(css, ".composer-tools")
	if !strings.Contains(toolsRow, "grid-column: 1 / -1") {
		t.Fatalf("host and model must sit under the message field so chips are not clipped, got %q", toolsRow)
	}
	mark := cssBlock(css, ".composer-field-wrap .composer-ref")
	if strings.Contains(mark, "color: transparent") {
		t.Fatalf("slash and @ marks must stay visible on the preview, got %q", mark)
	}
	closed := cssBlock(css, "dialog:not([open])")
	if !strings.Contains(closed, "display: none") {
		t.Fatalf("closed dialogs must not remain in hit testing, got %q", closed)
	}
	hostMenu := cssBlock(css, ".host-menu")
	if !strings.Contains(hostMenu, "bottom:") {
		t.Fatalf("host menu must open above the composer, got %q", hostMenu)
	}
	row := cssBlock(css, ".settings-row")
	if !strings.Contains(row, "space-between") {
		t.Fatalf("settings URL row must pin the label and value to opposite edges, got %q", row)
	}
	hosts := cssBlock(css, ".host-status li")
	if strings.Contains(hosts, "4.5rem") {
		t.Fatalf("host rows must not use a 4.5rem status column that wraps binary names, got %q", hosts)
	}
	if !strings.Contains(hosts, "flex-wrap") {
		t.Fatalf("host rows must wrap instead of clipping names, got %q", hosts)
	}
	pass := cssBlock(css, ".chip-pass")
	if !strings.Contains(pass, "--color-success") {
		t.Fatalf("ready tag must use the success token, got %q", pass)
	}
	md := cssBlock(css, ".event-md")
	if !strings.Contains(md, "--font") && !strings.Contains(md, "overflow-wrap") && !strings.Contains(md, "margin") {
		t.Fatalf("markdown body needs readable spacing, got %q", md)
	}
	chatSeq := cssBlock(css, ".is-chat-view .event-kicker .event-seq")
	if !strings.Contains(chatSeq, "display: none") {
		t.Fatalf("chat must hide trajectory seq labels, got %q", chatSeq)
	}
	chatList := cssBlock(css, ".is-chat-view .event-list")
	if !strings.Contains(chatList, "border-inline-start: 0") {
		t.Fatalf("chat must drop the trajectory gutter, got %q", chatList)
	}
	chatDefault := cssBlock(css, ".is-chat-view .event-list > li")
	if !strings.Contains(chatDefault, "display: none") {
		t.Fatalf("chat must hide every row until the allow-list shows it, got %q", chatDefault)
	}
	if !strings.Contains(css, `.is-chat-view .event-list > li[data-role="user"]`) ||
		!strings.Contains(css, `.is-chat-view .event-list > li[data-role="assistant"]`) ||
		!strings.Contains(css, `.is-chat-view .event-list > li[data-role="thinking"]`) {
		t.Fatal("chat allow-list must paint user, assistant, and thinking before JS runs")
	}
	chatHidden := cssBlock(css, ".is-chat-view .event-list > li[hidden]")
	if !strings.Contains(chatHidden, "display: none") {
		t.Fatalf("chat search hide must beat the allow-list display flex, got %q", chatHidden)
	}
	chatUser := cssBlock(css, `.is-chat-view li[data-role="user"] .event`)
	if !strings.Contains(chatUser, "--chat-user-max") {
		t.Fatalf("user Chat bubble must stay on the end of the column, got %q", chatUser)
	}
	chatTokens := cssBlock(css, ".is-chat-view .token-pair")
	if !strings.Contains(chatTokens, "margin-inline-start: 0") {
		t.Fatalf("chat token chips must sit under the assistant body, not the far edge, got %q", chatTokens)
	}
	thinkBody := cssBlock(css, `.is-chat-view li[data-role="thinking"] .event-body`)
	if !strings.Contains(thinkBody, "--color-text-caption") || !strings.Contains(thinkBody, "--font-size-meta") {
		t.Fatalf("thinking must use caption type, got %q", thinkBody)
	}
	graphNode := cssBlock(css, ".graph-node")
	if !strings.Contains(graphNode, "--graph-tag-col") {
		t.Fatalf("graph node must pin type tags left of the spine, got %q", graphNode)
	}
	graphType := cssBlock(css, ".graph-type")
	if !strings.Contains(graphType, "justify-self: end") {
		t.Fatalf("type tags must right-align toward the spine, got %q", graphType)
	}
	graphCopy := cssBlock(css, ".graph-copy")
	if !strings.Contains(graphCopy, "grid-column: 3") {
		t.Fatalf("graph title and description must skip the spine gutter, got %q", graphCopy)
	}
	graphCurrent := cssBlock(css, `.graph-node[aria-current="true"]`)
	if !strings.Contains(graphCurrent, "--color-accent") {
		t.Fatalf("current graph node needs an accent rail, got %q", graphCurrent)
	}
	busyGraph := cssBlock(css, ".is-graph-view #host-busy")
	if !strings.Contains(busyGraph, "display: none") {
		t.Fatalf("thinking bar must not cover Graph, got %q", busyGraph)
	}
}

func TestEmptySessionCentersReconstructCopyAndKeepsGraphInStream(t *testing.T) {
	root, slug := writeEmptySession(t)
	handler, err := NewHandlerWithPort(root, testToolkitRoot(t), 3080)
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/session/"+slug, nil)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "No host turns yet") {
		t.Fatal("expected empty trajectory copy that a delivery run is not a missing log")
	}
	if !strings.Contains(body, `id="event-list"`) {
		t.Fatal("empty session must keep event-list so a refresh can append reconstructed rows")
	}
	streamAt := strings.Index(body, `id="stream"`)
	graphAt := strings.Index(body, `id="graph-view"`)
	if streamAt < 0 || graphAt < 0 {
		t.Fatal("expected stream and graph-view")
	}
	if graphAt < streamAt {
		t.Fatal("graph-view must live inside the stream so it cannot cover the canvas")
	}
	dockAt := strings.Index(body, `class="dock"`)
	busyAt := strings.Index(body, `id="host-busy"`)
	if dockAt < 0 || busyAt < 0 {
		t.Fatal("expected dock and host-busy")
	}
	if busyAt < dockAt {
		t.Fatal("thinking bar must sit in the dock, not over the graph stream")
	}
	if !strings.Contains(body, `data-testid="kind-filter"`) {
		t.Fatal("expected kind filter menu")
	}
}

func cssBlock(src, selector string) string {
	needle := "\n" + selector + " {"
	i := strings.Index(src, needle)
	if i < 0 {
		return ""
	}
	rest := src[i:]
	j := strings.Index(rest, "}")
	if j < 0 {
		return rest
	}
	return rest[:j]
}

func writeEmptySession(t *testing.T) (root, slug string) {
	t.Helper()
	root = t.TempDir()
	slug = "empty-session"
	if err := os.MkdirAll(filepath.Join(root, "tmp", slug), 0o750); err != nil {
		t.Fatal(err)
	}
	run, err := state.NewRun(slug, "empty", "goal-delivery", 50, time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if err := state.Save(state.ManifestPath(root, slug), run); err != nil {
		t.Fatal(err)
	}
	return root, slug
}

func TestNewSessionRedirectsToChatView(t *testing.T) {
	root := t.TempDir()
	handler, err := NewHandlerWithPort(root, testToolkitRoot(t), 3080)
	if err != nil {
		t.Fatal(err)
	}
	form := strings.NewReader("slug=persist-chat&goal=Keep+chat+after+refresh&graph=goal-delivery")
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/session/new", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "/session/persist-chat") || !strings.Contains(loc, "view=chat") {
		t.Fatalf("location = %q", loc)
	}
}

func TestChatQueryKeepsHumanGateVisible(t *testing.T) {
	root, slug := writeFixtureSession(t)
	handler, err := NewHandlerWithPort(root, testToolkitRoot(t), 3080)
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/session/"+slug+"?view=chat", nil)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	marker := `data-testid="chat-prompts"`
	i := strings.Index(body, marker)
	if i < 0 {
		t.Fatal("expected chat-prompts")
	}
	snippet := body[i : i+len(marker)+16]
	if strings.Contains(snippet, "hidden") {
		t.Fatalf("chat view must render prompts visible so refresh keeps them: %q", snippet)
	}
	if !strings.Contains(body, `aria-selected="true" data-view="chat"`) && !strings.Contains(body, `data-view="chat" aria-selected="true"`) {
		t.Fatal("chat tab must be selected when view=chat")
	}
}

func TestChatVerifierPromptOmitsDuplicateAndComposerHint(t *testing.T) {
	root := t.TempDir()
	slug := "e2e-chat-prompt"
	if err := os.MkdirAll(filepath.Join(root, "tmp", slug), 0o750); err != nil {
		t.Fatal(err)
	}
	stamp := time.Date(2026, 8, 19, 2, 28, 0, 0, time.UTC)
	run, err := state.NewRun(slug, "verifier card copy", "goal-delivery", 50, stamp)
	if err != nil {
		t.Fatal(err)
	}
	run.CurrentNode = "e2e"
	run.Status = state.StatusRunning
	if err := state.Save(state.ManifestPath(root, slug), run); err != nil {
		t.Fatal(err)
	}
	handler, err := NewHandlerWithPort(root, testToolkitRoot(t), 3080)
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/session/"+slug+"?view=chat", nil)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	start := strings.Index(body, `data-prompt-type="verifier"`)
	if start < 0 {
		t.Fatal("expected verifier chat prompt")
	}
	end := strings.Index(body[start:], "</article>")
	if end < 0 {
		t.Fatal("expected prompt article")
	}
	article := body[start : start+end]
	if strings.Contains(article, "composer below") {
		t.Fatal("verifier hint must not point at the composer")
	}
	if !strings.Contains(article, `data-testid="chat-prompt-hint"`) {
		t.Fatal("expected verifier hint")
	}
	if strings.Count(article, "vibe-checks.yaml") > 1 {
		t.Fatal("e2e description must not repeat as title and body")
	}
	if !strings.Contains(body, `data-testid="composer"`) {
		t.Fatal("dock composer must still be on the page")
	}
}

func TestChatIntakePromptShowsRunGoal(t *testing.T) {
	root := t.TempDir()
	slug := "intake-goal-card"
	if err := os.MkdirAll(filepath.Join(root, "tmp", slug), 0o750); err != nil {
		t.Fatal(err)
	}
	stamp := time.Date(2026, 8, 19, 3, 0, 0, 0, time.UTC)
	run, err := state.NewRun(slug, "unique-goal-xyz", "goal-delivery", 50, stamp)
	if err != nil {
		t.Fatal(err)
	}
	run.CurrentNode = "intake"
	run.Status = state.StatusAwaitingHuman
	if err := state.Save(state.ManifestPath(root, slug), run); err != nil {
		t.Fatal(err)
	}
	handler, err := NewHandlerWithPort(root, testToolkitRoot(t), 3080)
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/session/"+slug+"?view=chat", nil)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	start := strings.Index(body, `data-prompt-type="human_gate"`)
	if start < 0 {
		t.Fatal("expected intake human_gate card")
	}
	end := strings.Index(body[start:], "</article>")
	if end < 0 {
		t.Fatal("expected prompt article")
	}
	article := body[start : start+end]
	if !strings.Contains(article, "unique-goal-xyz") {
		t.Fatalf("goal missing from card: %s", article)
	}
	if strings.Contains(article, "measurable done line") {
		t.Fatal("graph YAML must not be the only title")
	}
	if strings.Contains(article, `data-role="hook"`) || strings.Contains(article, `data-role="tool"`) || strings.Contains(article, `data-role="graph"`) {
		t.Fatal("chat prompt card must not use trajectory roles")
	}
}

func TestChatIntakePromptRedactsSecretInGoal(t *testing.T) {
	root := t.TempDir()
	slug := "intake-goal-secret"
	if err := os.MkdirAll(filepath.Join(root, "tmp", slug), 0o750); err != nil {
		t.Fatal(err)
	}
	stamp := time.Date(2026, 8, 19, 3, 5, 0, 0, time.UTC)
	run, err := state.NewRun(slug, "ship with "+testSecret, "goal-delivery", 50, stamp)
	if err != nil {
		t.Fatal(err)
	}
	run.CurrentNode = "intake"
	run.Status = state.StatusAwaitingHuman
	if err := state.Save(state.ManifestPath(root, slug), run); err != nil {
		t.Fatal(err)
	}
	if _, err := state.AppendEvent(state.EventLogPath(root, slug), state.Event{
		Type: "run_started", Node: "intake", At: stamp,
		Payload: []byte(`{"goal":"ship with ` + testSecret + `","graph":"goal-delivery"}`),
	}); err != nil {
		t.Fatal(err)
	}
	handler, err := NewHandlerWithPort(root, testToolkitRoot(t), 3080)
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/session/"+slug+"?view=chat", nil)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), testSecret) {
		t.Fatal("credential-shaped goal leaked into Chat HTML")
	}
}

func writeFixtureSession(t *testing.T) (root, slug string) {
	t.Helper()
	root = t.TempDir()
	slug = "fixture-session"
	if err := os.MkdirAll(filepath.Join(root, "tmp", slug), 0o750); err != nil {
		t.Fatal(err)
	}
	path := session.LogPath(root, slug)
	stamp := time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC)
	records := []session.Record{
		{Type: session.TypeSessionStart, Source: session.SourceHook, Client: "cursor", Event: "SessionStart"},
		{Type: session.TypePromptSubmit, Source: session.SourceHook, Client: "cursor", Body: "plan the UI"},
		{Type: session.TypeToolUse, Source: session.SourceHook, Client: "cursor", Tool: "Read", Command: "docs/SPEC.md"},
		{Type: session.TypeToolUse, Source: session.SourceHook, Client: "codex", Tool: "bash", Command: "gh pr view"},
		{Type: session.TypeTranscriptMessage, Source: session.SourceTranscript, Role: "assistant", Body: "I will implement the shell.", Usage: &session.Usage{Input: 100, Output: 40}},
	}
	for _, rec := range records {
		rec.At = stamp
		if _, err := session.Append(path, rec); err != nil {
			t.Fatal(err)
		}
	}
	gapLine := `{"sequence":6,"type":"tool_use","source":"hook","payload":{"source":"hook","client":"codex","tool":"bash","command":"gh pr view","hostGap":true},"at":"2026-08-18T10:00:00Z"}` + "\n"
	f, err := os.OpenFile(filepath.Clean(path), os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(gapLine); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	run, err := state.NewRun(slug, "fixture", "goal-delivery", 50, time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	run.Status = state.StatusAwaitingHuman
	run.CurrentNode = "approve_spec"
	if err := state.Save(state.ManifestPath(root, slug), run); err != nil {
		t.Fatal(err)
	}
	return root, slug
}

func TestSessionPageKeepsLastComposerHost(t *testing.T) {
	root := t.TempDir()
	slug := "host-persist"
	if err := os.MkdirAll(filepath.Join(root, "tmp", slug), 0o750); err != nil {
		t.Fatal(err)
	}
	path := session.LogPath(root, slug)
	stamp := time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC)
	if _, err := session.Append(path, session.Record{
		Type: session.TypePromptSubmit, Source: session.SourceHook, Client: "cursor-agent", Body: "hi", At: stamp,
	}); err != nil {
		t.Fatal(err)
	}
	run, err := state.NewRun(slug, "host persist", "goal-delivery", 50, stamp)
	if err != nil {
		t.Fatal(err)
	}
	if err := state.Save(state.ManifestPath(root, slug), run); err != nil {
		t.Fatal(err)
	}
	handler, err := NewHandlerWithPort(root, testToolkitRoot(t), 3080)
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/session/"+slug+"?view=chat", nil)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	if strings.Contains(body, `data-host-id="cursor-agent"`) {
		if !strings.Contains(body, `id="composer-host" value="cursor-agent"`) {
			t.Fatalf("host input did not keep cursor-agent:\n%s", body)
		}
		if !strings.Contains(body, `id="composer-host-label">cursor-agent`) {
			t.Fatalf("host label did not keep cursor-agent")
		}
	}
	if !strings.Contains(body, `data-testid="host-busy"`) {
		t.Fatal("expected host-busy indicator")
	}
	busy := `id="host-busy"`
	i := strings.Index(body, busy)
	if i < 0 {
		t.Fatal("missing host-busy")
	}
	snippet := body[i : i+180]
	if strings.Contains(snippet, "hidden") {
		t.Fatalf("busy indicator should show while last row is a prompt: %q", snippet)
	}
}

func TestCatalogIncludesWorkspaceCommand(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".cursor", "commands")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	md := "---\nname: vibe-unique-http-cmd\ndescription: HTTP catalog fixture\n---\n\n# x\n"
	if err := os.WriteFile(filepath.Join(dir, "vibe-unique-http-cmd.md"), []byte(md), 0o600); err != nil {
		t.Fatal(err)
	}
	handler, err := NewHandlerWithPort(root, testToolkitRoot(t), 3080)
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/catalog/commands?q=vibe-unique-http-cmd", nil)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "/vibe-unique-http-cmd") {
		t.Fatalf("missing workspace command: %s", body)
	}
}

func TestComposerJSHighlightsAbsoluteAttachPaths(t *testing.T) {
	root := t.TempDir()
	handler, err := NewHandlerWithPort(root, testToolkitRoot(t), 3080)
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/static/composer.js", nil)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	js := rec.Body.String()
	if !strings.Contains(js, "[A-Za-z]:[^\\s]+") {
		t.Fatal("composer must highlight Windows absolute @ attach paths through the drive colon")
	}
	if !strings.Contains(js, "highlightComposerAtRefs") {
		t.Fatal("composer @ highlight must handle quoted and drive-letter paths before workspace refs")
	}
	if !strings.Contains(js, "attachPathMap") || !strings.Contains(js, "expandAttachRefs") {
		t.Fatal("composer must shorten host attach picks to @basename and expand before send")
	}
	if !strings.Contains(js, "composer-attach-tooltip") {
		t.Fatal("composer must show full attach path on hover")
	}
	if !strings.Contains(js, "attachSuggestions") || !strings.Contains(js, "isCompleteAttachFragment") {
		t.Fatal("composer must suggest attached files and skip catalog on complete attach tokens")
	}
	if !strings.Contains(js, "getHighlightSuppressRange") {
		t.Fatal("composer must drop @ highlight while catalog shows no matches")
	}
}
