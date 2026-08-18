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
	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "/session/"+slug) || !strings.Contains(loc, "view=chat") {
		t.Fatalf("location = %q, want chat view so refresh keeps the thread", loc)
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
		`{"type":"turn.completed","usage":{"input_tokens":12,"output_tokens":3}}`
	got := parsePrintLines(raw)
	if len(got) != 1 || got[0] != "pong" {
		t.Fatalf("got %q", got)
	}
	fragments, usage := parsePrintOutput(raw)
	if len(fragments) != 1 || fragments[0].Body != "pong" {
		t.Fatalf("fragments = %+v", fragments)
	}
	if usage == nil || usage.Input != 12 || usage.Output != 3 {
		t.Fatalf("usage = %+v", usage)
	}
}

func TestParsePrintOutputQuestionAndTotalTokens(t *testing.T) {
	raw := `{"type":"ask_user","prompt":"Approve the spec?"}` + "\n" +
		`{"type":"turn.completed","usage":{"total_tokens":44}}`
	fragments, usage := parsePrintOutput(raw)
	if len(fragments) != 1 || fragments[0].Role != "question" || fragments[0].Body != "Approve the spec?" {
		t.Fatalf("fragments = %+v", fragments)
	}
	if usage == nil || usage.Total != 44 {
		t.Fatalf("usage = %+v", usage)
	}
}

func TestParsePrintOutputCursorStreamJSONUsage(t *testing.T) {
	raw := `{"type":"system","subtype":"init","cwd":"/tmp"}` + "\n" +
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"pong"}]}}` + "\n" +
		`{"type":"result","subtype":"success","result":"pong","usage":{"inputTokens":57151,"outputTokens":33,"cacheReadTokens":384}}`
	fragments, usage := parsePrintOutput(raw)
	if len(fragments) != 1 || fragments[0].Role != "assistant" || fragments[0].Body != "pong" {
		t.Fatalf("fragments = %+v", fragments)
	}
	if usage == nil || usage.Input != 57151 || usage.Output != 33 || usage.CacheRead != 384 {
		t.Fatalf("usage = %+v", usage)
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

func TestComposerPrintPersistsUsageOnPage(t *testing.T) {
	printed := make(chan struct{})
	hostPrint = func(ctx context.Context, host hosts.Host, prompt string) (string, error) {
		defer close(printed)
		return `{"type":"item.completed","item":{"type":"agent_message","text":"pong"}}` + "\n" +
			`{"type":"turn.completed","usage":{"input_tokens":9,"output_tokens":2}}`, nil
	}
	t.Cleanup(func() { hostPrint = runHostPrint })

	ready := false
	for _, entry := range hosts.Inventory() {
		if entry.Binary == "cursor-agent" && entry.OnPath {
			ready = true
			break
		}
	}
	if !ready {
		t.Skip("cursor-agent not on PATH")
	}

	root, slug := writeEmptySession(t)
	handler, err := NewHandlerWithPort(root, testToolkitRoot(t), 3080)
	if err != nil {
		t.Fatal(err)
	}
	form := strings.NewReader("host=cursor-agent&message=ping")
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/session/"+slug+"/send", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d", rec.Code)
	}
	select {
	case <-printed:
	case <-time.After(2 * time.Second):
		t.Fatal("print stub was not called")
	}
	deadline := time.Now().Add(2 * time.Second)
	var html string
	for time.Now().Before(deadline) {
		pageReq := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/session/"+slug+"?view=chat", nil)
		pageRec := httptest.NewRecorder()
		handler.ServeHTTP(pageRec, pageReq)
		html = pageRec.Body.String()
		if strings.Contains(html, `data-testid="token-usage">in 9 · out 2`) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("expected toolbar tokens in 9 / out 2:\n%s", html)
}

func TestComposerEmptyPrintWritesVisibleRow(t *testing.T) {
	printed := make(chan struct{})
	hostPrint = func(ctx context.Context, host hosts.Host, prompt string) (string, error) {
		defer close(printed)
		return "", nil
	}
	t.Cleanup(func() { hostPrint = runHostPrint })

	root, slug := writeEmptySession(t)
	handler, err := NewHandlerWithPort(root, testToolkitRoot(t), 3080)
	if err != nil {
		t.Fatal(err)
	}
	form := strings.NewReader("host=cursor-agent&message=ping")
	// Skip if cursor not on PATH: SendComposerMessage checks inventory.
	ready := false
	for _, entry := range hosts.Inventory() {
		if entry.Binary == "cursor-agent" && entry.OnPath {
			ready = true
			break
		}
	}
	if !ready {
		t.Skip("cursor-agent not on PATH")
	}
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
		t.Fatal("print stub was not called")
	}
	deadline := time.Now().Add(2 * time.Second)
	var html string
	for time.Now().Before(deadline) {
		pageReq := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/session/"+slug+"?view=chat", nil)
		pageRec := httptest.NewRecorder()
		handler.ServeHTTP(pageRec, pageReq)
		html = pageRec.Body.String()
		if strings.Contains(html, "Host produced no output.") {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("expected timeout/empty print row:\n%s", html)
}

func TestComposerPrintQuestionAppearsInChat(t *testing.T) {
	printed := make(chan struct{})
	hostPrint = func(ctx context.Context, host hosts.Host, prompt string) (string, error) {
		defer close(printed)
		return `{"type":"ask_user","prompt":"Approve the spec?"}`, nil
	}
	t.Cleanup(func() { hostPrint = runHostPrint })

	ready := false
	for _, entry := range hosts.Inventory() {
		if entry.Binary == "cursor-agent" && entry.OnPath {
			ready = true
			break
		}
	}
	if !ready {
		t.Skip("cursor-agent not on PATH")
	}
	root, slug := writeEmptySession(t)
	handler, err := NewHandlerWithPort(root, testToolkitRoot(t), 3080)
	if err != nil {
		t.Fatal(err)
	}
	form := strings.NewReader("host=cursor-agent&message=please+ask")
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/session/"+slug+"/send", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d", rec.Code)
	}
	select {
	case <-printed:
	case <-time.After(2 * time.Second):
		t.Fatal("print stub was not called")
	}
	deadline := time.Now().Add(2 * time.Second)
	var html string
	for time.Now().Before(deadline) {
		pageReq := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/session/"+slug+"?view=chat", nil)
		pageRec := httptest.NewRecorder()
		handler.ServeHTTP(pageRec, pageReq)
		html = pageRec.Body.String()
		if strings.Contains(html, `data-role="question"`) && strings.Contains(html, "Approve the spec?") && strings.Contains(html, "needs reply") {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("expected question row in chat:\n%s", html)
}
