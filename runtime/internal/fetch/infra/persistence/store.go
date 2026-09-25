// Package persistence keeps what has already been fetched, and puts a non-text
// source where a file reader can open it.
//
// It implements the Store and Assets ports declared in the fetch app package.
// Everything it writes lives under the workspace state directory, beside the
// memory database, because it is derived from a source and belongs to the
// checkout that asked for it.
package persistence

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/fetch/domain"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/infra/database"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/workspace"
)

// CacheLife is how long a fetched document is served without asking again.
//
// Documentation changes, and a cache with no expiry answers a question about
// today's API from whenever the page was first read, with nothing in the output
// to say so. A day is long enough that a session never pays twice and short
// enough that a stale answer is a day stale rather than a quarter.
const CacheLife = 24 * time.Hour

// CacheDir is where extracted documents are kept.
//
// Beside the memory database and the repository index, under the same gitignored
// state directory, for the same reason: this is derived from a source and
// belongs to the checkout that asked for it, not to the machine.
func CacheDir(workspaceRoot string) string {
	return workspace.FetchCacheDir(workspaceRoot)
}

// MaxAssetBytes is the largest non-text source this will retrieve.
//
// Higher than MaxSourceBytes because none of it enters a context window: an
// asset is written to disk and named. The cap remains because a fetch that
// quietly downloads a gigabyte is a surprise rather than a feature.
const MaxAssetBytes = 128 << 20

// AssetDir is where retrieved binaries are put.
func AssetDir(workspaceRoot string) string {
	return filepath.Join(CacheDir(workspaceRoot), "assets")
}

// assetExtension picks the suffix a saved file should carry.
//
// The source's own suffix first, because it is what the publisher chose and what
// a person will recognise. Only where there is none does this ask the mime
// database, which returns several spellings for some types and any of them
// opens correctly.
func assetExtension(source, contentType string) string {
	if suffix := filepath.Ext(source); suffix != "" && len(suffix) <= 6 &&
		!strings.ContainsAny(suffix, "/?#") {
		return strings.ToLower(suffix)
	}
	if suffixes, err := mime.ExtensionsByType(contentType); err == nil && len(suffixes) > 0 {
		return suffixes[0]
	}
	return ".bin"
}

// saveAsset writes retrieved bytes beside the rest of the fetch cache.
//
// What enters a context window is three facts: what the thing is, how big it is,
// and where it went. The host already has a reader that handles images and PDFs
// properly; this package's job is to put the file where that reader can reach it
// and then say so.
func saveAsset(workspaceRoot, source, contentType string, raw []byte) (domain.Document, error) {
	sum := sha256.Sum256([]byte(source))
	path := filepath.Join(AssetDir(workspaceRoot),
		hex.EncodeToString(sum[:])[:16]+assetExtension(source, contentType))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return domain.Document{}, fmt.Errorf("create asset directory: %w", err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		return domain.Document{}, fmt.Errorf("save %s: %w", source, err)
	}
	return describeAsset(source, path, contentType, len(raw)), nil
}

func describeAsset(source, path, contentType string, size int) domain.Document {
	kind := contentType
	if kind == "" {
		kind = strings.TrimPrefix(filepath.Ext(path), ".")
	}
	return domain.Document{
		Source:        source,
		Status:        domain.StatusAsset,
		LocalPath:     path,
		ContentType:   contentType,
		OriginalBytes: size,
		Text: fmt.Sprintf(
			"%s is %s, %d bytes, saved to %s. It is not text, so its bytes are "+
				"deliberately not printed: open the path with your own file reader, "+
				"which handles images and PDFs, or hand it to a tool that does.",
			source, kind, size, path),
	}
}

// retrieveAsset puts a non-text source where a file reader can open it.
//
// A local file is already somewhere openable, so it is named rather than copied:
// duplicating it would double the bytes on disk and give the reader two paths
// for one file.
func retrieveAsset(workspaceRoot, source, contentType string, raw []byte) (domain.Document, error) {
	if !strings.HasPrefix(source, "http://") && !strings.HasPrefix(source, "https://") {
		absolute, err := filepath.Abs(source)
		if err != nil {
			absolute = source
		}
		return describeAsset(source, absolute, contentType, len(raw)), nil
	}
	return saveAsset(workspaceRoot, source, contentType, raw)
}

// cacheKey addresses a document by its source.
//
// Hashed rather than slugged, because a URL contains characters no filesystem
// accepts and two URLs differing only in a query string must not collide.
func cacheKey(source string) string {
	sum := sha256.Sum256([]byte(source))
	return hex.EncodeToString(sum[:])
}

// fetchCacheTable is the row store for fetched document text and metadata.
// Large binaries never land here - AssetDir stays files, referenced by path
// from a Document's LocalPath field, exactly as before this table existed.
const fetchCacheTable = "fetch_cache"

func createFetchCacheTable(ctx context.Context, db *sql.DB) error {
	return database.CreateTableWithProvenance(ctx, db, fetchCacheTable, `
        key        TEXT PRIMARY KEY,
        source     TEXT NOT NULL,
        fetched_at TEXT NOT NULL,
        body       TEXT NOT NULL`)
}

