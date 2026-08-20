package main

import (
	"strings"
	"testing"

	"github.com/ducnd58233/vibe-agent/runtime/internal/graph"
	"github.com/ducnd58233/vibe-agent/runtime/internal/testutil"
)

const validPathEvals = `apiVersion: vibe-agent/v1
kind: GraphPathEvals
spec:
  scenarios:
    - name: only scenario
      graph: goal-delivery
      steps:
        - check: {name: intake_confirmed, passed: true, source: human_event}
      expect:
        path: [intake, spec]
`

func TestPathEvalsAcceptAWellFormedFile(t *testing.T) {
	file, err := parsePathEvals([]byte(validPathEvals))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(file.Spec.Scenarios) != 1 {
		t.Fatalf("scenarios = %d, want 1", len(file.Spec.Scenarios))
	}
}

func TestPathEvalsRejectAnUnknownKind(t *testing.T) {
	raw := strings.Replace(validPathEvals, "GraphPathEvals", "WorkflowGraph", 1)
	if _, err := parsePathEvals([]byte(raw)); err == nil {
		t.Fatal("a graph file parsed as path evals; the two are not interchangeable")
	}
}

func TestPathEvalsRejectAnUnknownAPIVersion(t *testing.T) {
	raw := strings.Replace(validPathEvals, "vibe-agent/v1", "vibe-agent/v2", 1)
	if _, err := parsePathEvals([]byte(raw)); err == nil {
		t.Fatal("an unknown apiVersion was accepted")
	}
}

// Provenance is the property the whole control plane rests on. A fixture that
// records a check without saying where the evidence came from describes a state
// the runtime cannot reach, so it must not parse.
func TestPathEvalsRejectACheckWithNoSource(t *testing.T) {
	raw := strings.Replace(validPathEvals, ", source: human_event", "", 1)
	_, err := parsePathEvals([]byte(raw))
	if err == nil {
		t.Fatal("a check with no source was accepted")
	}
	if !strings.Contains(err.Error(), "provenance") {
		t.Errorf("error = %q, want it to name provenance", err)
	}
}

func TestPathEvalsRejectADuplicateScenarioName(t *testing.T) {
	raw := validPathEvals + `    - name: only scenario
      graph: goal-delivery
      expect:
        path: [intake]
`
	if _, err := parsePathEvals([]byte(raw)); err == nil {
		t.Fatal("two scenarios shared a name; the second would silently redefine the first")
	}
}

func TestPathEvalsRejectAScenarioWithNoExpectedPath(t *testing.T) {
	raw := strings.Replace(validPathEvals, "        path: [intake, spec]\n", "", 1)
	if _, err := parsePathEvals([]byte(raw)); err == nil {
		t.Fatal("a scenario expecting nothing was accepted; it would pass whatever the graph did")
	}
}

// The point of the suite: a path that does not match is reported, not tolerated.
func TestAWrongExpectedPathIsCaught(t *testing.T) {
	root := testutil.ToolkitRoot(t)
	scenario := pathScenario{
		Name:  "wrong on purpose",
		Graph: "goal-delivery",
		Steps: []pathStep{{Check: &pathCheck{
			Name: "intake_confirmed", Passed: true, Source: "human_event",
		}}},
		Expect: pathExpect{Path: []string{"intake", "build"}},
	}
	err := runPathScenario(graph.DefaultDir(root), scenario)
	if err == nil {
		t.Fatal("a scenario claiming intake goes straight to build passed")
	}
	if !strings.Contains(err.Error(), "walked") {
		t.Errorf("error = %q, want it to report the walk", err)
	}
}

// A skipped check is not a passed check. Only a guard that opted in may treat
// them alike, and e2e_ok is one that does.
func TestASkippedCheckOpensOnlyAnOptedInGate(t *testing.T) {
	root := graph.DefaultDir(testutil.ToolkitRoot(t))
	steps := []pathStep{
		{Check: &pathCheck{Name: "intake_confirmed", Passed: true, Source: "human_event"}},
		{},
		{Check: &pathCheck{Name: "spec_approved", Passed: true, Source: "human_event"}},
		{},
		{Check: &pathCheck{Name: "plan_approved", Passed: true, Source: "human_event"}},
		{},
		{Check: &pathCheck{Name: "unit", Skipped: true, Source: "exit_code"}},
	}
	path, _, err := walkPathScenario(root, pathScenario{
		Name: "skipped unit", Graph: "goal-delivery", Steps: steps,
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	// unit_passed does not accept a skip, so the run returns to build.
	if last := path[len(path)-1]; last != "build" {
		t.Errorf("after a skipped unit check the run sits at %q, want build", last)
	}
}

func TestThisRepositorysOwnPathEvalsPass(t *testing.T) {
	root := testutil.ToolkitRoot(t)
	file, present, err := loadPathEvals(root)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !present {
		t.Fatalf("this repository ships %s; it was not found", PathEvalsFileName)
	}
	for _, scenario := range file.Spec.Scenarios {
		if err := runPathScenario(graph.DefaultDir(root), scenario); err != nil {
			t.Errorf("%v", err)
		}
	}
}

func TestAMissingFixtureFileIsNotAProblem(t *testing.T) {
	_, present, err := loadPathEvals(t.TempDir())
	if err != nil {
		t.Fatalf("a toolkit without fixtures errored: %v", err)
	}
	if present {
		t.Error("an empty toolkit reported fixtures present")
	}
}

func TestCoverageNamesTheNodesNoScenarioReaches(t *testing.T) {
	root := testutil.ToolkitRoot(t)
	file, _, err := loadPathEvals(root)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	covered, total, uncovered, err := pathEvalCoverage(graph.DefaultDir(root), file)
	if err != nil {
		t.Fatalf("coverage: %v", err)
	}
	if total == 0 {
		t.Fatal("no nodes counted")
	}
	if covered+len(uncovered) != total {
		t.Errorf("covered %d + uncovered %d != total %d", covered, len(uncovered), total)
	}
}
