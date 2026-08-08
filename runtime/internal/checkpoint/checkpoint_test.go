package checkpoint

import (
	"testing"
	"time"

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
