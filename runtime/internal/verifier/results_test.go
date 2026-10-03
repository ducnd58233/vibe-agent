package verifier

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
)

func TestResultsVerifierAcceptsMetThresholds(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	slug := "metrics-ok"
	allocateExperimentRun(t, root, slug, "status: done\n")
	writeMetrics(t, root, slug, `{
		"metrics": {"ndcg_at_10": 0.9},
		"thresholds": {"ndcg_at_10": {"op": ">=", "value": 0.82}},
		"integrity": {"kind": "not_applicable", "reason": "reproduction check, no tuning step"}
	}`)

	result, err := Results{}.Verify(context.Background(), Request{
		Check: "results_acceptable", WorkspaceRoot: root, Slug: slug,
	})
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !result.Check.Passed {
		t.Fatalf("expected pass, got %q", result.Summary)
	}
}

func TestResultsVerifierLoopsOnShortMetric(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	slug := "metrics-low"
	allocateExperimentRun(t, root, slug, "status: done\n")
	writeMetrics(t, root, slug, `{
		"metrics": {"ndcg_at_10": 0.7},
		"thresholds": {"ndcg_at_10": {"op": ">=", "value": 0.82}},
		"integrity": {"kind": "not_applicable", "reason": "reproduction check, no tuning step"}
	}`)

	result, err := Results{}.Verify(context.Background(), Request{
		Check: "results_acceptable", WorkspaceRoot: root, Slug: slug,
	})
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if result.Check.Passed {
		t.Fatal("expected fail for metric below threshold")
	}
}

