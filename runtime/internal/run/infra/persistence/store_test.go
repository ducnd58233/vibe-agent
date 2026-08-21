package persistence

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/run/domain"
)

func TestSaveThenLoadRoundTrips(t *testing.T) {
	dir := t.TempDir()
	run := newTestRun(t)
	path, _ := indexedPaths(t, dir, run.Slug)
	run.Flags = map[string]bool{"research_required": true, "e2e_required": false}
	if err := run.SetCheck("unit", domain.Check{Passed: true, Source: domain.SourceExitCode, Ref: "events.ndjson#41", At: fixedTime()}); err != nil {
		t.Fatalf("SetCheck: %v", err)
	}
	if err := Save(path, run); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.RunID != run.RunID || loaded.Slug != run.Slug {
		t.Errorf("identity lost: got %+v", loaded)
	}
	if !loaded.Flags["research_required"] || loaded.Flags["e2e_required"] {
		t.Errorf("flags lost: %+v", loaded.Flags)
	}
	if got := loaded.Checks["unit"]; !got.Passed || got.Source != domain.SourceExitCode {
		t.Errorf("check lost: %+v", got)
	}
}

// Load is the boundary a hand-edited or older manifest crosses. It must apply
// the same provenance rule as SetCheck, not trust the file.
func TestLoadRejectsAForgedCheckSource(t *testing.T) {
	dir := t.TempDir()
	run := newTestRun(t)
	path, _ := indexedPaths(t, dir, run.Slug)
	if err := Save(path, run); err != nil {
		t.Fatalf("Save: %v", err)
	}

	raw, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	doc["checks"] = map[string]any{
		"unit": map[string]any{"passed": true, "source": "model", "at": "2026-07-29T10:00:00Z"},
	}
	forged, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if err := os.WriteFile(path, forged, 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if _, err := Load(path); err == nil {
		t.Error("Load accepted a manifest whose check claims source \"model\"")
	}
}

func TestSaveIsAtomicAndLeavesNoTempFiles(t *testing.T) {
	dir := t.TempDir()
	run := newTestRun(t)
	path, _ := indexedPaths(t, dir, run.Slug)

	for i := 0; i < 3; i++ {
		if err := Save(path, run); err != nil {
			t.Fatalf("Save %d: %v", i, err)
		}
	}

	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, entry := range entries {
		if entry.Name() != "manifest.json" {
			t.Errorf("Save left behind %q", entry.Name())
		}
	}
}

func TestSaveWritesSchemaConformantShape(t *testing.T) {
	dir := t.TempDir()
	run := newTestRun(t)
	path, _ := indexedPaths(t, dir, run.Slug)
	if err := Save(path, run); err != nil {
		t.Fatalf("Save: %v", err)
	}
	raw, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("manifest is not valid JSON: %v", err)
	}
	for _, key := range []string{
		"schemaVersion", "runId", "graphId", "slug", "goal",
		"currentNode", "status", "iteration", "maxTransitions",
		"createdAt", "updatedAt",
	} {
		if _, ok := doc[key]; !ok {
			t.Errorf("manifest is missing required key %q", key)
		}
	}
	if !strings.HasSuffix(string(raw), "\n") {
		t.Error("manifest should end with a newline")
	}
}

func TestManifestAndEventPathsSitTogether(t *testing.T) {
	root := t.TempDir()
	manifest, events := indexedPaths(t, root, "my-slug")
	if filepath.Base(manifest) != "manifest.json" {
		t.Errorf("manifest basename = %q", filepath.Base(manifest))
	}
	if filepath.Base(events) != "events.ndjson" {
		t.Errorf("events basename = %q", filepath.Base(events))
	}
}

// fixedTime is the instant every test in this package writes with, so a golden
// file and a round trip compare against the same stamp rather than against now.
func fixedTime() time.Time {
	return time.Date(2026, 7, 29, 10, 0, 0, 0, time.UTC)
}

// newTestRun is the same fresh run the golden file pins, built through the
// domain constructor so a change to what a new run contains fails here rather
// than drifting silently.
func newTestRun(t *testing.T) *domain.Run {
	t.Helper()
	run, err := domain.NewRun("loop-graph-runtime", "add a control plane", "goal-delivery", 50, fixedTime())
	if err != nil {
		t.Fatalf("NewRun: %v", err)
	}
	return run
}
