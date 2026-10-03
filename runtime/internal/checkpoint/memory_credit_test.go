package checkpoint

import (
	"strings"
	"testing"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/memory"
	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
)

// exposedMemory seeds a confirmed memory and records it as exposed to the run.
func exposedMemory(t *testing.T, root string) (*memory.Store, string) {
	t.Helper()
	store, err := memory.Open(t.Context(), root)
	if err != nil {
		t.Fatalf("open memory: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	rec, _, err := store.Propose(t.Context(), memory.Record{
		WorkspaceID: memory.WorkspaceKey(root), Kind: memory.KindSemantic,
		Content: "the unit suite needs the fixtures directory generated first", Confidence: 0.9,
		SourceType: memory.SourceCommandResult, Evidence: []string{"go test ./... failed until make fixtures ran"},
	}, at().Add(-2*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Confirm(t.Context(), rec.ID, memory.SourceCommandResult, "events.ndjson#1", at().Add(-2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	run, err := state.Load(state.ManifestPath(root, "demo"))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.RecordExposures(t.Context(), []string{rec.ID}, []string{run.RunID}, at().Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	return store, rec.ID
}

func usedCount(t *testing.T, store *memory.Store, id string) int {
	t.Helper()
	rec, err := store.Get(t.Context(), id)
	if err != nil {
		t.Fatal(err)
	}
	return rec.UsedCount
}

const unitPlan = `apiVersion: vibe-agent/v1
kind: CheckPlan
spec:
  checks:
    unit:
      command: go
      args: [test, ./...]
`

// The strong signal: a memory that was in front of the model, followed by a
// check the runtime itself watched pass in that same run.
func TestARuntimeVerifiedPassCreditsTheMemoriesTheRunWasShown(t *testing.T) {
	root := workspace(t)
	declarePlan(t, root, unitPlan)
	atTestNode(t, root)
	store, id := exposedMemory(t, root)

	if _, err := Apply(t.Context(), Request{
		WorkspaceRoot: root, GraphDir: graphDir, Slug: "demo",
		Outcome: unitPassed(), origin: originRuntime, Now: at(),
	}); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if got := usedCount(t, store, id); got != 1 {
		t.Fatalf("used_count = %d, want 1", got)
	}
	history, _ := store.History(t.Context(), id)
	last := history[len(history)-1]
	if last.Action != "use" || last.Actor != "verifier" || !strings.Contains(last.Detail, "check unit passed") {
		t.Errorf("the ledger does not say what earned the use: %+v", last)
	}
}

// A human-declared check is a person's statement, and a caller's evidence is a
// claim; neither is a verifier watching it pass, so neither credits a memory.
func TestCallerEvidenceNeverCreditsAMemory(t *testing.T) {
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
	store, id := exposedMemory(t, root)

	human := unitPassed()
	human.Check.Check.Source = state.SourceHumanEvent
	if _, err := Apply(t.Context(), Request{
		WorkspaceRoot: root, GraphDir: graphDir, Slug: "demo", Outcome: human, Now: at(),
	}); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if got := usedCount(t, store, id); got != 0 {
		t.Errorf("a caller-recorded pass credited a memory: used_count = %d", got)
	}
}

func TestAFailedOrSkippedCheckCreditsNothing(t *testing.T) {
	for name, mutate := range map[string]func(*state.Check){
		"failed":  func(c *state.Check) { c.Passed = false },
		"skipped": func(c *state.Check) { c.Passed = false; c.Skipped = true; c.Source = state.SourceFileAssert },
	} {
		t.Run(name, func(t *testing.T) {
			root := workspace(t)
			declarePlan(t, root, unitPlan)
			atTestNode(t, root)
			store, id := exposedMemory(t, root)
			outcome := unitPassed()
			mutate(&outcome.Check.Check)
			_, _ = Apply(t.Context(), Request{
				WorkspaceRoot: root, GraphDir: graphDir, Slug: "demo",
				Outcome: outcome, origin: originRuntime, Now: at(),
			})
			if got := usedCount(t, store, id); got != 0 {
				t.Errorf("used_count = %d after a %s check", got, name)
			}
		})
	}
}
