package checkpoint

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ducnd58233/vibe-agent/runtime/internal/loop"
	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
)

func TestAWrongLoggedCalculationKeepsTheRunAtAutoResearch(t *testing.T) {
	root := t.TempDir()
	atAutoResearch(t, root)
	writeResearchArtifact(t, root, "Margin.\n\n```calc\n(250 - 200) / 250 * 100 => 25\n```\n")

	result, err := applyWithCitations(t, root, fakeCitations(new([]string)))
	if err == nil {
		t.Fatalf("a digest with a wrong figure advanced: %+v", result.Transition)
	}
	for _, want := range []string{"RESEARCH-2026-09-24.md", "line 4", "the result is 20"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error lacks %q: %v", want, err)
		}
	}
}

func TestACorrectLoggedCalculationLetsTheRunLeaveAutoResearch(t *testing.T) {
	root := t.TempDir()
	atAutoResearch(t, root)
	writeResearchArtifact(t, root, "```calc\n(250 - 200) / 250 * 100 => 20\n```\n")

	if _, err := applyWithCitations(t, root, fakeCitations(new([]string))); err != nil {
		t.Fatalf("a correct calculation was refused: %v", err)
	}
}

func TestACalcFailureIsReportedBeforeTheNetworkIsAsked(t *testing.T) {
	root := t.TempDir()
	atAutoResearch(t, root)
	writeResearchArtifact(t, root, "See https://dead.example/x\n\n```calc\n1 + 1 => 3\n```\n")

	var seen []string
	_, err := applyWithCitations(t, root, fakeCitations(&seen))
	if err == nil || !strings.Contains(err.Error(), "do not recompute") {
		t.Fatalf("want a calculation error, got %v", err)
	}
	if len(seen) != 0 {
		t.Errorf("the citation check ran first and saw %v", seen)
	}
}

func TestABlockerIsNotHeldUpByACalculation(t *testing.T) {
	root := t.TempDir()
	atAutoResearch(t, root)
	writeResearchArtifact(t, root, "```calc\n1 + 1 => 3\n```\n")
	outcome := loop.Outcome{Blocker: "no network", BlockerClass: "tool"}
	if _, err := Apply(t.Context(), Request{
		WorkspaceRoot: root, GraphDir: graphDir, Slug: "demo", Outcome: outcome, Now: at(),
	}); err != nil && strings.Contains(err.Error(), "recompute") {
		t.Fatalf("a blocker was refused over a calculation: %v", err)
	}
}

func calcRun(t *testing.T, root, graphID, node string, docs map[string]string) *state.Run {
	t.Helper()
	run, err := state.NewRun("demo", "check figures", graphID, 50, at())
	if err != nil {
		t.Fatal(err)
	}
	run.CurrentNode, run.Date, run.Version = node, "2026-09-24", 1
	dir := filepath.Join(root, "docs", "2026-09-24", "demo", "1")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	for name, body := range docs {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return run
}

const wrongCalc = "```calc\n1 + 1 => 3\n```\n"
const rightCalc = "```calc\n1 + 1 => 2\n```\n"

// Every graph's own artifact nodes are gated on the figures they just wrote.
func TestCalcsAreCheckedAtEveryArtifactNodeThatWritesFigures(t *testing.T) {
	cases := []struct {
		graph, node, file string
	}{
		{"goal-delivery", "spec", "SPEC-2026-09-24.md"},
		{"goal-delivery", "plan", "PLAN-2026-09-24.md"},
		{"goal-delivery", "plan", "TASKS-2026-09-24.md"},
		{"goal-delivery", "research", "RESEARCH-2026-09-24.md"},
		{"researcher-delivery", "hypothesis", "HYPOTHESIS-2026-09-24.md"},
		{"researcher-delivery", "findings", "FINDINGS-2026-09-24.md"},
		{"researcher-delivery", "writeup", "WRITEUP-2026-09-24.md"},
		{"task-delivery", "spec", "SPEC-2026-09-24.md"},
		{"study-delivery", "session", "STUDY-2026-09-24.md"},
		{"study-delivery", "study_plan", "STUDY-2026-09-24.md"},
	}
	for _, tc := range cases {
		root := t.TempDir()
		bad := calcRun(t, root, tc.graph, tc.node, map[string]string{tc.file: wrongCalc})
		err := checkCalcs(root, bad)
		if err == nil || !strings.Contains(err.Error(), tc.file) {
			t.Errorf("%s at %s: a wrong figure in %s was not refused: %v", tc.graph, tc.node, tc.file, err)
		}
		good := calcRun(t, root, tc.graph, tc.node, map[string]string{tc.file: rightCalc})
		if err := checkCalcs(root, good); err != nil {
			t.Errorf("%s at %s: a right figure was refused: %v", tc.graph, tc.node, err)
		}
	}
}

func TestCalcsAreNotCheckedWhereNoDocumentIsWritten(t *testing.T) {
	root := t.TempDir()
	// build is a code node, and a SPEC with a wrong line already passed its own gate.
	run := calcRun(t, root, "goal-delivery", "build", map[string]string{"SPEC-2026-09-24.md": wrongCalc})
	if err := checkCalcs(root, run); err != nil {
		t.Errorf("a code node was held up by a spec: %v", err)
	}
	// An unknown graph has no targets.
	run = calcRun(t, root, "some-other-graph", "spec", map[string]string{"SPEC-2026-09-24.md": wrongCalc})
	if err := checkCalcs(root, run); err != nil {
		t.Errorf("an unknown graph was gated: %v", err)
	}
}

func TestAMissingDocumentIsNotACalcFailure(t *testing.T) {
	root := t.TempDir()
	run := calcRun(t, root, "goal-delivery", "plan", map[string]string{"PLAN-2026-09-24.md": rightCalc})
	// TASKS is missing; PLAN is fine.
	if err := checkCalcs(root, run); err != nil {
		t.Errorf("a missing sibling document failed the check: %v", err)
	}
}

func TestEveryFailureAcrossDocumentsIsReported(t *testing.T) {
	root := t.TempDir()
	run := calcRun(t, root, "goal-delivery", "plan", map[string]string{
		"PLAN-2026-09-24.md":  wrongCalc,
		"TASKS-2026-09-24.md": wrongCalc,
	})
	err := checkCalcs(root, run)
	if err == nil || !strings.Contains(err.Error(), "PLAN-2026-09-24.md") || !strings.Contains(err.Error(), "TASKS-2026-09-24.md") {
		t.Errorf("want both documents named, got %v", err)
	}
}
