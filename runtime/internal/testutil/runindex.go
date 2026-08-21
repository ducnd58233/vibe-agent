package testutil

import (
	"os"
	"testing"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/runpath"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/workspace"
)

// EnsureRunIndex writes a run-index entry so ManifestPath and RunDir resolve
// under .agent-state/runs/<date>/<slug>/<version>/. Idempotent when the slug
// is already indexed.
func EnsureRunIndex(t testing.TB, workspaceRoot, slug string) {
	t.Helper()
	if _, err := runpath.Resolve(workspaceRoot, slug); err == nil {
		return
	}
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	entry, err := runpath.Allocate(workspaceRoot, slug, now)
	if err != nil {
		t.Fatalf("Allocate run index for %q: %v", slug, err)
	}
	dir := workspace.RunDirAt(workspaceRoot, entry.Date, entry.Slug, entry.Version)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}
}
