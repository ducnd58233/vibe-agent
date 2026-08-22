package auto

import (
	"strings"
	"testing"
)

func TestRequireApplicabilityNeedsSectionsAndMermaid(t *testing.T) {
	empty := "# Research\n\nSome prose.\n"
	found := RequireApplicability(empty)
	if len(found) < 3 {
		t.Fatalf("want applicability+refine+mermaid findings, got %d: %s", len(found), Report(found))
	}

	complete := strings.Join([]string{
		"# Research",
		"## Applicability",
		"",
		"| Source | Reuse | Reject | Gap |",
		"| --- | --- | --- | --- |",
		"| Paper A | method X | claim Y | no finance data |",
		"",
		"## Refine",
		"",
		"- Drop claim Y; add our ticker universe.",
		"",
		"```mermaid",
		"flowchart LR",
		"  lit --> apply",
		"```",
	}, "\n")
	if found := RequireApplicability(complete); len(found) != 0 {
		t.Errorf("complete RESEARCH flagged: %s", Report(found))
	}
}

func TestRequireExperimentDiagramNeedsMermaid(t *testing.T) {
	if found := RequireExperimentDiagram("# Plan\n\nDo the thing.\n"); len(found) != 1 {
		t.Fatalf("want one mermaid finding, got %v", found)
	}
	plan := "# Plan\n\n```mermaid\nflowchart TD\n  a --> b\n```\n"
	if found := RequireExperimentDiagram(plan); len(found) != 0 {
		t.Errorf("plan with mermaid flagged: %s", Report(found))
	}
}