func writeMetrics(t *testing.T, root, slug, body string) {
	t.Helper()
	dir := state.RunDir(root, slug)
	path := filepath.Join(dir, "experiment", metricsFileName)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

// heldOut builds a METRICS.json whose threshold is met, so a failure in a test
// below can only come from the integrity record.
func heldOut(integrity string) string {
	return `{
		"metrics": {"f1": 0.85},
		"thresholds": {"f1": {"op": ">=", "value": 0.8}},
		"integrity": ` + integrity + `
	}`
}

func verifyMetrics(t *testing.T, body string) Result {
	t.Helper()
	root := t.TempDir()
	slug := "integrity"
	allocateExperimentRun(t, root, slug, "status: done\n")
	writeMetrics(t, root, slug, body)
	result, err := Results{}.Verify(context.Background(), Request{
		Check: "results_acceptable", WorkspaceRoot: root, Slug: slug,
	})
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	return result
}

const soundHeldOut = `{
	"kind": "held_out_eval",
	"selectionSplit": "validation", "reportedSplit": "test",
	"selectionMetrics": {"f1": 0.87}, "maxGap": {"f1": 0.05},
	"trials": 14, "reportedSplitEvaluations": 1
}`

func TestResultsVerifierAcceptsSoundHeldOutRecord(t *testing.T) {
	t.Parallel()
	if result := verifyMetrics(t, heldOut(soundHeldOut)); !result.Check.Passed {
		t.Fatalf("expected pass, got %q", result.Summary)
	}
}

func TestResultsVerifierRefusesMissingIntegrityBlock(t *testing.T) {
	t.Parallel()
	result := verifyMetrics(t, `{"metrics":{"f1":0.85},"thresholds":{"f1":{"op":">=","value":0.8}}}`)
	if result.Check.Passed || !strings.Contains(result.Summary, "no integrity block") {
		t.Fatalf("a threshold met with no integrity record must fail, got passed=%v %q", result.Check.Passed, result.Summary)
	}
}

func TestResultsVerifierCatchesValidationHighTestLow(t *testing.T) {
	t.Parallel()
	// The threshold is met on test (0.85 >= 0.8), yet validation says 0.97.
	body := heldOut(`{
		"kind": "held_out_eval",
		"selectionSplit": "validation", "reportedSplit": "test",
		"selectionMetrics": {"f1": 0.97}, "maxGap": {"f1": 0.05},
		"trials": 3, "reportedSplitEvaluations": 1
	}`)
	result := verifyMetrics(t, body)
	if result.Check.Passed {
		t.Fatal("a 0.12 gap against an accepted 0.05 must fail even though the threshold is met")
	}
	if !strings.Contains(result.Summary, "leakage into the selection split") {
		t.Errorf("summary should name the likelier cause: %q", result.Summary)
	}
}

func TestResultsVerifierFlagsHeldOutBetterThanSelection(t *testing.T) {
	t.Parallel()
	body := heldOut(`{
		"kind": "held_out_eval",
		"selectionSplit": "validation", "reportedSplit": "test",
		"selectionMetrics": {"f1": 0.70}, "maxGap": {"f1": 0.05},
		"trials": 3, "reportedSplitEvaluations": 1
	}`)
	result := verifyMetrics(t, body)
	if result.Check.Passed || !strings.Contains(result.Summary, "better on the held-out split") {
		t.Fatalf("test far above validation must fail, got passed=%v %q", result.Check.Passed, result.Summary)
	}
}

func TestResultsVerifierGapDirectionFollowsThresholdOperator(t *testing.T) {
	t.Parallel()
	// For a loss, a larger reported value is the worse direction.
	body := `{
		"metrics": {"loss": 0.50},
		"thresholds": {"loss": {"op": "<=", "value": 0.6}},
		"integrity": {
			"kind": "held_out_eval",
			"selectionSplit": "validation", "reportedSplit": "test",
			"selectionMetrics": {"loss": 0.20}, "maxGap": {"loss": 0.05},
			"trials": 2, "reportedSplitEvaluations": 1
		}
	}`
	result := verifyMetrics(t, body)
	if result.Check.Passed || !strings.Contains(result.Summary, "leakage into the selection split") {
		t.Fatalf("loss rising from 0.20 to 0.50 is the worse direction, got passed=%v %q", result.Check.Passed, result.Summary)
	}
}

func TestResultsVerifierRefusesReusedHeldOutSplit(t *testing.T) {
	t.Parallel()
	body := heldOut(`{
		"kind": "held_out_eval",
		"selectionSplit": "validation", "reportedSplit": "test",
		"selectionMetrics": {"f1": 0.87}, "maxGap": {"f1": 0.05},
		"trials": 9, "reportedSplitEvaluations": 4
	}`)
	result := verifyMetrics(t, body)
	if result.Check.Passed || !strings.Contains(result.Summary, "no longer held out") {
		t.Fatalf("a split scored 4 times must fail, got passed=%v %q", result.Check.Passed, result.Summary)
	}
}

func TestResultsVerifierRefusesSameSplitForSelectionAndReport(t *testing.T) {
	t.Parallel()
	body := heldOut(`{
		"kind": "held_out_eval",
		"selectionSplit": "Test ", "reportedSplit": "test",
		"selectionMetrics": {"f1": 0.85}, "maxGap": {"f1": 0.05},
		"trials": 1, "reportedSplitEvaluations": 1
	}`)
	result := verifyMetrics(t, body)
	if result.Check.Passed || !strings.Contains(result.Summary, "not a held-out result") {
		t.Fatalf("same split under different case must fail, got passed=%v %q", result.Check.Passed, result.Summary)
	}
}

func TestResultsVerifierRequiresDeclaredGapAndSelectionMetric(t *testing.T) {
	t.Parallel()
	body := heldOut(`{
		"kind": "held_out_eval",
		"selectionSplit": "validation", "reportedSplit": "test",
		"trials": 1, "reportedSplitEvaluations": 1
	}`)
	result := verifyMetrics(t, body)
	if result.Check.Passed {
		t.Fatal("no selectionMetrics and no maxGap must fail")
	}
	if !strings.Contains(result.Summary, "selectionMetrics") {
		t.Errorf("summary should name the missing value: %q", result.Summary)
	}
}

func TestResultsVerifierNotApplicableNeedsAReason(t *testing.T) {
	t.Parallel()
	result := verifyMetrics(t, heldOut(`{"kind": "not_applicable", "reason": "  "}`))
	if result.Check.Passed || !strings.Contains(result.Summary, "needs a reason") {
		t.Fatalf("blank reason must fail, got passed=%v %q", result.Check.Passed, result.Summary)
	}
}

func TestResultsVerifierRefusesUnknownIntegrityKind(t *testing.T) {
	t.Parallel()
	if result := verifyMetrics(t, heldOut(`{"kind": "trust_me"}`)); result.Check.Passed {
		t.Fatal("an unknown kind must fail")
	}
}

// The checked-in example is what a new project copies, and the schema checker
// validates it from the Python side. This pins the same file from the Go side,
// so the two contracts cannot drift apart without a test failing.
func TestExampleLedgerMetricsSatisfyTheIntegrityRules(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "experiments", "_example", "001", "metrics.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc MetricsDocument
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if problems := checkIntegrity(doc); len(problems) != 0 {
		t.Fatalf("example metrics.json breaks its own integrity rules: %v", problems)
	}
}
