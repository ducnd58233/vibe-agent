package persistence

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/run/domain"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/runpath"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/workspace"
)

func TestSaveLoadRoundTripViaSQL(t *testing.T) {
	root := t.TempDir()
	run := newTestRun(t)
	run.Date = "2026-07-29"
	run.Version = 1
	path, eventsPath := indexedPaths(t, root, run.Slug)

	if err := Save(path, run); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := AppendRunEvent(eventsPath, domain.Event{
		Type: domain.EventRunStarted, At: fixedTime(),
	}); err != nil {
		t.Fatalf("AppendRunEvent: %v", err)
	}
	if _, err := AppendRunEvent(eventsPath, domain.Event{
		Type: domain.EventTransition, Node: "build", At: fixedTime(),
	}); err != nil {
		t.Fatalf("AppendRunEvent: %v", err)
	}

	// Remove the files so Load/ReadEvents must use SQL.
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(eventsPath); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load from SQL: %v", err)
	}
	if loaded.RunID != run.RunID || loaded.Slug != run.Slug {
		t.Fatalf("loaded %+v", loaded)
	}

	events, err := ReadEvents(eventsPath)
	if err != nil {
		t.Fatalf("ReadEvents from SQL: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("got %d events, want 2", len(events))
	}
	if events[0].Type != domain.EventRunStarted || events[1].Node != "build" {
		t.Fatalf("events = %+v", events)
	}
}

func TestLatestEntryAndResolveUseRunsTable(t *testing.T) {
	root := t.TempDir()
	run := newTestRun(t)
	run.Date = "2026-07-29"
	run.Version = 1
	path, _ := indexedPaths(t, root, run.Slug)
	if err := Save(path, run); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	_ = os.Remove(runpath.IndexPath(root, run.Slug))

	entry, ok, err := LatestEntry(root, run.Slug)
	if err != nil || !ok {
		t.Fatalf("LatestEntry: ok=%v err=%v", ok, err)
	}
	if entry.Date != "2026-07-29" || entry.Version != 1 {
		t.Fatalf("entry = %+v", entry)
	}

	resolved, err := runpath.Resolve(root, run.Slug)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if resolved.Version != 1 || resolved.Date != "2026-07-29" {
		t.Fatalf("Resolve = %+v", resolved)
	}
}

func TestListIncludesSlugsFromRunsTable(t *testing.T) {
	root := t.TempDir()
	run := newTestRun(t)
	run.Date = "2026-07-29"
	run.Version = 1
	path, _ := indexedPaths(t, root, run.Slug)
	if err := Save(path, run); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	_ = os.Remove(runpath.IndexPath(root, run.Slug))
	// Remove the versioned directory so the filesystem walk sees nothing.
	if err := os.RemoveAll(workspace.RunsDir(root)); err != nil {
		t.Fatal(err)
	}

	got, err := List(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != run.Slug {
		t.Fatalf("List = %v, want [%s]", got, run.Slug)
	}
}

func TestRunsTableHasProvenanceColumns(t *testing.T) {
	root := t.TempDir()
	ctx := context.Background()
	db, err := openDB(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	for _, table := range []string{"runs", "run_events"} {
		func() {
			rows, err := db.QueryContext(ctx, `PRAGMA table_info(`+table+`)`)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = rows.Close() }()
			cols := map[string]bool{}
			for rows.Next() {
				var cid int
				var name, ctype string
				var notnull, pk int
				var dflt any
				if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
					t.Fatal(err)
				}
				cols[name] = true
			}
			if err := rows.Err(); err != nil {
				t.Fatal(err)
			}
			for _, want := range []string{"created_by", "reviewed_by_agents", "created_at", "updated_at"} {
				if !cols[want] {
					t.Errorf("%s missing provenance column %q", table, want)
				}
			}
		}()
	}
}

func TestParseRunPath(t *testing.T) {
	root := filepath.Join(t.TempDir(), "ws")
	path := filepath.Join(root, ".agent-state", "runs", "2026-07-29", "demo-slug", "2", "manifest.json")
	loc, base, ok := parseRunPath(path)
	if !ok || base != "manifest.json" {
		t.Fatalf("parse ok=%v base=%q", ok, base)
	}
	if loc.WorkspaceRoot != root || loc.Date != "2026-07-29" || loc.Slug != "demo-slug" || loc.Version != 2 {
		t.Fatalf("loc = %+v", loc)
	}
}

