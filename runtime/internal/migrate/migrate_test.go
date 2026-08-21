package migrate_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/migrate"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/runpath"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/workspace"
)

func TestDryRunDoesNotWrite(t *testing.T) {
	root := t.TempDir()
	docs := filepath.Join(root, "docs", "demo")
	if err := os.MkdirAll(docs, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(docs, "SPEC.md"), []byte("---\nslug: demo\ndate: 2026-08-21\nversion: 1\n---\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	plans, err := migrate.PlanWorkspace(root, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(plans) != 1 {
		t.Fatalf("plans = %d, want 1", len(plans))
	}
	if err := migrate.Apply(root, plans, migrate.Options{DryRun: true, Now: now}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(docs); err != nil {
		t.Fatal("dry-run moved the source")
	}
}

func TestApplyMovesAndRenames(t *testing.T) {
	root := t.TempDir()
	docs := filepath.Join(root, "docs", "demo")
	tmp := filepath.Join(root, "tmp", "demo")
	for _, dir := range []string{docs, tmp} {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(docs, "SPEC.md"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(docs, "tasks.json"), []byte(`{"schemaVersion":1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "manifest.json"), []byte(`{
	  "schemaVersion":1,"runId":"r","graphId":"g","slug":"demo","goal":"g",
	  "currentNode":"build","status":"running","iteration":1,"maxTransitions":50,
	  "createdAt":"2026-08-20T10:00:00Z","updatedAt":"2026-08-20T10:00:00Z"
	}`), 0o600); err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	plans, err := migrate.PlanWorkspace(root, now)
	if err != nil {
		t.Fatal(err)
	}
	listing := migrate.FormatPlan(plans)
	if !strings.Contains(listing, "SPEC.md -> SPEC-2026-08-20.md") && !strings.Contains(listing, "rename SPEC.md") {
		// date comes from manifest createdAt
		if !strings.Contains(listing, "SPEC-2026-08-20.md") {
			t.Fatalf("plan missing dated rename:\n%s", listing)
		}
	}
	if err := migrate.Apply(root, plans, migrate.Options{Now: now}); err != nil {
		t.Fatal(err)
	}

	wantDocs := workspace.DocsDirAt(root, "2026-08-20", "demo", 1)
	if _, err := os.Stat(filepath.Join(wantDocs, "SPEC-2026-08-20.md")); err != nil {
		t.Fatalf("dated SPEC missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(wantDocs, "tasks-2026-08-20.json")); err != nil {
		t.Fatalf("dated tasks missing: %v", err)
	}
	wantRuns := workspace.RunDirAt(root, "2026-08-20", "demo", 1)
	if _, err := os.Stat(filepath.Join(wantRuns, "manifest.json")); err != nil {
		t.Fatalf("run evidence missing under .agent-state/runs: %v", err)
	}
	if _, err := os.Stat(tmp); !os.IsNotExist(err) {
		t.Fatal("legacy flat tmp/demo should have moved")
	}

	entry, err := runpath.Resolve(root, "demo")
	if err != nil {
		t.Fatal(err)
	}
	if entry.Date != "2026-08-20" || entry.Version != 1 {
		t.Fatalf("index = %+v", entry)
	}

	// Re-run is a no-op / skip.
	plans2, err := migrate.PlanWorkspace(root, now)
	if err != nil {
		t.Fatal(err)
	}
	for _, plan := range plans2 {
		if plan.Slug == "demo" && plan.SkipReason == "" {
			t.Fatalf("second plan tried to move demo again: %+v", plan)
		}
	}
}

func TestApplyMovesVersionedTmpIntoAgentStateRuns(t *testing.T) {
	root := t.TempDir()
	from := filepath.Join(root, "tmp", "2026-08-20", "demo", "2")
	if err := os.MkdirAll(from, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(from, "manifest.json"), []byte(`{"schemaVersion":1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	plans, err := migrate.PlanWorkspace(root, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := migrate.Apply(root, plans, migrate.Options{Now: now}); err != nil {
		t.Fatal(err)
	}
	want := workspace.RunDirAt(root, "2026-08-20", "demo", 2)
	if _, err := os.Stat(filepath.Join(want, "manifest.json")); err != nil {
		t.Fatalf("versioned migrate missing: %v", err)
	}
}
