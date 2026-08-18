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
	for _, id := range []string{"app-shell", "rail", "trajectory-empty", "workspace-path"} {
		if !strings.Contains(text, `data-testid="`+id+`"`) {
			t.Fatalf("missing test id %q in %s", id, text)
		}
	}
	if !strings.Contains(text, root) {
		t.Fatalf("workspace path missing from shell")
	}
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
		"kind-filter-clear", "event-tokens", "token-usage", "inspector", "chat-empty",
		"graph-view",
	} {
		if !strings.Contains(body, `data-testid="`+id+`"`) {
			t.Fatalf("missing test id %q", id)
		}
	}
	if !strings.Contains(body, "<ol class=\"event-list\"") {
		t.Fatal("expected ol event-list")
	}
	if strings.Contains(body, testSecret) {
		t.Fatalf("secret leaked into HTML: %s", body)
	}
	if !strings.Contains(body, "host gap") {
		t.Fatal("expected host gap chip in fixture")
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
