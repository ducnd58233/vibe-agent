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

func allocateExpectationRun(t *testing.T, root, slug, body string) {
	t.Helper()
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	if _, err := runpath.Allocate(root, slug, now); err != nil {
		t.Fatalf("Allocate: %v", err)
	}
	if body == "" {
		return
	}
	dir := filepath.Join(state.RunDir(root, slug), "expectation")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ExpectationReviewFile), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestExpectationMissingREVIEWFails(t *testing.T) {
	root := t.TempDir()
	allocateExpectationRun(t, root, "exp-miss", "")

	result, err := Expectation{}.Verify(t.Context(), Request{Slug: "exp-miss", WorkspaceRoot: root})
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

func TestExpectationPassAndFailRows(t *testing.T) {
	root := t.TempDir()

	passBody := `# Expectation review
status: pass
attempt: 1
updated: 2026-08-25T12:00:00Z

| AC id | Spec reference | Observed evidence path | result |
|-------|----------------|------------------------|--------|
| AC1   | SPEC G1        | unit/log               | pass   |
| AC2   | SPEC G2        | graph-path-evals       | pass   |
`
	allocateExpectationRun(t, root, "exp-pass", passBody)
	result, err := Expectation{}.Verify(t.Context(), Request{Slug: "exp-pass", WorkspaceRoot: root})
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if !result.Check.Passed {
		t.Fatalf("pass review must pass: %s", result.Summary)
	}

	failBody := strings.Replace(passBody, "status: pass", "status: fail", 1)
	failBody = strings.Replace(failBody, "| AC2   | SPEC G2        | graph-path-evals       | pass   |",
		"| AC2   | SPEC G2        | missing                | fail   |", 1)
	allocateExpectationRun(t, root, "exp-fail", failBody)
	result, err = Expectation{}.Verify(t.Context(), Request{Slug: "exp-fail", WorkspaceRoot: root})
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if result.Check.Passed {
		t.Fatal("fail review must not pass")
	}
	if !strings.Contains(result.Summary, "fail_rows=1") {
		t.Errorf("summary = %q, want fail_rows=1", result.Summary)
	}
}

func TestExpectationBadStatusFails(t *testing.T) {
	root := t.TempDir()
	allocateExpectationRun(t, root, "exp-bad", "# Expectation review\nstatus: maybe\n")
	result, err := Expectation{}.Verify(t.Context(), Request{Slug: "exp-bad", WorkspaceRoot: root})
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if result.Check.Passed {
		t.Fatal("bad status must not pass")
	}
}

func TestExpectationAttemptCapNote(t *testing.T) {
	root := t.TempDir()
	body := `# Expectation review
status: fail
attempt: 3

| AC id | Spec reference | Observed evidence path | result |
|-------|----------------|------------------------|--------|
| AC1   | SPEC G1        | none                   | fail   |
`
	allocateExpectationRun(t, root, "exp-cap", body)
	result, err := Expectation{}.Verify(t.Context(), Request{Slug: "exp-cap", WorkspaceRoot: root})
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if result.Check.Passed {
		t.Fatal("capped fail must not pass")
	}
	if !strings.Contains(result.Summary, "soft attempt cap exceeded") {
		t.Errorf("summary = %q, want cap note", result.Summary)
	}
}

func TestExpectationNeedsASlug(t *testing.T) {
	if _, err := (Expectation{}).Verify(t.Context(), Request{WorkspaceRoot: t.TempDir()}); err == nil {
		t.Fatal("Verify accepted an empty slug")
	}
}
