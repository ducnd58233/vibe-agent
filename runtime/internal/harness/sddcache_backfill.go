package harness

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/infra/database"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/workspace"
)

// sddCacheTable is the row store the Python hooks read and write directly
// through stdlib sqlite3 (sdd-cache-pre.py, sdd-cache-post.py). Its DDL is
// duplicated in both languages on purpose (SPEC A8): Python cannot import
// this constant, and this string is the contract both sides hold to.
const sddCacheTable = "sdd_cache"

func createSDDCacheTable(ctx context.Context, db *sql.DB) error {
	return database.CreateTableWithProvenance(ctx, db, sddCacheTable, `
        key           TEXT PRIMARY KEY,
        url           TEXT NOT NULL,
        prompt        TEXT NOT NULL DEFAULT '',
        etag          TEXT NOT NULL DEFAULT '',
        last_modified TEXT NOT NULL DEFAULT '',
        content       TEXT NOT NULL,
        fetched_at    INTEGER NOT NULL`)
}

// legacySDDCacheEntry is the pre-database on-disk shape one cache entry used
// to have, written by the Python hooks before they moved to sqlite3.
type legacySDDCacheEntry struct {
	URL          string `json:"url"`
	Prompt       string `json:"prompt"`
	ETag         string `json:"etag"`
	LastModified string `json:"last_modified"`
	Content      string `json:"content"`
	FetchedAt    int64  `json:"fetched_at"`
}

// SDDCacheBackfill moves every existing file-based sdd-cache entry into
// sdd_cache, then removes the files it moved. Safe to run more than once: a
// workspace with nothing left under SDDCacheDir migrates zero entries rather
// than erroring.
func SDDCacheBackfill(ctx context.Context, workspaceRoot string) (int, error) {
	dir := workspace.SDDCacheDir(workspaceRoot)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("read %s: %w", dir, err)
	}

	path := workspace.MemoryDBPath(workspaceRoot)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return 0, fmt.Errorf("create state directory: %w", err)
	}
	db, err := database.Open(ctx, path)
	if err != nil {
		return 0, fmt.Errorf("open sdd-cache: %w", err)
	}
	defer func() { _ = db.Close() }()
	if err := createSDDCacheTable(ctx, db); err != nil {
		return 0, err
	}

	migrated := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		filePath := filepath.Join(dir, entry.Name())
		raw, err := os.ReadFile(filepath.Clean(filePath))
		if err != nil {
			return migrated, fmt.Errorf("read %s: %w", filePath, err)
		}
		var stored legacySDDCacheEntry
		if err := json.Unmarshal(raw, &stored); err != nil {
			return migrated, fmt.Errorf("parse %s: %w", filePath, err)
		}
		key := strings.TrimSuffix(entry.Name(), ".json")
		now := time.Now().UTC().Format(time.RFC3339)
		if _, err := db.ExecContext(ctx, `
            INSERT INTO sdd_cache (key, url, prompt, etag, last_modified, content, fetched_at, created_by, created_at, updated_at)
            VALUES (?, ?, ?, ?, ?, ?, ?, '', ?, ?)
            ON CONFLICT(key) DO UPDATE SET
                prompt        = excluded.prompt,
                etag          = excluded.etag,
                last_modified = excluded.last_modified,
                content       = excluded.content,
                fetched_at    = excluded.fetched_at,
                updated_at    = excluded.updated_at`,
			key, stored.URL, stored.Prompt, stored.ETag, stored.LastModified, stored.Content, stored.FetchedAt, now, now); err != nil {
			return migrated, fmt.Errorf("insert %s: %w", filePath, err)
		}
		if err := os.Remove(filePath); err != nil {
			return migrated, fmt.Errorf("remove %s: %w", filePath, err)
		}
		migrated++
	}

	// Unlike fetch's cache dir, sdd-cache has no assets/ subdirectory to keep -
	// once every file is a row, the directory itself is done. Remove ignores
	// ENOTEMPTY/ENOENT silently: a stray file a later run adds, or a directory
	// removed by a previous run, are both fine outcomes, not failures.
	_ = os.Remove(dir)
	return migrated, nil
}
