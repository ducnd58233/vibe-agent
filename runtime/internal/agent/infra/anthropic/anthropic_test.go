package anthropic

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	agent "github.com/ducnd58233/vibe-agent/runtime/internal/agent/domain"
)

// sentinelKey stands in for a credential without being shaped like one. A
// realistic-looking fixture would be refused by this repository's own write
// gate, which is the gate working rather than getting in the way.
const sentinelKey = "KEY-MUST-NOT-LEAK"

// serve stands up a local endpoint and returns a transport pointed at it, plus
// a pointer to the request body it last received.
//
// Recorded shapes rather than a live call: a test that needs a key and a
// network is a test that does not run in CI, and the thing under test here is
// the mapping, not the provider.
func serve(t *testing.T, status int, response string) (*Transport, *map[string]any) {
	t.Helper()
	captured := map[string]any{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &captured)
		captured["_header_version"] = r.Header.Get("anthropic-version")
		captured["_header_key"] = r.Header.Get("x-api-key")
		captured["_header_beta"] = r.Header.Get("anthropic-beta")
		w.Header().Set("content-type", "application/json")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, response)
	}))
	t.Cleanup(server.Close)
	return &Transport{APIKey: sentinelKey, BaseURL: server.URL, Client: server.Client()}, &captured
}

func ask(text string) agent.Conversation {
	return agent.Conversation{Messages: []agent.Message{{Role: agent.RoleUser, Text: text}}}
}

func TestAPlainReplyIsMappedToTheDomain(t *testing.T) {
	transport, sent := serve(t, http.StatusOK, `{
	  "content": [{"type": "text", "text": "the capital is Paris"}],
	  "stop_reason": "end_turn",
	  "usage": {"input_tokens": 12, "output_tokens": 5, "cache_read_input_tokens": 100}
	}`)

	reply, err := transport.Send(context.Background(), ask("what is the capital of France?"))
	if err != nil {
		t.Fatal(err)
	}
	if reply.Message.Text != "the capital is Paris" {
		t.Errorf("text = %q", reply.Message.Text)
	}
	if reply.StopReason != agent.StopEndTurn {
		t.Errorf("stop = %q", reply.StopReason)
	}
	if reply.Usage.Input != 12 || reply.Usage.Output != 5 || reply.Usage.CacheRead != 100 {
		t.Errorf("usage = %+v", reply.Usage)
	}
	if (*sent)["_header_version"] != Version {
		t.Errorf("anthropic-version = %v", (*sent)["_header_version"])
	}
	if (*sent)["_header_key"] != sentinelKey {
		t.Error("the key did not reach the header")
	}
}

// budget_tokens and the sampling parameters are rejected with a 400 on this
// model. Sending one because an older example used it is the failure this test
// exists to prevent.
func TestTheRequestOmitsWhatTheModelRejects(t *testing.T) {
	transport, sent := serve(t, http.StatusOK, `{"content":[],"stop_reason":"end_turn","usage":{}}`)
	if _, err := transport.Send(context.Background(), ask("hello")); err != nil {
		t.Fatal(err)
	}

	for _, rejected := range []string{"temperature", "top_p", "top_k", "budget_tokens"} {
		if _, present := (*sent)[rejected]; present {
			t.Errorf("%q was sent, and this model answers that with a 400", rejected)
		}
	}
	thinking, ok := (*sent)["thinking"].(map[string]any)
	if !ok || thinking["type"] != "adaptive" {
		t.Errorf("thinking = %v, want adaptive", (*sent)["thinking"])
	}
	if (*sent)["_header_beta"] != RefusalFallbackBeta {
		t.Errorf("beta header = %v, want the refusal fallback", (*sent)["_header_beta"])
	}
	if (*sent)["fallbacks"] != "default" {
		t.Errorf("fallbacks = %v", (*sent)["fallbacks"])
	}
}