// openFetchCache opens the shared database and makes sure this store's table
// exists. A fresh connection per call rather than one held by Store: the Store
// port has no Close, and a workspace-scoped SQLite file is cheap to open.
func openFetchCache(ctx context.Context, workspaceRoot string) (*sql.DB, error) {
	path := workspace.MemoryDBPath(workspaceRoot)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, fmt.Errorf("create state directory: %w", err)
	}
	db, err := database.Open(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("open fetch cache: %w", err)
	}
	if err := createFetchCacheTable(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

// Store implements the Store port over the shared workspace database.
type Store struct {
	// Root is the workspace whose cache this is.
	Root string
}

// Load returns a stored document if one is present and still fresh. Any
// failure - no row, a closed workspace, a corrupt body - is a cache miss:
// a cold cache is the normal state the Store port already documents, not an
// error a caller needs to see.
func (s Store) Load(source string) (domain.Document, bool) {
	ctx := context.Background()
	db, err := openFetchCache(ctx, s.Root)
	if err != nil {
		return domain.Document{}, false
	}
	defer func() { _ = db.Close() }()

	var fetchedAtRaw, body string
	err = db.QueryRowContext(ctx, `SELECT fetched_at, body FROM fetch_cache WHERE key = ?`, cacheKey(source)).
		Scan(&fetchedAtRaw, &body)
	if err != nil {
		return domain.Document{}, false
	}
	fetchedAt, err := time.Parse(time.RFC3339, fetchedAtRaw)
	if err != nil || time.Since(fetchedAt) > CacheLife {
		return domain.Document{}, false
	}
	var doc domain.Document
	if json.Unmarshal([]byte(body), &doc) != nil {
		return domain.Document{}, false
	}
	return doc, true
}

// Save records a document for the next ask, replacing any prior entry for the
// same source rather than accumulating one row per fetch.
func (s Store) Save(source string, doc domain.Document) error {
	ctx := context.Background()
	db, err := openFetchCache(ctx, s.Root)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	body, err := json.Marshal(doc)
	if err != nil {
		return fmt.Errorf("encode document: %w", err)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	key := cacheKey(source)
	_, err = db.ExecContext(ctx, `
        INSERT INTO fetch_cache (key, source, fetched_at, body, created_by, created_at, updated_at)
        VALUES (?, ?, ?, ?, '', ?, ?)
        ON CONFLICT(key) DO UPDATE SET
            fetched_at = excluded.fetched_at,
            body       = excluded.body,
            updated_at = excluded.updated_at`,
		key, source, now, string(body), now, now)
	if err != nil {
		return fmt.Errorf("save fetch cache entry: %w", err)
	}
	return nil
}

// Assets implements the Assets port.
type Assets struct {
	// Root is the workspace the asset directory belongs to.
	Root string
}

// IsText reports whether a media type holds text an extractor can read.
func (Assets) IsText(contentType string) bool { return domain.IsText(contentType) }

// Keep puts a non-text source where a file reader can open it.
func (a Assets) Keep(source, contentType string, raw []byte) (domain.Document, error) {
	return retrieveAsset(a.Root, source, contentType, raw)
}

// legacyCached is the pre-database on-disk shape one cache entry used to have.
type legacyCached struct {
	Document  domain.Document `json:"document"`
	FetchedAt time.Time       `json:"fetchedAt"`
}

// Backfill moves every existing file-based cache entry into fetch_cache, then
// removes the files it moved. Safe to run more than once: a workspace with
// nothing left under CacheDir migrates zero entries rather than erroring.
//
// assets/ is a sibling of the cache files this reads, not one of them - it
// holds binaries Backfill never touches (SPEC A4).
func Backfill(ctx context.Context, workspaceRoot string) (int, error) {
	dir := CacheDir(workspaceRoot)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("read %s: %w", dir, err)
	}

	db, err := openFetchCache(ctx, workspaceRoot)
	if err != nil {
		return 0, err
	}
	defer func() { _ = db.Close() }()

	migrated := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		raw, err := os.ReadFile(filepath.Clean(path))
		if err != nil {
			return migrated, fmt.Errorf("read %s: %w", path, err)
		}
		var stored legacyCached
		if err := json.Unmarshal(raw, &stored); err != nil {
			return migrated, fmt.Errorf("parse %s: %w", path, err)
		}
		body, err := json.Marshal(stored.Document)
		if err != nil {
			return migrated, fmt.Errorf("encode %s: %w", path, err)
		}
		key := strings.TrimSuffix(entry.Name(), ".json")
		fetchedAt := stored.FetchedAt.UTC().Format(time.RFC3339)
		if _, err := db.ExecContext(ctx, `
            INSERT INTO fetch_cache (key, source, fetched_at, body, created_by, created_at, updated_at)
            VALUES (?, ?, ?, ?, '', ?, ?)
            ON CONFLICT(key) DO UPDATE SET
                fetched_at = excluded.fetched_at,
                body       = excluded.body,
                updated_at = excluded.updated_at`,
			key, stored.Document.Source, fetchedAt, string(body), fetchedAt, fetchedAt); err != nil {
			return migrated, fmt.Errorf("insert %s: %w", path, err)
		}
		if err := os.Remove(path); err != nil {
			return migrated, fmt.Errorf("remove %s: %w", path, err)
		}
		migrated++
	}
	return migrated, nil
}
