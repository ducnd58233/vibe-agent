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
	assertNewSessionForm(t, body)
	if !strings.Contains(body, `class="dock"`) || !strings.Contains(body, `data-testid="composer"`) {
		t.Fatal("expected composer dock on the session page")
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
	if !strings.Contains(body, "Nothing to reconstruct yet") {
		t.Fatal("expected empty trajectory copy")
	}
	streamAt := strings.Index(body, `id="stream"`)
	graphAt := strings.Index(body, `id="graph-view"`)
	if streamAt < 0 || graphAt < 0 {
		t.Fatal("expected stream and graph-view")
	}
	if graphAt < streamAt {
		t.Fatal("graph-view must live inside the stream so it cannot cover the canvas")
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
