package checkpoint

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/checkplan"
	"github.com/ducnd58233/vibe-agent/runtime/internal/graph"
	"github.com/ducnd58233/vibe-agent/runtime/internal/loop"
	"github.com/ducnd58233/vibe-agent/runtime/internal/state"
)

// graphDir points at the toolkit's own graphs, so these tests exercise the
// delivery graph that ships rather than a fixture that can drift from it.
var graphDir = graph.DefaultDir("../../..")

func at() time.Time { return time.Date(2026, 8, 8, 10, 0, 0, 0, time.UTC) }

func workspace(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	run, err := state.NewRun("demo", "prove checkpoints are idempotent", "goal-delivery", 50, at())
	if err != nil {
		t.Fatalf("NewRun: %v", err)
	}
	loaded, err := graph.LoadByID(graphDir, "goal-delivery")
	if err != nil {
		t.Fatalf("load graph: %v", err)
	}
	if err := loop.New(loaded).Enter(run); err != nil {
		t.Fatalf("enter: %v", err)
	}
	if err := state.Save(state.ManifestPath(root, "demo"), run); err != nil {
		t.Fatalf("save: %v", err)
	}
	return root
}

// intake is a human gate writing intake_confirmed, so this is the evidence that
// moves a fresh run off its first node.
func intakeConfirmed() loop.Outcome {
	return loop.Outcome{Check: &loop.NamedCheck{
		Name: "intake_confirmed",
		Check: state.Check{
			Passed: true, Source: state.SourceHumanEvent,
			Ref: "agreed in the kickoff", At: at(),
		},
	}}
}

