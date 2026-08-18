package app

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/hosts"
)

func TestComposerSendRecordsCursorPromptOnTrajectory(t *testing.T) {
	cursorReady := false
	for _, entry := range hosts.Inventory() {
		if entry.Binary == "cursor-agent" && entry.OnPath {
			cursorReady = true
			break
		}
	}
	if !cursorReady {
		t.Skip("cursor-agent not on PATH")
	}

	printed := make(chan struct{})
	hostPrint = func(ctx context.Context, host hosts.Host, prompt string) (string, error) {
		defer close(printed)
		if host.Binary != "cursor-agent" {
			t.Errorf("print host = %q, want cursor-agent", host.Binary)
		}
		return `{"type":"message","content":"cursor trajectory reply"}`, nil
	}
	t.Cleanup(func() { hostPrint = runHostPrint })

	root, slug := writeFixtureSession(t)
	handler, err := NewHandlerWithPort(root, testToolkitRoot(t), 3080)
	if err != nil {
		t.Fatal(err)
	}

	form := strings.NewReader("host=cursor-agent&message=show+the+trajectory+row")
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/session/"+slug+"/send", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	select {
	case <-printed:
	case <-time.After(2 * time.Second):
		t.Fatal("cursor print stub was not called")
	}

	deadline := time.Now().Add(2 * time.Second)
	var html string
	for time.Now().Before(deadline) {
		pageReq := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/session/"+slug, nil)
		pageRec := httptest.NewRecorder()
		handler.ServeHTTP(pageRec, pageReq)
		body, err := io.ReadAll(pageRec.Body)
		if err != nil {
			t.Fatal(err)
		}
		html = string(body)
		if strings.Contains(html, "show the trajectory row") && strings.Contains(html, `data-role="user"`) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !strings.Contains(html, "show the trajectory row") {
		t.Fatalf("prompt missing from trajectory:\n%s", html)
	}
	if !strings.Contains(html, `data-role="user"`) {
		t.Fatal("expected user event row")
	}
}

func TestParsePrintLinesExtractsCodexAgentMessage(t *testing.T) {
	raw := `{"type":"thread.started","thread_id":"1"}` + "\n" +
		`{"type":"turn.started"}` + "\n" +
		`{"type":"item.completed","item":{"id":"item_0","type":"error","message":"shortened"}}` + "\n" +
		`{"type":"item.completed","item":{"id":"item_1","type":"agent_message","text":"pong"}}` + "\n" +
		`{"type":"turn.completed"}`
	got := parsePrintLines(raw)
	if len(got) != 1 || got[0] != "pong" {
		t.Fatalf("got %q", got)
	}
}

func TestComposerSendRejectsUnknownHost(t *testing.T) {
	root, slug := writeFixtureSession(t)
	handler, err := NewHandlerWithPort(root, testToolkitRoot(t), 3080)
	if err != nil {
		t.Fatal(err)
	}
	form := strings.NewReader("host=not-a-host&message=hello")
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/session/"+slug+"/send", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}