func TestToolCallsAndResultsMakeTheRoundTrip(t *testing.T) {
	transport, sent := serve(t, http.StatusOK, `{
	  "content": [
	    {"type": "text", "text": "checking"},
	    {"type": "tool_use", "id": "toolu_1", "name": "read", "input": {"path": "a.txt"}}
	  ],
	  "stop_reason": "tool_use",
	  "usage": {"input_tokens": 1, "output_tokens": 1}
	}`)

	conversation := agent.Conversation{
		Tools: []agent.ToolSpec{{Name: "read", Description: "read a file"}},
		Messages: []agent.Message{
			{Role: agent.RoleUser, Text: "read a.txt"},
			{Role: agent.RoleAssistant, ToolCalls: []agent.ToolCall{
				{ID: "toolu_0", Name: "read", Input: json.RawMessage(`{"path":"a.txt"}`)},
			}},
			{Role: agent.RoleUser, ToolResults: []agent.ToolResult{
				{CallID: "toolu_0", Content: "boom", IsError: true},
			}},
		},
	}

	reply, err := transport.Send(context.Background(), conversation)
	if err != nil {
		t.Fatal(err)
	}
	if reply.StopReason != agent.StopToolUse {
		t.Fatalf("stop = %q", reply.StopReason)
	}
	if len(reply.Message.ToolCalls) != 1 || reply.Message.ToolCalls[0].ID != "toolu_1" {
		t.Fatalf("calls = %+v", reply.Message.ToolCalls)
	}
	if reply.Message.Text != "checking" {
		t.Errorf("text alongside a tool call was dropped: %q", reply.Message.Text)
	}

	// The failed result has to travel as a tool_result with is_error, or the
	// model reads a call as unanswered.
	body, err := json.Marshal((*sent)["messages"])
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"tool_result"`, `"tool_use_id":"toolu_0"`, `"is_error":true`, `"tool_use"`} {
		if !strings.Contains(string(body), want) {
			t.Errorf("request messages are missing %s: %s", want, body)
		}
	}
}

// A refusal arrives as HTTP 200 with a stop reason, so it must not be read as
// success. stop_details is null for every other reason, so it is guarded.
func TestARefusalCarriesItsCategory(t *testing.T) {
	transport, _ := serve(t, http.StatusOK, `{
	  "content": [],
	  "stop_reason": "refusal",
	  "stop_details": {"type": "refusal", "category": "cyber"},
	  "usage": {"input_tokens": 3, "output_tokens": 0}
	}`)

	reply, err := transport.Send(context.Background(), ask("do something"))
	if err != nil {
		t.Fatalf("a refusal was reported as a transport error: %v", err)
	}
	if reply.StopReason != agent.StopRefusal || reply.RefusalCategory != "cyber" {
		t.Errorf("stop = %q, category = %q", reply.StopReason, reply.RefusalCategory)
	}
}

func TestAStopReasonWithNoDetailsDoesNotPanic(t *testing.T) {
	transport, _ := serve(t, http.StatusOK,
		`{"content":[{"type":"text","text":"cut off"}],"stop_reason":"max_tokens","usage":{}}`)

	reply, err := transport.Send(context.Background(), ask("write an essay"))
	if err != nil {
		t.Fatal(err)
	}
	if reply.StopReason != agent.StopMaxTokens || reply.RefusalCategory != "" {
		t.Errorf("reply = %+v", reply)
	}
}

// 429 and 529 mean retry while 400 and 401 do not, so the status has to survive
// into the error a caller reads.
func TestAnErrorStatusKeepsItsCodeAndMessage(t *testing.T) {
	transport, _ := serve(t, http.StatusTooManyRequests,
		`{"type":"error","error":{"type":"rate_limit_error","message":"slow down"}}`)

	_, err := transport.Send(context.Background(), ask("hello"))
	if err == nil {
		t.Fatal("a 429 was read as success")
	}
	for _, want := range []string{"429", "rate_limit_error", "slow down"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error = %q, want it to contain %q", err, want)
		}
	}
}

// The response body is the API's own error object and carries no credential.
// The request does, so it never appears in an error.
func TestAnErrorNeverQuotesTheKey(t *testing.T) {
	transport, _ := serve(t, http.StatusUnauthorized,
		`{"error":{"type":"authentication_error","message":"bad key"}}`)

	_, err := transport.Send(context.Background(), ask("hello"))
	if err == nil {
		t.Fatal("a 401 was read as success")
	}
	if strings.Contains(err.Error(), sentinelKey) {
		t.Errorf("the key reached an error message: %q", err)
	}
}

func TestAMissingKeyIsRefusedBeforeAnyCall(t *testing.T) {
	transport := &Transport{BaseURL: "http://127.0.0.1:1"}
	if _, err := transport.Send(context.Background(), ask("hello")); err == nil {
		t.Fatal("a request went out with no key")
	}
}

func TestATransportSatisfiesThePort(t *testing.T) {
	var port agent.Transport = &Transport{}
	if port.Name() != DefaultModel {
		t.Errorf("name = %q, want %q", port.Name(), DefaultModel)
	}
}
