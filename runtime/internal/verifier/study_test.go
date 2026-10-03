package verifier

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/runpath"
	sharedworkspace "github.com/ducnd58233/vibe-agent/runtime/internal/shared/workspace"
)

const studyTable = `# Study record

## Topics
| # | Can do | Status | Last tested | Score | Next review |
|---|--------|--------|-------------|-------|-------------|
| 1 | can add vectors | %s | 2026-10-03 | 3 of 3 | 2026-10-09 |
| 2 | can scale vectors | %s | | | |

## Misconceptions
| Date | What they believed |
|------|--------------------|
| 2026-10-03 | learned is a feeling |
`

func studyRun(t *testing.T, slug, record string) string {
	t.Helper()
	root := t.TempDir()
	now := time.Date(2026, 10, 3, 9, 0, 0, 0, time.UTC)
	if _, err := runpath.Allocate(root, slug, now); err != nil {
		t.Fatal(err)
	}
	run, err := state.NewRun(slug, "learn it", "study-delivery", 400, now)
	if err != nil {
		t.Fatal(err)
	}
	run.Date, run.Version = "2026-10-03", 1
	if err := state.Save(state.ManifestPath(root, slug), run); err != nil {
		t.Fatal(err)
	}
	if record != "" {
		dir := sharedworkspace.DocsDirAt(root, run.Date, slug, run.Version)
		if err := os.MkdirAll(dir, 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "STUDY-2026-10-03.md"), []byte(record), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func verifyStudy(t *testing.T, root, slug string) Result {
	t.Helper()
	result, err := Study{}.Verify(t.Context(), Request{Slug: slug, WorkspaceRoot: root})
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	return result
}

func TestStudyContinuesWhileATopicIsNotLearned(t *testing.T) {
	root := studyRun(t, "s-open", fmt.Sprintf(studyTable, "learned", "learning"))
	result := verifyStudy(t, root, "s-open")
	if !result.Check.Passed {
		t.Fatalf("study should continue: %s", result.Summary)
	}
}

func TestStudyEndsOnlyWhenEveryTopicIsLearned(t *testing.T) {
	root := studyRun(t, "s-done", fmt.Sprintf(studyTable, "learned", "Learned"))
	result := verifyStudy(t, root, "s-done")
	if result.Check.Passed {
		t.Fatalf("every topic is learned, so topics-remaining must be false: %s", result.Summary)
	}
}

// Every doubtful case leaves the study going. A mistake can keep a learner
// studying; it must never end the study early.
func TestStudyContinuesOnEveryDoubtfulRecord(t *testing.T) {
	cases := map[string]string{
		"missing":     "",
		"empty table": "## Topics\n| # | Can do | Status |\n|---|---|---|\n",
		"no table":    "just prose, no table\n",
		"no status":   "| # | Can do |\n|---|---|\n| 1 | a |\n",
		"short row":   "| # | Can do | Status |\n|---|---|---|\n| 1 | a |\n",
	}
	for name, record := range cases {
		root := studyRun(t, "s-"+sanitize(name), record)
		result := verifyStudy(t, root, "s-"+sanitize(name))
		if !result.Check.Passed {
			t.Errorf("%s: study ended on a doubtful record: %s", name, result.Summary)
		}
	}
}

func TestStudyIgnoresOtherTables(t *testing.T) {
	// The misconceptions table has no Status column, and a table that follows the
	// topics must not be counted as topics.
	root := studyRun(t, "s-other", fmt.Sprintf(studyTable, "learned", "learned"))
	if verifyStudy(t, root, "s-other").Check.Passed {
		t.Fatal("rows from another table were counted")
	}
}

func sanitize(s string) string {
	out := []rune(s)
	for i, r := range out {
		if r == ' ' {
			out[i] = '-'
		}
	}
	return string(out)
}
