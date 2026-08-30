package checkpoint

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/checkplan"
	"github.com/ducnd58233/vibe-agent/runtime/internal/graph"
	"github.com/ducnd58233/vibe-agent/runtime/internal/loop"
	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shipdecision"
	"github.com/ducnd58233/vibe-agent/runtime/internal/testutil"
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
	testutil.EnsureRunIndex(t, root, "demo")
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

func TestAutoCheckpointCannotBypassADeclaredVerifier(t *testing.T) {
	root := workspaceAuto(t)
	declarePlan(t, root, `apiVersion: vibe-agent/v1
kind: CheckPlan
spec:
  checks:
    unit:
      command: go
      args: [version]
    e2e:
      command: go
      args: [version]
`)
	atTestNode(t, root)

	current, err := state.Load(state.ManifestPath(root, "demo"))
	if err != nil {
		t.Fatalf("load e2e state: %v", err)
	}
	current.CurrentNode = "e2e"
	if err := state.Save(state.ManifestPath(root, "demo"), current); err != nil {
		t.Fatalf("save e2e state: %v", err)
	}

	_, err = Apply(Request{
		WorkspaceRoot: root, GraphDir: graphDir, Slug: "demo",
		Outcome: loop.Outcome{}, Now: at(),
	})
	if err == nil {
		t.Fatal("a bare auto checkpoint bypassed a declared e2e verifier")
	}
	if !strings.Contains(err.Error(), "must report a check") {
		t.Errorf("the refusal does not identify the verifier boundary: %v", err)
	}
}