func TestAppendEventOnSessionPathDoesNotTouchRunsTable(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "session.ndjson")
	if _, err := AppendEvent(path, domain.Event{Type: "message", At: fixedTime()}); err != nil {
		t.Fatal(err)
	}
	dbPath := workspace.MemoryDBPath(root)
	if _, err := os.Stat(dbPath); err == nil {
		t.Fatal("session AppendEvent opened memory.db")
	}
}

func TestSaveThenLoadWithFileStillPresentPrefersSQL(t *testing.T) {
	root := t.TempDir()
	run := newTestRun(t)
	path, _ := indexedPaths(t, root, run.Slug)
	if err := Save(path, run); err != nil {
		t.Fatal(err)
	}
	// Corrupt the file; SQL still has the good body.
	if err := os.WriteFile(path, []byte(`{"schemaVersion":1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.RunID != run.RunID {
		t.Fatalf("Load did not prefer SQL: got %s", loaded.RunID)
	}
}

func TestPrepareStartStillAllocatesDirs(t *testing.T) {
	root := t.TempDir()
	entry, err := PrepareStart(root, "fresh-slug", time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	runDir := workspace.RunDirAt(root, entry.Date, entry.Slug, entry.Version)
	if _, err := os.Stat(runDir); err != nil {
		t.Fatalf("run dir missing: %v", err)
	}
}

func TestSaveStoresChecksInRunChecksNotBody(t *testing.T) {
	root := t.TempDir()
	run := newTestRun(t)
	run.Date = "2026-07-29"
	run.Version = 1
	path, _ := indexedPaths(t, root, run.Slug)
	if err := run.SetCheck("unit", domain.Check{
		Passed: true, Source: domain.SourceExitCode, Ref: "unit.log", At: fixedTime(),
	}); err != nil {
		t.Fatal(err)
	}
	if err := Save(path, run); err != nil {
		t.Fatal(err)
	}
	_ = os.Remove(path)

	db, err := openDB(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	var body string
	if err := db.QueryRowContext(context.Background(),
		`SELECT body FROM runs WHERE run_id = ?`, run.RunID).Scan(&body); err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(body), &doc); err != nil {
		t.Fatal(err)
	}
	if _, ok := doc["checks"]; ok {
		t.Fatalf("body still embeds checks: %s", body)
	}

	var count int
	if err := db.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM run_checks WHERE run_id = ?`, run.RunID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("run_checks count = %d, want 1", count)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	got := loaded.Checks["unit"]
	if !got.Passed || got.Source != domain.SourceExitCode || got.Ref != "unit.log" {
		t.Fatalf("loaded check = %+v", got)
	}
}

func TestLoadFallsBackToBodyChecksWhenTableEmpty(t *testing.T) {
	root := t.TempDir()
	run := newTestRun(t)
	run.Date = "2026-07-29"
	run.Version = 1
	if err := run.SetCheck("lint", domain.Check{
		Passed: true, Source: domain.SourceExitCode, At: fixedTime(),
	}); err != nil {
		t.Fatal(err)
	}
	path, _ := indexedPaths(t, root, run.Slug)

	ctx := context.Background()
	db, err := openDB(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	raw, err := json.Marshal(run)
	if err != nil {
		t.Fatal(err)
	}
	now := fixedTime().Format(time.RFC3339)
	_, err = db.ExecContext(ctx, `
        INSERT INTO runs (
            run_id, slug, date, version, graph_id, current_node, status,
            iteration, max_transitions, token_budget, wallclock_seconds,
            tokens_used, stopped_by, body, created_by, created_at, updated_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 0, 0, 0, '', ?, '', ?, ?)`,
		run.RunID, run.Slug, run.Date, run.Version, run.GraphID, run.CurrentNode,
		string(run.Status), run.Iteration, run.MaxTransitions, string(raw), now, now)
	if err != nil {
		t.Fatal(err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := loaded.Checks["lint"]; !got.Passed {
		t.Fatalf("legacy body checks lost: %+v", loaded.Checks)
	}
}
