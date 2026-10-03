package legacy

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
	"github.com/ducnd58233/vibe-agent/runtime/internal/session"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/infra/database"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/workspace"
	"github.com/ducnd58233/vibe-agent/runtime/internal/tasks"
)

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func rows(t *testing.T, root, table string) int {
	t.Helper()
	db, err := database.Open(t.Context(), workspace.MemoryDBPath(root))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	var n int
	if err := db.QueryRowContext(t.Context(), "SELECT COUNT(*) FROM "+table).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func gone(t *testing.T, paths ...string) {
	t.Helper()
	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			t.Errorf("%s is still on disk after migrating", path)
		}
	}
}

func eventLine(seq int, typ, body string) string {
	raw, _ := json.Marshal(map[string]any{
		"sequence": seq, "type": typ, "at": "2026-08-01T10:00:00Z",
		"payload": map[string]string{"body": body},
	})
	return string(raw) + "\n"
}

func TestCachesAndJournalMoveIntoTheirTables(t *testing.T) {
	root := t.TempDir()
	fetchFile := filepath.Join(workspace.FetchCacheDir(root), "deadbeef.json")
	write(t, fetchFile, `{"document":{"source":"https://example.com/a","text":"a"},"fetchedAt":"2026-08-01T10:00:00Z"}`)
	asset := filepath.Join(workspace.FetchCacheDir(root), "assets", "x.png")
	write(t, asset, "binary")
	sddFile := filepath.Join(workspace.SDDCacheDir(root), "feedface.json")
	write(t, sddFile, `{"url":"https://example.com/s","prompt":"p","content":"c","fetched_at":1}`)
	journal := ambientJournalPath(root)
	write(t, journal, eventLine(1, "tool_use", "ls")+eventLine(2, "tool_use", "pwd"))

	results, err := Migrate(t.Context(), root)
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	moved := map[string]int{}
	for _, r := range results {
		moved[r.Source.Name] = r.Moved
	}
	if moved["fetch cache"] != 1 || moved["sdd-cache"] != 1 || moved["ambient journal"] != 2 {
		t.Errorf("moved = %v", moved)
	}
	if rows(t, root, "fetch_cache") != 1 || rows(t, root, "sdd_cache") != 1 || rows(t, root, "journal_entries") != 2 {
		t.Error("rows missing after migrating")
	}
	gone(t, fetchFile, sddFile, journal)
	if _, err := os.Stat(asset); err != nil {
		t.Error("a fetched asset, which is still a file today, was removed")
	}
}