func apply(t *testing.T, root string, outcome loop.Outcome) *Result {
	t.Helper()
	result, err := Apply(Request{
		WorkspaceRoot: root, GraphDir: graphDir, Slug: "demo",
		Outcome: outcome, Now: at(),
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	return result
}

// declarePlan writes a check plan into the workspace. The plan is what decides
// who may write a verifier node's check, so most of these tests need one.
func declarePlan(t *testing.T, root, body string) {
	t.Helper()
	if err := os.WriteFile(checkplan.DefaultPath(root), []byte(body), 0o600); err != nil {
		t.Fatalf("write plan: %v", err)
	}
}

// atTestNode walks a fresh run to the `test` node, the first verifier in the
// delivery graph. Human gates and agent nodes are unaffected by the origin rule,
// so reaching a verifier is the only way to exercise it.
func atTestNode(t *testing.T, root string) {
	t.Helper()
	apply(t, root, intakeConfirmed())           // intake -> spec
	apply(t, root, loop.Outcome{})              // spec -> approve_spec
	apply(t, root, humanCheck("spec_approved")) // -> plan
	apply(t, root, loop.Outcome{})              // plan -> approve_plan
	apply(t, root, humanCheck("plan_approved")) // -> build
	apply(t, root, loop.Outcome{})              // build -> test

	current, err := state.Load(state.ManifestPath(root, "demo"))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if current.CurrentNode != "test" {
		t.Fatalf("walked to %q, want test", current.CurrentNode)
	}
}

func humanCheck(name string) loop.Outcome {
	return loop.Outcome{Check: &loop.NamedCheck{
		Name:  name,
		Check: state.Check{Passed: true, Source: state.SourceHumanEvent, At: at()},
	}}
}

func unitPassed() loop.Outcome {
	return loop.Outcome{Check: &loop.NamedCheck{
		Name:  "unit",
		Check: state.Check{Passed: true, Source: state.SourceExitCode, At: at()},
	}}
}

// The hole this closes. `--source exit_code` proves the string is one of four
// allowed values; it does not prove a process ran. Anyone who could type it
// could walk a verifier node without verifying, which is how a mobile app on a
// white screen reached a passing e2e check.
func TestCallerSuppliedEvidenceIsRefusedAtAVerifierNode(t *testing.T) {
	root := workspace(t)
	declarePlan(t, root, `apiVersion: vibe-agent/v1
kind: CheckPlan
spec:
  checks:
    unit:
      command: go
      args: [test, ./...]
`)
	atTestNode(t, root)

	_, err := Apply(Request{
		WorkspaceRoot: root, GraphDir: graphDir, Slug: "demo",
		Outcome: unitPassed(), Now: at(),
	})
	if err == nil {
		t.Fatal("a typed exit_code advanced a verifier node; nothing ran")
	}
	if !strings.Contains(err.Error(), "verify") {
		t.Errorf("the refusal does not point at the command that would work: %v", err)
	}
}

// The same evidence, produced by a verifier in this process, must advance. A
// rule that refused both would just wedge the loop.
func TestRuntimeProducedEvidenceAdvancesAVerifierNode(t *testing.T) {
	root := workspace(t)
	declarePlan(t, root, `apiVersion: vibe-agent/v1
kind: CheckPlan
spec:
  checks:
    unit:
      command: go
      args: [test, ./...]
`)
	atTestNode(t, root)

	result, err := Apply(Request{
		WorkspaceRoot: root, GraphDir: graphDir, Slug: "demo",
		Outcome: unitPassed(), origin: originRuntime, Now: at(),
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if result.Transition == nil || result.Transition.To != "e2e" {
		t.Fatalf("runtime evidence did not advance to e2e: %+v", result.Transition)
	}
}

// Some checks genuinely have no runtime verifier. The escape hatch stays in git:
// the workspace declares `verifier: human` for that check, and only then may a
// person record it. Declaring it is a reviewable diff; typing a flag is not.
func TestAHumanDeclaredCheckStillTakesAHumanEvent(t *testing.T) {
	root := workspace(t)
	declarePlan(t, root, `apiVersion: vibe-agent/v1
kind: CheckPlan
spec:
  checks:
    unit:
      verifier: human
      description: a person runs the suite and reports it
`)
	atTestNode(t, root)

	human := unitPassed()
	human.Check.Check.Source = state.SourceHumanEvent
	result, err := Apply(Request{
		WorkspaceRoot: root, GraphDir: graphDir, Slug: "demo",
		Outcome: human, Now: at(),
	})
	if err != nil {
		t.Fatalf("a declared human check was refused: %v", err)
	}
	if result.Transition == nil || result.Transition.To != "e2e" {
		t.Fatalf("did not advance: %+v", result.Transition)
	}
}

// A human-declared check is a person's word, so it must not be accepted while
// claiming to be a process. Otherwise the declaration would launder provenance.
func TestAHumanDeclaredCheckRefusesAProcessSource(t *testing.T) {
	root := workspace(t)
	declarePlan(t, root, `apiVersion: vibe-agent/v1
kind: CheckPlan
spec:
  checks:
    unit:
      verifier: human
`)
	atTestNode(t, root)

	if _, err := Apply(Request{
		WorkspaceRoot: root, GraphDir: graphDir, Slug: "demo",
		Outcome: unitPassed(), Now: at(),
	}); err == nil {
		t.Fatal("a human-declared check accepted exit_code from a caller")
	}
}

// A caller may not write a check the plan does not declare, and the refusal has
// to name the file, because the fix is to edit it.
func TestACallerCannotWriteAnUndeclaredCheck(t *testing.T) {
	root := workspace(t)
	declarePlan(t, root, `apiVersion: vibe-agent/v1
kind: CheckPlan
spec:
  checks:
    e2e:
      command: npm
      args: [run, e2e]
`)
	atTestNode(t, root)

	_, err := Apply(Request{
		WorkspaceRoot: root, GraphDir: graphDir, Slug: "demo",
		Outcome: unitPassed(), Now: at(),
	})
	if err == nil {
		t.Fatal("an undeclared check advanced")
	}
	if !strings.Contains(err.Error(), checkplan.FileName) {
		t.Errorf("the refusal does not name the file to edit: %v", err)
	}
}

// A plan that omits a check is the workspace saying it has no such check, which
// is a statement in a tracked file. The run may pass the node on it, but the
// manifest must record a skip and not a pass, and the reason must name what
// caused it.
func TestAnUndeclaredCheckIsSkippedNotPassed(t *testing.T) {
	root := workspace(t)
	declarePlan(t, root, `apiVersion: vibe-agent/v1
kind: CheckPlan
spec:
  checks:
    e2e:
      command: npm
      args: [run, e2e]
`)
	atTestNode(t, root)

	result, err := Verify(context.Background(), VerifyRequest{
		WorkspaceRoot: root, GraphDir: graphDir, Slug: "demo", Now: at(),
	})
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if result.Verifier.Check.Passed {
		t.Fatal("an undeclared check was recorded as passed")
	}
	if !result.Verifier.Check.Skipped {
		t.Error("an undeclared check was not marked skipped")
	}
	if !strings.Contains(result.Verifier.Summary, checkplan.FileName) {
		t.Errorf("the skip reason does not say what caused it: %q", result.Verifier.Summary)
	}
}

// skipWhen is now honored at execution time, which makes a declared skip
// condition load-bearing. A flag absent from run state reads as false, so
// `skipWhen: "!x"` on a node skips that node by default, and the delivery graph's
// e2e_ok guard accepts a skip as satisfying. That combination is the reported bug
// with the runtime cooperating.
//
// So the shipped graph must not carry one. This fails if anyone adds it back
// without deciding, per node, whether skipping by default is safe there.
func TestTheShippedGraphNeverSkipsAVerifierByDefault(t *testing.T) {
	loaded, err := graph.LoadByID(graphDir, "goal-delivery")
	if err != nil {
		t.Fatalf("load graph: %v", err)
	}
	for id, node := range loaded.Spec.Nodes {
		if node.SkipWhen != "" {
			t.Fatalf("node %q declares skipWhen %q; a flag absent from run state reads as false, "+
				"so this would skip by default. Whether that is safe needs deciding per node.",
				id, node.SkipWhen)
		}
	}
}

func TestApplyAdvancesTheRun(t *testing.T) {
	root := workspace(t)
	result := apply(t, root, intakeConfirmed())

	if result.Duplicate {
		t.Fatal("a first checkpoint was treated as a replay")
	}
	if result.Transition == nil || result.Transition.From != "intake" {
		t.Fatalf("did not leave intake: %+v", result.Transition)
	}
	if result.Run.Iteration != 1 {
		t.Errorf("iteration is %d, want 1", result.Run.Iteration)
	}
}

// The failure this exists to prevent: a tool call that timed out after the
// write, retried, and burned a second iteration recording what was already
// recorded.
func TestTheSameEvidenceTwiceAdvancesOnce(t *testing.T) {
	root := workspace(t)
	first := apply(t, root, intakeConfirmed())
	second := apply(t, root, intakeConfirmed())

	if !second.Duplicate {
		t.Fatal("a replayed checkpoint advanced the run again")
	}
	if second.Transition != nil {
		t.Errorf("a duplicate reported a transition: %+v", second.Transition)
	}
	if second.Run.Iteration != first.Run.Iteration {
		t.Errorf("iteration moved from %d to %d on a replay",
			first.Run.Iteration, second.Run.Iteration)
	}
	if second.Run.CurrentNode != first.Run.CurrentNode {
		t.Errorf("node moved from %q to %q on a replay",
			first.Run.CurrentNode, second.Run.CurrentNode)
	}
}

// Idempotency must not turn into paralysis: the next real result still moves.
func TestDifferentEvidenceStillAdvances(t *testing.T) {
	root := workspace(t)
	apply(t, root, intakeConfirmed())

	before, err := state.Load(state.ManifestPath(root, "demo"))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	result := apply(t, root, loop.Outcome{})

	if result.Duplicate {
		t.Fatal("a different outcome was mistaken for a replay")
	}
	if result.Run.Iteration <= before.Iteration {
		t.Errorf("iteration did not advance: %d then %d", before.Iteration, result.Run.Iteration)
	}
}

// The key answers "is this the same assertion", so the clock must not be in it.
func TestKeyIgnoresTheClockAndResultOrder(t *testing.T) {
	early := intakeConfirmed()
	late := intakeConfirmed()
	late.Check.Check.At = at().Add(time.Hour)
	if Key(early) != Key(late) {
		t.Error("the same evidence recorded an hour later got a different key")
	}

	forward := loop.Outcome{Result: map[string]bool{"a": true, "b": false}}
	backward := loop.Outcome{Result: map[string]bool{"b": false, "a": true}}
	if Key(forward) != Key(backward) {
		t.Error("map iteration order changed the key")
	}
}

func TestKeyDistinguishesDifferentEvidence(t *testing.T) {
	passing := intakeConfirmed()
	failing := intakeConfirmed()
	failing.Check.Check.Passed = false
	if Key(passing) == Key(failing) {
		t.Error("a pass and a fail share a key")
	}

	other := intakeConfirmed()
	other.Check.Name = "spec_approved"
	if Key(passing) == Key(other) {
		t.Error("two different checks share a key")
	}
}

// A bare advance asserts nothing, so consecutive ones must not be mistaken for
// each other. A run walking spec then plan would otherwise stall on the second.
func TestABareAdvanceIsNeverAReplay(t *testing.T) {
	if Key(loop.Outcome{}) != "" {
		t.Fatal("an outcome asserting nothing was given a key")
	}

	root := workspace(t)
	apply(t, root, intakeConfirmed())
	first := apply(t, root, loop.Outcome{})
	second := apply(t, root, loop.Outcome{})

	if second.Duplicate {
		t.Error("a second bare advance was refused as a replay")
	}
	if second.Run.Iteration <= first.Run.Iteration {
		t.Errorf("the run stalled: iteration %d then %d",
			first.Run.Iteration, second.Run.Iteration)
	}
}
