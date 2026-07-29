package loop

import (
	"testing"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/graph"
	"github.com/ducnd58233/vibe-agent/runtime/internal/state"
)

const repoGraph = "../../../.ai-agents/graphs/goal-delivery.yaml"

func at() time.Time { return time.Date(2026, 7, 29, 10, 0, 0, 0, time.UTC) }

func newRunner(t *testing.T) *Runner {
	t.Helper()
	loaded, err := graph.Load(repoGraph)
	if err != nil {
		t.Fatalf("load graph: %v", err)
	}
	return &Runner{Graph: loaded, Now: at}
}

func newRun(t *testing.T, runner *Runner) *state.Run {
	t.Helper()
	run, err := state.NewRun("demo", "goal", runner.Graph.Metadata.ID, runner.Graph.Spec.MaxTransitions, at())
	if err != nil {
		t.Fatalf("NewRun: %v", err)
	}
	if err := runner.Enter(run); err != nil {
		t.Fatalf("Enter: %v", err)
	}
	return run
}

func pass(name string) *NamedCheck {
	return &NamedCheck{Name: name, Check: state.Check{Passed: true, Source: state.SourceExitCode, At: at()}}
}

func fail(name string) *NamedCheck {
	return &NamedCheck{Name: name, Check: state.Check{Passed: false, Source: state.SourceExitCode, At: at()}}
}

func approve(name string) *NamedCheck {
	return &NamedCheck{Name: name, Check: state.Check{Passed: true, Source: state.SourceHumanEvent, At: at()}}
}

func advance(t *testing.T, runner *Runner, run *state.Run, outcome Outcome) *Transition {
	t.Helper()
	transition, err := runner.Advance(run, outcome)
	if err != nil {
		t.Fatalf("Advance from %q: %v", run.CurrentNode, err)
	}
	return transition
}

func TestEnterStartsAtTheGraphInitialNode(t *testing.T) {
	runner := newRunner(t)
	run := newRun(t, runner)
	if run.CurrentNode != "intake" {
		t.Errorf("CurrentNode = %q, want intake", run.CurrentNode)
	}
	if run.MaxTransitions != runner.Graph.Spec.MaxTransitions {
		t.Errorf("budget not copied from graph: %d", run.MaxTransitions)
	}
}

// The six transitions named in the spec. These are the contract between the
// graph file and the runner.
func TestSpecTransitions(t *testing.T) {
	cases := []struct {
		name    string
		from    string
		setup   func(*state.Run)
		outcome Outcome
		want    string
	}{
		{
			name:    "test fail returns to build",
			from:    "test",
			outcome: Outcome{Check: fail("unit")},
			want:    "build",
		},
		{
			name:    "test pass advances to e2e",
			from:    "test",
			outcome: Outcome{Check: pass("unit")},
			want:    "e2e",
		},
		{
			name:    "e2e skipped advances to review",
			from:    "e2e",
			outcome: Outcome{Check: &NamedCheck{Name: "e2e", Check: state.Check{Skipped: true, Source: state.SourceFileAssert, At: at()}}},
			want:    "review",
		},
		{
			name:    "ship no-go returns to build",
			from:    "ship",
			outcome: Outcome{Check: fail("ship")},
			want:    "build",
		},
		{
			name:    "ship go advances to the merge gate",
			from:    "ship",
			outcome: Outcome{Check: pass("ship")},
			want:    "approve_merge",
		},
		{
			name:    "merge approval advances toward done",
			from:    "approve_merge",
			outcome: Outcome{Check: approve("merge_approved")},
			want:    "task_complete",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			runner := newRunner(t)
			run := newRun(t, runner)
			run.CurrentNode = testCase.from
			if testCase.setup != nil {
				testCase.setup(run)
			}
			transition := advance(t, runner, run, testCase.outcome)
			if transition.To != testCase.want {
				t.Errorf("%s -> %s, want %s", testCase.from, transition.To, testCase.want)
			}
		})
	}
}

