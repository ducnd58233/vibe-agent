package persistence

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/runpath"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/workspace"
)

// indexedPaths allocates a revision so ManifestPath/EventLogPath resolve under
// .agent-state/runs/<date>/<slug>/<version>/. It also plants a leftover
// run-index file so Save/Append still dual-write files (production stays
// SQL-only once that leftover is gone).
func indexedPaths(t *testing.T, root, slug string) (manifest, events string) {
	t.Helper()
	now := time.Date(2026, 7, 29, 10, 0, 0, 0, time.UTC)
	entry, err := runpath.Allocate(root, slug, now)
	if err != nil {
		t.Fatalf("Allocate: %v", err)
	}
	writeLegacyIndex(t, root, entry.Slug, entry.Date, entry.Version)
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

func writeLegacyIndex(t *testing.T, root, slug, date string, version int) {
	t.Helper()
	dir := workspace.RunIndexDir(root)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	body := fmt.Sprintf(`{"schemaVersion":1,"slug":%q,"date":%q,"version":%d}`+"\n", slug, date, version)
	if err := os.WriteFile(runpath.IndexPath(root, slug), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}
