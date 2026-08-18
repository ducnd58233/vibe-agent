package app

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
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
	hostPrint = func(ctx context.Context, host hosts.Host, prompt string, opts hosts.PrintOptions) (string, error) {
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

func TestParsePrintOutputCursorToolCallStarted(t *testing.T) {
	raw := `{"type":"system","subtype":"init"}` + "\n" +
		`{"type":"tool_call","subtype":"started","call_id":"1","tool_call":{"readToolCall":{"args":{"path":"README.md"}}}}` + "\n" +
		`{"type":"tool_call","subtype":"completed","call_id":"1","tool_call":{"readToolCall":{"args":{"path":"README.md"},"result":{"success":{"content":"secret-should-not-land"}}}}}` + "\n" +
		`{"type":"assistant","message":{"content":[{"type":"text","text":"done"}]}}` + "\n" +
		`{"type":"result","subtype":"success","result":"done"}`
	fragments, _ := parsePrintOutput(raw)
	if len(fragments) != 2 {
		t.Fatalf("fragments = %+v", fragments)
	}
	if fragments[0].Role != "tool" || fragments[0].Tool != "read" || fragments[0].Type != "tool_use" {
		t.Fatalf("tool fragment = %+v", fragments[0])
	}
	if strings.Contains(fragments[0].Body, "secret-should-not-land") {
		t.Fatal("tool result body must not be stored")
	}
	if fragments[1].Role != "assistant" || fragments[1].Body != "done" {
		t.Fatalf("assistant = %+v", fragments[1])
	}
}

func TestParsePrintOutputSystemResultIsNotRawJSON(t *testing.T) {
	raw := `{"type":"system","subtype":"init","cwd":"/tmp"}` + "\n" +
		`{"type":"result","subtype":"success","result":""}`
	fragments, _ := parsePrintOutput(raw)
	if len(fragments) != 0 {
		t.Fatalf("expected no displayable fragments, got %+v", fragments)
	}
}

func TestParsePrintOutputTruncatedThenAssistant(t *testing.T) {
	raw := `{` + "\n" +
		`{"type":"assistant","message":{"content":[{"type":"text","text":"recovered"}]}}`
	fragments, _ := parsePrintOutput(raw)
	if len(fragments) != 1 || fragments[0].Body != "recovered" {
		t.Fatalf("fragments = %+v", fragments)
	}
}

func TestParsePrintOutputThinkingDeltas(t *testing.T) {
	raw := `{"type":"thinking","subtype":"delta","text":"I will "}` + "\n" +
		`{"type":"thinking","subtype":"delta","text":"read the file."}` + "\n" +
		`{"type":"thinking","subtype":"completed"}` + "\n" +
		`{"type":"assistant","message":{"content":[{"type":"thinking","thinking":"nested thought"},{"type":"text","text":"ok"}]}}`
	fragments, _ := parsePrintOutput(raw)
	var thinking []string
	var assistant []string
	for _, frag := range fragments {
		switch frag.Role {
		case "thinking":
			thinking = append(thinking, frag.Body)
		case "assistant":
			assistant = append(assistant, frag.Body)
		}
	}
	if len(thinking) != 2 || thinking[0] != "I will read the file." || thinking[1] != "nested thought" {
		t.Fatalf("thinking = %q fragments = %+v", thinking, fragments)
	}
	if len(assistant) != 1 || assistant[0] != "ok" {
		t.Fatalf("assistant = %q", assistant)
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
	hostPrint = func(ctx context.Context, host hosts.Host, prompt string, opts hosts.PrintOptions) (string, error) {
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
	hostPrint = func(ctx context.Context, host hosts.Host, prompt string, opts hosts.PrintOptions) (string, error) {
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

func TestComposerJSONStreamWithoutDisplayableText(t *testing.T) {
	printed := make(chan struct{})
	hostPrint = func(ctx context.Context, host hosts.Host, prompt string, opts hosts.PrintOptions) (string, error) {
		defer close(printed)
		return `{"type":"system","subtype":"init"}` + "\n" + `{"type":"result","subtype":"success","result":""}`, nil
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
		pageReq := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/session/"+slug, nil)
		pageRec := httptest.NewRecorder()
		handler.ServeHTTP(pageRec, pageReq)
		html = pageRec.Body.String()
		if strings.Contains(html, emptyPrintCopy) {
			if strings.Contains(html, `"type":"system"`) {
				t.Fatal("raw stream-json must not become the assistant body")
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("expected empty-print copy, not raw JSON:\n%s", html)
}

func TestComposerPrintQuestionAppearsInChat(t *testing.T) {
	printed := make(chan struct{})
	hostPrint = func(ctx context.Context, host hosts.Host, prompt string, opts hosts.PrintOptions) (string, error) {
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

func hostOnPath(binary string) bool {
	for _, entry := range hosts.Inventory() {
		if entry.Binary == binary && entry.OnPath {
			return true
		}
	}
	return false
}

func TestComposerModelAndAgentModeReachPrint(t *testing.T) {
	hostID := ""
	switch {
	case hostOnPath("cursor-agent"):
		hostID = "cursor-agent"
	case hostOnPath("claude"):
		hostID = "claude"
	default:
		t.Skip("no composer host on PATH")
	}
	printed := make(chan hosts.PrintOptions, 1)
	hostPrint = func(ctx context.Context, host hosts.Host, prompt string, opts hosts.PrintOptions) (string, error) {
		printed <- opts
		return "ok", nil
	}
	t.Cleanup(func() { hostPrint = runHostPrint })
	root, slug := writeEmptySession(t)
	handler, err := NewHandlerWithPort(root, testToolkitRoot(t), 3080)
	if err != nil {
		t.Fatal(err)
	}
	form := strings.NewReader("host=" + hostID + "&model=opus&mode=agent&message=ping")
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/session/"+slug+"/send", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d", rec.Code)
	}
	select {
	case opts := <-printed:
		if opts.Model != "opus" {
			t.Fatalf("model = %q", opts.Model)
		}
		if opts.Mode != "agent" {
			t.Fatalf("mode = %q", opts.Mode)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("print stub was not called")
	}
}

func TestComposerReplayPrefixesFollowUpAndRedacts(t *testing.T) {
	if !hostOnPath("cursor-agent") && !hostOnPath("claude") {
		t.Skip("no composer host on PATH")
	}
	hostID := "claude"
	if hostOnPath("cursor-agent") {
		hostID = "cursor-agent"
	}
	secret := "sk-0123456789abcdef0123456789ab"
	var prompts []string
	wait := make(chan struct{}, 8)
	hostPrint = func(ctx context.Context, host hosts.Host, prompt string, opts hosts.PrintOptions) (string, error) {
		prompts = append(prompts, prompt)
		wait <- struct{}{}
		return "ok", nil
	}
	t.Cleanup(func() { hostPrint = runHostPrint })
	root, slug := writeEmptySession(t)
	handler, err := NewHandlerWithPort(root, testToolkitRoot(t), 3080)
	if err != nil {
		t.Fatal(err)
	}
	send := func(host, message string) {
		t.Helper()
		form := strings.NewReader(url.Values{"host": {host}, "message": {message}}.Encode())
		req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/session/"+slug+"/send", form)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("status = %d", rec.Code)
		}
		select {
		case <-wait:
		case <-time.After(2 * time.Second):
			t.Fatal("print stub was not called")
		}
	}
	send(hostID, "deploy "+secret)
	if len(prompts) != 1 {
		t.Fatalf("prompts = %d", len(prompts))
	}
	if strings.Contains(prompts[0], "User:") {
		t.Fatalf("first send should have no prefix: %q", prompts[0])
	}
	if strings.Contains(prompts[0], secret) {
		t.Fatal("secret leaked to host print")
	}
	send(hostID, "follow-up")
	if len(prompts) < 2 {
		t.Fatal("missing follow-up print")
	}
	if !strings.Contains(prompts[1], "User:") || !strings.Contains(prompts[1], "follow-up") {
		t.Fatalf("follow-up missing prefix: %q", prompts[1])
	}
	if strings.Contains(prompts[1], secret) {
		t.Fatal("secret leaked in replay prefix")
	}
	alt := ""
	if hostID == "cursor-agent" && hostOnPath("claude") {
		alt = "claude"
	} else if hostID == "claude" && hostOnPath("cursor-agent") {
		alt = "cursor-agent"
	}
	if alt == "" {
		return
	}
	send(alt, "switched")
	last := prompts[len(prompts)-1]
	if !strings.Contains(last, "follow-up") {
		t.Fatalf("host switch lost prefix: %q", last)
	}
}
