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

func TestRequireEvaluationProtocolNeedsBothSections(t *testing.T) {
	found := RequireEvaluationProtocol("# Plan\n\n```mermaid\nflowchart TD\n  a --> b\n```\n")
	if len(found) != 2 {
		t.Fatalf("want protocol and data-terms findings, got %d: %s", len(found), Report(found))
	}

	complete := strings.Join([]string{
		"# Plan",
		"## Evaluation protocol",
		"",
		"- Split: 70/15/15 by user id, fixed before any model is fit.",
		"- Tune on validation only; the test split is scored once.",
		"- Frozen: f1 >= 0.80, maxGap 0.05. Trial budget: 20.",
		"",
		"## Data and terms",
		"",
		"- Source: internal export v3, licence internal, no PII after hashing.",
		"- No model output is reused as training data.",
	}, "\n")
	if found := RequireEvaluationProtocol(complete); len(found) != 0 {
		t.Errorf("complete PLAN flagged: %s", Report(found))
	}
}

func TestRequireEvaluationProtocolRejectsBareNotApplicable(t *testing.T) {
	plan := "# Plan\n\n## Evaluation protocol\n\nN/A\n\n## Data and terms\n\nNone.\n"
	if found := RequireEvaluationProtocol(plan); len(found) != 2 {
		t.Fatalf("a bare N/A must not satisfy either section, got %d: %s", len(found), Report(found))
	}
	stated := "# Plan\n\n## Evaluation protocol\n\nNo model is trained; the claim is a closed-form identity checked symbolically.\n\n" +
		"## Data and terms\n\nNo external data is read; every input is generated in the script.\n"
	if found := RequireEvaluationProtocol(stated); len(found) != 0 {
		t.Errorf("a stated reason should satisfy the gate: %s", Report(found))
	}
}
