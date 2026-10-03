package legacy

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/workspace"
)

// fetchCacheSource: one JSON file per fetched document under
// .agent-state/fetch/, before fetch_cache. assets/ beside them holds binaries
// that are still files today and is never touched.
var fetchCacheSource = Source{
	Name: "fetch cache",
	Into: "fetch_cache",
	Pending: func(root string) bool {
		files, _ := jsonFiles(workspace.FetchCacheDir(root))
		return len(files) > 0
	},
	Migrate: migrateFetchCache,
}

func migrateFetchCache(ctx context.Context, root string) (int, error) {
	files, err := jsonFiles(workspace.FetchCacheDir(root))
	if err != nil || len(files) == 0 {
		return 0, err
	}
	db, err := openDB(ctx, root)
	if err != nil {
		return 0, err
	}
	defer func() { _ = db.Close() }()

	moved := 0
	for _, path := range files {
		raw, err := os.ReadFile(filepath.Clean(path))
		if err != nil {
			return moved, fmt.Errorf("read %s: %w", path, err)
		}
		var stored struct {
			Document  json.RawMessage `json:"document"`
			FetchedAt time.Time       `json:"fetchedAt"`
		}
		if err := json.Unmarshal(raw, &stored); err != nil {
			return moved, setAside(path, fmt.Errorf("parse %s: %w", path, err))
		}
		var source struct {
			Source string `json:"source"`
		}
		_ = json.Unmarshal(stored.Document, &source)
		key := strings.TrimSuffix(filepath.Base(path), ".json")
		fetchedAt := stored.FetchedAt.UTC().Format(time.RFC3339)
		if _, err := db.ExecContext(ctx, `
            INSERT INTO fetch_cache (key, source, fetched_at, body, created_by, created_at, updated_at)
            VALUES (?, ?, ?, ?, '', ?, ?)
            ON CONFLICT(key) DO UPDATE SET
                fetched_at = excluded.fetched_at,
                body       = excluded.body,
                updated_at = excluded.updated_at`,
			key, source.Source, fetchedAt, string(stored.Document), fetchedAt, fetchedAt); err != nil {
			return moved, fmt.Errorf("insert %s: %w", path, err)
		}
		if err := removeMoved(path); err != nil {
			return moved, err
		}
		moved++
	}
	return moved, nil
}

// sddCacheSource: one JSON file per cached page under .agent-state/sdd-cache/,
// written by the Python hooks before they moved to sqlite3.
var sddCacheSource = Source{
	Name: "sdd-cache",
	Into: "sdd_cache",
	Pending: func(root string) bool {
		files, _ := jsonFiles(workspace.SDDCacheDir(root))
		return len(files) > 0
	},
	Migrate: migrateSDDCache,
}

func migrateSDDCache(ctx context.Context, root string) (int, error) {
	dir := workspace.SDDCacheDir(root)
	files, err := jsonFiles(dir)
	if err != nil || len(files) == 0 {
		return 0, err
	}
	db, err := openDB(ctx, root)
	if err != nil {
		return 0, err
	}
	defer func() { _ = db.Close() }()

	moved := 0
	for _, path := range files {
		raw, err := os.ReadFile(filepath.Clean(path))
		if err != nil {
			return moved, fmt.Errorf("read %s: %w", path, err)
		}
		var stored struct {
			URL          string `json:"url"`
			Prompt       string `json:"prompt"`
			ETag         string `json:"etag"`
			LastModified string `json:"last_modified"`
			Content      string `json:"content"`
			FetchedAt    int64  `json:"fetched_at"`
		}
		if err := json.Unmarshal(raw, &stored); err != nil {
			return moved, setAside(path, fmt.Errorf("parse %s: %w", path, err))
		}
		key := strings.TrimSuffix(filepath.Base(path), ".json")
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
			return moved, fmt.Errorf("insert %s: %w", path, err)
		}
		if err := removeMoved(path); err != nil {
			return moved, err
		}
		moved++
	}
	_ = os.Remove(dir)
	return moved, nil
}
