package loop

import (
	"strings"
	"testing"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/graph"
	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
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

func TestEnterParksAnInitialHumanGate(t *testing.T) {
	runner := newRunner(t)
	run, err := state.NewRun("demo", "goal", runner.Graph.Metadata.ID, runner.Graph.Spec.MaxTransitions, at())
	if err != nil {
		t.Fatalf("NewRun: %v", err)
	}
	if err := runner.Enter(run); err != nil {
		t.Fatalf("Enter: %v", err)
	}
	if run.CurrentNode != "intake" {
		t.Errorf("CurrentNode = %q, want intake", run.CurrentNode)
	}
	if run.Status != state.StatusAwaitingHuman {
		t.Errorf("Status = %q, want %s", run.Status, state.StatusAwaitingHuman)
	}
}

func TestEnterLeavesANonHumanGateRunning(t *testing.T) {
	runner, run := skippableRunner(t)
	if run.CurrentNode != "check" {
		t.Errorf("CurrentNode = %q, want check", run.CurrentNode)
	}
	if run.Status != state.StatusRunning {
		t.Errorf("Status = %q, want %s", run.Status, state.StatusRunning)
	}
	node, ok := runner.Graph.Node(run.CurrentNode)
	if !ok || node.Type == graph.NodeHumanGate {
		t.Fatalf("fixture initial node must not be a human_gate, got %+v", node)
	}
}

// The transitions named in the spec. These are the contract between the
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
			name:    "e2e skipped advances to slop",
			from:    "e2e",
			outcome: Outcome{Check: &NamedCheck{Name: "e2e", Check: state.Check{Skipped: true, Source: state.SourceFileAssert, At: at()}}},
			want:    "slop",
		},
		{
			name:    "slop skipped advances to review",
			from:    "slop",
			outcome: Outcome{Check: &NamedCheck{Name: "slop", Check: state.Check{Skipped: true, Source: state.SourceFileAssert, At: at()}}},
			want:    "review",
		},
		{
			name:    "e2e pass advances to slop",
			from:    "e2e",
			outcome: Outcome{Check: pass("e2e")},
			want:    "slop",
		},
		{
			name:    "slop pass advances to review",
			from:    "slop",
			outcome: Outcome{Check: pass("slop")},
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

func TestABlockerParksAtTheNodeInsteadOfSkippingIt(t *testing.T) {
	runner := newRunner(t)
	run := newRun(t, runner)
	run.CurrentNode = "build"

	transition := advance(t, runner, run, Outcome{Blocker: "intake_confirmed requires source human_event"})

	if transition.To != "build" {
		t.Errorf("To = %q, want build; skipping would treat a blocked step as done", transition.To)
	}
	if run.CurrentNode != "build" {
		t.Errorf("CurrentNode = %q, want build", run.CurrentNode)
	}
	if run.Status != state.StatusAwaitingHuman {
		t.Errorf("Status = %q, want %s so stop does not spin", run.Status, state.StatusAwaitingHuman)
	}
	if transition.Terminal {
		t.Error("a first blocker must not fail the run")
	}
	if transition.Via != "blocker" {
		t.Errorf("Via = %q, want blocker", transition.Via)
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
			if run.CurrentNode != "build" {
				t.Fatalf("attempt %d left the node (%s); a blocked step stays put", attempt, run.CurrentNode)
			}
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
	advance(t, runner, run, Outcome{Blocker: "second problem"})

	if len(run.Blockers) != 2 {
		t.Fatalf("got %d blockers, want 2", len(run.Blockers))
	}
	if run.Status != state.StatusAwaitingHuman {
		t.Errorf("status = %q; two different blockers should park, not fail or skip", run.Status)
	}
	if run.CurrentNode != "build" {
		t.Errorf("CurrentNode = %q, want build", run.CurrentNode)
	}
}

func TestBudgetExceededStopsTheRun(t *testing.T) {
	runner := newRunner(t)
	run := newRun(t, runner)
	run.CurrentNode = "build"
	runner.Graph.Spec.MaxTransitions = 2
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

func TestAdvanceRefreshesBudgetFromGraph(t *testing.T) {
	runner := newRunner(t)
	run := newRun(t, runner)
	run.CurrentNode = "build"
	run.Status = state.StatusRunning
	run.Iteration = 50
	run.MaxTransitions = 50
	runner.Graph.Spec.MaxTransitions = 100

	transition := advance(t, runner, run, Outcome{})
	if run.Status == state.StatusBudgetExceeded {
		t.Fatal("stale run budget blocked a graph raise")
	}
	if transition.To != "test" {
		t.Fatalf("build -> %q, want test", transition.To)
	}
	if run.MaxTransitions != 100 {
		t.Errorf("MaxTransitions = %d, want 100 from the graph", run.MaxTransitions)
	}
	if run.Iteration != 51 {
		t.Errorf("Iteration = %d, want 51", run.Iteration)
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
		{"slop", Outcome{Check: pass("e2e")}},
		{"review", Outcome{Check: pass("slop")}},
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

// skipGraph is a fixture rather than the shipped graph. The delivery graph has
// no skipWhen after this change, on purpose: relying on a flag whose absence
// reads as false would skip e2e by default, which is the failure the whole
// design is about. skipWhen stays a runner feature, so it needs a graph that
// uses it.
const skipGraph = `apiVersion: vibe-agent/v1
kind: WorkflowGraph
metadata:
  id: skippable
  description: One verifier node the graph can declare out of scope.
spec:
  initial: check
  maxTransitions: 10
  guards:
    - name: not_applicable
      description: The workspace declared this check out of scope.
      source: flag
    - name: check_ok
      description: The check passed or was declared out of scope.
      source: check
      reads: unit
      acceptsSkipped: true
  nodes:
    check:
      type: verifier
      description: The only real step.
      verifier: command
      check: unit
      skipWhen: not_applicable
    done:
      type: terminal
      description: Finished.
      status: done
    failed:
      type: terminal
      description: Stopped.
      status: failed
  edges:
    - from: check
      to: done
      when: check_ok
    - from: check
      to: failed
      when: "!check_ok"
`

func skippableRunner(t *testing.T) (*Runner, *state.Run) {
	t.Helper()
	loaded, err := graph.Parse([]byte(skipGraph))
	if err != nil {
		t.Fatalf("parse fixture graph: %v", err)
	}
	runner := &Runner{Graph: loaded, Now: at}
	run, err := state.NewRun("demo", "goal", loaded.Metadata.ID, loaded.Spec.MaxTransitions, at())
	if err != nil {
		t.Fatalf("NewRun: %v", err)
	}
	if err := runner.Enter(run); err != nil {
		t.Fatalf("Enter: %v", err)
	}
	return runner, run
}

// skipWhen was parsed and validated but never read at execution time, so a graph
// could declare a skip condition and the runtime would ignore it. Whichever way
// it resolved, one of the two states the graph described did not exist.
func TestSkipWhenIsHonoredAtExecutionTime(t *testing.T) {
	runner, run := skippableRunner(t)

	reason, skip := runner.SkipReason(run, run.CurrentNode)
	if skip {
		t.Fatalf("a node whose flag is unset was skipped: %q", reason)
	}

	run.Flags = map[string]bool{"not_applicable": true}
	reason, skip = runner.SkipReason(run, run.CurrentNode)
	if !skip {
		t.Fatal("skipWhen was declared and the flag was set, but the node was not skipped")
	}
	if !strings.Contains(reason, "not_applicable") {
		t.Errorf("the reason does not name the guard that caused it: %q", reason)
	}
}

// A node with no skipWhen must never be skippable, whatever the flags say.
// Otherwise a flag name colliding with something else would silently drop a step.
func TestANodeWithoutSkipWhenIsNeverSkipped(t *testing.T) {
	runner := newRunner(t)
	run := newRun(t, runner)
	run.Flags = map[string]bool{"not_applicable": true, "e2e_required": false}

	for id := range runner.Graph.Spec.Nodes {
		if reason, skip := runner.SkipReason(run, id); skip {
			node, _ := runner.Graph.Node(id)
			if node.SkipWhen == "" {
				t.Errorf("node %q has no skipWhen but was skipped: %q", id, reason)
			}
		}
	}
}

// A skipped check is recorded as skipped. The delivery graph's e2e_ok guard
// accepts either, which is a routing decision; the manifest must still keep the
// two apart so a reader can tell what actually ran.
func TestASkippedCheckAdvancesWithoutBeingAPass(t *testing.T) {
	runner, run := skippableRunner(t)
	run.Flags = map[string]bool{"not_applicable": true}

	transition, err := runner.Advance(run, Outcome{Check: &NamedCheck{
		Name:  "unit",
		Check: state.Check{Skipped: true, Source: state.SourceFileAssert, At: at()},
	}})
	if err != nil {
		t.Fatalf("Advance: %v", err)
	}
	if transition.To != "done" {
		t.Fatalf("a skipped check did not satisfy the guard: went to %q", transition.To)
	}
	if run.Checks["unit"].Passed {
		t.Error("a skipped check was stored as passed")
	}
	if !run.Checks["unit"].Skipped {
		t.Error("a skipped check lost its skipped marker")
	}
}

// The bug this closes. The runner used to return `check.Passed || check.Skipped`
// for every check-sourced guard, so any check that got skipped satisfied its own
// gate. Skipped now satisfies a guard only where the graph opted in.
//
// The delivery graph opts in two gates: e2e_ok and slop_ok. Both sit on
// verifier nodes every workspace shares, while each workspace keeps its own
// vibe-checks.yaml. Without the opt-in, adding either node would stall every
// consumer that has not declared that check.
func TestASkippedCheckSatisfiesOnlyAGuardThatOptedIn(t *testing.T) {
	runner := newRunner(t)

	// An allowlist, with a reason beside each entry, because acceptsSkipped is
	// the one setting that lets a gate open on something that never ran.
	allowed := map[string]string{
		"e2e_ok":         "a workspace that declares no e2e check must not stall",
		"slop_ok":        "the same, for a workspace with no slop threshold",
		"lint_ok":        "the same, for a workspace that declares no lint command",
		"main_ci_passed": "the same, for a workspace with no default-branch CI to ask about",
		"spec_approved":  "auto mode skips the gate when the spec holds no open question; the check records skipped, never passed",
		"plan_approved":  "the same, for the plan gate",
	}
	for name := range allowed {
		optedIn, ok := runner.Graph.Guard(name)
		if !ok {
			t.Fatalf("the delivery graph has no %s guard", name)
		}
		if !optedIn.AcceptsSkipped {
			t.Errorf("%s does not accept a skip, so a workspace with no matching check would stall", name)
		}
	}

	for _, guard := range runner.Graph.Spec.Guards {
		if guard.AcceptsSkipped && allowed[guard.Name] == "" {
			t.Errorf("guard %q accepts a skip and is not on the allowlist; add it with the reason, or take the setting off", guard.Name)
		}
	}
}

// A skipped unit check must not open the unit gate. Before the opt-in, a
// workspace whose plan omitted `unit` walked straight past it.
func TestASkippedCheckDoesNotOpenAGateThatDidNotOptIn(t *testing.T) {
	runner := newRunner(t)
	run := newRun(t, runner)

	// intake -> spec -> approve_spec -> plan -> approve_plan -> build -> test
	advance(t, runner, run, Outcome{Check: approve("intake_confirmed")})
	advance(t, runner, run, Outcome{})
	advance(t, runner, run, Outcome{Check: approve("spec_approved")})
	advance(t, runner, run, Outcome{})
	advance(t, runner, run, Outcome{Check: approve("plan_approved")})
	advance(t, runner, run, Outcome{})
	if run.CurrentNode != "test" {
		t.Fatalf("walked to %q, want test", run.CurrentNode)
	}

	transition := advance(t, runner, run, Outcome{Check: &NamedCheck{
		Name:  "unit",
		Check: state.Check{Skipped: true, Source: state.SourceFileAssert, At: at()},
	}})
	if transition.To != "build" {
		t.Errorf("a skipped unit check routed to %q; skipping the suite must not pass its gate", transition.To)
	}
}
