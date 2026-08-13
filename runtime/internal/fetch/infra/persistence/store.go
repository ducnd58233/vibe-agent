// Package persistence keeps what has already been fetched, and puts a non-text
// source where a file reader can open it.
//
// It implements the Store and Assets ports declared in the fetch app package.
// Everything it writes lives under the workspace state directory, beside the
// memory database, because it is derived from a source and belongs to the
// checkout that asked for it.
package persistence

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/fetch/domain"
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
	return filepath.Join(workspace.StateDir(workspaceRoot), "fetch")
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

// cached is one stored document plus when it was retrieved.
type cached struct {
	Document  domain.Document `json:"document"`
	FetchedAt time.Time       `json:"fetchedAt"`
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

// cachePath addresses a document by its source.
//
// Hashed rather than slugged, because a URL contains characters no filesystem
// accepts and two URLs differing only in a query string must not collide.
func cachePath(workspaceRoot, source string) string {
	sum := sha256.Sum256([]byte(source))
	return filepath.Join(CacheDir(workspaceRoot), hex.EncodeToString(sum[:])+".json")
}

// readCache returns a stored document if it is still within CacheLife.
//
// An expired entry is ignored rather than deleted: the next successful fetch
// overwrites it, and a read that deletes on a slow network leaves the caller
// with neither the fresh copy nor the old one.
func readCache(path string) (domain.Document, bool) {
	raw, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return domain.Document{}, false
	}
	var stored cached
	if json.Unmarshal(raw, &stored) != nil {
		return domain.Document{}, false
	}
	if time.Since(stored.FetchedAt) > CacheLife {
		return domain.Document{}, false
	}
	return stored.Document, true
}

func writeCache(path string, doc domain.Document) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	raw, err := json.Marshal(cached{Document: doc, FetchedAt: time.Now().UTC()})
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o600)
}

// Store implements the Store port over the workspace cache directory.
type Store struct {
	// Root is the workspace whose cache this is.
	Root string
}

// Load returns a stored document if one is present and still fresh.
func (s Store) Load(source string) (domain.Document, bool) {
	return readCache(cachePath(s.Root, source))
}

// Save records a document for the next ask.
func (s Store) Save(source string, doc domain.Document) error {
	return writeCache(cachePath(s.Root, source), doc)
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
