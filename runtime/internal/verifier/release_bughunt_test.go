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

func allocateNamedReview(t *testing.T, root, slug, subdir, file, body string) {
	t.Helper()
	now := time.Date(2026, 8, 25, 14, 0, 0, 0, time.UTC)
	if _, err := runpath.Allocate(root, slug, now); err != nil {
		t.Fatalf("Allocate: %v", err)
	}
	if body == "" {
		return
	}
	dir := filepath.Join(state.RunDir(root, slug), subdir)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, file), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestReleasePassAndFail(t *testing.T) {
	root := t.TempDir()
	passBody := `# Release readiness
status: pass
attempt: 1

| Gate id | Evidence source | Pointer | result |
|---------|-----------------|---------|--------|
| R1 | file_assert | ship/DECISION.md | pass |
`
	allocateNamedReview(t, root, "rel-pass", "release", ReleaseReviewFile, passBody)
	result, err := Release{}.Verify(t.Context(), Request{Slug: "rel-pass", WorkspaceRoot: root})
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if !result.Check.Passed {
		t.Fatalf("pass: %s", result.Summary)
	}

	failBody := strings.Replace(passBody, "status: pass", "status: fail", 1)
	failBody = strings.Replace(failBody, "| pass |", "| fail |", 1)
	allocateNamedReview(t, root, "rel-fail", "release", ReleaseReviewFile, failBody)
	result, err = Release{}.Verify(t.Context(), Request{Slug: "rel-fail", WorkspaceRoot: root})
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if result.Check.Passed {
		t.Fatal("fail must not pass")
	}
}

func TestBugHuntMissingFails(t *testing.T) {
	root := t.TempDir()
	allocateNamedReview(t, root, "bh-miss", "bug_hunt", BugHuntFindingsFile, "")
	result, err := BugHunt{}.Verify(t.Context(), Request{Slug: "bh-miss", WorkspaceRoot: root})
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if result.Check.Passed {
		t.Fatal("missing FINDINGS must not pass")
	}
}

func TestBugHuntSoftCapNote(t *testing.T) {
	root := t.TempDir()
	body := `# Bug hunt
status: fail
attempt: 3

| Case | Evidence | result |
|------|----------|--------|
| F1 | none | fail |
`
	allocateNamedReview(t, root, "bh-cap", "bug_hunt", BugHuntFindingsFile, body)
	result, err := BugHunt{}.Verify(t.Context(), Request{Slug: "bh-cap", WorkspaceRoot: root})
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if result.Check.Passed {
		t.Fatal("capped fail must not pass")
	}
	if !strings.Contains(result.Summary, "soft attempt cap exceeded") {
		t.Errorf("summary = %q", result.Summary)
	}
}
