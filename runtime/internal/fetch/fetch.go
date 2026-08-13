package fetch

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/memory"
)

// MaxSourceBytes is the largest source this will read.
//
// Past this it is a download, not a document, and pulling it into memory to
// extract three paragraphs is the failure the cap exists to prevent.
const MaxSourceBytes = 8 << 20

// DefaultTimeout bounds a request. A hook or a command that hangs on a slow host
// is worse than one that reports a timeout, because the session waits on it.
const DefaultTimeout = 20 * time.Second

// probeTimeout bounds the look for an alternative route. It runs only after a
// failure, so it must be short: a dead end reported quickly beats a better
// suggestion that arrives after the session has moved on.
const probeTimeout = 5 * time.Second

// userAgent identifies this honestly. A blank agent is refused by more hosts
// than a named one, and pretending to be a browser to get past that is not this
// tool's business.
const userAgent = "vibe-agent/fetch"

// CacheLife is how long a fetched document is served without asking again.
//
// Documentation changes, and a cache with no expiry answers a question about
// today's API from whenever the page was first read, with nothing in the output
// to say so. A day is long enough that a session never pays twice and short
// enough that a stale answer is a day stale rather than a quarter.
const CacheLife = 24 * time.Hour

// Options adjusts one retrieval.
type Options struct {
	// Refresh ignores any cached copy and asks the source again.
	Refresh bool
	Timeout time.Duration
}

// CacheDir is where extracted documents are kept.
//
// Beside the memory database and the repository index, under the same gitignored
// state directory, for the same reason: this is derived from a source and
// belongs to the checkout that asked for it, not to the machine.
func CacheDir(workspaceRoot string) string {
	return filepath.Join(workspaceRoot, memory.StateDirName, "fetch")
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

// mediaType is the type alone, without charset or boundary parameters.
func mediaType(contentType string) string {
	parsed, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	}
	return parsed
}

// isTextType reports whether a media type holds text this package can extract.
//
// The categories come from the media type itself rather than a list of formats.
// A list is wrong the moment something new appears, and something new always
// appears: this asks what the type says it is. text/* is text by definition, and
// json, xml, and javascript are text carried under application/* for historical
// reasons rather than technical ones.
func isTextType(contentType string) bool {
	media := mediaType(contentType)
	if strings.HasPrefix(media, "text/") {
		return true
	}
	return strings.Contains(media, "json") || strings.Contains(media, "xml") ||
		strings.Contains(media, "javascript") || strings.Contains(media, "ecmascript")
}

