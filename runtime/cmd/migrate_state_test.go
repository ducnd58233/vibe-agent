package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	fetchpersistence "github.com/ducnd58233/vibe-agent/runtime/internal/fetch/infra/persistence"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/infra/database"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/workspace"
)

func TestMigrateStateMovesFetchCacheFiles(t *testing.T) {
	root := t.TempDir()
	dir := fetchpersistence.CacheDir(root)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(map[string]any{
		"document":  map[string]any{"source": "https://example.com/z", "text": "z"},
		"fetchedAt": time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "deadbeef.json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}

	sddDir := workspace.SDDCacheDir(root)
	if err := os.MkdirAll(sddDir, 0o750); err != nil {
		t.Fatal(err)
	}
	sddRaw, err := json.Marshal(map[string]any{
		"url": "https://example.com/sdd", "prompt": "p", "content": "c", "fetched_at": 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sddDir, "feedface.json"), sddRaw, 0o600); err != nil {
		t.Fatal(err)
	}

	if err := migrateStateCommand([]string{"--workspace", root, "--toolkit", toolkitRoot}); err != nil {
		t.Fatalf("migrate state: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "deadbeef.json")); err == nil {
		t.Error("the legacy fetch cache file still exists after migrate state")
	}
	if _, err := os.Stat(filepath.Join(sddDir, "feedface.json")); err == nil {
		t.Error("the legacy sdd-cache file still exists after migrate state")
	}

	db, err := database.Open(t.Context(), workspace.MemoryDBPath(root))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close: %v", err)
		}
	})
	var fetchCount, sddCount int
	if err := db.QueryRowContext(t.Context(), `SELECT count(*) FROM fetch_cache`).Scan(&fetchCount); err != nil {
		t.Fatal(err)
	}
	if fetchCount != 1 {
		t.Errorf("fetch_cache has %d rows, want 1", fetchCount)
	}
	if err := db.QueryRowContext(t.Context(), `SELECT count(*) FROM sdd_cache`).Scan(&sddCount); err != nil {
		t.Fatal(err)
	}
	if sddCount != 1 {
		t.Errorf("sdd_cache has %d rows, want 1", sddCount)
	}
}

func TestMigrateStateIsSafeWithNothingToMove(t *testing.T) {
	root := t.TempDir()
	if err := migrateStateCommand([]string{"--workspace", root, "--toolkit", toolkitRoot}); err != nil {
		t.Fatalf("migrate state on an empty workspace: %v", err)
	}
}

func TestMigrateCommandDispatchesToState(t *testing.T) {
	root := t.TempDir()
	if err := migrateCommand([]string{"state", "--workspace", root, "--toolkit", toolkitRoot}); err != nil {
		t.Fatalf("migrate state via dispatch: %v", err)
	}
}

func TestMigrateCommandRefusesAnUnknownSubcommand(t *testing.T) {
	if err := migrateCommand([]string{"nonsense"}); err == nil {
		t.Error("an unknown migrate subcommand was accepted")
	}
}