func TestIntakeBranchesOnTheResearchFlag(t *testing.T) {
	for _, testCase := range []struct {
		research bool
		want     string
	}{{true, "research"}, {false, "spec"}} {
		runner := newRunner(t)
		run := newRun(t, runner)
		run.Flags["research_required"] = testCase.research
		transition := advance(t, runner, run, Outcome{Check: approve("intake_confirmed")})
		if transition.To != testCase.want {
			t.Errorf("research_required=%v went to %q, want %q", testCase.research, transition.To, testCase.want)
		}
	}
}

// A verifier node exists to write evidence. Letting it advance without any
// would be exactly the hole the provenance rule closes.
func TestVerifierNodeMustReportACheck(t *testing.T) {
	runner := newRunner(t)
	run := newRun(t, runner)
	run.CurrentNode = "test"
	if _, err := runner.Advance(run, Outcome{}); err == nil {
		t.Error("Advance let a verifier node move on without reporting a check")
	}
}

func TestNodeCannotWriteAnotherNodesCheck(t *testing.T) {
	runner := newRunner(t)
	run := newRun(t, runner)
	run.CurrentNode = "test"
	_, err := runner.Advance(run, Outcome{Check: pass("ship")})
	if err == nil {
		t.Error("the test node was allowed to write the ship check")
	}
}

func TestAdvanceRejectsACheckWithoutRealProvenance(t *testing.T) {
	runner := newRunner(t)
	run := newRun(t, runner)
	run.CurrentNode = "test"
	forged := &NamedCheck{Name: "unit", Check: state.Check{Passed: true, Source: "model", At: at()}}
	if _, err := runner.Advance(run, Outcome{Check: forged}); err == nil {
		t.Error("Advance accepted a check whose source is model")
	}
}

func TestThreeFailuresOnTheSameBlockerStopTheRun(t *testing.T) {
	runner := newRunner(t)
	run := newRun(t, runner)
	run.CurrentNode = "build"

	for attempt := 1; attempt <= MaxBlockerAttempts; attempt++ {
		transition := advance(t, runner, run, Outcome{Blocker: "flaky integration suite"})
		if attempt < MaxBlockerAttempts {
			if transition.Terminal {
				t.Fatalf("run stopped after %d attempts, want %d", attempt, MaxBlockerAttempts)
			}
			run.CurrentNode = "build"
			continue
		}
		if !transition.Terminal || run.Status != state.StatusFailed {
			t.Errorf("attempt %d: terminal=%v status=%q, want terminal failed", attempt, transition.Terminal, run.Status)
		}
	}
	if len(run.Blockers) != 1 {
		t.Errorf("got %d blockers, want 1 with a rising count", len(run.Blockers))
	}
	if run.Blockers[0].Attempts != MaxBlockerAttempts {
		t.Errorf("attempts = %d, want %d", run.Blockers[0].Attempts, MaxBlockerAttempts)
	}
}

func TestDifferentBlockersCountSeparately(t *testing.T) {
	runner := newRunner(t)
	run := newRun(t, runner)
	run.CurrentNode = "build"

	advance(t, runner, run, Outcome{Blocker: "first problem"})
	run.CurrentNode = "build"
	advance(t, runner, run, Outcome{Blocker: "second problem"})

	if len(run.Blockers) != 2 {
		t.Fatalf("got %d blockers, want 2", len(run.Blockers))
	}
	if run.Status != state.StatusRunning {
		t.Errorf("status = %q; two different blockers should not trip the stop rule", run.Status)
	}
}

func TestBudgetExceededStopsTheRun(t *testing.T) {
	runner := newRunner(t)
	run := newRun(t, runner)
	run.CurrentNode = "build"
	run.MaxTransitions = 2

	advance(t, runner, run, Outcome{})
	run.CurrentNode = "build"
	advance(t, runner, run, Outcome{})
	run.CurrentNode = "build"
	transition := advance(t, runner, run, Outcome{})

	if !transition.Terminal || run.Status != state.StatusBudgetExceeded {
		t.Errorf("terminal=%v status=%q, want terminal budget_exceeded", transition.Terminal, run.Status)
	}
}

