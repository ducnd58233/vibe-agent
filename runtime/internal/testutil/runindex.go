package testutil

import (
	"testing"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/runpath"
)

// EnsureRunIndex allocates a revision so ManifestPath and RunDir resolve
// under .agent-state/runs/<date>/<slug>/<version>/. Idempotent when the slug
// is already known. Name kept for call-site stability after run-index files
// were removed.
func EnsureRunIndex(t testing.TB, workspaceRoot, slug string) {
	t.Helper()
	if _, err := runpath.Resolve(workspaceRoot, slug); err == nil {
		return
	}
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	if _, err := runpath.Allocate(workspaceRoot, slug, now); err != nil {
		t.Fatalf("Allocate run for %q: %v", slug, err)
	}
}
