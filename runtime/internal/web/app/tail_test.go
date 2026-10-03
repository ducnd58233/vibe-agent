package app

import (
	"context"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/infra/database"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/workspace"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/session"
	"github.com/ducnd58233/vibe-agent/runtime/internal/testutil"
)

func TestSessionEventsTailAppendsNewRows(t *testing.T) {
	root, slug := writeFixtureSession(t)
	handler, err := NewHandlerWithPort(root, testToolkitRoot(t), 3080)
	if err != nil {
		t.Fatal(err)
	}
	testutil.EnsureRunIndex(t, root, slug)
	path := session.LogPath(root, slug)
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/session/"+slug+"/events?after=6", nil)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if body := rec.Body.String(); body != "" {
		t.Fatalf("expected empty tail before append, got %q", body)
	}
	if _, err := session.Append(path, session.Record{
		Type:   session.TypePromptSubmit,
		Source: session.SourceHook,
		Client: "cursor",
		Body:   "late append",
		At:     time.Date(2026, 8, 18, 11, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatal(err)
	}
	rec = httptest.NewRecorder()
	req = httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/session/"+slug+"/events?after=6", nil)
	handler.ServeHTTP(rec, req)
	body := rec.Body.String()
	if !strings.Contains(body, `data-seq="7"`) {
		t.Fatalf("expected seq 7 row, got %s", body)
	}
	if strings.Contains(body, testSecret) {
		t.Fatal("secret leaked in tail fragment")
	}
}

func TestSessionEventsTailUnreadableLog(t *testing.T) {
	root := t.TempDir()
	handler, err := NewHandlerWithPort(root, testToolkitRoot(t), 3080)
	if err != nil {
		t.Fatal(err)
	}
	slug := "broken"
	testutil.EnsureRunIndex(t, root, slug)
	// A stored row whose payload is not JSON is the one way a session log can
	// be unreadable now that logs live in the database.
	logPath := session.LogPath(root, slug)
	if _, err := session.Append(logPath, session.Record{Type: session.TypePromptSubmit, Source: session.SourceHook, Body: "hi"}); err != nil {
		t.Fatal(err)
	}
	db, err := database.Open(t.Context(), workspace.MemoryDBPath(root))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(t.Context(), `UPDATE session_events SET payload = 'not json'`); err != nil {
		t.Fatal(err)
	}
	_ = db.Close()
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/session/"+slug+"/events?after=0", nil)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Could not read the session log") {
		t.Fatalf("expected error copy, got %s", rec.Body.String())
	}
}
