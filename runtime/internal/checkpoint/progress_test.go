package checkpoint

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ducnd58233/vibe-agent/runtime/internal/loop"
	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
	"github.com/ducnd58233/vibe-agent/runtime/internal/testutil"
)

// atAutoResearch parks a fresh run directly at auto_research, bypassing the
// long walk through spec/plan/build/.../results_eval a real retry would
// take - Apply does not care how a run arrived at a node, only that the node
// is real and the outcome is well-formed.
func atAutoResearch(t *testing.T, root string) {
	t.Helper()
	run, err := state.NewRun("demo", "prove the no-progress check works", "goal-delivery", 50, at())
	if err != nil {
		t.Fatalf("NewRun: %v", err)
	}
	run.CurrentNode = "auto_research"
	run.Date = "2026-09-24"
	run.Version = 1
	testutil.EnsureRunIndex(t, root, "demo")
	if err := state.Save(state.ManifestPath(root, "demo"), run); err != nil {
		t.Fatalf("save: %v", err)
	}
}

func writeResearchArtifact(t *testing.T, root, content string) {
	t.Helper()
	path := filepath.Join(root, "docs", "2026-09-24", "demo", "1", "RESEARCH-2026-09-24.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

func TestApplyAllowsTheFirstVisitToAutoResearchRegardlessOfContent(t *testing.T) {
	root := t.TempDir()
	atAutoResearch(t, root)
	writeResearchArtifact(t, root, "first attempt, whatever it says\n")

	if _, err := Apply(Request{
		WorkspaceRoot: root, GraphDir: graphDir, Slug: "demo",
		Outcome: loop.Outcome{}, Now: at(),
	}); err != nil {
		t.Fatalf("a first visit must never be refused: %v", err)
	}
}

// The failure this closes: results_eval routes back to auto_research on a
// missed threshold, but nothing stopped a resubmission of the same content
// from mechanically satisfying the node forever, spinning the loop until
// budget_exceeded instead of ever trying something different.
func TestApplyRefusesAnIdenticalResubmissionAtAutoResearch(t *testing.T) {
	root := t.TempDir()
	atAutoResearch(t, root)
	content := "the approach and findings, unchanged across attempts\n"
	writeResearchArtifact(t, root, content)

	// First visit: accepted, and the snapshot is recorded.
	if _, err := Apply(Request{
		WorkspaceRoot: root, GraphDir: graphDir, Slug: "demo",
		Outcome: loop.Outcome{}, Now: at(),
	}); err != nil {
		t.Fatalf("first visit: %v", err)
	}

	// Simulate the retry cycle landing back at auto_research with the exact
	// same content already on disk.
	atAutoResearch(t, root)
	writeResearchArtifact(t, root, content)

	_, err := Apply(Request{
		WorkspaceRoot: root, GraphDir: graphDir, Slug: "demo",
		Outcome: loop.Outcome{}, Now: at(),
	})
	if err == nil {
		t.Fatal("an identical resubmission at auto_research must be refused")
	}
}

func TestApplyAllowsAGenuinelyRevisedResubmissionAtAutoResearch(t *testing.T) {
	root := t.TempDir()
	atAutoResearch(t, root)
	writeResearchArtifact(t, root, "The approach is X.\nTried A.\nTried B.\nConclusion: unclear.\n")

	if _, err := Apply(Request{
		WorkspaceRoot: root, GraphDir: graphDir, Slug: "demo",
		Outcome: loop.Outcome{}, Now: at(),
	}); err != nil {
		t.Fatalf("first visit: %v", err)
	}

	atAutoResearch(t, root)
	writeResearchArtifact(t, root, "The approach is Y.\nTried C.\nTried D.\nConclusion: works.\n")

	if _, err := Apply(Request{
		WorkspaceRoot: root, GraphDir: graphDir, Slug: "demo",
		Outcome: loop.Outcome{}, Now: at(),
	}); err != nil {
		t.Errorf("a genuinely revised resubmission must be allowed: %v", err)
	}
}

// A blocker outcome is an unrelated escape valve (AGENTS.md "Blocker vs.
// retry") and must not be caught up in a content comparison that has nothing
// to do with why it was recorded.
func TestApplyDoesNotApplyTheNoProgressCheckToABlockerOutcome(t *testing.T) {
	root := t.TempDir()
	atAutoResearch(t, root)
	content := "same content every time\n"
	writeResearchArtifact(t, root, content)

	if _, err := Apply(Request{
		WorkspaceRoot: root, GraphDir: graphDir, Slug: "demo",
		Outcome: loop.Outcome{}, Now: at(),
	}); err != nil {
		t.Fatalf("first visit: %v", err)
	}

	atAutoResearch(t, root)
	writeResearchArtifact(t, root, content)

	if _, err := Apply(Request{
		WorkspaceRoot: root, GraphDir: graphDir, Slug: "demo",
		Outcome: loop.Outcome{Blocker: "research tool unavailable", BlockerClass: state.FailureTool},
		Now:     at(),
	}); err != nil {
		t.Errorf("a blocker outcome must not be refused by the no-progress check: %v", err)
	}
}
