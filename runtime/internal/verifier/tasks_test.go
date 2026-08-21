package verifier

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
)

func workspaceWithTasks(t *testing.T, body string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "docs", "demo")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tasks.json"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestTasksPassesWhileWorkRemains(t *testing.T) {
	root := workspaceWithTasks(t, `{
	  "schemaVersion": 1, "slug": "demo", "date": "2026-08-21", "version": 1,
	  "tasks": [
	    {"id": "T1", "title": "shipped", "status": "done"},
	    {"id": "T2", "title": "still open", "status": "blocked"}
	  ]
	}`)

	result, err := Tasks{}.Verify(context.Background(), Request{WorkspaceRoot: root, Slug: "demo"})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Check.Passed {
		t.Error("a blocked task did not count as remaining; blocked work is still in scope")
	}
	if result.Check.Source != state.SourceFileAssert {
		t.Errorf("source = %q, want file_assert", result.Check.Source)
	}
	if !strings.Contains(result.Detail, "T2") {
		t.Errorf("detail = %q, want it to name the remaining task", result.Detail)
	}
}

func TestTasksFailsWhenEveryTaskIsSettled(t *testing.T) {
	root := workspaceWithTasks(t, `{
	  "schemaVersion": 1, "slug": "demo", "date": "2026-08-21", "version": 1,
	  "tasks": [
	    {"id": "T1", "title": "shipped", "status": "done"},
	    {"id": "T2", "title": "dropped", "status": "canceled"}
	  ]
	}`)
	writeTasksProse(t, root, `
## T1: shipped  [done]
**Acceptance criteria:**
- [x] done work landed
## T2: dropped  [canceled]
`)

	result, err := Tasks{}.Verify(context.Background(), Request{WorkspaceRoot: root, Slug: "demo"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Check.Passed {
		t.Error("the check passed with nothing left to do; the run would loop back to build forever")
	}
}

func TestTasksKeepsDoneWithOpenAcceptanceBoxes(t *testing.T) {
	root := workspaceWithTasks(t, `{
	  "schemaVersion": 1, "slug": "demo", "date": "2026-08-21", "version": 1,
	  "tasks": [
	    {"id": "T1", "title": "claimed done", "status": "done"}
	  ]
	}`)
	writeTasksProse(t, root, `
## T1: claimed done  [done]
**Acceptance criteria:**
- [x] one
- [ ] two still open
`)

	result, err := Tasks{}.Verify(context.Background(), Request{WorkspaceRoot: root, Slug: "demo"})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Check.Passed {
		t.Fatal("open acceptance boxes did not keep the task remaining")
	}
	if !strings.Contains(result.Detail, "T1") {
		t.Errorf("detail = %q, want T1 named", result.Detail)
	}
}

func writeTasksProse(t *testing.T, root, body string) {
	t.Helper()
	path := filepath.Join(root, "docs", "demo", "TASKS.md")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

// Treating an absent file as an empty one would end a run on a file somebody
// forgot to write.
func TestAMissingTaskListIsAnErrorRatherThanAnEnding(t *testing.T) {
	if _, err := (Tasks{}).Verify(context.Background(), Request{WorkspaceRoot: t.TempDir(), Slug: "demo"}); err == nil {
		t.Fatal("a missing task list verified as though nothing remained")
	}
}

func TestTasksNeedsASlugToFindItsFile(t *testing.T) {
	if _, err := (Tasks{}).Verify(context.Background(), Request{WorkspaceRoot: t.TempDir()}); err == nil {
		t.Fatal("the verifier ran without a slug")
	}
}

func TestTheRegistryOffersTasks(t *testing.T) {
	if _, err := Default().Get("tasks"); err != nil {
		t.Fatalf("tasks is not registered: %v", err)
	}
}
