package verifier

import (
	"os"
	"path/filepath"
	"strings"
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

	allocateExperimentRun(t, root, "exp-done", "status: done\njudgement: confirmed\n")
	result, err = Experiment{}.Verify(t.Context(), Request{Slug: "exp-done", WorkspaceRoot: root})
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if !result.Check.Passed {
		t.Fatalf("done with a judgement must pass: %s", result.Summary)
	}

	allocateExperimentRun(t, root, "exp-fail", "status: failed\njudgement: refuted\n")
	result, err = Experiment{}.Verify(t.Context(), Request{Slug: "exp-fail", WorkspaceRoot: root})
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if !result.Check.Passed {
		t.Fatalf("failed with a judgement is terminal and must pass the monitor: %s", result.Summary)
	}
}

// A terminal status with no stated judgement must not pass: "done" or
// "failed" alone says the experiment stopped, not whether anyone looked at
// what it meant against the hypothesis it was testing. See
// docs/2026-10-01/generalize-experiment-monitoring-must/1/RESEARCH-2026-10-01.md.
func TestExperimentTerminalWithoutJudgementFails(t *testing.T) {
	for _, status := range []string{"done", "failed"} {
		root := t.TempDir()
		allocateExperimentRun(t, root, "exp-"+status, "status: "+status+"\n")
		result, err := Experiment{}.Verify(t.Context(), Request{Slug: "exp-" + status, WorkspaceRoot: root})
		if err != nil {
			t.Fatalf("Verify: %v", err)
		}
		if result.Check.Passed {
			t.Errorf("status %s with no judgement: line must not pass: %s", status, result.Summary)
		}
	}
}

func TestExperimentTerminalWithARecognizedJudgementPasses(t *testing.T) {
	for _, judgement := range []string{"confirmed", "refuted", "inconclusive", "not_applicable"} {
		root := t.TempDir()
		slug := "exp-judgement-" + strings.ReplaceAll(judgement, "_", "-")
		body := "status: done\njudgement: " + judgement + "\n"
		allocateExperimentRun(t, root, slug, body)
		result, err := Experiment{}.Verify(t.Context(), Request{Slug: slug, WorkspaceRoot: root})
		if err != nil {
			t.Fatalf("Verify: %v", err)
		}
		if !result.Check.Passed {
			t.Errorf("judgement %q must pass: %s", judgement, result.Summary)
		}
	}
}

func TestExperimentUnrecognizedJudgementFails(t *testing.T) {
	root := t.TempDir()
	allocateExperimentRun(t, root, "exp-bad-judgement", "status: done\njudgement: looks-good-to-me\n")
	result, err := Experiment{}.Verify(t.Context(), Request{Slug: "exp-bad-judgement", WorkspaceRoot: root})
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if result.Check.Passed {
		t.Error("an unrecognized judgement value must not pass")
	}
}

// Still running has nothing to judge yet; a missing judgement: line must not
// change why this fails, and must not itself be enough to fail only on that.
func TestExperimentRunningNeverNeedsAJudgement(t *testing.T) {
	root := t.TempDir()
	allocateExperimentRun(t, root, "exp-running-no-judgement", "status: running\n")
	result, err := Experiment{}.Verify(t.Context(), Request{Slug: "exp-running-no-judgement", WorkspaceRoot: root})
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if result.Check.Passed {
		t.Fatal("running must still not pass")
	}
	if result.Summary != "experiment status running" {
		t.Errorf("running's failure reason must stay about status, not judgement: %s", result.Summary)
	}
}

func TestExperimentNeedsASlug(t *testing.T) {
	if _, err := (Experiment{}).Verify(t.Context(), Request{WorkspaceRoot: t.TempDir()}); err == nil {
		t.Fatal("Verify accepted an empty slug")
	}
}
