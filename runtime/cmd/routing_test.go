package main

import (
	"strings"
	"testing"

	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
)

// startGoal runs `goal` in a fresh workspace and returns the run it started.
func startGoal(t *testing.T, args ...string) (*state.Run, string) {
	t.Helper()
	root := t.TempDir()
	full := append([]string{"--workspace", root, "--toolkit", toolkitRoot, "--slug", "route-check"}, args...)
	if err := goalCommand(full); err != nil {
		t.Fatalf("goal %v: %v", args, err)
	}
	run, err := state.Load(state.ManifestPath(root, "route-check"))
	if err != nil {
		t.Fatal(err)
	}
	return run, root
}

func TestGoalWithAWorkflowWordStartsThatGraph(t *testing.T) {
	cases := []struct {
		args  []string
		graph string
	}{
		{[]string{"task", "Summarize", "the", "vendor", "contract"}, "task-delivery"},
		{[]string{"research", "Compare", "chunking", "strategies"}, "researcher-delivery"},
		{[]string{"experiment", "Benchmark", "two", "embedding", "models"}, "researcher-delivery"},
		{[]string{"tutor", "Linear", "algebra"}, "study-delivery"},
		{[]string{"delivery", "research", "tools", "stay", "listed"}, "goal-delivery"},
	}
	for _, tc := range cases {
		run, _ := startGoal(t, tc.args...)
		if run.GraphID != tc.graph {
			t.Errorf("goal %v: graph %q, want %q", tc.args, run.GraphID, tc.graph)
		}
		if run.Flags["auto"] {
			t.Errorf("goal %v set the auto flag", tc.args)
		}
	}
}

func TestGoalReadsTheObjectiveWhenNoWordIsGiven(t *testing.T) {
	run, _ := startGoal(t, "Summarize the vendor contract and write a memo for the team")
	if run.GraphID != "task-delivery" {
		t.Errorf("graph %q, want task-delivery", run.GraphID)
	}
	run, _ = startGoal(t, "Add retry to the webhook API")
	if run.GraphID != "goal-delivery" {
		t.Errorf("graph %q, want goal-delivery", run.GraphID)
	}
}

func TestAMixedObjectiveSetsTheTaskFlag(t *testing.T) {
	run, _ := startGoal(t, "Implement the export endpoint and write a report on how it works")
	if run.GraphID != "goal-delivery" || !run.Flags["task_required"] {
		t.Errorf("graph %q flags %v", run.GraphID, run.Flags)
	}
	run, _ = startGoal(t, "--with-task", "Ship the thing")
	if !run.Flags["task_required"] {
		t.Errorf("--with-task did not set the flag: %v", run.Flags)
	}
}

func TestGoalAsksWhenTheObjectiveCouldBeTwoKinds(t *testing.T) {
	root := t.TempDir()
	err := goalCommand([]string{"--workspace", root, "--toolkit", toolkitRoot,
		"Research the competitors and draft a report for the team"})
	if err == nil || !strings.Contains(err.Error(), "Say which") {
		t.Errorf("want a question, got %v", err)
	}
}

func TestAutoRoutesAndRefusesATutor(t *testing.T) {
	root := t.TempDir()
	optedIn(t, root, false)
	startAuto(t, root, "task", "Summarize the vendor contract and write a memo for the team")
	run, err := state.Load(state.ManifestPath(root, "summarize-vendor-contract-write"))
	if err != nil {
		t.Fatal(err)
	}
	if run.GraphID != "task-delivery" || !run.Flags["auto"] {
		t.Errorf("graph %q auto %v", run.GraphID, run.Flags["auto"])
	}

	err = autoCommand([]string{"--workspace", root, "--toolkit", toolkitRoot, "tutor", "Linear algebra"})
	if err == nil || !strings.Contains(err.Error(), "goal tutor") {
		t.Errorf("auto tutor: want a refusal naming goal tutor, got %v", err)
	}
}

func TestAutoExperimentIsTheResearcherGraph(t *testing.T) {
	root := t.TempDir()
	optedIn(t, root, false)
	startAuto(t, root, "experiment", "Benchmark two embedding models on recall")
	run, err := state.Load(state.ManifestPath(root, "benchmark-two-embedding-models"))
	if err != nil {
		t.Fatal(err)
	}
	if run.GraphID != "researcher-delivery" || !run.Flags["auto"] {
		t.Errorf("graph %q auto %v", run.GraphID, run.Flags["auto"])
	}
}

func TestTopLevelStartCommands(t *testing.T) {
	for name, tc := range map[string]struct {
		start func([]string) error
		graph string
	}{
		"task":       {taskCommand, "task-delivery"},
		"tutor":      {tutorCommand, "study-delivery"},
		"experiment": {experimentCommand, "researcher-delivery"},
		"research":   {researchCommand, "researcher-delivery"},
	} {
		root := t.TempDir()
		if err := tc.start([]string{"--workspace", root, "--toolkit", toolkitRoot, "--slug", "named", "Some objective"}); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		run, err := state.Load(state.ManifestPath(root, "named"))
		if err != nil || run.GraphID != tc.graph {
			t.Errorf("%s: graph %v, %v", name, run, err)
		}
	}
}
