package persistence

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/runpath"
)

// indexedPaths allocates a revision so ManifestPath/EventLogPath resolve under
// .agent-state/runs/<date>/<slug>/<version>/.
func indexedPaths(t *testing.T, root, slug string) (manifest, events string) {
	t.Helper()
	now := time.Date(2026, 7, 29, 10, 0, 0, 0, time.UTC)
	if _, err := runpath.Allocate(root, slug, now); err != nil {
		t.Fatalf("Allocate: %v", err)
	}
	manifest = ManifestPath(root, slug)
	events = EventLogPath(root, slug)
	if manifest == "" || events == "" {
		t.Fatal("indexed paths resolved empty")
	}
	if filepath.Dir(manifest) != filepath.Dir(events) {
		t.Fatalf("manifest and events dirs differ: %q vs %q", manifest, events)
	}
	return manifest, events
}
