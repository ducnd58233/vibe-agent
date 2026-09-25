package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/harness/infra/persistence"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/workspace"
)

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

	db, err := persistence.OpenSDDCache(ctx, workspaceRoot)
	if err != nil {
		return 0, fmt.Errorf("open sdd-cache: %w", err)
	}
	defer func() { _ = db.Close() }()

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

	_ = os.Remove(dir)
	return migrated, nil
}
