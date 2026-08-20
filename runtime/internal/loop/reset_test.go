package loop

import (
	"testing"

	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
)

// walkOneTask drives a run from build to task_complete the way a delivery
// cycle does, recording each check as its node writes it.
func walkOneTask(t *testing.T, runner *Runner, run *state.Run, tasksRemain bool) {
	t.Helper()
	steps := []Outcome{
		{},                    // build
		{Check: pass("unit")}, // test
		{Check: pass("e2e")},  // e2e
		{Check: pass("slop")}, // slop
		{Result: map[string]bool{"changes_requested": false}}, // review
		{Check: pass("pr_open")},                              // open_pr
		{Check: pass("ci")},                                   // pr_checks
		{Check: pass("reviews")},                              // external_reviews
		{Check: pass("ship")},                                 // ship
		{Check: approve("merge_approved")},                    // approve_merge
	}
	for i, outcome := range steps {
		if _, err := runner.Advance(run, outcome); err != nil {
			t.Fatalf("step %d from %s: %v", i, run.CurrentNode, err)
		}
	}
	if run.CurrentNode != "task_complete" {
		t.Fatalf("after one task the run is at %s, want task_complete", run.CurrentNode)
	}
	check := pass("tasks_remaining")
	if !tasksRemain {
		check = &NamedCheck{Name: "tasks_remaining", Check: state.Check{
			Passed: false, Source: state.SourceExitCode, At: at(),
		}}
	}
	if _, err := runner.Advance(run, Outcome{Check: check}); err != nil {
		t.Fatal(err)
	}
}

// The gate in front of the irreversible action must be earned per task. An
// approval recorded for task one is not an approval of task two, and nothing
// used to clear it: transitions clear blockers, not checks.
//
// The same stale check is what stops pre-tool-use refusing a push to a
// protected branch, because its rule is "no active run has recorded
// merge_approved".
func TestASecondTaskCannotMergeOnTheFirstTasksApproval(t *testing.T) {
	runner := newRunner(t)
	run := newRun(t, runner)

	// Reach build the way a run does.
	for _, outcome := range []Outcome{
		{Check: approve("intake_confirmed")},
		{},
		{Check: approve("spec_approved")},
		{},
		{Check: approve("plan_approved")},
	} {
		if _, err := runner.Advance(run, outcome); err != nil {
			t.Fatal(err)
		}
	}
	if run.CurrentNode != "build" {
		t.Fatalf("run is at %s, want build", run.CurrentNode)
	}

	walkOneTask(t, runner, run, true)
	if run.CurrentNode != "build" {
		t.Fatalf("a remaining task should return the run to build, got %s", run.CurrentNode)
	}

	// Every per-task check must be gone. Leaving one behind means the next
	// task starts with something already proven that it has not proven.
	for _, name := range []string{
		"unit", "e2e", "slop", "pr_open", "ci", "reviews", "ship", "merge_approved",
		"tasks_remaining",
	} {
		if _, held := run.Checks[name]; held {
			t.Errorf("check %q survived into the next task", name)
		}
	}

	// What a run earns once must survive.
	for _, name := range []string{"intake_confirmed", "spec_approved", "plan_approved"} {
		if _, held := run.Checks[name]; !held {
			t.Errorf("check %q was cleared, and it belongs to the run rather than the task", name)
		}
	}
}

// Reaching the merge gate a second time with no fresh approval must not carry
// the run into the merge.
func TestTheMergeGateStopsAgainOnTheSecondTask(t *testing.T) {
	runner := newRunner(t)
	run := newRun(t, runner)
	for _, outcome := range []Outcome{
		{Check: approve("intake_confirmed")},
		{},
		{Check: approve("spec_approved")},
		{},
		{Check: approve("plan_approved")},
	} {
		if _, err := runner.Advance(run, outcome); err != nil {
			t.Fatal(err)
		}
	}
	walkOneTask(t, runner, run, true)

	// Second cycle, up to the gate.
	for _, outcome := range []Outcome{
		{},
		{Check: pass("unit")},
		{Check: pass("e2e")},
		{Check: pass("slop")},
		{Result: map[string]bool{"changes_requested": false}},
		{Check: pass("pr_open")},
		{Check: pass("ci")},
		{Check: pass("reviews")},
		{Check: pass("ship")},
	} {
		if _, err := runner.Advance(run, outcome); err != nil {
			t.Fatal(err)
		}
	}
	if run.CurrentNode != "approve_merge" {
		t.Fatalf("second cycle is at %s, want approve_merge", run.CurrentNode)
	}
	if run.Status != state.StatusAwaitingHuman {
		t.Errorf("status = %s, want the gate to be waiting on a person", run.Status)
	}

	// No fresh approval. The run must not proceed into the merge.
	if _, err := runner.Advance(run, Outcome{}); err != nil {
		t.Fatal(err)
	}
	if run.CurrentNode == "task_complete" {
		t.Fatal("the second task merged on the first task's approval")
	}
}

// A run whose tasks are finished still ends, and clearing per-task checks must
// not break the path to done.
func TestTheLastTaskStillReachesDone(t *testing.T) {
	runner := newRunner(t)
	run := newRun(t, runner)
	for _, outcome := range []Outcome{
		{Check: approve("intake_confirmed")},
		{},
		{Check: approve("spec_approved")},
		{},
		{Check: approve("plan_approved")},
	} {
		if _, err := runner.Advance(run, outcome); err != nil {
			t.Fatal(err)
		}
	}
	walkOneTask(t, runner, run, false)

	if run.CurrentNode != "done" || run.Status != state.StatusDone {
		t.Fatalf("run is at %s with status %s, want done", run.CurrentNode, run.Status)
	}
}
