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

func allocateReviewRun(t *testing.T, root, slug, body string) {
	t.Helper()
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	if _, err := runpath.Allocate(root, slug, now); err != nil {
		t.Fatalf("Allocate: %v", err)
	}
	if body == "" {
		return
	}
	dir := filepath.Join(state.RunDir(root, slug), "review")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ReviewFile), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

const completeReviewBody = `# Review

## Correctness
No issues found.

## Readability
No issues found.

## Architecture
No issues found.

## Security
No issues found.

## Performance
No issues found.
`

func TestReviewMissingREVIEWFails(t *testing.T) {
	root := t.TempDir()
	allocateReviewRun(t, root, "review-miss", "")

	result, err := Review{}.Verify(t.Context(), Request{Slug: "review-miss", WorkspaceRoot: root})
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if result.Check.Passed {
		t.Fatal("missing REVIEW must not pass")
	}
	if result.Check.Source != state.SourceFileAssert {
		t.Errorf("source = %q", result.Check.Source)
	}
}

func TestReviewAllFiveAxesPasses(t *testing.T) {
	root := t.TempDir()
	allocateReviewRun(t, root, "review-pass", completeReviewBody)

	result, err := Review{}.Verify(t.Context(), Request{Slug: "review-pass", WorkspaceRoot: root})
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if !result.Check.Passed {
		t.Fatalf("a review naming all five axes must pass: %s", result.Summary)
	}
}

func TestReviewMissingOneAxisFails(t *testing.T) {
	root := t.TempDir()
	body := strings.Replace(completeReviewBody, "## Security\nNo issues found.\n\n", "", 1)
	allocateReviewRun(t, root, "review-partial", body)

	result, err := Review{}.Verify(t.Context(), Request{Slug: "review-partial", WorkspaceRoot: root})
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if result.Check.Passed {
		t.Fatal("a review missing the security axis must not pass")
	}
	if !strings.Contains(result.Summary, "security") {
		t.Errorf("summary = %q, want it to name the missing axis", result.Summary)
	}
}

func TestReviewDoesNotGradeQuality(t *testing.T) {
	root := t.TempDir()
	body := `# Review

Critical: correctness bug at file.go:10.
Important: readability could improve naming.
architecture is coupled to the wrong package.
security: no findings.
performance: no findings.
`
	allocateReviewRun(t, root, "review-narrative", body)

	result, err := Review{}.Verify(t.Context(), Request{Slug: "review-narrative", WorkspaceRoot: root})
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if !result.Check.Passed {
		t.Fatalf("presence of all five axis names must pass regardless of what the findings say: %s", result.Summary)
	}
}

func TestReviewDoesNotMatchAxisNamesInsideOtherWords(t *testing.T) {
	root := t.TempDir()
	body := strings.Replace(completeReviewBody, "## Correctness\nNo issues found.\n\n",
		"## Incorrectness risk\nNo issues found.\n\n", 1)
	allocateReviewRun(t, root, "review-substring", body)

	result, err := Review{}.Verify(t.Context(), Request{Slug: "review-substring", WorkspaceRoot: root})
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if result.Check.Passed {
		t.Fatal("\"incorrectness\" must not satisfy the \"correctness\" axis")
	}
}

func TestReviewNeedsASlug(t *testing.T) {
	if _, err := (Review{}).Verify(t.Context(), Request{WorkspaceRoot: t.TempDir()}); err == nil {
		t.Fatal("Verify accepted an empty slug")
	}
}
