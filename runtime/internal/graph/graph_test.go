package graph

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// repoGraph is the real goal-delivery graph. Loading it here is the contract
// test between this loader and the asset the toolkit actually ships.
const repoGraph = "../../../.ai-agents/graphs/goal-delivery.yaml"

func TestLoadsTheRealGoalDeliveryGraph(t *testing.T) {
	loaded, err := Load(repoGraph)
	if err != nil {
		t.Fatalf("the shipped graph does not load: %v", err)
	}
	if loaded.Metadata.ID != "goal-delivery" {
		t.Errorf("id = %q", loaded.Metadata.ID)
	}
	if loaded.Spec.Initial != "intake" {
		t.Errorf("initial = %q, want intake", loaded.Spec.Initial)
	}
	if len(loaded.Spec.Nodes) < 10 {
		t.Errorf("got %d nodes, expected the full delivery loop", len(loaded.Spec.Nodes))
	}
}

func TestOutgoingEdgesPutTheFallbackLast(t *testing.T) {
	loaded, err := Load(repoGraph)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	// build has a conditional edge to failed and an unconditional one to test.
	edges := loaded.OutgoingEdges("build")
	if len(edges) < 2 {
		t.Fatalf("got %d edges from build, want at least 2", len(edges))
	}
	if edges[len(edges)-1].When != "" {
		t.Errorf("last edge from build is %q, want the unconditional fallback", edges[len(edges)-1].When)
	}
	for _, edge := range edges[:len(edges)-1] {
		if edge.When == "" {
			t.Errorf("an unconditional edge appears before the end: %+v", edge)
		}
	}
}

func TestGuardKeyFallsBackToItsName(t *testing.T) {
	explicit := Guard{Name: "unit_passed", Reads: "unit"}
	if explicit.Key() != "unit" {
		t.Errorf("Key() = %q, want unit", explicit.Key())
	}
	implicit := Guard{Name: "merge_approved"}
	if implicit.Key() != "merge_approved" {
		t.Errorf("Key() = %q, want merge_approved", implicit.Key())
	}
}

func mutate(t *testing.T, change func(map[string]any)) []byte {
	t.Helper()
	raw, err := os.ReadFile(repoGraph)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	var doc map[string]any
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	change(doc)
	out, err := yaml.Marshal(doc)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	return out
}

// object and list narrow a decoded fixture, failing the test rather than
// panicking, so a broken fixture names itself.
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

func spec(t *testing.T, doc map[string]any) map[string]any {
	t.Helper()
	return object(t, doc["spec"], "spec")
}

func nodes(t *testing.T, doc map[string]any) map[string]any {
	t.Helper()
	return object(t, spec(t, doc)["nodes"], "spec.nodes")
}

func edges(t *testing.T, doc map[string]any) []any {
	t.Helper()
	return list(t, spec(t, doc)["edges"], "spec.edges")
}