func TestRunsMoveWithTheirEventsAndIndexPointersGo(t *testing.T) {
	root := t.TempDir()
	run, err := state.NewRun("demo", "prove migration", "goal-delivery", 50, time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(run)
	dir := filepath.Join(workspace.RunsDir(root), "2026-08-01", "demo", "1")
	manifest := filepath.Join(dir, manifestName)
	events := filepath.Join(dir, state.EventLogName)
	write(t, manifest, string(encoded))
	write(t, events, eventLine(1, "run_started", "")+eventLine(2, "transition", ""))
	index := filepath.Join(workspace.RunIndexDir(root), "demo.json")
	write(t, index, `{"slug":"demo","date":"2026-08-01","version":1}`)
	// A manifest that is not a versioned run (a backup someone parked here).
	stray := filepath.Join(workspace.RunsDir(root), "2026-08-01", "backups", "dump", manifestName)
	write(t, stray, `{}`)

	moved, err := migrateRuns(t.Context(), root)
	if err != nil || moved != 1 {
		t.Fatalf("migrateRuns = %d, %v", moved, err)
	}
	gone(t, manifest, events, index)
	if _, err := os.Stat(stray); err != nil {
		t.Error("a manifest outside the run layout was removed")
	}
	loaded, err := state.Load(state.ManifestPath(root, "demo"))
	if err != nil || loaded.RunID != run.RunID {
		t.Fatalf("the run is not readable from the database: %v", err)
	}
	read, err := state.ReadEvents(state.EventLogPath(root, "demo"))
	if err != nil || len(read) != 2 {
		t.Errorf("events = %d (%v), want 2", len(read), err)
	}
}

func TestSessionLogsMoveAndACorruptOneIsSetAside(t *testing.T) {
	root := t.TempDir()
	runLog := filepath.Join(workspace.RunsDir(root), "2026-08-01", "demo", "1", state.SessionLogName)
	ambient := filepath.Join(workspace.StateDir(root), state.SessionLogName)
	write(t, runLog, eventLine(1, "prompt_submit", "first")+eventLine(2, "message", "second"))
	write(t, ambient, "not json\n")

	moved, err := migrateSessions(t.Context(), root)
	if err == nil || !strings.Contains(err.Error(), "manual repair") {
		t.Fatalf("a corrupt log was not reported for repair: %v", err)
	}
	if moved != 2 {
		t.Errorf("moved = %d, want the readable log's 2 events", moved)
	}
	gone(t, runLog, ambient)
	if _, err := os.Stat(ambient + corruptSuffix); err != nil {
		t.Error("the corrupt log was not kept for repair")
	}
	replayed, err := session.Replay(runLog)
	if err != nil || len(replayed) != 2 {
		t.Errorf("replayed %d (%v), want the 2 migrated events", len(replayed), err)
	}
}

func TestTaskListsMoveAndAnUnparseableOneStays(t *testing.T) {
	root := t.TempDir()
	good := filepath.Join(root, "docs", "2026-08-01", "demo", "1", "tasks-2026-08-01.json")
	write(t, good, `{"schemaVersion":1,"slug":"demo","date":"2026-08-01","version":1,`+
		`"tasks":[{"id":"T1","title":"first","status":"queued"}]}`)
	bad := filepath.Join(root, "docs", "2026-08-01", "other", "1", "tasks-2026-08-01.json")
	write(t, bad, `{not json`)
	prose := filepath.Join(root, "docs", "2026-08-01", "demo", "1", "TASKS-2026-08-01.md")
	write(t, prose, "# Tasks\n")

	moved, err := migrateTaskLists(t.Context(), root)
	if err != nil || moved != 1 {
		t.Fatalf("migrateTaskLists = %d, %v", moved, err)
	}
	gone(t, good)
	for _, kept := range []string{bad, prose} {
		if _, err := os.Stat(kept); err != nil {
			t.Errorf("%s should have been left in place", kept)
		}
	}
	if _, err := tasks.Load(root, "demo"); err != nil {
		t.Errorf("the migrated task list is not readable: %v", err)
	}
}

func TestPendingNamesWhatIsLeftAndMigratingClearsIt(t *testing.T) {
	root := t.TempDir()
	if pending := Pending(root); len(pending) != 0 {
		t.Fatalf("a fresh workspace reports pending layouts: %v", pending)
	}
	write(t, nodeReminderPath(root), `{"slug":"demo","node":"build"}`)
	write(t, ambientJournalPath(root), eventLine(1, "tool_use", "ls"))
	if pending := Pending(root); len(pending) != 2 {
		t.Fatalf("pending = %v, want the marker and the journal", pending)
	}
	if _, err := Migrate(t.Context(), root); err != nil {
		t.Fatal(err)
	}
	if pending := Pending(root); len(pending) != 0 {
		t.Errorf("still pending after migrating: %v", pending)
	}
	again, err := Migrate(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range again {
		if r.Moved != 0 {
			t.Errorf("%s moved %d on a second run", r.Source.Name, r.Moved)
		}
	}
}

// Pending is a read: a workspace that never had a database must not get one
// from being asked.
func TestPendingNeverCreatesTheDatabase(t *testing.T) {
	root := t.TempDir()
	_ = Pending(root)
	if _, err := os.Stat(workspace.MemoryDBPath(root)); err == nil {
		t.Error("Pending created memory.db")
	}
}
