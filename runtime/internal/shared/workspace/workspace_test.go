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

// Runs live under .agent-state but in their own subdirectory so caches are not
// treated as evidence.
func TestRunsSitUnderStateDirAsOwnFolder(t *testing.T) {
	root := filepath.FromSlash("/w")
	runs := RunsDir(root)
	if runs == StateDir(root) {
		t.Fatal("runs and state resolved to one directory")
	}
	if !strings.HasPrefix(runs, StateDir(root)+string(filepath.Separator)) && runs != filepath.Join(StateDir(root), RunsDirName) {
		t.Errorf("RunsDir = %q, want under %q", runs, StateDir(root))
	}
	if filepath.Base(runs) != RunsDirName {
		t.Errorf("RunsDir base = %q, want %q", filepath.Base(runs), RunsDirName)
	}
}

func TestLegacyRunsStayAtWorkspaceRootTmp(t *testing.T) {
	root := filepath.FromSlash("/w")
	legacy := LegacyRunsDir(root)
	if legacy != filepath.Join(root, LegacyRunsDirName) {
		t.Errorf("LegacyRunsDir = %q", legacy)
	}
	if strings.Contains(legacy, StateDirName) {
		t.Errorf("legacy tmp must not sit under %s", StateDirName)
	}
}

func TestVersionedDirsSitUnderDateSlugVersion(t *testing.T) {
	root := filepath.FromSlash("/w")
	docs := DocsDirAt(root, "2026-08-21", "demo", 2)
	run := RunDirAt(root, "2026-08-21", "demo", 2)
	wantDocs := filepath.Join(root, DocsDirName, "2026-08-21", "demo", "2")
	wantRun := filepath.Join(root, StateDirName, RunsDirName, "2026-08-21", "demo", "2")
	if docs != wantDocs {
		t.Errorf("DocsDirAt = %q, want %q", docs, wantDocs)
	}
	if run != wantRun {
		t.Errorf("RunDirAt = %q, want %q", run, wantRun)
	}
}

func TestCheckRevisionRejectsBadInputs(t *testing.T) {
	if err := CheckRevision("nope", 1); err == nil {
		t.Fatal("bad date accepted")
	}
	if err := CheckRevision("2026-08-21", 0); err == nil {
		t.Fatal("version 0 accepted")
	}
}
