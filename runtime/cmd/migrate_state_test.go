package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	fetchpersistence "github.com/ducnd58233/vibe-agent/runtime/internal/fetch/infra/persistence"
	"github.com/ducnd58233/vibe-agent/runtime/internal/run/domain"
	runpersistence "github.com/ducnd58233/vibe-agent/runtime/internal/run/infra/persistence"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/infra/database"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/runpath"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/workspace"
)

func TestMigrateStateMovesFetchCacheFiles(t *testing.T) {
	root := t.TempDir()
	dir := fetchpersistence.CacheDir(root)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(map[string]any{
		"document":  map[string]any{"source": "https://example.com/z", "text": "z"},
		"fetchedAt": time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "deadbeef.json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}

	sddDir := workspace.SDDCacheDir(root)
	if err := os.MkdirAll(sddDir, 0o750); err != nil {
		t.Fatal(err)
	}
	sddRaw, err := json.Marshal(map[string]any{
		"url": "https://example.com/sdd", "prompt": "p", "content": "c", "fetched_at": 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sddDir, "feedface.json"), sddRaw, 0o600); err != nil {
		t.Fatal(err)
	}

	journalPath := filepath.Join(workspace.StateDir(root), "journal.ndjson")
	journalLine, err := json.Marshal(map[string]any{
		"sequence": 1, "type": "tool_use",
		"payload": map[string]any{"tool": "Bash", "command": "ls"},
		"at":      time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(journalPath, append(journalLine, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}

	tasksDir := workspace.DocsDirAt(root, "2026-09-25", "migrate-demo", 1)
	if err := os.MkdirAll(tasksDir, 0o750); err != nil {
		t.Fatal(err)
	}
	tasksRaw := []byte(`{
  "schemaVersion": 1,
  "slug": "migrate-demo",
  "date": "2026-09-25",
  "version": 1,
  "tasks": [{"id": "T1", "title": "one", "status": "queued"}]
}
`)
	tasksPath := filepath.Join(tasksDir, "tasks-2026-09-25.json")
	if err := os.WriteFile(tasksPath, tasksRaw, 0o600); err != nil {
		t.Fatal(err)
	}

	run, err := domain.NewRun("migrate-demo", "goal", "goal-delivery", 50, time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	run.Date = "2026-09-25"
	run.Version = 1
	if _, err := runpath.Allocate(root, run.Slug, time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	manifestPath := runpersistence.ManifestPath(root, run.Slug)
	if err := runpersistence.Save(manifestPath, run); err != nil {
		t.Fatal(err)
	}
	eventsPath := runpersistence.EventLogPath(root, run.Slug)
	if _, err := runpersistence.AppendRunEvent(eventsPath, domain.Event{
		Type: domain.EventRunStarted, At: time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatal(err)
	}

	if err := migrateStateCommand([]string{"--workspace", root, "--toolkit", toolkitRoot}); err != nil {
		t.Fatalf("migrate state: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "deadbeef.json")); err == nil {
		t.Error("the legacy fetch cache file still exists after migrate state")
	}
	if _, err := os.Stat(filepath.Join(sddDir, "feedface.json")); err == nil {
		t.Error("the legacy sdd-cache file still exists after migrate state")
	}
	if _, err := os.Stat(journalPath); err == nil {
		t.Error("the legacy ambient journal file still exists after migrate state")
	}
	if _, err := os.Stat(tasksPath); err == nil {
		t.Error("the legacy tasks JSON file still exists after migrate state")
	}
	if _, err := os.Stat(manifestPath); err == nil {
		t.Error("the legacy run manifest still exists after migrate state")
	}
	if _, err := os.Stat(eventsPath); err == nil {
		t.Error("the legacy run events log still exists after migrate state")
	}
	if _, err := os.Stat(runpath.IndexPath(root, run.Slug)); err == nil {
		t.Error("the legacy run-index file still exists after migrate state")
	}

	db, err := database.Open(t.Context(), workspace.MemoryDBPath(root))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close: %v", err)
		}
	})
	var fetchCount, sddCount int
	if err := db.QueryRowContext(t.Context(), `SELECT count(*) FROM fetch_cache`).Scan(&fetchCount); err != nil {
		t.Fatal(err)
	}
	if fetchCount != 1 {
		t.Errorf("fetch_cache has %d rows, want 1", fetchCount)
	}
	if err := db.QueryRowContext(t.Context(), `SELECT count(*) FROM sdd_cache`).Scan(&sddCount); err != nil {
		t.Fatal(err)
	}
	if sddCount != 1 {
		t.Errorf("sdd_cache has %d rows, want 1", sddCount)
	}
	var journalCount int
	if err := db.QueryRowContext(t.Context(), `SELECT count(*) FROM journal_entries WHERE run_id IS NULL`).
		Scan(&journalCount); err != nil {
		t.Fatal(err)
	}
	if journalCount != 1 {
		t.Errorf("journal_entries has %d ambient rows, want 1", journalCount)
	}
	var taskCount int
	if err := db.QueryRowContext(t.Context(), `SELECT count(*) FROM task_lists`).Scan(&taskCount); err != nil {
		t.Fatal(err)
	}
	if taskCount != 1 {
		t.Errorf("task_lists has %d rows, want 1", taskCount)
	}
	var runsCount, runEventsCount int
	if err := db.QueryRowContext(t.Context(), `SELECT count(*) FROM runs`).Scan(&runsCount); err != nil {
		t.Fatal(err)
	}
	if runsCount != 1 {
		t.Errorf("runs has %d rows, want 1", runsCount)
	}
	if err := db.QueryRowContext(t.Context(), `SELECT count(*) FROM run_events`).Scan(&runEventsCount); err != nil {
		t.Fatal(err)
	}
	if runEventsCount != 1 {
		t.Errorf("run_events has %d rows, want 1", runEventsCount)
	}
}

func TestMigrateStateIsSafeWithNothingToMove(t *testing.T) {
	root := t.TempDir()
	if err := migrateStateCommand([]string{"--workspace", root, "--toolkit", toolkitRoot}); err != nil {
		t.Fatalf("migrate state on an empty workspace: %v", err)
	}
}

func TestMigrateCommandDispatchesToState(t *testing.T) {
	root := t.TempDir()
	if err := migrateCommand([]string{"state", "--workspace", root, "--toolkit", toolkitRoot}); err != nil {
		t.Fatalf("migrate state via dispatch: %v", err)
	}
}

func TestMigrateCommandRefusesAnUnknownSubcommand(t *testing.T) {
	if err := migrateCommand([]string{"nonsense"}); err == nil {
		t.Error("an unknown migrate subcommand was accepted")
	}
}
