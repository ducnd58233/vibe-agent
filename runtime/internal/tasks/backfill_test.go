package tasks_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/workspace"
	"github.com/ducnd58233/vibe-agent/runtime/internal/tasks"
)

const sample = `{
  "schemaVersion": 1,
  "slug": "demo-task-sql",
  "date": "2026-09-25",
  "version": 1,
  "tasks": [
    {"id": "T1", "title": "first", "status": "done"},
    {"id": "T2", "title": "second", "status": "queued", "dependsOn": ["T1"]}
  ]
}
`

func writeSampleJSON(t *testing.T, root string) string {
	t.Helper()
	dir := workspace.DocsDirAt(root, "2026-09-25", "demo-task-sql", 1)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(workspace.RunDirAt(root, "2026-09-25", "demo-task-sql", 1), 0o750); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "tasks-2026-09-25.json")
	if err := os.WriteFile(path, []byte(sample), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestBackfillMovesEveryTasksJSONIntoTaskListsThenDeletesTheFile(t *testing.T) {
	root := t.TempDir()
	path := writeSampleJSON(t, root)

	migrated, err := tasks.Backfill(context.Background(), root)
	if err != nil {
		t.Fatalf("backfill: %v", err)
	}
	if migrated != 1 {
		t.Fatalf("migrated = %d, want 1", migrated)
	}
	if _, err := os.Stat(path); err == nil {
		t.Fatal("tasks JSON file still on disk after backfill")
	}

	file, err := tasks.Load(root, "demo-task-sql")
	if err != nil {
		t.Fatalf("load after backfill: %v", err)
	}
	if len(file.Tasks) != 2 || file.Tasks[0].ID != "T1" {
		t.Fatalf("loaded %+v", file.Tasks)
	}
}

func TestSaveRoundTripsThroughTaskLists(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(workspace.RunDirAt(root, "2026-09-25", "demo-task-sql", 1), 0o750); err != nil {
		t.Fatal(err)
	}
	parsed, err := tasks.Parse([]byte(sample))
	if err != nil {
		t.Fatal(err)
	}
	if err := tasks.Save(root, parsed); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := tasks.Load(root, "demo-task-sql")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.Tasks[1].Status != tasks.StatusQueued {
		t.Errorf("status = %s, want queued", got.Tasks[1].Status)
	}
}

func TestLoadRequiresTaskListsRow(t *testing.T) {
	root := t.TempDir()
	writeSampleJSON(t, root)
	if _, err := tasks.Load(root, "demo-task-sql"); err == nil {
		t.Fatal("Load succeeded from a leftover JSON file; task lists are SQL-only")
	}
	if _, err := tasks.Backfill(context.Background(), root); err != nil {
		t.Fatal(err)
	}
	file, err := tasks.Load(root, "demo-task-sql")
	if err != nil {
		t.Fatalf("load after backfill: %v", err)
	}
	if file.Slug != "demo-task-sql" {
		t.Errorf("slug = %q", file.Slug)
	}
}

func TestBackfillLeavesTASKSMarkdownUntouched(t *testing.T) {
	root := t.TempDir()
	path := writeSampleJSON(t, root)
	prose := filepath.Join(filepath.Dir(path), "TASKS-2026-09-25.md")
	const body = "## T1: first\n\nAcceptance criteria:\n- [x] done\n"
	if err := os.WriteFile(prose, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := tasks.Backfill(context.Background(), root); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Clean(prose))
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != body {
		t.Fatalf("prose changed: %q", raw)
	}
}

func TestDoctorCountMismatchNoteStillFiresAgainstSQLSource(t *testing.T) {
	// Mirrors doctor_tasks.go's heading-count note: Load from SQL, compare to
	// prose headings, without needing the JSON file on disk.
	root := t.TempDir()
	path := writeSampleJSON(t, root)
	prosePath := filepath.Join(filepath.Dir(path), "TASKS-2026-09-25.md")
	prose := "## T1: first\n\n## T2: second\n\n## T3: prose-only context\n\n"
	if err := os.WriteFile(prosePath, []byte(prose), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := tasks.Backfill(context.Background(), root); err != nil {
		t.Fatal(err)
	}
	file, err := tasks.Load(root, "demo-task-sql")
	if err != nil {
		t.Fatal(err)
	}
	headings := 0
	for _, line := range strings.Split(prose, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## T") || strings.HasPrefix(trimmed, "### T") {
			headings++
		}
	}
	if headings == len(file.Tasks) {
		t.Fatalf("fixture must produce a heading/task count mismatch; headings=%d tasks=%d", headings, len(file.Tasks))
	}
	if !tasks.HasTaskList(root, "demo-task-sql") {
		t.Fatal("HasTaskList should see the SQL row")
	}
}