func TestReachingAHumanGateParksTheRun(t *testing.T) {
	runner := newRunner(t)
	run := newRun(t, runner)
	run.CurrentNode = "spec"

	transition := advance(t, runner, run, Outcome{})
	if transition.To != "approve_spec" {
		t.Fatalf("spec -> %q, want approve_spec", transition.To)
	}
	if run.Status != state.StatusAwaitingHuman {
		t.Errorf("status = %q, want awaiting_human", run.Status)
	}
}

func TestATerminalRunRefusesToAdvance(t *testing.T) {
	runner := newRunner(t)
	run := newRun(t, runner)
	run.Status = state.StatusDone
	if _, err := runner.Advance(run, Outcome{}); err == nil {
		t.Error("a finished run was allowed to advance")
	}
}

// A run is resumable when its manifest alone determines the next node. Nothing
// may live in the runner between calls.
func TestAFreshRunnerResumesToTheSameNode(t *testing.T) {
	first := newRunner(t)
	run := newRun(t, first)
	run.CurrentNode = "test"
	advance(t, first, run, Outcome{Check: pass("unit")})
	afterFirst := run.CurrentNode

	second := newRunner(t)
	rerun := newRun(t, second)
	rerun.CurrentNode = "test"
	advance(t, second, rerun, Outcome{Check: pass("unit")})

	if rerun.CurrentNode != afterFirst {
		t.Errorf("resumed to %q, first run reached %q", rerun.CurrentNode, afterFirst)
	}
}

// The whole loop, driven only by evidence, from intake to done.
func TestFullDeliveryPathReachesDone(t *testing.T) {
	runner := newRunner(t)
	run := newRun(t, runner)
	run.Flags["research_required"] = false
	run.Flags["e2e_required"] = true

	steps := []struct {
		expect  string
		outcome Outcome
	}{
		{"spec", Outcome{Check: approve("intake_confirmed")}},
		{"approve_spec", Outcome{}},
		{"plan", Outcome{Check: approve("spec_approved")}},
		{"approve_plan", Outcome{}},
		{"build", Outcome{Check: approve("plan_approved")}},
		{"test", Outcome{}},
		{"e2e", Outcome{Check: pass("unit")}},
		{"review", Outcome{Check: pass("e2e")}},
		{"open_pr", Outcome{}},
		{"pr_checks", Outcome{Check: pass("pr_open")}},
		{"external_reviews", Outcome{Check: pass("ci")}},
		{"ship", Outcome{Check: pass("reviews")}},
		{"approve_merge", Outcome{Check: pass("ship")}},
		{"task_complete", Outcome{Check: approve("merge_approved")}},
		{"done", Outcome{Check: &NamedCheck{Name: "tasks_remaining", Check: state.Check{Passed: false, Source: state.SourceFileAssert, At: at()}}}},
	}

	for i, step := range steps {
		transition := advance(t, runner, run, step.outcome)
		if transition.To != step.expect {
			t.Fatalf("step %d: went to %q, want %q", i, transition.To, step.expect)
		}
	}
	if run.Status != state.StatusDone {
		t.Errorf("status = %q, want done", run.Status)
	}
}

// A remaining task sends the run back to build rather than to done.
func TestRemainingTasksLoopBackToBuild(t *testing.T) {
	runner := newRunner(t)
	run := newRun(t, runner)
	run.CurrentNode = "task_complete"

	transition := advance(t, runner, run, Outcome{
		Check: &NamedCheck{Name: "tasks_remaining", Check: state.Check{Passed: true, Source: state.SourceFileAssert, At: at()}},
	})
	if transition.To != "build" {
		t.Errorf("task_complete -> %q, want build", transition.To)
	}
}
