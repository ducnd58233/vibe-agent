package persistence

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/ducnd58233/vibe-agent/runtime/internal/run/domain"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/infra/database"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/runpath"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/workspace"
)

func TestBackfillMovesManifestAndEventsThenDeletesFiles(t *testing.T) {
	root := t.TempDir()
	run := newTestRun(t)
	run.Date = "2026-07-29"
	run.Version = 1
	path, eventsPath := indexedPaths(t, root, run.Slug)
	if err := Save(path, run); err != nil {
		t.Fatal(err)
	}
	if _, err := AppendRunEvent(eventsPath, domain.Event{
		Type: domain.EventTransition, Node: "build", At: fixedTime(),
	}); err != nil {
		t.Fatal(err)
	}

	// Evidence beside the machine files must survive.
	evidence := filepath.Join(filepath.Dir(path), "review", "REVIEW.md")
	if err := os.MkdirAll(filepath.Dir(evidence), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(evidence, []byte("ok\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	// Drop SQL rows written by Save/Append so Backfill exercises the file path.
	if err := deleteRunSQLRow(root, run.RunID); err != nil {
		t.Fatal(err)
	}

	migrated, err := Backfill(context.Background(), root)
	if err != nil {
		t.Fatalf("Backfill: %v", err)
	}
	if migrated != 1 {
		t.Fatalf("migrated = %d, want 1", migrated)
	}

	if _, err := os.Stat(path); err == nil {
		t.Fatal("manifest.json still exists after backfill")
	}
	if _, err := os.Stat(eventsPath); err == nil {
		t.Fatal("events.ndjson still exists after backfill")
	}
	if _, err := os.Stat(runpath.IndexPath(root, run.Slug)); err == nil {
		t.Fatal("run-index file still exists after backfill")
	}
	if _, err := os.Stat(evidence); err != nil {
		t.Fatalf("evidence was removed: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load after backfill: %v", err)
	}
	if loaded.RunID != run.RunID {
		t.Fatalf("loaded runId = %s", loaded.RunID)
	}
	events, err := ReadEvents(eventsPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Node != "build" {
		t.Fatalf("events = %+v", events)
	}

	db, err := database.Open(context.Background(), workspace.MemoryDBPath(root))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	var runCount, eventCount int
	if err := db.QueryRowContext(context.Background(), `SELECT count(*) FROM runs`).Scan(&runCount); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(context.Background(), `SELECT count(*) FROM run_events`).Scan(&eventCount); err != nil {
		t.Fatal(err)
	}
	if runCount != 1 || eventCount != 1 {
		t.Fatalf("runs=%d events=%d", runCount, eventCount)
	}
}

func TestBackfillIsSafeWithNothingToMove(t *testing.T) {
	root := t.TempDir()
	migrated, err := Backfill(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if migrated != 0 {
		t.Fatalf("migrated = %d, want 0", migrated)
	}
}

func TestBackfillRemovesRunIndexEvenWithoutManifests(t *testing.T) {
	root := t.TempDir()
	if err := runpath.SaveIndex(root, runpath.Entry{
		Slug: "orphan", Date: "2026-07-29", Version: 1,
	}); err != nil {
		t.Fatal(err)
	}
	migrated, err := Backfill(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if migrated != 0 {
		t.Fatalf("migrated = %d", migrated)
	}
	if _, err := os.Stat(runpath.IndexPath(root, "orphan")); err == nil {
		t.Fatal("orphan run-index file was not removed")
	}
}

func TestBackfillThenResolveWithoutIndex(t *testing.T) {
	root := t.TempDir()
	run := newTestRun(t)
	run.Date = "2026-07-29"
	run.Version = 1
	path, _ := indexedPaths(t, root, run.Slug)
	if err := Save(path, run); err != nil {
		t.Fatal(err)
	}
	if err := deleteRunSQLRow(root, run.RunID); err != nil {
		t.Fatal(err)
	}
	if _, err := Backfill(context.Background(), root); err != nil {
		t.Fatal(err)
	}

	entry, err := runpath.Resolve(root, run.Slug)
	if err != nil {
		t.Fatalf("Resolve after backfill: %v", err)
	}
	if entry.Version != 1 {
		t.Fatalf("entry = %+v", entry)
	}
}

func TestBackfillWritesFullJSONBody(t *testing.T) {
	root := t.TempDir()
	run := newTestRun(t)
	run.Flags = map[string]bool{"auto": true}
	path, _ := indexedPaths(t, root, run.Slug)
	if err := Save(path, run); err != nil {
		t.Fatal(err)
	}
	if err := deleteRunSQLRow(root, run.RunID); err != nil {
		t.Fatal(err)
	}
	if _, err := Backfill(context.Background(), root); err != nil {
		t.Fatal(err)
	}

	db, err := openDB(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	var body string
	if err := db.QueryRowContext(context.Background(), `SELECT body FROM runs WHERE run_id = ?`, run.RunID).Scan(&body); err != nil {
		t.Fatal(err)
	}
	var decoded domain.Run
	if err := json.Unmarshal([]byte(body), &decoded); err != nil {
		t.Fatal(err)
	}
	if !decoded.Flags["auto"] {
		t.Fatalf("body flags lost: %+v", decoded.Flags)
	}
}

func TestBackfillLeavesEvidenceSubdirs(t *testing.T) {
	root := t.TempDir()
	run := newTestRun(t)
	path, _ := indexedPaths(t, root, run.Slug)
	if err := Save(path, run); err != nil {
		t.Fatal(err)
	}
	ship := filepath.Join(filepath.Dir(path), "ship", "DECISION.md")
	if err := os.MkdirAll(filepath.Dir(ship), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ship, []byte("GO\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := deleteRunSQLRow(root, run.RunID); err != nil {
		t.Fatal(err)
	}
	if _, err := Backfill(context.Background(), root); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(ship); err != nil {
		t.Fatalf("ship evidence missing: %v", err)
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() == "manifest.json" || e.Name() == domain.EventLogName {
			t.Fatalf("machine file %q still present", e.Name())
		}
	}
}
