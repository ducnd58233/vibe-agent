// Package anthropic maps the agent domain onto the Messages API.
//
// It is the only file in this runtime that knows a provider exists. The loop
// above it sends a Conversation and reads a Reply; swapping this adapter for
// another one is a change to this directory and nothing else.
//
// Raw net/http rather than the official SDK, because the spec for this work
// (docs/harness-autonomy/SPEC.md, A4) fixed the runtime's dependency set at
// what go.mod already holds plus one standard-library HTTP path. That is a
// deliberate cost: the SDK carries typed request builders and retry handling
// this file has to be careful about by hand. The port is what makes the choice
// reversible.
//
// Every wire detail here is read from the claude-api skill rather than
// recalled. The ones that would fail silently if guessed are noted where they
// are used.
package anthropic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	agent "github.com/ducnd58233/vibe-agent/runtime/internal/agent/domain"
)

const (
	// Endpoint is the Messages API. Everything goes through it: tools and
	// output constraints are features of this one endpoint, not separate APIs.
	Endpoint = "https://api.anthropic.com/v1/messages"

	// Version is the required anthropic-version header.
	Version = "2023-06-01"

	// KeyVariable names the environment variable the credential is read from.
	// It is a variable name, not a value: naming it after the key was both
	// misleading and enough to trip two different secret scanners.
	//
	// The key itself is never logged, never written to run state, and never
	// returned in an error.
	KeyVariable = "ANTHROPIC_API_KEY"

	// DefaultModel is Claude Opus 5: 1M context, 128K max output.
	DefaultModel = "claude-opus-5"

	// DefaultMaxTokens is the skill's non-streaming default. Lowballing it
	// truncates output mid-thought and costs a retry; this file does not
	// stream, so it stays under the SDK-style HTTP timeout rather than going
	// to the 128K ceiling.
	DefaultMaxTokens = 16000

	// RefusalFallbackBeta lets the server route a refused request to another
	// model by refusal category, rather than every caller maintaining a model
	// list. The skill asks for it by default on Opus 5.
	RefusalFallbackBeta = "server-side-fallback-2026-07-01"

	// DefaultTimeout bounds one request. The loop's wallclock budget bounds a
	// run; this bounds a single call that never returns.
	DefaultTimeout = 10 * time.Minute
)

// Transport calls the Messages API.
type Transport struct {
	// APIKey is read from the environment by New. Set directly only in tests.
	APIKey string
	Model  string
	// MaxTokens is the enforced per-response ceiling. The model is not aware of
	// it, which is why hitting it produces a truncated answer rather than a
	// shorter one.
	MaxTokens int
	// Effort is output_config.effort: low, medium, high, xhigh, or max. Empty
	// leaves the server default, which is high.
	Effort string
	// BaseURL is overridable so a test can point at a local server.
	BaseURL string
	Client  *http.Client
}

// New builds a transport from the environment.
func New() (*Transport, error) {
	key := strings.TrimSpace(os.Getenv(KeyVariable))
	if key == "" {
		return nil, fmt.Errorf("%s is not set", KeyVariable)
	}
	return &Transport{APIKey: key}, nil
}

func (t *Transport) Name() string { return t.model() }

func (t *Transport) model() string {
	if t.Model != "" {
		return t.Model
	}
	return DefaultModel
}

func (t *Transport) maxTokens() int {
	if t.MaxTokens > 0 {
		return t.MaxTokens
	}
	return DefaultMaxTokens
}

func (t *Transport) endpoint() string {
	if t.BaseURL != "" {
		return t.BaseURL
	}
	return Endpoint
}

func (t *Transport) client() *http.Client {
	if t.Client != nil {
		return t.Client
	}
	return &http.Client{Timeout: DefaultTimeout}
}

