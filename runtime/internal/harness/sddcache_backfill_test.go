package harness

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/infra/database"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/workspace"
)

func TestSDDCacheBackfillMovesEveryExistingFileThenDeletesIt(t *testing.T) {
	root := t.TempDir()
	dir := workspace.SDDCacheDir(root)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}

	entries := []legacySDDCacheEntry{
		{URL: "https://example.com/a", Prompt: "p1", ETag: `"e1"`, Content: "c1", FetchedAt: 111},
		{URL: "https://example.com/b", Prompt: "p2", LastModified: "Mon, 01 Jan 2026", Content: "c2", FetchedAt: 222},
	}
	keys := make([]string, len(entries))
	for i, e := range entries {
		raw, err := json.Marshal(e)
		if err != nil {
			t.Fatal(err)
		}
		key := "key" + string(rune('a'+i))
		keys[i] = key
		if err := os.WriteFile(filepath.Join(dir, key+".json"), raw, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	migrated, err := SDDCacheBackfill(context.Background(), root)
	if err != nil {
		t.Fatalf("backfill: %v", err)
	}
	if migrated != len(entries) {
		t.Errorf("migrated %d entries, want %d", migrated, len(entries))
	}

	ctx := context.Background()
	db, err := database.Open(ctx, workspace.MemoryDBPath(root))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close: %v", err)
		}
	})
	for i, key := range keys {
		var url, content string
		if err := db.QueryRowContext(ctx, `SELECT url, content FROM sdd_cache WHERE key = ?`, key).
			Scan(&url, &content); err != nil {
			t.Errorf("row %s missing: %v", key, err)
			continue
		}
		if url != entries[i].URL || content != entries[i].Content {
			t.Errorf("row %s = (%s, %s), want (%s, %s)", key, url, content, entries[i].URL, entries[i].Content)
		}
	}

	for _, key := range keys {
		if _, err := os.Stat(filepath.Join(dir, key+".json")); err == nil {
			t.Errorf("%s still exists after backfill", key)
		}
	}

	if _, err := os.Stat(dir); err == nil {
		t.Error("sdd-cache directory still exists after every entry was migrated")
	}
}

func TestSDDCacheBackfillIsSafeWithNothingToMove(t *testing.T) {
	root := t.TempDir()
	migrated, err := SDDCacheBackfill(context.Background(), root)
	if err != nil {
		t.Fatalf("backfill on an empty workspace: %v", err)
	}
	if migrated != 0 {
		t.Errorf("migrated %d entries from an empty workspace, want 0", migrated)
	}
}

// A corrupted legacy file stops the backfill rather than silently skipping
// it: os.ReadDir visits files in name order, so "corrupt" sorts before
// "valid" here, and this pins down that "valid" is never touched, its file
// never deleted, and the count reflects only what actually landed in the
// table - not a claim of progress the caller cannot trust.
func TestSDDCacheBackfillStopsOnACorruptedFileRatherThanSkippingIt(t *testing.T) {
	root := t.TempDir()
	dir := workspace.SDDCacheDir(root)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "corrupt.json"), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	valid, err := json.Marshal(legacySDDCacheEntry{URL: "https://example.com/v", Content: "v"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "valid.json"), valid, 0o600); err != nil {
		t.Fatal(err)
	}

	migrated, err := SDDCacheBackfill(context.Background(), root)
	if err == nil {
		t.Fatal("a corrupted legacy file did not stop the backfill")
	}
	if migrated != 0 {
		t.Errorf("migrated = %d, want 0 (corrupt.json sorts before valid.json)", migrated)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "corrupt.json")); statErr != nil {
		t.Error("the corrupted file was removed despite failing to migrate")
	}
	if _, statErr := os.Stat(filepath.Join(dir, "valid.json")); statErr != nil {
		t.Error("the valid file was removed even though the run reported an error")
	}
}
