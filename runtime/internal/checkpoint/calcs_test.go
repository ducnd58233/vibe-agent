package checkpoint

import (
	"strings"
	"testing"

	"github.com/ducnd58233/vibe-agent/runtime/internal/loop"
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
