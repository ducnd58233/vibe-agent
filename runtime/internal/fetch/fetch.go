package fetch

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
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

// binaryFormats are the document types this cannot read.
//
// Named rather than attempted. Emitting the bytes of a PDF as though they were
// text costs a large number of tokens on mojibake and the agent has no way to
// tell that is what happened, so a refusal that names the format is the more
// useful answer. Converting them needs a real extractor, which is a dependency
// this module does not have.
var binaryFormats = map[string]string{
	".pdf": "pdf", ".docx": "docx", ".doc": "doc", ".xlsx": "xlsx",
	".pptx": "pptx", ".zip": "zip", ".png": "png", ".jpg": "jpg",
	".jpeg": "jpeg", ".gif": "gif", ".webp": "webp", ".mp4": "mp4",
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
func Get(workspaceRoot, source string, options Options) (Document, bool, error) {
	if strings.TrimSpace(source) == "" {
		return Document{}, false, fmt.Errorf("fetch needs a URL or a file path")
	}
	if format := binaryFormats[strings.ToLower(filepath.Ext(source))]; format != "" {
		return Document{}, false, fmt.Errorf(
			"%s is %s, which this cannot read as text; convert it first", source, format)
	}

	path := cachePath(workspaceRoot, source)
	if !options.Refresh {
		if stored, ok := readCache(path); ok {
			return stored, true, nil
		}
	}

	raw, isHTML, err := read(source, options)
	if err != nil {
		return Document{}, false, err
	}

	var doc Document
	if isHTML {
		doc = ExtractHTML(raw)
	} else {
		doc = Document{Text: tidyPlain(string(raw)), OriginalBytes: len(raw)}
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

// read returns the source's bytes and whether they are HTML.
func read(source string, options Options) (raw []byte, isHTML bool, err error) {
	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		return request(source, options)
	}

	info, err := os.Stat(source)
	if err != nil {
		return nil, false, fmt.Errorf("read %s: %w", source, err)
	}
	if info.Size() > MaxSourceBytes {
		return nil, false, fmt.Errorf("%s is %d bytes, over the %d-byte limit",
			source, info.Size(), MaxSourceBytes)
	}
	raw, err = os.ReadFile(source)
	if err != nil {
		return nil, false, fmt.Errorf("read %s: %w", source, err)
	}
	extension := strings.ToLower(filepath.Ext(source))
	return raw, extension == ".html" || extension == ".htm", nil
}

// request fetches a URL.
func request(url string, options Options) ([]byte, bool, error) {
	timeout := options.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	client := &http.Client{Timeout: timeout}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, false, fmt.Errorf("request %s: %w", url, err)
	}
	// Identify honestly. A blank agent gets refused by more hosts than a named
	// one, and pretending to be a browser to get past that is not this tool's
	// business.
	req.Header.Set("User-Agent", "vibe-agent/fetch")
	req.Header.Set("Accept", "text/html,text/plain,text/markdown,application/json;q=0.9,*/*;q=0.1")

	resp, err := client.Do(req)
	if err != nil {
		return nil, false, fmt.Errorf("fetch %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// 403 and 429 from a plain GET are usually a bot check rather than a
		// missing page, and the distinction changes what to do next. Naming it
		// is the whole of the help offered: working around a bot check is not
		// this tool's business, and a browser-driving tool is the honest route
		// to a page whose owner wants a browser.
		if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests {
			return nil, false, fmt.Errorf(
				"fetch %s: %d %s. A plain request was refused, which usually means a bot "+
					"check or a rate limit. Read it with a browser-driving tool, or look for "+
					"a server-rendered equivalent such as a raw or API URL",
				url, resp.StatusCode, http.StatusText(resp.StatusCode))
		}
		return nil, false, fmt.Errorf("fetch %s: %d %s", url, resp.StatusCode,
			http.StatusText(resp.StatusCode))
	}

	// One byte past the cap, so a body that exactly fills it is still an error
	// rather than a silent truncation reported as a whole document.
	raw, err := io.ReadAll(io.LimitReader(resp.Body, MaxSourceBytes+1))
	if err != nil {
		return nil, false, fmt.Errorf("read %s: %w", url, err)
	}
	if len(raw) > MaxSourceBytes {
		return nil, false, fmt.Errorf("%s returned more than %d bytes", url, MaxSourceBytes)
	}

	contentType := resp.Header.Get("Content-Type")
	isHTML := strings.Contains(contentType, "text/html") ||
		strings.Contains(contentType, "application/xhtml")
	// A host that sends no content type is common, and a body that opens with a
	// doctype or an html tag says what it is well enough.
	if contentType == "" {
		head := strings.ToLower(string(raw[:min(len(raw), 256)]))
		isHTML = strings.Contains(head, "<!doctype html") || strings.Contains(head, "<html")
	}
	return raw, isHTML, nil
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

func readCache(path string) (Document, bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Document{}, false
	}
	var stored cached
	if json.Unmarshal(raw, &stored) != nil {
		return Document{}, false
	}
	return stored.Document, true
}

func writeCache(path string, doc Document) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	raw, err := json.Marshal(cached{Document: doc, FetchedAt: time.Now().UTC()})
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o644)
}
