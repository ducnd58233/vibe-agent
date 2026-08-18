package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/session"
)

func TestSessionEventsStreamReturnsSSE(t *testing.T) {
	root, slug := writeFixtureSession(t)
	handler, err := NewHandlerWithPort(root, testToolkitRoot(t), 3080)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/session/"+slug+"/events/stream?after=0", nil)
	rec := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		handler.ServeHTTP(rec, req)
		close(done)
	}()
	<-done
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/event-stream") {
		t.Fatalf("content-type = %q", ct)
	}
	if !strings.Contains(rec.Body.String(), "data:") {
		t.Fatalf("expected SSE data: %s", rec.Body.String())
	}
}

func TestSessionEventsStreamAfterAppend(t *testing.T) {
	root, slug := writeFixtureSession(t)
	handler, err := NewHandlerWithPort(root, testToolkitRoot(t), 3080)
	if err != nil {
		t.Fatal(err)
	}
	path := session.LogPath(root, slug)
	ctx, cancel := context.WithTimeout(context.Background(), 2500*time.Millisecond)
	defer cancel()
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/session/"+slug+"/events/stream?after=6", nil)
	rec := httptest.NewRecorder()
	go func() {
		time.Sleep(200 * time.Millisecond)
		_, _ = session.Append(path, session.Record{
			Type: session.TypePromptSubmit, Source: session.SourceHook, Client: "claude", Body: "sse tail",
		})
	}()
	handler.ServeHTTP(rec, req)
	if !strings.Contains(rec.Body.String(), "sse tail") {
		t.Fatalf("missing appended row: %s", rec.Body.String())
	}
}
