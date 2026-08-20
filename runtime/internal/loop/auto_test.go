package loop

import (
	"testing"

	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
)

// walk drives the run from its current node along the fallback path, recording
// the checks each verifier would write, and returns the node ids it passed.
//
// The point of the tests below is the shape of the path, not any one node, so
// they compare the sequence rather than asserting a node at a time.
func walk(t *testing.T, runner *Runner, run *state.Run, steps int, checks map[string]string) []string {
	t.Helper()
	var path []string
	for range steps {
		node, ok := runner.Graph.Node(run.CurrentNode)
		if !ok {
			t.Fatalf("node %q is not in the graph", run.CurrentNode)
		}
		outcome := Outcome{}
		if node.Type == "verifier" {
			if checks[node.Check] == "fail" {
				outcome.Check = fail(node.Check)
			} else {
				outcome.Check = pass(node.Check)
			}
		}
		transition := advance(t, runner, run, outcome)
		path = append(path, transition.To)
		if transition.Terminal || run.Status == state.StatusAwaitingHuman {
			break
		}
	}
	return path
}

func contains(path []string, node string) bool {
	for _, id := range path {
		if id == node {
			return true
		}
	}
	return false
}

// The flag off has to leave the walk exactly as it was, or every workspace that
// shares this graph gets auto mode's extra steps without asking for them.
func TestWithoutTheAutoFlagTheWalkIsUnchanged(t *testing.T) {
	runner := newRunner(t)
	run := newRun(t, runner)
	run.CurrentNode = "test"
	run.Status = state.StatusRunning

	path := walk(t, runner, run, 4, nil)
	if len(path) == 0 || path[0] != "e2e" {
		t.Fatalf("test led to %v, want e2e first", path)
	}
	for _, node := range []string{"simplify", "lint", "commit"} {
		if contains(path, node) {
			t.Errorf("%q ran with the auto flag off: %v", node, path)
		}
	}
}

func TestWithTheAutoFlagTestLeadsThroughSimplifyLintAndCommit(t *testing.T) {
	runner := newRunner(t)
	run := newRun(t, runner)
	run.CurrentNode = "test"
	run.Status = state.StatusRunning
	run.Flags = map[string]bool{"auto": true}

	path := walk(t, runner, run, 4, nil)
	for i, want := range []string{"simplify", "lint", "commit", "e2e"} {
		if i >= len(path) || path[i] != want {
			t.Fatalf("path = %v, want simplify, lint, commit, e2e", path)
		}
	}
}

// simplify is a refactor. It must not be reachable with the suite red, whatever
// the flag says.
func TestSimplifyIsUnreachableWhileUnitTestsFail(t *testing.T) {
	runner := newRunner(t)
	run := newRun(t, runner)
	run.CurrentNode = "test"
	run.Status = state.StatusRunning
	run.Flags = map[string]bool{"auto": true}

	transition := advance(t, runner, run, Outcome{Check: fail("unit")})
	if transition.To != "build" {
		t.Errorf("a failing unit check led to %q, want build", transition.To)
	}
}

// A red linter routes back to build rather than onward. Suppressing the rule is
// the failure the node exists to catch, so there is no edge that carries on.
func TestAFailingLintRoutesBackToBuild(t *testing.T) {
	runner := newRunner(t)
	run := newRun(t, runner)
	run.CurrentNode = "lint"
	run.Status = state.StatusRunning
	run.Flags = map[string]bool{"auto": true}

	transition := advance(t, runner, run, Outcome{Check: fail("lint")})
	if transition.To != "build" {
		t.Errorf("a failing lint led to %q, want build", transition.To)
	}
}

// merge, wait, fix CI. The wait only exists on the auto path.
func TestTheMergeCiWatchIsOnTheAutoPathOnly(t *testing.T) {
	for _, testCase := range []struct {
		name string
		auto bool
		want string
	}{
		{"manual", false, "task_complete"},
		{"auto", true, "merge_ci"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			runner := newRunner(t)
			run := newRun(t, runner)
			run.CurrentNode = "approve_merge"
			run.Status = state.StatusAwaitingHuman
			if testCase.auto {
				run.Flags = map[string]bool{"auto": true}
			}
			if err := run.SetCheckAt("merge_approved", approve("merge_approved").Check, at()); err != nil {
				t.Fatal(err)
			}

			transition := advance(t, runner, run, Outcome{})
			if transition.To != testCase.want {
				t.Errorf("approve_merge led to %q, want %q", transition.To, testCase.want)
			}
		})
	}
}

func TestARedDefaultBranchRoutesBackToBuild(t *testing.T) {
	runner := newRunner(t)
	run := newRun(t, runner)
	run.CurrentNode = "merge_ci"
	run.Status = state.StatusRunning
	run.Flags = map[string]bool{"auto": true}

	transition := advance(t, runner, run, Outcome{Check: fail("main_ci")})
	if transition.To != "build" {
		t.Errorf("a red main led to %q, want build", transition.To)
	}
}

// The gate holds in any session that does not set the flag, which is the
// direction a gate has to fail in.
func TestAGateHoldsWhenNothingSetItsFlag(t *testing.T) {
	runner := newRunner(t)
	run := newRun(t, runner)
	if run.Status != state.StatusAwaitingHuman {
		t.Fatalf("intake status = %q, want awaiting_human", run.Status)
	}
	if check, recorded := run.Checks["intake_confirmed"]; recorded {
		t.Errorf("a gate nobody answered recorded %+v", check)
	}
}