// detectType decides what a source is, preferring what the server said.
//
// Where the server said nothing, http.DetectContentType implements the WHATWG
// sniffing algorithm the browsers use, which is both more accurate and more
// maintained than any extension table this package could carry. A path ending
// .png that returns HTML is an error page, and a CDN serving an image from a
// path with no extension is ordinary; sniffing the bytes is right about both.
func detectType(source, declared string, raw []byte) string {
	if media := mediaType(declared); media != "" && media != "application/octet-stream" {
		return media
	}
	if suffix := filepath.Ext(source); suffix != "" {
		if byExtension := mediaType(mime.TypeByExtension(suffix)); byExtension != "" {
			return byExtension
		}
	}
	return mediaType(http.DetectContentType(raw))
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
func saveAsset(workspaceRoot, source, contentType string, raw []byte) (Document, error) {
	sum := sha256.Sum256([]byte(source))
	path := filepath.Join(AssetDir(workspaceRoot),
		hex.EncodeToString(sum[:])[:16]+assetExtension(source, contentType))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return Document{}, fmt.Errorf("create asset directory: %w", err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		return Document{}, fmt.Errorf("save %s: %w", source, err)
	}
	return describeAsset(source, path, contentType, len(raw)), nil
}

func describeAsset(source, path, contentType string, size int) Document {
	kind := contentType
	if kind == "" {
		kind = strings.TrimPrefix(filepath.Ext(path), ".")
	}
	return Document{
		Source:        source,
		Status:        StatusAsset,
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
	Document  Document  `json:"document"`
	FetchedAt time.Time `json:"fetchedAt"`
}

// Get retrieves a source, extracts its readable text, and caches the result.
//
// The second return value reports whether the cache answered, so a caller can
// say so rather than implying a request happened.
func Get(ctx context.Context, workspaceRoot, source string, options Options) (Document, bool, error) {
	if strings.TrimSpace(source) == "" {
		return Document{}, false, fmt.Errorf("fetch needs a URL or a file path")
	}
	path := cachePath(workspaceRoot, source)
	if !options.Refresh {
		if stored, ok := readCache(path); ok {
			return stored, true, nil
		}
	}

	raw, contentType, err := read(ctx, source, options)
	if err != nil {
		return Document{}, false, err
	}

	// Not text: keep the bytes out of context and hand back a path. Refusing was
	// the old behaviour and it left the runtime unable to help with an
	// illustration, a screenshot, a spec PDF, or a demo video, all of which are
	// ordinary things to want from a page.
	if !isTextType(contentType) {
		doc, err := retrieveAsset(workspaceRoot, source, contentType, raw)
		if err != nil {
			return Document{}, false, err
		}
		if err := writeCache(path, doc); err != nil {
			return doc, false, nil
		}
		return doc, false, nil
	}

	// Text is held to the smaller cap. The larger one exists because an asset
	// never enters a context window; a document does, and one this size is a
	// dump rather than a page.
	if len(raw) > MaxSourceBytes {
		return Document{}, false, fmt.Errorf(
			"%s is %d bytes of text, over the %d-byte limit for a document",
			source, len(raw), MaxSourceBytes)
	}

	var doc Document
	if mediaType(contentType) == "text/html" || mediaType(contentType) == "application/xhtml+xml" {
		doc = ExtractHTMLFrom(raw, source)
		// A bot wall answers 200 with a page that reads like prose, so nothing
		// upstream of here can tell it from the article. Refusing is the whole
		// response: there is no bypass in this package, and letting an agent
		// read "Verifying you are human" as the answer is the failure that
		// cannot be recovered from.
		if reason := challengeReason(raw, doc); reason != "" {
			probeCtx, cancel := context.WithTimeout(ctx, probeTimeout)
			defer cancel()
			return Document{}, false, fmt.Errorf("fetch %s: %s",
				source, Advise(probeCtx, source, "answered with a bot check, matching "+reason))
		}
	} else {
		text := tidyPlain(string(raw))
		doc = Document{Text: text, OriginalBytes: len(raw), Status: classify(text, len(raw)),
			Empty: text == ""}
	}
	doc.Source = source

	if err := writeCache(path, doc); err != nil {
		// A cache that cannot be written is a lost saving, not a failed fetch.
		// The document is already in hand and refusing it here would turn a disk
		// problem into a missing answer.
		return doc, false, nil
	}
	return doc, false, nil
}

// CharsPerToken is the ratio budgets are estimated with. An approximation, and
// named as one: the consequence of it being off is a clip slightly early or
// late, not a wrong document.
const CharsPerToken = 4

// EstimateTokens approximates what a string costs to read.
func EstimateTokens(text string) int {
	return (len(text) + CharsPerToken - 1) / CharsPerToken
}

// Clip cuts text to a token budget at a line boundary, and reports what it left.
//
// The remainder is a number rather than nothing, because this is the "summary
// plus a handle" shape the research settles on: an agent told that 400 lines
// remain asks for them, and one told nothing assumes it read the page. Cutting
// at a line keeps the last thing it sees a whole thought.
func Clip(text string, budget int) (clipped string, omittedLines int) {
	if budget <= 0 || EstimateTokens(text) <= budget {
		return text, 0
	}
	limit := budget * CharsPerToken
	head := text[:limit]
	if cut := strings.LastIndexByte(head, '\n'); cut > 0 {
		head = head[:cut]
	}
	return head, strings.Count(text[len(head):], "\n") + 1
}

// read returns the source's bytes and the media type they turned out to be.
func read(ctx context.Context, source string, options Options) (raw []byte, contentType string, err error) {
	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		return request(ctx, source, options)
	}

	info, err := os.Stat(source)
	if err != nil {
		return nil, "", fmt.Errorf("read %s: %w", source, err)
	}
	if info.Size() > MaxAssetBytes {
		return nil, "", fmt.Errorf("%s is %d bytes, over the %d-byte limit",
			source, info.Size(), MaxAssetBytes)
	}
	raw, err = os.ReadFile(filepath.Clean(source))
	if err != nil {
		return nil, "", fmt.Errorf("read %s: %w", source, err)
	}
	return raw, detectType(source, "", raw), nil
}

// retrieveAsset puts a non-text source where a file reader can open it.
//
// A local file is already somewhere openable, so it is named rather than copied:
// duplicating it would double the bytes on disk and give the reader two paths
// for one file.
func retrieveAsset(workspaceRoot, source, contentType string, raw []byte) (Document, error) {
	if !strings.HasPrefix(source, "http://") && !strings.HasPrefix(source, "https://") {
		absolute, err := filepath.Abs(source)
		if err != nil {
			absolute = source
		}
		return describeAsset(source, absolute, contentType, len(raw)), nil
	}
	return saveAsset(workspaceRoot, source, contentType, raw)
}

// request fetches a URL.
func request(ctx context.Context, url string, options Options) ([]byte, string, error) {
	timeout := options.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	client := &http.Client{Timeout: timeout}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", fmt.Errorf("request %s: %w", url, err)
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html,text/plain,text/markdown,application/json;q=0.9,*/*;q=0.1")

	resp, err := client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("fetch %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// 403 and 429 from a plain GET are usually a bot check rather than a
		// missing page, and the distinction changes what to do next. Naming it
		// is the whole of the help offered: working around a bot check is not
		// this tool's business, and a browser-driving tool is the honest route
		// to a page whose owner wants a browser.
		if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests {
			probeCtx, cancel := context.WithTimeout(ctx, probeTimeout)
			defer cancel()
			return nil, "", fmt.Errorf("fetch %s: %s", url, Advise(probeCtx, url, fmt.Sprintf(
				"%d %s, which for a plain request usually means a bot check or a rate limit",
				resp.StatusCode, http.StatusText(resp.StatusCode))))
		}
		return nil, "", fmt.Errorf("fetch %s: %d %s", url, resp.StatusCode,
			http.StatusText(resp.StatusCode))
	}

	// One byte past the cap, so a body that exactly fills it is still an error
	// rather than a silent truncation reported as a whole document.
	raw, err := io.ReadAll(io.LimitReader(resp.Body, MaxAssetBytes+1))
	if err != nil {
		return nil, "", fmt.Errorf("read %s: %w", url, err)
	}
	if len(raw) > MaxAssetBytes {
		return nil, "", fmt.Errorf("%s returned more than %d bytes", url, MaxAssetBytes)
	}

	// A host that sends no content type is common, and sniffing the bytes is the
	// same thing a browser does about it.
	return raw, detectType(url, resp.Header.Get("Content-Type"), raw), nil
}

// tidyPlain normalises text that needed no extraction.
//
// Trailing whitespace and runs of blank lines are tokens that carry nothing, and
// a file written on Windows would otherwise spend one per line on a carriage
// return.
func tidyPlain(s string) string {
	lines := strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
	kept := make([]string, 0, len(lines))
	blank := false
	for _, line := range lines {
		trimmed := strings.TrimRight(line, " \t")
		if trimmed == "" {
			if !blank && len(kept) > 0 {
				kept = append(kept, "")
			}
			blank = true
			continue
		}
		blank = false
		kept = append(kept, trimmed)
	}
	return strings.TrimSpace(strings.Join(kept, "\n"))
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
func readCache(path string) (Document, bool) {
	raw, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return Document{}, false
	}
	var stored cached
	if json.Unmarshal(raw, &stored) != nil {
		return Document{}, false
	}
	if time.Since(stored.FetchedAt) > CacheLife {
		return Document{}, false
	}
	return stored.Document, true
}

func writeCache(path string, doc Document) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	raw, err := json.Marshal(cached{Document: doc, FetchedAt: time.Now().UTC()})
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o600)
}
