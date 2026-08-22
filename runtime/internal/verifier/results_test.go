package verifier

import (
	"context"
	"os"
	"path/filepath"
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
		"thresholds": {"ndcg_at_10": {"op": ">=", "value": 0.82}}
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
		"thresholds": {"ndcg_at_10": {"op": ">=", "value": 0.82}}
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
