package mcp

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/memory"
)

const toolkitRoot = "../../.."

func at() time.Time { return time.Date(2026, 7, 29, 10, 0, 0, 0, time.UTC) }

func newDeps(t *testing.T) Deps {
	t.Helper()
	store, err := memory.OpenAt(filepath.Join(t.TempDir(), "memory.db"))
	if err != nil {
		t.Fatalf("OpenAt: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return Deps{
		WorkspaceRoot: t.TempDir(),
		ToolkitRoot:   toolkitRoot,
		WorkspaceID:   "ws1",
		Memory:        store,
		Now:           at,
	}
}

func exchange(t *testing.T, server *Server, lines ...string) []map[string]any {
	t.Helper()
	var out bytes.Buffer
	if err := server.Serve(strings.NewReader(strings.Join(lines, "\n")+"\n"), &out); err != nil {
		t.Fatalf("Serve: %v", err)
	}
	var replies []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		if line == "" {
			continue
		}
		var reply map[string]any
		if err := json.Unmarshal([]byte(line), &reply); err != nil {
			t.Fatalf("reply is not JSON: %v: %q", err, line)
		}
		replies = append(replies, reply)
	}
	return replies
}

func call(name string, args map[string]any) string {
	payload, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": name, "arguments": args},
	})
	return string(payload)
}

func toolText(t *testing.T, reply map[string]any) string {
	t.Helper()
	result, ok := reply["result"].(map[string]any)
	if !ok {
		t.Fatalf("no result in reply: %v", reply)
	}
	content, ok := result["content"].([]any)
	if !ok || len(content) == 0 {
		t.Fatalf("no content in result: %v", result)
	}
	return content[0].(map[string]any)["text"].(string)
}

// Six tools, deliberately. A long tool list makes routing worse.
func TestSurfaceIsExactlySixTools(t *testing.T) {
	server := NewServer("test", newDeps(t))
	want := map[string]bool{
		"vibe_bootstrap": true, "vibe_memory_search": true, "vibe_memory_propose": true,
		"vibe_run_start": true, "vibe_run_status": true, "vibe_checkpoint": true,
	}
	if len(server.Tools) != len(want) {
		t.Errorf("got %d tools, want %d", len(server.Tools), len(want))
	}
	for _, tool := range server.Tools {
		if !want[tool.Name] {
			t.Errorf("unexpected tool %q", tool.Name)
		}
		if tool.Description == "" {
			t.Errorf("tool %q has no description", tool.Name)
		}
		var parsed map[string]any
		if err := json.Unmarshal(tool.InputSchema, &parsed); err != nil {
			t.Errorf("tool %q has an invalid input schema: %v", tool.Name, err)
		}
	}
}

// Verifiers are not exposed individually: evidence is recorded with the
// transition it justifies, never on its own.
func TestVerifiersAreNotExposedAsTools(t *testing.T) {
	server := NewServer("test", newDeps(t))
	for _, tool := range server.Tools {
		if strings.Contains(tool.Name, "verify") || strings.Contains(tool.Name, "verifier") {
			t.Errorf("verifier exposed as a tool: %q", tool.Name)
		}
	}
}

func TestHandshakeAndToolListing(t *testing.T) {
	server := NewServer("test", newDeps(t))
	replies := exchange(t, server,
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
	)
	if len(replies) != 2 {
		t.Fatalf("got %d replies, want 2", len(replies))
	}
	result := replies[0]["result"].(map[string]any)
	if result["protocolVersion"] != ProtocolVersion {
		t.Errorf("protocolVersion = %v", result["protocolVersion"])
	}
	tools := replies[1]["result"].(map[string]any)["tools"].([]any)
	if len(tools) != 6 {
		t.Errorf("listed %d tools, want 6", len(tools))
	}
}

// A server that exits on bad input takes the host session down with it.
func TestMalformedRequestDoesNotStopTheServer(t *testing.T) {
	server := NewServer("test", newDeps(t))
	replies := exchange(t, server,
		`this is not json`,
		`{"jsonrpc":"2.0","id":2,"method":"ping"}`,
	)
	if len(replies) != 2 {
		t.Fatalf("got %d replies, want an error then a working ping", len(replies))
	}
	if replies[0]["error"] == nil {
		t.Error("malformed input did not produce an error reply")
	}
	if replies[1]["result"] == nil {
		t.Error("server stopped serving after malformed input")
	}
}