// A skipped gate is recorded skipped, never passed. Run state has to keep the
// difference between a person who approved and a gate auto mode walked through.
func TestASkippedGateRecordsSkippedNotPassed(t *testing.T) {
	runner := newRunner(t)
	run, err := state.NewRun("demo", "goal", runner.Graph.Metadata.ID, runner.Graph.Spec.MaxTransitions, at())
	if err != nil {
		t.Fatal(err)
	}
	run.Flags = map[string]bool{"auto": true}
	if err := runner.Enter(run); err != nil {
		t.Fatal(err)
	}

	if run.Status != state.StatusRunning {
		t.Errorf("status = %q, want running: auto set the flag the gate declares", run.Status)
	}
	check, recorded := run.Checks["intake_confirmed"]
	if !recorded {
		t.Fatal("the skipped gate recorded nothing")
	}
	if check.Passed {
		t.Error("a skipped gate recorded as passed; the two states must stay distinguishable")
	}
	if !check.Skipped {
		t.Error("the gate did not record skipped")
	}
	if check.Ref == "" {
		t.Error("a skipped gate has to say why it was skipped")
	}
}

// The spec gate opens for auto only once auto has said the spec holds no open
// question. The flag is the answer to "is a person needed here", and it is set
// per run, not per session.
func TestTheSpecGateStillHoldsForAutoUntilTheSpecIsCalledUnambiguous(t *testing.T) {
	runner := newRunner(t)
	run := newRun(t, runner)
	run.Flags = map[string]bool{"auto": true}
	run.Status = state.StatusRunning
	run.CurrentNode = "spec"

	transition := advance(t, runner, run, Outcome{})
	if transition.To != "approve_spec" {
		t.Fatalf("spec led to %q", transition.To)
	}
	if run.Status != state.StatusAwaitingHuman {
		t.Errorf("status = %q, want awaiting_human: auto alone does not answer the spec gate", run.Status)
	}

	run.Flags["spec_unambiguous"] = true
	run.CurrentNode = "spec"
	run.Status = state.StatusRunning
	transition = advance(t, runner, run, Outcome{})
	if !transition.Skipped {
		t.Error("the transition did not report the gate as skipped")
	}
	if run.Status != state.StatusRunning {
		t.Errorf("status = %q, want running", run.Status)
	}
}

// A skipped gate has to open the edge behind it, or the run walks back to the
// artifact node and loops there forever.
func TestASkippedSpecGateLeadsToPlan(t *testing.T) {
	runner := newRunner(t)
	run := newRun(t, runner)
	run.Flags = map[string]bool{"auto": true, "spec_unambiguous": true}
	run.Status = state.StatusRunning
	run.CurrentNode = "approve_spec"
	if _, err := runner.enterGate(run, "approve_spec", at()); err != nil {
		t.Fatal(err)
	}

	transition := advance(t, runner, run, Outcome{})
	if transition.To != "plan" {
		t.Errorf("a skipped spec gate led to %q, want plan", transition.To)
	}
}

// The fix-forward edge out of merge_ci is the only route back to build that
// crosses a merge. Without resets the next approve_merge opens on a pass a
// person gave to a different diff, and pre-tool-use stops refusing pushes to a
// protected branch, its rule being that no active run has recorded
// merge_approved.
func TestFixingForwardAfterAMergeDropsTheMergeApproval(t *testing.T) {
	runner := newRunner(t)
	run := newRun(t, runner)
	run.CurrentNode = "merge_ci"
	run.Status = state.StatusRunning
	run.Flags = map[string]bool{"auto": true}
	for _, name := range []string{"merge_approved", "ship", "ci", "reviews", "unit", "lint", "pr_open"} {
		if err := run.SetCheckAt(name, approve(name).Check, at()); err != nil {
			t.Fatal(err)
		}
	}

	advance(t, runner, run, Outcome{Check: fail("main_ci")})

	for _, name := range []string{"merge_approved", "ship", "ci", "reviews", "unit", "lint", "pr_open"} {
		if check, recorded := run.Checks[name]; recorded && check.Passed {
			t.Errorf("check %q survived the fix-forward edge; the next merge would open on it", name)
		}
	}
}

// The same list as the task_complete edge, because the two cross the same
// boundary. Comparing them is what keeps one from drifting behind the other.
func TestBothEdgesLeavingAMergeResetTheSameChecks(t *testing.T) {
	runner := newRunner(t)

	collect := func(from string) map[string]bool {
		for _, edge := range runner.Graph.OutgoingEdges(from) {
			if edge.To != "build" || len(edge.Resets) == 0 {
				continue
			}
			out := map[string]bool{}
			for _, name := range edge.Resets {
				out[name] = true
			}
			return out
		}
		t.Fatalf("no resetting edge from %q to build", from)
		return nil
	}

	afterMerge, afterTask := collect("merge_ci"), collect("task_complete")
	for name := range afterTask {
		if name == "tasks_remaining" {
			continue // the task list is re-read at that node; it is not per-task evidence
		}
		if !afterMerge[name] {
			t.Errorf("task_complete resets %q and merge_ci does not", name)
		}
	}
	for name := range afterMerge {
		if !afterTask[name] {
			t.Errorf("merge_ci resets %q and task_complete does not", name)
		}
	}
}
