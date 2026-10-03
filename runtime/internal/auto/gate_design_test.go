package auto

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/workspace"
)

const diagramOnlyPlan = "# Plan\n\n```mermaid\nflowchart TD\n  a --> b\n```\n"

func writeGateDocs(t *testing.T, root, slug, plan string) {
	t.Helper()
	dir := workspace.DocsDir(root, slug)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{"PLAN.md": plan, "TASKS.md": "# Tasks\n\n- T1\n"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func rulesOf(found []Ambiguity) map[string]bool {
	rules := map[string]bool{}
	for _, f := range found {
		rules[f.Rule] = true
	}
	return rules
}

// The design gate is where auto mode would otherwise skip the only look a
// person gets at how the result will be judged, so a diagram alone must not
// open it.
func TestApproveDesignGateRefusesAPlanWithoutAnEvaluationProtocol(t *testing.T) {
	root := t.TempDir()
	writeGateDocs(t, root, "design-gate", diagramOnlyPlan)

	found, _, err := ScanGateDocuments(root, "design-gate", "approve_design", "")
	if err != nil {
		t.Fatal(err)
	}
	rules := rulesOf(found)
	if !rules[RuleMissingEvaluationProtocol] || !rules[RuleMissingDataTerms] {
		t.Fatalf("want both protocol and data-terms findings, got %s", Report(found))
	}
}

// The same PLAN file is read at approve_plan on the product graph, where a
// software plan has no evaluation split. Only the researcher design gate asks.
func TestApprovePlanGateDoesNotRequireAnEvaluationProtocol(t *testing.T) {
	root := t.TempDir()
	writeGateDocs(t, root, "plan-gate", diagramOnlyPlan)

	found, _, err := ScanGateDocuments(root, "plan-gate", "approve_plan", "")
	if err != nil {
		t.Fatal(err)
	}
	if rules := rulesOf(found); rules[RuleMissingEvaluationProtocol] || rules[RuleMissingDataTerms] {
		t.Fatalf("approve_plan must not require the researcher sections, got %s", Report(found))
	}
}
