package loop

import (
	"testing"

	"github.com/ducnd58233/vibe-agent/runtime/internal/graph"
	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
)

func TestNeighborsMarksFirstMatchingEdgeActive(t *testing.T) {
	t.Parallel()
	runner := newRunner(t)
	run := newRun(t, runner)
	if _, err := runner.Advance(run, Outcome{Check: pass("intake_confirmed")}); err != nil {
		t.Fatalf("advance past intake: %v", err)
	}
	run.CurrentNode = "test"
	run.Checks["unit"] = state.Check{Passed: false, Source: state.SourceExitCode, At: at()}

	neighbors, err := runner.Neighbors(run)
	if err != nil {
		t.Fatalf("neighbors: %v", err)
	}
	active := 0
	for _, nb := range neighbors {
		if nb.ActivePath {
			active++
			if nb.To != "build" {
				t.Errorf("active path to %q, want build when unit fails", nb.To)
			}
		}
	}
	if active != 1 {
		t.Errorf("active paths = %d, want exactly one", active)
	}
}

func TestNeighborsResearchMonitorPaths(t *testing.T) {
	t.Parallel()
	loaded, err := graph.LoadByID(graph.DefaultDir("../../../"), "researcher-delivery")
	if err != nil {
		t.Fatalf("load graph: %v", err)
	}
	runner := &Runner{Graph: loaded, Now: at}
	run, err := state.NewRun("exp", "research", loaded.Metadata.ID, loaded.Spec.MaxTransitions, at())
	if err != nil {
		t.Fatalf("new run: %v", err)
	}
	run.CurrentNode = "experiment_monitor"

	neighbors, err := runner.Neighbors(run)
	if err != nil {
		t.Fatalf("neighbors: %v", err)
	}
	var hasRun, hasEval bool
	for _, nb := range neighbors {
		switch nb.To {
		case "experiment_run":
			hasRun = true
		case "results_eval":
			hasEval = true
		}
	}
	if !hasRun || !hasEval {
		t.Fatalf("neighbors = %+v, want experiment_run and results_eval", neighbors)
	}
}
