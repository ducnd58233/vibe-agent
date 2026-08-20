package loop

import (
	"testing"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/graph"
	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
)

// budgetRun puts a run at an agent node with a fallback edge out of it, so a
// plain advance is enough to exercise the budget checks.
func budgetRun(t *testing.T, loaded *graph.Graph, at time.Time) *state.Run {
	t.Helper()
	run, err := state.NewRun("budget", "budget test", loaded.Metadata.ID, loaded.Spec.MaxTransitions, at)
	if err != nil {
		t.Fatal(err)
	}
	run.CurrentNode = "build"
	run.Status = state.StatusRunning
	return run
}

func budgetGraph(t *testing.T) *graph.Graph {
	t.Helper()
	loaded, err := graph.Load(repoGraph)
	if err != nil {
		t.Fatal(err)
	}
	return loaded
}

// Zero is no budget. Adding a limit must not retroactively stop runs nobody set
// one for.
func TestAZeroBudgetStopsNothing(t *testing.T) {
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	loaded := budgetGraph(t)
	run := budgetRun(t, loaded, at)
	run.TokensUsed = 10_000_000

	runner := New(loaded)
	runner.Now = func() time.Time { return at.Add(365 * 24 * time.Hour) }
	transition, err := runner.Advance(run, Outcome{})
	if err != nil {
		t.Fatal(err)
	}
	if transition.Terminal {
		t.Errorf("a run with no budgets stopped anyway, status %s", run.Status)
	}
}

func TestATokenBudgetStopsTheRunAndSaysSo(t *testing.T) {
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	loaded := budgetGraph(t)
	run := budgetRun(t, loaded, at)
	run.TokenBudget = 1000
	run.TokensUsed = 1001

	runner := New(loaded)
	runner.Now = func() time.Time { return at }
	transition, err := runner.Advance(run, Outcome{})
	if err != nil {
		t.Fatal(err)
	}
	if !transition.Terminal || run.Status != state.StatusBudgetExceeded {
		t.Fatalf("status = %s, terminal = %t; want a budget stop", run.Status, transition.Terminal)
	}
	if run.StoppedBy != "tokens" {
		t.Errorf("stoppedBy = %q, want tokens; three budgets share one status", run.StoppedBy)
	}
}

func TestAWallclockBudgetStopsTheRunAndSaysSo(t *testing.T) {
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	loaded := budgetGraph(t)
	run := budgetRun(t, loaded, at)
	run.WallclockSeconds = 3600

	runner := New(loaded)
	runner.Now = func() time.Time { return at.Add(2 * time.Hour) }
	if _, err := runner.Advance(run, Outcome{}); err != nil {
		t.Fatal(err)
	}
	if run.Status != state.StatusBudgetExceeded || run.StoppedBy != "wallclock" {
		t.Errorf("status = %s, stoppedBy = %q; want a wallclock stop", run.Status, run.StoppedBy)
	}
}

// The transition budget kept working when two more were added beside it.
func TestTheTransitionBudgetStillStopsAndIsNamed(t *testing.T) {
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	loaded := budgetGraph(t)
	run := budgetRun(t, loaded, at)
	// The runner raises a run's limit to the graph's, so the only way past the
	// transition budget is to be at the graph's own ceiling.
	run.Iteration = loaded.Spec.MaxTransitions

	runner := New(loaded)
	runner.Now = func() time.Time { return at }
	if _, err := runner.Advance(run, Outcome{}); err != nil {
		t.Fatal(err)
	}
	if run.Status != state.StatusBudgetExceeded || run.StoppedBy != "transitions" {
		t.Errorf("status = %s, stoppedBy = %q; want a transition stop", run.Status, run.StoppedBy)
	}
}

// A class makes failures countable. Without one, "repeated failures produce
// generic retries" is the only thing the record can support.
func TestABlockerKeepsItsClass(t *testing.T) {
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	loaded := budgetGraph(t)
	run := budgetRun(t, loaded, at)

	runner := New(loaded)
	runner.Now = func() time.Time { return at }
	if _, err := runner.Advance(run, Outcome{Blocker: "npm registry down", BlockerClass: state.FailureTool}); err != nil {
		t.Fatal(err)
	}
	if len(run.Blockers) != 1 || run.Blockers[0].Class != state.FailureTool {
		t.Fatalf("blockers = %+v, want one classified tool failure", run.Blockers)
	}
}

// A repeat may arrive better classified than the first sighting, and the later
// reading is the one worth keeping.
func TestARepeatUpgradesAnUnclassifiedBlocker(t *testing.T) {
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	loaded := budgetGraph(t)
	run := budgetRun(t, loaded, at)

	runner := New(loaded)
	runner.Now = func() time.Time { return at }
	if _, err := runner.Advance(run, Outcome{Blocker: "flaky spec"}); err != nil {
		t.Fatal(err)
	}
	if _, err := runner.Advance(run, Outcome{Blocker: "flaky spec", BlockerClass: state.FailureTest}); err != nil {
		t.Fatal(err)
	}
	if run.Blockers[0].Class != state.FailureTest {
		t.Errorf("class = %q, want test", run.Blockers[0].Class)
	}
	if run.Blockers[0].Attempts != 2 {
		t.Errorf("attempts = %d, want 2", run.Blockers[0].Attempts)
	}
}
