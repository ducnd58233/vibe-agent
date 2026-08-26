package workspace

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestFetchCacheSitsUnderTheStateDir(t *testing.T) {
	root := filepath.FromSlash("/w")
	cache := FetchCacheDir(root)
	if !strings.HasPrefix(cache, StateDir(root)) {
		t.Errorf("FetchCacheDir = %q, want it under %q", cache, StateDir(root))
	}
	if filepath.Base(cache) != FetchCacheDirName {
		t.Errorf("FetchCacheDir base = %q, want %q", filepath.Base(cache), FetchCacheDirName)
	}
}

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

func TestRunsSitUnderStateDirAsOwnFolder(t *testing.T) {
	root := filepath.FromSlash("/w")
	runs := RunsDir(root)
	if runs == StateDir(root) {
		t.Fatal("runs and state resolved to one directory")
	}
	if filepath.Base(runs) != RunsDirName {
		t.Errorf("RunsDir base = %q, want %q", filepath.Base(runs), RunsDirName)
	}
	want := filepath.Join(StateDir(root), RunsDirName)
	if runs != want {
		t.Errorf("RunsDir = %q, want %q", runs, want)
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
