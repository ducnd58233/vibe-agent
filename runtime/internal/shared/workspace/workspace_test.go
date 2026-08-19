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
