package workspace

import (
	"path/filepath"
	"strings"
	"testing"
)

// Derived state has one home. The WebFetch cache used to sit under .claude/,
// which made it a per-host directory for something no host owns.
func TestSDDCacheSitsUnderTheStateDir(t *testing.T) {
	root := filepath.FromSlash("/w")
	cache := SDDCacheDir(root)

	if !strings.HasPrefix(cache, StateDir(root)) {
		t.Errorf("SDDCacheDir = %q, want it under %q", cache, StateDir(root))
	}
	if strings.Contains(cache, ".claude") || strings.Contains(cache, ".cursor") {
		t.Errorf("SDDCacheDir = %q, want no host directory in it", cache)
	}
}

// Run evidence and rebuildable cache are different things, and the split is the
// reason RunsDirName exists separately. A change that collapsed them would put
// a person's evidence somewhere a tool feels free to delete.
func TestRunsAndStateAreDifferentDirectories(t *testing.T) {
	root := filepath.FromSlash("/w")
	if StateDir(root) == RunsDir(root) {
		t.Fatal("state and runs resolved to one directory")
	}
}

func TestVersionedDirsSitUnderDateSlugVersion(t *testing.T) {
	root := filepath.FromSlash("/w")
	docs := DocsDirAt(root, "2026-08-21", "demo", 2)
	run := RunDirAt(root, "2026-08-21", "demo", 2)
	wantDocs := filepath.Join(root, DocsDirName, "2026-08-21", "demo", "2")
	wantRun := filepath.Join(root, RunsDirName, "2026-08-21", "demo", "2")
	if docs != wantDocs {
		t.Errorf("DocsDirAt = %q, want %q", docs, wantDocs)
	}
	if run != wantRun {
		t.Errorf("RunDirAt = %q, want %q", run, wantRun)
	}
	if !strings.HasPrefix(RunIndexDir(root), StateDir(root)) {
		t.Errorf("RunIndexDir = %q, want under state", RunIndexDir(root))
	}
}

func TestDocsArtifactDatedBasename(t *testing.T) {
	got, err := DocsArtifact("SPEC", "2026-08-21")
	if err != nil || got != "SPEC-2026-08-21.md" {
		t.Fatalf("DocsArtifact(SPEC) = %q, %v", got, err)
	}
	got, err = DocsArtifact("tasks", "2026-08-21")
	if err != nil || got != "tasks-2026-08-21.json" {
		t.Fatalf("DocsArtifact(tasks) = %q, %v", got, err)
	}
	if _, err := DocsArtifact("PLAN", "bad"); err == nil {
		t.Fatal("expected error for bad date")
	}
}