// Every case here has a matching case in scripts/check-graphs-test.py. The two
// checkers must agree, so the same broken shapes are listed in both.
func TestParseRejectsBrokenGraphs(t *testing.T) {
	cases := []struct {
		name   string
		change func(map[string]any)
		want   string
	}{
		{
			name: "unreachable node",
			change: func(doc map[string]any) {
				nodes(t, doc)["orphan"] = map[string]any{"type": "agent", "command": "build"}
			},
			want: "unreachable",
		},
		{
			name: "node cannot reach a terminal",
			change: func(doc map[string]any) {
				nodes(t, doc)["spin_a"] = map[string]any{"type": "agent", "command": "build"}
				nodes(t, doc)["spin_b"] = map[string]any{"type": "agent", "command": "build"}
				spec(t, doc)["edges"] = append(edges(t, doc),
					map[string]any{"from": "build", "to": "spin_a", "when": "blocked"},
					map[string]any{"from": "spin_a", "to": "spin_b"},
					map[string]any{"from": "spin_b", "to": "spin_a"},
				)
			},
			want: "cannot reach any terminal",
		},
		{
			// A negated skip on a gate holds in a fresh run, because a flag
			// nobody set reads as false. The gate would be gone before anything
			// had a chance to set anything.
			name: "human gate skipped by a negated condition",
			change: func(doc map[string]any) {
				gate, ok := nodes(t, doc)["approve_merge"].(map[string]any)
				if !ok {
					t.Fatal("approve_merge is not a node map")
				}
				gate["skipWhen"] = "!auto"
			},
			want: "skipped by default",
		},
		{
			name: "agent node claiming a skip condition",
			change: func(doc map[string]any) {
				build, ok := nodes(t, doc)["build"].(map[string]any)
				if !ok {
					t.Fatal("build is not a node map")
				}
				build["skipWhen"] = "auto"
			},
			want: "may not declare skipWhen",
		},
		{
			name: "undeclared guard",
			change: func(doc map[string]any) {
				spec(t, doc)["edges"] = append(edges(t, doc),
					map[string]any{"from": "build", "to": "done", "when": "nobody_declared_me"})
			},
			want: "undeclared guard",
		},
		{
			name: "expression instead of a guard name",
			change: func(doc map[string]any) {
				spec(t, doc)["edges"] = append(edges(t, doc),
					map[string]any{"from": "build", "to": "done", "when": "state.e2eRequired == false"})
			},
			want: "never expressions",
		},
		{
			name: "terminal with an outgoing edge",
			change: func(doc map[string]any) {
				spec(t, doc)["edges"] = append(edges(t, doc),
					map[string]any{"from": "done", "to": "build"})
			},
			want: "terminal node \"done\" has outgoing edges",
		},
		{
			name: "two unconditional edges from one node",
			change: func(doc map[string]any) {
				spec(t, doc)["edges"] = append(edges(t, doc),
					map[string]any{"from": "test", "to": "done"},
					map[string]any{"from": "test", "to": "failed"})
			},
			want: "unconditional edges",
		},
		{
			name: "unknown node type",
			change: func(doc map[string]any) {
				nodes(t, doc)["waiter"] = map[string]any{"type": "wait"}
				spec(t, doc)["edges"] = append(edges(t, doc),
					map[string]any{"from": "build", "to": "waiter", "when": "blocked"})
			},
			want: "unknown type",
		},
		{
			name: "human gate without a prompt",
			change: func(doc map[string]any) {
				gate := object(t, nodes(t, doc)["approve_spec"], "approve_spec")
				delete(gate, "prompt")
			},
			want: "needs a prompt",
		},
		{
			name: "two nodes writing the same check",
			change: func(doc map[string]any) {
				nodes(t, doc)["second_test"] = map[string]any{
					"type": "verifier", "verifier": "command", "check": "unit",
				}
				spec(t, doc)["edges"] = append(edges(t, doc),
					map[string]any{"from": "build", "to": "second_test", "when": "blocked"},
					map[string]any{"from": "second_test", "to": "done"})
			},
			want: "would overwrite",
		},
		{
			name: "guard reads a check no node writes",
			change: func(doc map[string]any) {
				for _, item := range list(t, spec(t, doc)["guards"], "spec.guards") {
					guard := object(t, item, "guard")
					if guard["name"] == "unit_passed" {
						guard["reads"] = "no_node_writes_this"
					}
				}
			},
			want: "permanently false",
		},
		{
			name: "guard declared but never used",
			change: func(doc map[string]any) {
				spec(t, doc)["guards"] = append(list(t, spec(t, doc)["guards"], "spec.guards"),
					map[string]any{"name": "never_referenced", "description": "d", "source": "flag"})
			},
			want: "never used",
		},
		{
			name: "unknown field",
			change: func(doc map[string]any) {
				spec(t, doc)["surpriseField"] = true
			},
			want: "field surpriseField not found",
		},
		{
			name: "initial node does not exist",
			change: func(doc map[string]any) {
				spec(t, doc)["initial"] = "nowhere"
			},
			want: "not a declared node",
		},
		{
			name: "wrong apiVersion",
			change: func(doc map[string]any) {
				doc["apiVersion"] = "vibe-agent/v2"
			},
			want: "apiVersion",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := Parse(mutate(t, testCase.change))
			if err == nil {
				t.Fatalf("Parse accepted a graph with %s", testCase.name)
			}
			if !strings.Contains(err.Error(), testCase.want) {
				t.Errorf("error does not mention %q:\n%v", testCase.want, err)
			}
		})
	}
}

func TestParseAcceptsTheUnmodifiedGraph(t *testing.T) {
	if _, err := Parse(mutate(t, func(map[string]any) {})); err != nil {
		t.Fatalf("Parse rejected the unmodified graph: %v", err)
	}
}

func TestLoadRejectsAnIDThatDoesNotMatchTheFilename(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "renamed.yaml")
	if err := os.WriteFile(path, mutate(t, func(map[string]any) {}), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, err := Load(path); err == nil {
		t.Error("Load accepted a graph whose id does not match its filename")
	}
}

func TestLoadByIDFindsTheShippedGraph(t *testing.T) {
	loaded, err := LoadByID(filepath.Dir(repoGraph), "goal-delivery")
	if err != nil {
		t.Fatalf("LoadByID: %v", err)
	}
	if loaded.Metadata.ID != "goal-delivery" {
		t.Errorf("id = %q", loaded.Metadata.ID)
	}
}

// A reset naming evidence nothing produces is a typo that reads as working:
// the transition would clear a key nobody writes, and the check it meant to
// clear would survive into the next task.
func TestParseRejectsAResetNamingNoCheck(t *testing.T) {
	_, err := Parse(mutate(t, func(doc map[string]any) {
		spec, _ := doc["spec"].(map[string]any)
		edges, _ := spec["edges"].([]any)
		first, _ := edges[0].(map[string]any)
		first["resets"] = []any{"not_a_check"}
	}))
	if err == nil {
		t.Fatal("a reset naming an unknown check parsed")
	}
	if !strings.Contains(err.Error(), "not_a_check") {
		t.Errorf("error = %q, want it to name the reset", err)
	}
}

// The shipped graph resets the per-task checks when a task cycle restarts, and
// leaves what a run earns once alone.
func TestTheShippedGraphResetsPerTaskChecks(t *testing.T) {
	loaded, err := Load(repoGraph)
	if err != nil {
		t.Fatal(err)
	}
	var resets []string
	for _, edge := range loaded.Spec.Edges {
		if edge.From == "task_complete" && edge.To == "build" {
			resets = edge.Resets
		}
	}
	if len(resets) == 0 {
		t.Fatal("the task cycle restarts without clearing anything")
	}
	held := map[string]bool{}
	for _, name := range resets {
		held[name] = true
	}
	if !held["merge_approved"] {
		t.Error("merge_approved survives its own task, which opens the gate on a stale approval")
	}
	for _, perRun := range []string{"spec_approved", "plan_approved", "intake_confirmed"} {
		if held[perRun] {
			t.Errorf("%q is earned once per run and must not be cleared per task", perRun)
		}
	}
}