// Send makes one request and returns what came back.
func (t *Transport) Send(ctx context.Context, conversation agent.Conversation) (agent.Reply, error) {
	if t.APIKey == "" {
		return agent.Reply{}, fmt.Errorf("%s is not set", KeyVariable)
	}

	body, err := json.Marshal(t.request(conversation))
	if err != nil {
		return agent.Reply{}, fmt.Errorf("encode request: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, t.endpoint(), bytes.NewReader(body))
	if err != nil {
		return agent.Reply{}, err
	}
	request.Header.Set("content-type", "application/json")
	request.Header.Set("anthropic-version", Version)
	request.Header.Set("anthropic-beta", RefusalFallbackBeta)
	request.Header.Set("x-api-key", t.APIKey)

	response, err := t.client().Do(request)
	if err != nil {
		// The key can appear in a URL-shaped error from some transports, so the
		// error is rebuilt rather than wrapped.
		return agent.Reply{}, fmt.Errorf("call %s: request failed", t.model())
	}
	defer func() { _ = response.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(response.Body, 32<<20))
	if err != nil {
		return agent.Reply{}, fmt.Errorf("read response: %w", err)
	}
	if response.StatusCode != http.StatusOK {
		return agent.Reply{}, statusError(response.StatusCode, raw)
	}
	return decodeReply(raw)
}

// request builds the JSON body.
//
// Two parameters are deliberately absent. budget_tokens is rejected with a 400
// on Opus 5, replaced by adaptive thinking. temperature, top_p, and top_k are
// rejected too. Sending either because an older example used it is the failure
// this comment exists to prevent.
func (t *Transport) request(conversation agent.Conversation) map[string]any {
	body := map[string]any{
		"model":      t.model(),
		"max_tokens": t.maxTokens(),
		"messages":   wireMessages(conversation.Messages),
		"thinking":   map[string]any{"type": "adaptive"},
		// Route a refused request by category rather than keeping a model list
		// in every caller.
		"fallbacks": "default",
	}
	if conversation.System != "" {
		body["system"] = conversation.System
	}
	if t.Effort != "" {
		body["output_config"] = map[string]any{"effort": t.Effort}
	}
	if tools := wireTools(conversation.Tools); len(tools) > 0 {
		body["tools"] = tools
	}
	return body
}

func wireTools(tools []agent.ToolSpec) []map[string]any {
	out := make([]map[string]any, 0, len(tools))
	for _, tool := range tools {
		schema := json.RawMessage(`{"type":"object","properties":{}}`)
		if len(tool.Schema) > 0 {
			schema = tool.Schema
		}
		out = append(out, map[string]any{
			"name":         tool.Name,
			"description":  tool.Description,
			"input_schema": schema,
		})
	}
	return out
}

// wireMessages renders the conversation as content blocks.
//
// Tool results all sit in one user message because that is how the domain
// carries them, and splitting them here would undo the rule the loop keeps.
func wireMessages(messages []agent.Message) []map[string]any {
	out := make([]map[string]any, 0, len(messages))
	for _, message := range messages {
		blocks := make([]map[string]any, 0, 1+len(message.ToolCalls)+len(message.ToolResults))
		if message.Text != "" {
			blocks = append(blocks, map[string]any{"type": "text", "text": message.Text})
		}
		for _, call := range message.ToolCalls {
			input := json.RawMessage(`{}`)
			if len(call.Input) > 0 {
				input = call.Input
			}
			blocks = append(blocks, map[string]any{
				"type": "tool_use", "id": call.ID, "name": call.Name, "input": input,
			})
		}
		for _, result := range message.ToolResults {
			block := map[string]any{
				"type": "tool_result", "tool_use_id": result.CallID, "content": result.Content,
			}
			if result.IsError {
				block["is_error"] = true
			}
			blocks = append(blocks, block)
		}
		if len(blocks) == 0 {
			continue
		}
		out = append(out, map[string]any{"role": string(message.Role), "content": blocks})
	}
	return out
}

// wireReply is the response shape this adapter reads.
type wireReply struct {
	Content []struct {
		Type  string          `json:"type"`
		Text  string          `json:"text"`
		ID    string          `json:"id"`
		Name  string          `json:"name"`
		Input json.RawMessage `json:"input"`
	} `json:"content"`
	StopReason string `json:"stop_reason"`
	// StopDetails is populated only when stop_reason is refusal, and is null
	// for every other reason, so it is always guarded before being read.
	StopDetails *struct {
		Category string `json:"category"`
	} `json:"stop_details"`
	Usage struct {
		Input     int `json:"input_tokens"`
		Output    int `json:"output_tokens"`
		CacheRead int `json:"cache_read_input_tokens"`
	} `json:"usage"`
}

func decodeReply(raw []byte) (agent.Reply, error) {
	var wire wireReply
	if err := json.Unmarshal(raw, &wire); err != nil {
		return agent.Reply{}, fmt.Errorf("decode response: %w", err)
	}

	reply := agent.Reply{
		Message:    agent.Message{Role: agent.RoleAssistant},
		StopReason: agent.StopReason(wire.StopReason),
		Usage: agent.Usage{
			Input:     wire.Usage.Input,
			Output:    wire.Usage.Output,
			CacheRead: wire.Usage.CacheRead,
		},
	}
	if reply.StopReason == agent.StopRefusal && wire.StopDetails != nil {
		reply.RefusalCategory = wire.StopDetails.Category
	}

	var text strings.Builder
	for _, block := range wire.Content {
		switch block.Type {
		case "text":
			text.WriteString(block.Text)
		case "tool_use":
			reply.Message.ToolCalls = append(reply.Message.ToolCalls, agent.ToolCall{
				ID: block.ID, Name: block.Name, Input: block.Input,
			})
		}
	}
	reply.Message.Text = text.String()
	return reply, nil
}

// statusError renders a failure without ever quoting the request.
//
// The response body is the API's own error object and carries no credential;
// the request does, so it is never included. The status code is kept because
// 429 and 529 mean retry while 400 and 401 do not, and a caller that cannot
// tell them apart retries the wrong ones.
func statusError(status int, raw []byte) error {
	var wire struct {
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(raw, &wire) == nil && wire.Error.Message != "" {
		return fmt.Errorf("anthropic %d %s: %s", status, wire.Error.Type, wire.Error.Message)
	}
	return fmt.Errorf("anthropic %d: %s", status, strings.TrimSpace(string(raw)))
}