func TestAutoCheckpointRejectsCallerHumanEvidenceWhenAnAutoVerifierExists(t *testing.T) {
	root := workspaceAuto(t)
	declarePlan(t, root, `apiVersion: vibe-agent/v1
kind: CheckPlan
spec:
  checks:
    unit:
      verifier: human
      auto:
        verifier: shipdecision
`)
	atTestNode(t, root)

	human := unitPassed()
	human.Check.Check.Source = state.SourceHumanEvent
	if _, err := Apply(Request{
		WorkspaceRoot: root, GraphDir: graphDir, Slug: "demo",
		Outcome: human, Now: at(),
	}); err == nil {
		t.Fatal("an auto run accepted caller-supplied human_event evidence")
	} else if !strings.Contains(err.Error(), "auto verifier") {
		t.Errorf("wrong refusal for caller-supplied human_event: %v", err)
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

// workspaceAuto is workspace with the run's auto flag set, for tests
// exercising an Auto entry's fallback path (docs/auto-ship-reviews).
func workspaceAuto(t *testing.T) string {
	t.Helper()
	root := workspace(t)
	run, err := state.Load(state.ManifestPath(root, "demo"))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if err := run.SetFlagAt("auto", true, at()); err != nil {
		t.Fatalf("SetFlagAt: %v", err)
	}
	testutil.EnsureRunIndex(t, root, "demo")
	if err := state.Save(state.ManifestPath(root, "demo"), run); err != nil {
		t.Fatalf("save: %v", err)
	}
	return root
}

// Without the auto flag, a check declaring both verifier: human and an auto
// entry must behave exactly as if the auto entry did not exist. This is the
// test that keeps /goal provably unaffected by docs/auto-ship-reviews.
func TestAutoEntryIsUnreachableWithoutTheAutoFlag(t *testing.T) {
	root := workspace(t)
	declarePlan(t, root, `apiVersion: vibe-agent/v1
kind: CheckPlan
spec:
  checks:
    unit:
      verifier: human
      auto:
        verifier: shipdecision
`)
	atTestNode(t, root)

	_, err := Resolve(VerifyRequest{WorkspaceRoot: root, GraphDir: graphDir, Slug: "demo", Now: at()})
	if err == nil {
		t.Fatal("resolved without the auto flag; the auto entry must stay unreachable")
	}
	if !strings.Contains(err.Error(), "person records it") {
		t.Errorf("wrong refusal without the auto flag: %v", err)
	}
}

// With the auto flag set, the same check resolves through its Auto entry
// instead of refusing.
func TestAutoEntryIsUsedWhenTheAutoFlagIsSet(t *testing.T) {
	root := workspaceAuto(t)
	declarePlan(t, root, `apiVersion: vibe-agent/v1
kind: CheckPlan
spec:
  checks:
    unit:
      verifier: human
      auto:
        verifier: shipdecision
`)
	atTestNode(t, root)

	plan, err := Resolve(VerifyRequest{WorkspaceRoot: root, GraphDir: graphDir, Slug: "demo", Now: at()})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if plan.Kind != "shipdecision" {
		t.Errorf("Kind = %q, want shipdecision", plan.Kind)
	}
}

// End to end: on the auto path, a real DECISION.md saying GO is read by the
// shipdecision verifier and genuinely advances the node — runtime-origin
// evidence, not a caller's assertion.
func TestVerifyRunsTheAutoEntrysVerifierAndAdvances(t *testing.T) {
	root := workspaceAuto(t)
	declarePlan(t, root, `apiVersion: vibe-agent/v1
kind: CheckPlan
spec:
  checks:
    unit:
      verifier: human
      auto:
        verifier: shipdecision
`)
	atTestNode(t, root)

	decisionPath := shipdecision.Path(root, "demo")
	if err := os.MkdirAll(filepath.Dir(decisionPath), 0o750); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(decisionPath, []byte("Ship Decision: GO\n"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	result, err := Verify(context.Background(), VerifyRequest{WorkspaceRoot: root, GraphDir: graphDir, Slug: "demo", Now: at()})
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if !result.Verifier.Check.Passed {
		t.Errorf("Passed = false, want true: %s", result.Verifier.Summary)
	}
	// workspaceAuto sets the auto flag, so the graph's auto edge out of `test`
	// goes straight to `simplify` rather than `e2e` — this is the existing
	// auto-path shortcut, not something this change alters.
	if result.Applied.Transition == nil || result.Applied.Transition.To != "simplify" {
		t.Fatalf("did not advance via the auto edge: %+v", result.Applied.Transition)
	}
}

// A missing DECISION.md must not pass. The check has never run yet, and
// fail-closed means that reports as not-passed, never a default GO.
func TestVerifyFailsClosedOnAMissingDecisionFile(t *testing.T) {
	root := workspaceAuto(t)
	declarePlan(t, root, `apiVersion: vibe-agent/v1
kind: CheckPlan
spec:
  checks:
    unit:
      verifier: human
      auto:
        verifier: shipdecision
`)
	atTestNode(t, root)

	result, err := Verify(context.Background(), VerifyRequest{WorkspaceRoot: root, GraphDir: graphDir, Slug: "demo", Now: at()})
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if result.Verifier.Check.Passed {
		t.Fatal("a missing decision file passed")
	}
	// Fail-closed means the run does not proceed forward, not that the graph
	// freezes: the existing !unit_passed edge correctly routes back to `build`
	// for a fix cycle, same as any other failing check at this node.
	if result.Applied.Transition == nil || result.Applied.Transition.To != "build" {
		t.Fatalf("did not route back to build on a missing decision file: %+v", result.Applied.Transition)
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

	result, err := Verify(t.Context(), VerifyRequest{
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

func TestAnUndeclaredE2ECheckSkipsAfterAnAutoBugHuntReentry(t *testing.T) {
	root := workspaceAuto(t)
	declarePlan(t, root, `apiVersion: vibe-agent/v1
kind: CheckPlan
spec:
  checks:
    unit:
      command: go
      args: [version]
    bug_hunt_ok:
      verifier: bughunt
`)
	atTestNode(t, root)

	advanceAutoCycle := func(verifyE2E bool) {
		t.Helper()
		current, err := state.Load(state.ManifestPath(root, "demo"))
		if err != nil {
			t.Fatalf("load cycle start: %v", err)
		}
		if current.CurrentNode == "build" {
			apply(t, root, loop.Outcome{})
		}
		if _, err := Verify(t.Context(), VerifyRequest{
			WorkspaceRoot: root, GraphDir: graphDir, Slug: "demo", Now: at(),
		}); err != nil {
			t.Fatalf("verify unit: %v", err)
		}
		apply(t, root, loop.Outcome{})
		if _, err := Verify(t.Context(), VerifyRequest{
			WorkspaceRoot: root, GraphDir: graphDir, Slug: "demo", Now: at(),
		}); err != nil {
			t.Fatalf("verify omitted lint: %v", err)
		}
		apply(t, root, loop.Outcome{})
		var e2eCheck state.Check
		if verifyE2E {
			result, err := Verify(t.Context(), VerifyRequest{
				WorkspaceRoot: root, GraphDir: graphDir, Slug: "demo", Now: at(),
			})
			if err != nil {
				t.Fatalf("verify omitted e2e: %v", err)
			}
			e2eCheck = result.Verifier.Check
		} else {
			applied := apply(t, root, loop.Outcome{})
			e2eCheck = applied.Run.Checks["e2e"]
		}
		if !e2eCheck.Skipped || e2eCheck.Passed ||
			e2eCheck.Source != state.SourceFileAssert {
			t.Fatalf("e2e result = %+v, want file_assert skip", e2eCheck)
		}
		if _, err := Verify(t.Context(), VerifyRequest{
			WorkspaceRoot: root, GraphDir: graphDir, Slug: "demo", Now: at(),
		}); err != nil {
			t.Fatalf("verify bug hunt: %v", err)
		}

		current, err = state.Load(state.ManifestPath(root, "demo"))
		if err != nil {
			t.Fatalf("load plan re-entry: %v", err)
		}
		current.Flags["plan_unambiguous"] = true
		if err := state.Save(state.ManifestPath(root, "demo"), current); err != nil {
			t.Fatalf("save plan re-entry: %v", err)
		}
		apply(t, root, loop.Outcome{})
		current, err = state.Load(state.ManifestPath(root, "demo"))
		if err != nil {
			t.Fatalf("load approval gate: %v", err)
		}
		if check, recorded := current.Checks["plan_approved"]; !recorded ||
			!check.Skipped || check.Passed {
			t.Fatalf("plan gate = %+v, want skipped and not passed", check)
		}
		apply(t, root, loop.Outcome{})
	}

	advanceAutoCycle(true)
	advanceAutoCycle(false)
}

// skipWhen is honored at execution time, which makes a declared skip condition
// load-bearing. A flag absent from run state reads as false, so `skipWhen: "!x"`
// skips that node by default, and a guard that accepts a skip then treats the
// node as satisfied. That combination is the reported bug with the runtime
// cooperating.
//
// The hazard is the negated form, not the setting. A positive condition reads
// false in a fresh run and the node runs, which is the direction it has to fail
// in. So this rejects negation outright and keeps the positive ones on a list
// with a reason each, which is the per-node decision the earlier blanket ban
// was standing in for.
func TestTheShippedGraphNeverSkipsANodeByDefault(t *testing.T) {
	loaded, err := graph.LoadByID(graphDir, "goal-delivery")
	if err != nil {
		t.Fatalf("load graph: %v", err)
	}

	decided := map[string]string{
		"intake":       "auto mode agreed the objective from the prompt it was given; there was no one to ask",
		"approve_spec": "auto mode reported the spec holds no open question, and a person is still asked when it does",
		"approve_plan": "the same, for the plan",
	}

	for id, node := range loaded.Spec.Nodes {
		if node.SkipWhen == "" {
			continue
		}
		if _, negated := node.SkipCondition(); negated {
			t.Errorf("node %q declares skipWhen %q; a negated condition holds in a fresh run, "+
				"so the node would be skipped by default", id, node.SkipWhen)
			continue
		}
		if decided[id] == "" {
			t.Errorf("node %q declares skipWhen %q and is not on the decided list; "+
				"add it with the reason skipping it is safe, or take the condition off", id, node.SkipWhen)
		}
	}

	for id := range decided {
		if node, ok := loaded.Spec.Nodes[id]; !ok || node.SkipWhen == "" {
			t.Errorf("node %q is listed as a decided skip but declares none; drop the entry", id)
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