func TestUnknownToolIsAnError(t *testing.T) {
	server := NewServer("test", newDeps(t))
	replies := exchange(t, server, call("vibe_do_my_job", map[string]any{}))
	if replies[0]["error"] == nil {
		t.Error("unknown tool did not error")
	}
}

func TestRunLifecycleThroughTools(t *testing.T) {
	deps := newDeps(t)
	server := NewServer("test", deps)

	started := toolText(t, exchange(t, server, call("vibe_run_start", map[string]any{
		"slug": "demo", "goal": "prove the tools work",
	}))[0])
	if !strings.Contains(started, `"currentNode": "intake"`) {
		t.Fatalf("run did not start at intake: %s", started)
	}
	if !strings.Contains(started, "requiredAction") {
		t.Error("run start does not say what to do next")
	}

	status := toolText(t, exchange(t, server, call("vibe_run_status", map[string]any{"slug": "demo"}))[0])
	if !strings.Contains(status, `"graph": "goal-delivery"`) {
		t.Errorf("status does not name the graph: %s", status)
	}

	advanced := toolText(t, exchange(t, server, call("vibe_checkpoint", map[string]any{
		"slug": "demo",
		"check": map[string]any{
			"name": "intake_confirmed", "passed": true, "source": "human_event",
		},
	}))[0])
	if !strings.Contains(advanced, `"to": "spec"`) {
		t.Errorf("checkpoint did not advance to spec: %s", advanced)
	}
}

// The provenance rule has to hold at the MCP boundary too, since this is the
// surface a model reaches directly.
func TestCheckpointRejectsAModelAssertedCheck(t *testing.T) {
	deps := newDeps(t)
	server := NewServer("test", deps)
	exchange(t, server, call("vibe_run_start", map[string]any{"slug": "demo", "goal": "g"}))

	text := toolText(t, exchange(t, server, call("vibe_checkpoint", map[string]any{
		"slug": "demo",
		"check": map[string]any{
			"name": "intake_confirmed", "passed": true, "source": "model",
		},
	}))[0])
	if !strings.Contains(text, "error") {
		t.Errorf("a model-asserted check was accepted: %s", text)
	}
}

func TestMemoryToolsRoundTripAndCarryTheDisclaimer(t *testing.T) {
	deps := newDeps(t)
	server := NewServer("test", deps)

	proposed := toolText(t, exchange(t, server, call("vibe_memory_propose", map[string]any{
		"kind":       "episodic",
		"content":    "Integration tests require Redis on localhost:6379.",
		"evidence":   []string{"make integration-test failed with connection refused"},
		"sourceType": "command_result",
	}))[0])
	if !strings.Contains(proposed, `"verdict": "store"`) {
		t.Fatalf("evidence-backed memory not stored: %s", proposed)
	}
	if !strings.Contains(proposed, "proposed") {
		t.Error("propose did not report the proposed status")
	}

	found := toolText(t, exchange(t, server, call("vibe_memory_search", map[string]any{
		"query": "Redis",
	}))[0])
	if !strings.Contains(found, "source of truth") {
		t.Errorf("search results do not carry the memory disclaimer: %s", found)
	}
}

func TestMemoryProposeRefusesASecret(t *testing.T) {
	deps := newDeps(t)
	server := NewServer("test", deps)
	text := toolText(t, exchange(t, server, call("vibe_memory_propose", map[string]any{
		"kind":       "semantic",
		"content":    "The staging api_key = sk-0123456789abcdef0123",
		"evidence":   []string{"found while reading the deployment configuration"},
		"sourceType": "file_content",
	}))[0])
	if !strings.Contains(text, "reject") {
		t.Errorf("a credential was accepted into memory: %s", text)
	}
}

func TestBootstrapReportsTheSourceOfTruthOrder(t *testing.T) {
	deps := newDeps(t)
	server := NewServer("test", deps)
	text := toolText(t, exchange(t, server, call("vibe_bootstrap", map[string]any{}))[0])
	if !strings.Contains(text, "repository code and config") {
		t.Errorf("bootstrap does not state the source-of-truth order: %s", text)
	}
	if !strings.Contains(text, "model assumptions") {
		t.Error("bootstrap does not rank model assumptions last")
	}
}
