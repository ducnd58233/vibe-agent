package verifier

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/runpath"
)

func allocateExperimentRun(t *testing.T, root, slug, body string) {
	t.Helper()
	now := time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)
	if _, err := runpath.Allocate(root, slug, now); err != nil {
		t.Fatalf("Allocate: %v", err)
	}
	if body == "" {
		return
	}
	dir := filepath.Join(state.RunDir(root, slug), "experiment")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ExperimentStatusFile), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestExperimentMissingSTATUSFails(t *testing.T) {
	root := t.TempDir()
	allocateExperimentRun(t, root, "exp-miss", "")

	result, err := Experiment{}.Verify(t.Context(), Request{Slug: "exp-miss", WorkspaceRoot: root})
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if result.Check.Passed {
		t.Fatal("missing STATUS must not pass")
	}
	if result.Check.Source != state.SourceFileAssert {
		t.Errorf("source = %q", result.Check.Source)
	}
}

func TestExperimentRunningFailsDonePasses(t *testing.T) {
	root := t.TempDir()

	allocateExperimentRun(t, root, "exp-run", "status: running\nnote: epoch 1\n")
	result, err := Experiment{}.Verify(t.Context(), Request{Slug: "exp-run", WorkspaceRoot: root})
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if result.Check.Passed {
		t.Fatal("running must not pass")
	}

	allocateExperimentRun(t, root, "exp-done", "status: done\n")
	result, err = Experiment{}.Verify(t.Context(), Request{Slug: "exp-done", WorkspaceRoot: root})
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if !result.Check.Passed {
		t.Fatalf("done must pass: %s", result.Summary)
	}

	allocateExperimentRun(t, root, "exp-fail", "status: failed\n")
	result, err = Experiment{}.Verify(t.Context(), Request{Slug: "exp-fail", WorkspaceRoot: root})
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if !result.Check.Passed {
		t.Fatalf("failed is terminal and must pass the monitor: %s", result.Summary)
	}
}

func TestExperimentNeedsASlug(t *testing.T) {
	if _, err := (Experiment{}).Verify(t.Context(), Request{WorkspaceRoot: t.TempDir()}); err == nil {
		t.Fatal("Verify accepted an empty slug")
	}
}
