package persistence

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/fetch/domain"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/infra/database"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/workspace"
)

func TestStoreSavesAndLoadsThroughTheDatabase(t *testing.T) {
	root := t.TempDir()
	store := Store{Root: root}
	doc := domain.Document{Source: "https://example.com/a", Title: "A", Text: "hello", Status: domain.StatusOK}

	if err := store.Save(doc.Source, doc); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, ok := store.Load(doc.Source)
	if !ok {
		t.Fatal("a saved document was not found")
	}
	if got.Text != "hello" || got.Title != "A" {
		t.Errorf("got %+v, want the saved document back", got)
	}
}

func TestStoreLoadMissesOnANeverSavedKey(t *testing.T) {
	store := Store{Root: t.TempDir()}
	if _, ok := store.Load("https://example.com/never-fetched"); ok {
		t.Error("a cold cache reported a hit")
	}
}

func TestStoreLoadMissesOnAnExpiredEntry(t *testing.T) {
	root := t.TempDir()
	store := Store{Root: root}
	source := "https://example.com/old"

	ctx := context.Background()
	if err := os.MkdirAll(filepath.Dir(workspace.MemoryDBPath(root)), 0o750); err != nil {
		t.Fatal(err)
	}
	db, err := database.Open(ctx, workspace.MemoryDBPath(root))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close: %v", err)
		}
	})
	body, err := json.Marshal(domain.Document{Source: source, Text: "stale"})
	if err != nil {
		t.Fatal(err)
	}
	stale := time.Now().Add(-CacheLife * 2).UTC().Format(time.RFC3339)
	if _, err := db.ExecContext(ctx, `
        INSERT INTO fetch_cache (key, source, fetched_at, body, created_by, created_at, updated_at)
        VALUES (?, ?, ?, ?, '', ?, ?)`,
		cacheKey(source), source, stale, string(body), stale, stale); err != nil {
		t.Fatal(err)
	}

	if _, ok := store.Load(source); ok {
		t.Error("an expired entry was served as a hit")
	}
}

func TestSaveOverwritesAnExistingEntryRatherThanDuplicating(t *testing.T) {
	root := t.TempDir()
	store := Store{Root: root}
	source := "https://example.com/b"

	if err := store.Save(source, domain.Document{Source: source, Text: "v1"}); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(source, domain.Document{Source: source, Text: "v2"}); err != nil {
		t.Fatal(err)
	}

	got, ok := store.Load(source)
	if !ok || got.Text != "v2" {
		t.Errorf("got %+v, ok=%v; want the second save to win", got, ok)
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
	var count int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM fetch_cache WHERE key = ?`, cacheKey(source)).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("fetch_cache has %d rows for one key, want 1", count)
	}
}

func TestFetchCacheTableHasProvenanceColumns(t *testing.T) {
	root := t.TempDir()
	if err := (Store{Root: root}).Save("https://example.com/c", domain.Document{Source: "https://example.com/c", Text: "x"}); err != nil {
		t.Fatal(err)
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
	rows, err := db.QueryContext(ctx, `PRAGMA table_info(fetch_cache)`)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	present := map[string]bool{}
	for rows.Next() {
		var (
			index, notNull, primary int
			name, columnType        string
			preset                  any
		)
		if err := rows.Scan(&index, &name, &columnType, &notNull, &preset, &primary); err != nil {
			t.Fatal(err)
		}
		present[name] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"key", "source", "fetched_at", "body", "created_by", "reviewed_by_agents", "created_at", "updated_at"} {
		if !present[want] {
			t.Errorf("fetch_cache has no column %q", want)
		}
	}
}

// Backfill is the one-time move: every existing cache file becomes a row,
// verified present, before the files are deleted.
func TestBackfillMovesEveryExistingCacheFileThenDeletesIt(t *testing.T) {
	root := t.TempDir()
	dir := CacheDir(root)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	// assets/ is a sibling directory Backfill must leave alone.
	if err := os.MkdirAll(filepath.Join(dir, "assets"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "assets", "keepme.bin"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	sources := []string{"https://example.com/x", "https://example.com/y"}
	for _, source := range sources {
		raw, err := json.Marshal(legacyCached{
			Document:  domain.Document{Source: source, Text: "legacy " + source},
			FetchedAt: time.Now().UTC(),
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, cacheKey(source)+".json"), raw, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	migrated, err := Backfill(context.Background(), root)
	if err != nil {
		t.Fatalf("backfill: %v", err)
	}
	if migrated != len(sources) {
		t.Errorf("migrated %d entries, want %d", migrated, len(sources))
	}

	store := Store{Root: root}
	for _, source := range sources {
		got, ok := store.Load(source)
		if !ok || got.Text != "legacy "+source {
			t.Errorf("Load(%s) = %+v, %v; want the migrated row", source, got, ok)
		}
	}

	for _, source := range sources {
		if _, err := os.Stat(filepath.Join(dir, cacheKey(source)+".json")); err == nil {
			t.Errorf("%s still exists after backfill", source)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "assets", "keepme.bin")); err != nil {
		t.Errorf("backfill touched assets/: %v", err)
	}
}

// A second Backfill call, with nothing left to migrate, is a no-op rather
// than an error - the command this backs is meant to be safe to re-run.
func TestBackfillIsSafeToRunTwice(t *testing.T) {
	root := t.TempDir()
	for range 2 {
		if _, err := Backfill(context.Background(), root); err != nil {
			t.Fatalf("backfill: %v", err)
		}
	}
}

// Assets are unaffected by the DB swap: large binaries stay files, not rows.
func TestAssetsStillWriteToDisk(t *testing.T) {
	root := t.TempDir()
	doc, err := (Assets{Root: root}).Keep("https://example.com/d.png", "image/png", []byte{0x89, 'P', 'N', 'G'})
	if err != nil {
		t.Fatal(err)
	}
	if doc.LocalPath == "" {
		t.Fatal("an asset was not written to a local path")
	}
	if _, err := os.Stat(doc.LocalPath); err != nil {
		t.Errorf("asset file missing: %v", err)
	}
}
