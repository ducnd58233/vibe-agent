package mcp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/graph"
	"github.com/ducnd58233/vibe-agent/runtime/internal/loop"
	"github.com/ducnd58233/vibe-agent/runtime/internal/memory"
	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
)

const toolkitRoot = "../../.."

func at() time.Time { return time.Date(2026, 7, 29, 10, 0, 0, 0, time.UTC) }

func newDeps(t *testing.T) Deps {
	t.Helper()
	store, err := memory.OpenAt(t.Context(), filepath.Join(t.TempDir(), "memory.db"))
	if err != nil {
		t.Fatalf("OpenAt: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return Deps{
		WorkspaceRoot: t.TempDir(),
		ToolkitRoot:   toolkitRoot,
		WorkspaceID:   "ws1",
		Memory:        memory.Adopt(store),
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
	return text(t, object(t, content[0], "content[0]")["text"], "content[0].text")
}

// A short list, deliberately. A long tool list makes routing worse, so every
// entry carries the reason it is worth a slot and adding one means writing that
// reason down. A name with no reason fails, which is the rule made checkable
// rather than left in a comment.
func TestEveryToolOnTheSurfaceEarnsItsSlot(t *testing.T) {
	server := NewServer("test", newDeps(t))
	want := map[string]string{
		"vibe_bootstrap":         "the entry point; without it a session works from a memorised asset list",
		"vibe_memory_search":     "what earlier runs established, so this one does not re-derive it",
		"vibe_memory_propose":    "the only way a memory is written, and it still needs confirming",
		"vibe_run_start":         "hosts without hooks have no other way to begin a run",
		"vibe_run_status":        "the node and its evidence, which is the one thing never to infer",
		"vibe_task_packet":       "the next actionable task in one call, instead of a manual re-read of tasks.json and TASKS.md",
		"vibe_repo_map":          "a token-budgeted map of referenced definitions, so the model reads less of the tree by hand",
		"vibe_experiment_status": "STATUS.md for researcher-delivery monitor loops; compute stays on host/CI",
		"vibe_checkpoint":        "records evidence; the verifiers stay behind vibe_verify on purpose",
		"vibe_verify":            "runs what the check plan declares, so the command is not chosen at the keyboard",
		"vibe_fetch":             "the only tool here that returns tokens rather than spending them",
	}
	if len(server.Tools) != len(want) {
		t.Errorf("got %d tools, want %d", len(server.Tools), len(want))
	}
	for _, tool := range server.Tools {
		if want[tool.Name] == "" {
			t.Errorf("tool %q is on the surface with no reason recorded; add one here or take it off", tool.Name)
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

// Individual verifiers stay unexposed. vibe_verify runs whichever one the graph
// and the check plan name, and records the result with the transition it
// justifies. A per-verifier tool would let a caller pick the weakest one, or run
// a verifier and then decide separately what to write down.
func TestIndividualVerifiersAreNotExposedAsTools(t *testing.T) {
	server := NewServer("test", newDeps(t))
	for _, tool := range server.Tools {
		for _, kind := range []string{"command", "files", "git"} {
			if strings.Contains(tool.Name, kind) {
				t.Errorf("tool %q exposes the %s verifier directly", tool.Name, kind)
			}
		}
	}
}

// vibe_verify must not accept a verdict. A "passed" parameter would make it a
// synonym for vibe_checkpoint and reopen the hole it exists to close.
func TestVerifyTakesNoVerdict(t *testing.T) {
	server := NewServer("test", newDeps(t))
	for _, tool := range server.Tools {
		if tool.Name != "vibe_verify" {
			continue
		}
		var parsed struct {
			Properties map[string]any `json:"properties"`
		}
		if err := json.Unmarshal(tool.InputSchema, &parsed); err != nil {
			t.Fatalf("schema: %v", err)
		}
		for _, forbidden := range []string{"passed", "failed", "skipped", "source", "exitCode"} {
			if _, present := parsed.Properties[forbidden]; present {
				t.Errorf("vibe_verify accepts %q, so a caller could supply the outcome", forbidden)
			}
		}
		return
	}
	t.Fatal("vibe_verify is not on the surface")
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
	result := object(t, replies[0]["result"], "replies[0].result")
	if result["protocolVersion"] != ProtocolVersion {
		t.Errorf("protocolVersion = %v", result["protocolVersion"])
	}
	tools := list(t, object(t, replies[1]["result"], "replies[1].result")["tools"], "tools")
	if len(tools) != len(server.Tools) {
		t.Errorf("listed %d tools, want %d", len(tools), len(server.Tools))
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
	if !strings.Contains(started, `"currentNode":"intake"`) {
		t.Fatalf("run did not start at intake: %s", started)
	}
	if !strings.Contains(started, "requiredAction") {
		t.Error("run start does not say what to do next")
	}

	status := toolText(t, exchange(t, server, call("vibe_run_status", map[string]any{"slug": "demo"}))[0])
	if !strings.Contains(status, `"graph":"goal-delivery"`) {
		t.Errorf("status does not name the graph: %s", status)
	}

	advanced := toolText(t, exchange(t, server, call("vibe_checkpoint", map[string]any{
		"slug": "demo",
		"check": map[string]any{
			"name": "intake_confirmed", "passed": true, "source": "human_event",
		},
	}))[0])
	if !strings.Contains(advanced, `"to":"spec"`) {
		t.Errorf("checkpoint did not advance to spec: %s", advanced)
	}
}

// The goal text does not change between calls on the same run. Repeating it on
// every status/checkpoint/verify response spends tokens for no new
// information; a caller who needs the text can call bootstrap or run_start.
func TestGoalIsNotRepeatedOnEveryCall(t *testing.T) {
	deps := newDeps(t)
	server := NewServer("test", deps)

	started := toolText(t, exchange(t, server, call("vibe_run_start", map[string]any{
		"slug": "demo", "goal": "a goal that must not repeat forever",
	}))[0])
	if !strings.Contains(started, "a goal that must not repeat forever") {
		t.Fatal("run_start does not report the goal it was given")
	}

	status := toolText(t, exchange(t, server, call("vibe_run_status", map[string]any{"slug": "demo"}))[0])
	if strings.Contains(status, "a goal that must not repeat forever") {
		t.Error("run_status repeats the full goal text; it should not")
	}

	advanced := toolText(t, exchange(t, server, call("vibe_checkpoint", map[string]any{
		"slug": "demo",
		"check": map[string]any{
			"name": "intake_confirmed", "passed": true, "source": "human_event",
		},
	}))[0])
	if strings.Contains(advanced, "a goal that must not repeat forever") {
		t.Error("checkpoint repeats the full goal text; it should not")
	}
}

// A long run accumulates many checks. Serializing the whole history on every
// call repeats what earlier calls already reported; a count is enough unless a
// caller asks for the full picture.
func TestChecksAreCompactNotTheFullHistory(t *testing.T) {
	deps := newDeps(t)
	server := NewServer("test", deps)
	exchange(t, server, call("vibe_run_start", map[string]any{"slug": "demo", "goal": "g"}))
	advanced := toolText(t, exchange(t, server, call("vibe_checkpoint", map[string]any{
		"slug": "demo",
		"check": map[string]any{
			"name": "intake_confirmed", "passed": true, "source": "human_event",
		},
	}))[0])
	if strings.Contains(advanced, `"intake_confirmed":`) {
		t.Errorf("response still serializes the full checks map by name: %s", advanced)
	}
	if !strings.Contains(advanced, "checksSummary") {
		t.Errorf("response has no compact checks summary: %s", advanced)
	}
}

// Every description follows one shape: when to call, when not to. A model
// routing between tools reads this once per session; a consistent shape costs
// less to parse than free prose that varies tool to tool.
func TestDescriptionsStateWhenToCallAndWhenNotTo(t *testing.T) {
	server := NewServer("test", newDeps(t))
	for _, tool := range server.Tools {
		if !strings.Contains(tool.Description, "Call") {
			t.Errorf("tool %q description does not say when to call it: %q", tool.Name, tool.Description)
		}
		if !strings.Contains(tool.Description, "not call") {
			t.Errorf("tool %q description does not say when not to call it: %q", tool.Name, tool.Description)
		}
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
	if !strings.Contains(proposed, `"verdict":"store"`) {
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

// object and list narrow a decoded reply, failing the test rather than
// panicking, so a protocol change names itself instead of arriving as an
// interface conversion three frames down.
// A host starts this server in every workspace it opens, and Codex and opencode
// reach the control plane no other way. Creating the database at startup put an
// empty .agent-state/ into repositories that had never used the toolkit, which
// is the rule recall and doctor already keep and this surface did not.
func TestServingAWorkspaceDoesNotCreateADatabase(t *testing.T) {
	root := t.TempDir()
	deps := Deps{
		WorkspaceRoot: root,
		WorkspaceID:   root,
		Memory:        memory.NewLazy(root),
	}
	defer func() { _ = deps.Memory.Close() }()

	server := NewServer("test", deps)

	// Both reads, on a workspace that has stored nothing.
	if _, err := bootstrap(deps, json.RawMessage(`{}`)); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	if _, err := searchMemory(deps, json.RawMessage(`{"query":"anything"}`)); err != nil {
		t.Fatalf("searchMemory: %v", err)
	}
	_ = server

	if _, err := os.Stat(memory.DBPath(root)); err == nil {
		t.Errorf("reading an untouched workspace created %s", memory.DBPath(root))
	}
}

// The write path is what creates it, because that is what a write is for.
func TestProposingCreatesTheDatabase(t *testing.T) {
	root := t.TempDir()
	deps := Deps{
		WorkspaceRoot: root,
		WorkspaceID:   root,
		Memory:        memory.NewLazy(root),
	}
	defer func() { _ = deps.Memory.Close() }()

	if _, err := proposeMemory(deps, json.RawMessage(`{
		"kind":"episodic","content":"the build needs CGO_ENABLED=0",
		"evidence":["Makefile line 3"],"sourceType":"file_assert","sourceRef":"Makefile:3"
	}`)); err != nil {
		t.Fatalf("proposeMemory: %v", err)
	}
	if _, err := os.Stat(memory.DBPath(root)); err != nil {
		t.Errorf("a stored memory left no database at %s", memory.DBPath(root))
	}
}

func loadGoalDeliveryGraphForTest(t *testing.T) *graph.Graph {
	t.Helper()
	g, err := graph.LoadByID(graph.DefaultDir(toolkitRoot), "goal-delivery")
	if err != nil {
		t.Fatalf("load goal-delivery graph: %v", err)
	}
	return g
}

func newRunAt(t *testing.T, g *graph.Graph, node string) *state.Run {
	t.Helper()
	run, err := state.NewRun("demo", "goal", g.Metadata.ID, g.Spec.MaxTransitions, at())
	if err != nil {
		t.Fatalf("new run: %v", err)
	}
	run.CurrentNode = node
	return run
}

// relevantTools narrows what a model should call next without narrowing what
// it can call: only vibe_checkpoint/vibe_verify visibility varies by node
// type, since those two are the only tools that already error at the wrong
// node.
func TestRelevantToolsHintPerNodeType(t *testing.T) {
	g := loadGoalDeliveryGraphForTest(t)
	cases := []struct {
		node string
		want []string
	}{
		{"intake", []string{"vibe_checkpoint"}}, // human_gate
		{"spec", []string{"vibe_checkpoint"}},   // artifact
		{"build", []string{"vibe_checkpoint"}},  // agent
		{"test", []string{"vibe_verify"}},       // verifier
		{"done", []string{}},                    // terminal
	}
	for _, tc := range cases {
		run := newRunAt(t, g, tc.node)
		out := describe(g, run)
		action := object(t, out["requiredAction"], "requiredAction")
		got := action["relevantTools"]
		gotList, _ := got.([]string)
		if len(gotList) != len(tc.want) {
			t.Errorf("node %q: relevantTools = %v, want %v", tc.node, got, tc.want)
			continue
		}
		for i, name := range tc.want {
			if gotList[i] != name {
				t.Errorf("node %q: relevantTools = %v, want %v", tc.node, got, tc.want)
				break
			}
		}
	}
}

func writeTaskList(t *testing.T, root, slug, date string, version int, tasksJSON, tasksMD string) {
	t.Helper()
	dir := filepath.Join(root, "docs", date, slug, strconv.Itoa(version))
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tasks-"+date+".json"), []byte(tasksJSON), 0o600); err != nil {
		t.Fatal(err)
	}
	if tasksMD != "" {
		if err := os.WriteFile(filepath.Join(dir, "TASKS-"+date+".md"), []byte(tasksMD), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

// vibe_task_packet answers "what's next" in one call instead of a manual
// re-read of tasks.json and TASKS.md, and must never invent a task past what
// the files actually say.
func TestTaskPacketPicksTheRightNextTask(t *testing.T) {
	const date = "2026-08-18"

	t.Run("queued with satisfied deps", func(t *testing.T) {
		root := t.TempDir()
		writeTaskList(t, root, "demo", date, 1, `{
			"schemaVersion":1,"slug":"demo","date":"`+date+`","version":1,
			"tasks":[
				{"id":"T1","title":"first","status":"done"},
				{"id":"T2","title":"second","status":"queued","dependsOn":["T1"],"acceptance":"one line","branch":"feat/demo-t2"}
			]}`, "## T2: second  [queued]\n\nFuller detail here.\n")
		out, err := taskPacket(Deps{WorkspaceRoot: root}, json.RawMessage(`{"slug":"demo"}`))
		if err != nil {
			t.Fatal(err)
		}
		result := object(t, out, "taskPacket result")
		if result["status"] != "task_ready" {
			t.Fatalf("status = %v", result["status"])
		}
		task := object(t, result["task"], "task")
		if task["id"] != "T2" {
			t.Errorf("picked task %v, want T2", task["id"])
		}
		if task["acceptanceDetail"] != "## T2: second  [queued]\n\nFuller detail here." {
			t.Errorf("acceptanceDetail = %q", task["acceptanceDetail"])
		}
	})

	t.Run("in progress task wins over a queued one", func(t *testing.T) {
		root := t.TempDir()
		writeTaskList(t, root, "demo", date, 1, `{
			"schemaVersion":1,"slug":"demo","date":"`+date+`","version":1,
			"tasks":[
				{"id":"T1","title":"first","status":"in_progress"},
				{"id":"T2","title":"second","status":"queued"}
			]}`, "")
		out, err := taskPacket(Deps{WorkspaceRoot: root}, json.RawMessage(`{"slug":"demo"}`))
		if err != nil {
			t.Fatal(err)
		}
		task := object(t, object(t, out, "taskPacket result")["task"], "task")
		if task["id"] != "T1" {
			t.Errorf("picked task %v, want T1 (in progress)", task["id"])
		}
	})

	t.Run("queued task blocked on an unsettled dependency is skipped", func(t *testing.T) {
		root := t.TempDir()
		writeTaskList(t, root, "demo", date, 1, `{
			"schemaVersion":1,"slug":"demo","date":"`+date+`","version":1,
			"tasks":[
				{"id":"T1","title":"first","status":"queued"},
				{"id":"T2","title":"second","status":"queued","dependsOn":["T1"]}
			]}`, "")
		out, err := taskPacket(Deps{WorkspaceRoot: root}, json.RawMessage(`{"slug":"demo"}`))
		if err != nil {
			t.Fatal(err)
		}
		task := object(t, object(t, out, "taskPacket result")["task"], "task")
		if task["id"] != "T1" {
			t.Errorf("picked task %v, want T1 (T2 depends on it and is not settled)", task["id"])
		}
	})

	t.Run("all tasks done", func(t *testing.T) {
		root := t.TempDir()
		writeTaskList(t, root, "demo", date, 1, `{
			"schemaVersion":1,"slug":"demo","date":"`+date+`","version":1,
			"tasks":[{"id":"T1","title":"first","status":"done"}]}`,
			"## T1: first  [done]\n\n**Acceptance criteria:**\n- [x] shipped\n")
		out, err := taskPacket(Deps{WorkspaceRoot: root}, json.RawMessage(`{"slug":"demo"}`))
		if err != nil {
			t.Fatal(err)
		}
		if status := object(t, out, "taskPacket result")["status"]; status != "all_done" {
			t.Errorf("status = %v, want all_done", status)
		}
	})

	t.Run("done with open acceptance boxes is not finished", func(t *testing.T) {
		root := t.TempDir()
		writeTaskList(t, root, "demo", date, 1, `{
			"schemaVersion":1,"slug":"demo","date":"`+date+`","version":1,
			"tasks":[{"id":"T1","title":"first","status":"done"}]}`,
			"## T1: first  [done]\n\n**Acceptance criteria:**\n- [ ] still open\n")
		out, err := taskPacket(Deps{WorkspaceRoot: root}, json.RawMessage(`{"slug":"demo"}`))
		if err != nil {
			t.Fatal(err)
		}
		result := object(t, out, "taskPacket result")
		if result["status"] != "task_ready" {
			t.Fatalf("status = %v, want task_ready while AC is open", result["status"])
		}
		task := object(t, result["task"], "task")
		if task["id"] != "T1" {
			t.Errorf("picked %v, want T1", task["id"])
		}
	})

	t.Run("no tasks file yet", func(t *testing.T) {
		root := t.TempDir()
		out, err := taskPacket(Deps{WorkspaceRoot: root}, json.RawMessage(`{"slug":"demo"}`))
		if err != nil {
			t.Fatal(err)
		}
		if status := object(t, out, "taskPacket result")["status"]; status != "no_plan_yet" {
			t.Errorf("status = %v, want no_plan_yet", status)
		}
	})
}

func object(t *testing.T, value any, what string) map[string]any {
	t.Helper()
	narrowed, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("%s is %T, want an object", what, value)
	}
	return narrowed
}

func list(t *testing.T, value any, what string) []any {
	t.Helper()
	narrowed, ok := value.([]any)
	if !ok {
		t.Fatalf("%s is %T, want a list", what, value)
	}
	return narrowed
}

func text(t *testing.T, value any, what string) string {
	t.Helper()
	narrowed, ok := value.(string)
	if !ok {
		t.Fatalf("%s is %T, want a string", what, value)
	}
	return narrowed
}

// The eighth tool exists so Codex and opencode, which have no hooks, can reach
// the one runtime capability that returns tokens rather than spending them.
func TestFetchIsExposedAsATool(t *testing.T) {
	tools := Tools(Deps{WorkspaceRoot: t.TempDir()})
	var found *Tool
	for i := range tools {
		if tools[i].Name == "vibe_fetch" {
			found = &tools[i]
			break
		}
	}
	if found == nil {
		t.Fatal("vibe_fetch is not in the tool list")
	}
	for _, want := range []string{"source", "budget", "refresh"} {
		if !strings.Contains(string(found.InputSchema), want) {
			t.Errorf("schema does not name %q: %s", want, found.InputSchema)
		}
	}
	if !strings.Contains(found.Description, "budget") {
		t.Errorf("the description does not say it budgets: %q", found.Description)
	}
}

// Clipped by the tool, not by the caller. The point is that the untrimmed text
// never reaches a context window.
func TestFetchClipsToTheBudget(t *testing.T) {
	root := t.TempDir()
	long := strings.Repeat("a line of text that goes on and on\n", 400)
	path := filepath.Join(root, "long.md")
	if err := os.WriteFile(path, []byte(long), 0o600); err != nil {
		t.Fatal(err)
	}

	out, err := fetchSource(Deps{WorkspaceRoot: root}, json.RawMessage(
		`{"source":`+quote(path)+`,"budget":50}`))
	if err != nil {
		t.Fatalf("fetchSource: %v", err)
	}
	result, ok := out.(map[string]any)
	if !ok {
		t.Fatalf("result is %T, want a map", out)
	}
	text, _ := result["text"].(string)
	if text == "" {
		t.Fatal("no text returned")
	}
	if len(text) >= len(long) {
		t.Error("the text was not clipped")
	}
	if omitted, _ := result["omittedLines"].(int); omitted <= 0 {
		t.Error("clipping happened but omittedLines did not say so")
	}
}

// An error a model can act on, not a panic. It has to name the source, or the
// next move is the same call again.
func TestAFailedFetchReturnsAnActionableError(t *testing.T) {
	_, err := fetchSource(Deps{WorkspaceRoot: t.TempDir()}, json.RawMessage(
		`{"source":"./does-not-exist-anywhere.md"}`))
	if err == nil {
		t.Fatal("a missing source returned no error")
	}
	if !strings.Contains(err.Error(), "does-not-exist-anywhere") {
		t.Errorf("the error does not name the source: %v", err)
	}
}

func TestFetchRefusesAnEmptySource(t *testing.T) {
	if _, err := fetchSource(Deps{WorkspaceRoot: t.TempDir()}, json.RawMessage(`{"source":"  "}`)); err == nil {
		t.Error("an empty source was accepted")
	}
}

func TestFetchClipFromTailKeepsTheEnd(t *testing.T) {
	root := t.TempDir()
	var b strings.Builder
	for i := 0; i < 300; i++ {
		fmt.Fprintf(&b, "line-%03d\n", i)
	}
	path := filepath.Join(root, "long.log")
	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := fetchSource(Deps{WorkspaceRoot: root}, json.RawMessage(
		`{"source":`+quote(path)+`,"budget":40,"clipFrom":"tail"}`))
	if err != nil {
		t.Fatalf("fetchSource: %v", err)
	}
	result, ok := out.(map[string]any)
	if !ok {
		t.Fatalf("result is %T, want a map", out)
	}
	text, _ := result["text"].(string)
	if !strings.Contains(text, "line-299") {
		t.Fatalf("tail clip missing last line: %q", text)
	}
	if strings.Contains(text, "line-000") {
		t.Fatalf("tail clip kept the start: %q", text)
	}
	if omitted, _ := result["omittedLines"].(int); omitted <= 0 {
		t.Error("omittedLines should report dropped leading lines")
	}
	if from, _ := result["clipFrom"].(string); from != "tail" {
		t.Errorf("clipFrom=%q, want tail", from)
	}
}

func TestFetchCheckShorthandReadsVerifierLog(t *testing.T) {
	root := t.TempDir()
	entry, err := state.PrepareStart(root, "clip-check", at())
	if err != nil {
		t.Fatalf("PrepareStart: %v", err)
	}
	runDir := state.RunDir(root, "clip-check")
	if runDir == "" {
		t.Fatal("RunDir empty after PrepareStart")
	}
	_ = entry
	logDir := filepath.Join(runDir, "unit")
	if err := os.MkdirAll(logDir, 0o750); err != nil {
		t.Fatal(err)
	}
	body := strings.Repeat("HEAD\n", 80) + "TAIL-MARKER\n"
	if err := os.WriteFile(filepath.Join(logDir, "unit.log"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	out, err := fetchSource(Deps{WorkspaceRoot: root}, json.RawMessage(
		`{"source":"check:clip-check:unit","budget":30,"clipFrom":"tail"}`))
	if err != nil {
		t.Fatalf("fetchSource check: %v", err)
	}
	result, ok := out.(map[string]any)
	if !ok {
		t.Fatalf("result is %T, want a map", out)
	}
	text, _ := result["text"].(string)
	if !strings.Contains(text, "TAIL-MARKER") {
		t.Fatalf("check: fetch missed log tail: %q", text)
	}

	_, err = fetchSource(Deps{WorkspaceRoot: root}, json.RawMessage(
		`{"source":"check:clip-check:missing"}`))
	if err == nil {
		t.Fatal("missing check log returned no error")
	}
	if !strings.Contains(err.Error(), "not run yet") {
		t.Errorf("want not-run-yet error, got %v", err)
	}
}

// quote renders a path as a JSON string, so a Windows separator survives.
func quote(s string) string {
	out, _ := json.Marshal(s)
	return string(out)
}

func TestInitializeDeclaresListChanged(t *testing.T) {
	server := NewServer("test", newDeps(t))
	replies := exchange(t, server, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`)
	caps := object(t, object(t, replies[0]["result"], "result")["capabilities"], "capabilities")
	toolsCap := object(t, caps["tools"], "tools")
	if toolsCap["listChanged"] != true {
		t.Fatalf("listChanged = %v, want true", toolsCap["listChanged"])
	}
}

func TestToolsListIsFullWithNoActiveRun(t *testing.T) {
	server := NewServer("test", newDeps(t))
	replies := exchange(t, server,
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
	)
	tools := list(t, object(t, replies[1]["result"], "result")["tools"], "tools")
	if len(tools) != len(server.Tools) {
		t.Fatalf("listed %d, want full %d", len(tools), len(server.Tools))
	}
}

func toolNames(t *testing.T, tools []any) map[string]bool {
	t.Helper()
	out := map[string]bool{}
	for _, raw := range tools {
		name, _ := object(t, raw, "tool")["name"].(string)
		out[name] = true
	}
	return out
}

func TestToolsListNarrowsCheckpointAndVerify(t *testing.T) {
	deps := newDeps(t)
	server := NewServer("test", deps)
	exchange(t, server, call("vibe_run_start", map[string]any{
		"slug": "narrow", "goal": "narrow tools/list",
	}))
	// intake is a human_gate -> checkpoint only
	replies := exchange(t, server, `{"jsonrpc":"2.0","id":3,"method":"tools/list"}`)
	names := toolNames(t, list(t, object(t, replies[0]["result"], "result")["tools"], "tools"))
	if !names["vibe_checkpoint"] || names["vibe_verify"] {
		t.Fatalf("at human_gate: checkpoint=%v verify=%v", names["vibe_checkpoint"], names["vibe_verify"])
	}
	if !names["vibe_repo_map"] || !names["vibe_fetch"] {
		t.Fatal("other tools must stay listed")
	}

	// Move to a verifier node (test) via empty checkpoint through the graph until test is hard;
	// instead load a fixture run parked at unit by writing status through checkpoint to test.
	// Simpler: set Session slug and write a run manifest at a verifier node via PrepareStart + Save.
	root := deps.WorkspaceRoot
	entry, err := state.PrepareStart(root, "verify-node", at())
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := graph.LoadByID(graph.DefaultDir(deps.ToolkitRoot), "goal-delivery")
	if err != nil {
		t.Fatal(err)
	}
	run, err := state.NewRun("verify-node", "goal", loaded.Metadata.ID, loaded.Spec.MaxTransitions, at())
	if err != nil {
		t.Fatal(err)
	}
	run.Date = entry.Date
	run.Version = entry.Version
	run.CurrentNode = "test"
	run.Status = state.StatusRunning
	if err := state.Save(state.ManifestPath(root, "verify-node"), run); err != nil {
		t.Fatal(err)
	}
	server.Session.Touch("verify-node")
	replies = exchange(t, server, `{"jsonrpc":"2.0","id":4,"method":"tools/list"}`)
	names = toolNames(t, list(t, object(t, replies[0]["result"], "result")["tools"], "tools"))
	if names["vibe_checkpoint"] || !names["vibe_verify"] {
		t.Fatalf("at verifier: checkpoint=%v verify=%v", names["vibe_checkpoint"], names["vibe_verify"])
	}

	run.CurrentNode = "done"
	run.Status = state.StatusDone
	if err := state.Save(state.ManifestPath(root, "verify-node"), run); err != nil {
		t.Fatal(err)
	}
	replies = exchange(t, server, `{"jsonrpc":"2.0","id":5,"method":"tools/list"}`)
	names = toolNames(t, list(t, object(t, replies[0]["result"], "result")["tools"], "tools"))
	if names["vibe_checkpoint"] || names["vibe_verify"] {
		t.Fatalf("at terminal both should be absent: %v", names)
	}
}

func TestListChangedNotificationOnRealTransitionOnly(t *testing.T) {
	deps := newDeps(t)
	server := NewServer("test", deps)
	exchange(t, server,
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		call("vibe_run_start", map[string]any{"slug": "notify", "goal": "emit list_changed"}),
	)
	// Real checkpoint from intake (empty outcome advances via !research_required path needs research flag;
	// intake with auto? Use checkpoint with no check - from intake, need research_required false.
	// Enter leaves at intake awaiting_human for non-auto. run_start Enter on non-auto parks at intake.
	// Checkpoint without check from awaiting intake may fail. Set auto flag like CLI auto does.
	run, err := state.Load(state.ManifestPath(deps.WorkspaceRoot, "notify"))
	if err != nil {
		t.Fatal(err)
	}
	if err := run.SetFlagAt("auto", true, at()); err != nil {
		t.Fatal(err)
	}
	// Re-enter gate so intake skips
	loaded, err := graph.LoadByID(graph.DefaultDir(deps.ToolkitRoot), run.GraphID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := loop.New(loaded).SettleGate(run); err != nil {
		t.Fatal(err)
	}
	if err := state.Save(state.ManifestPath(deps.WorkspaceRoot, "notify"), run); err != nil {
		t.Fatal(err)
	}

	replies := exchange(t, server, call("vibe_checkpoint", map[string]any{"slug": "notify"}))
	var sawNotify bool
	for _, reply := range replies {
		if reply["method"] == "notifications/tools/list_changed" {
			sawNotify = true
		}
	}
	if !sawNotify {
		t.Fatalf("expected list_changed after real transition, got %#v", replies)
	}

	// Duplicate: same empty checkpoint again may still advance; call with identical check.
	// Force duplicate by replaying exact same evidence: first write a check then replay.
	_ = exchange(t, server, call("vibe_checkpoint", map[string]any{
		"slug":  "notify",
		"check": map[string]any{"name": "dup", "passed": true, "source": "file_assert"},
	}))
	// Second identical
	replies = exchange(t, server, call("vibe_checkpoint", map[string]any{
		"slug":  "notify",
		"check": map[string]any{"name": "dup", "passed": true, "source": "file_assert"},
	}))
	sawNotify = false
	duplicateNoted := false
	for _, reply := range replies {
		if reply["method"] == "notifications/tools/list_changed" {
			sawNotify = true
		}
		if reply["result"] != nil {
			if strings.Contains(toolText(t, reply), `"duplicate":true`) {
				duplicateNoted = true
			}
		}
	}
	if !duplicateNoted {
		t.Fatal("second identical checkpoint should report duplicate")
	}
	if sawNotify {
		t.Fatal("duplicate checkpoint must not emit list_changed")
	}
}

func TestExperimentStatusReadsSTATUS(t *testing.T) {
	deps := newDeps(t)
	server := NewServer("test", deps)
	exchange(t, server, call("vibe_run_start", map[string]any{
		"slug": "exp-mcp", "goal": "status tool", "graph": "researcher-delivery",
	}))

	missing := toolText(t, exchange(t, server, call("vibe_experiment_status", map[string]any{
		"slug": "exp-mcp",
	}))[0])
	if !strings.Contains(missing, `"status":"missing"`) {
		t.Fatalf("want missing STATUS, got %s", missing)
	}
	if !strings.Contains(missing, "host_or_ci") {
		t.Error("must name the host/CI compute port")
	}

	path := filepath.Join(state.RunDir(deps.WorkspaceRoot, "exp-mcp"), "experiment", "STATUS.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("status: running\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	running := toolText(t, exchange(t, server, call("vibe_experiment_status", map[string]any{
		"slug": "exp-mcp",
	}))[0])
	if !strings.Contains(running, `"status":"running"`) {
		t.Fatalf("want running, got %s", running)
	}
}
