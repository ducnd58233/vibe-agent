// Package httpx retrieves a source over HTTP and tells a page from a bot wall.
//
// It implements the Source and Guard ports declared in the fetch app package.
// There is no bypass here and there will not be one: working around a bot check
// is a different activity from reading documentation. What this does instead is
// name a route that works.
package httpx

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/fetch/domain"
)

// MaxDownloadBytes is the largest response this will read.
//
// Higher than a document's limit because an asset never enters a context
// window: it is written to disk and named. The cap remains because a fetch that
// quietly downloads a gigabyte is a surprise rather than a feature.
const MaxDownloadBytes = 128 << 20

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

// read returns the source's bytes and the media type they turned out to be.
// Read implements the Source port: bytes plus what they turned out to be.
func Read(ctx context.Context, source string, timeout time.Duration) (raw []byte, contentType string, err error) {
	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		return request(ctx, source, timeout)
	}

	info, err := os.Stat(source)
	if err != nil {
		return nil, "", fmt.Errorf("read %s: %w", source, err)
	}
	if info.Size() > MaxDownloadBytes {
		return nil, "", fmt.Errorf("%s is %d bytes, over the %d-byte limit",
			source, info.Size(), MaxDownloadBytes)
	}
	raw, err = os.ReadFile(filepath.Clean(source))
	if err != nil {
		return nil, "", fmt.Errorf("read %s: %w", source, err)
	}
	return raw, detectType(source, "", raw), nil
}

// request fetches a URL.
func request(ctx context.Context, url string, timeout time.Duration) ([]byte, string, error) {
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
	raw, err := io.ReadAll(io.LimitReader(resp.Body, MaxDownloadBytes+1))
	if err != nil {
		return nil, "", fmt.Errorf("read %s: %w", url, err)
	}
	if len(raw) > MaxDownloadBytes {
		return nil, "", fmt.Errorf("%s returned more than %d bytes", url, MaxDownloadBytes)
	}

	// A host that sends no content type is common, and sniffing the bytes is the
	// same thing a browser does about it.
	return raw, detectType(url, resp.Header.Get("Content-Type"), raw), nil
}

// detectType decides what a source is, preferring what the server said.
//
// Where the server said nothing, http.DetectContentType implements the sniffing
// algorithm browsers use, which is more accurate and better maintained than any
// extension table this package could carry.
func detectType(source, declared string, raw []byte) string {
	if media := domain.MediaType(declared); media != "" && media != "application/octet-stream" {
		return media
	}
	if suffix := filepath.Ext(source); suffix != "" {
		if byExtension := domain.MediaType(mime.TypeByExtension(suffix)); byExtension != "" {
			return byExtension
		}
	}
	return domain.MediaType(http.DetectContentType(raw))
}

// challengeMarkers are the phrases a bot wall puts on the page it serves instead
// of the one that was asked for.
//
// Two of them have to line up before this fires, because each is a phrase a real
// page can contain: a guide to webhook signatures says "verifying", and a page
// about dates can be titled "Just a moment". Requiring a title signature and a
// body signature together is what keeps the detector from eating documentation
// about the very thing it detects.
//
// Signatures from Cloudflare's challenge-page documentation and the public
// writeups of them.
var challengeTitles = []string{
	"just a moment",
	"attention required",
	"security check",
	"access denied",
	"checking your browser",
	"verifying you are human",
	"one more step",
	"are you a robot",
}

var challengeBodies = []string{
	"/cdn-cgi/challenge-platform",
	"challenges.cloudflare.com/turnstile",
	"enable javascript and cookies to continue",
	"needs to review the security of your connection",
	"ddos protection by",
	"performance & security by cloudflare",
	"__cf_chl",
	"px-captcha",
	"_incapsula_resource",
	"g-recaptcha",
	"h-captcha",
}

// challengeReason names the bot wall a response is, or returns "".
//
// It is not a bypass and there is no bypass here. Detecting the wall is what
// keeps an agent from reading "Verifying you are human" as the answer to its
// question, which is the failure that matters: a refusal is recoverable and a
// confident wrong answer is not.
func challengeReason(raw []byte, doc domain.Document) string {
	lowerTitle := strings.ToLower(doc.Title)
	var title string
	for _, marker := range challengeTitles {
		if strings.Contains(lowerTitle, marker) {
			title = marker
			break
		}
	}
	if title == "" {
		return ""
	}

	// The body signature is the corroboration, and script paths are the strong
	// ones because no article contains them.
	lowerBody := strings.ToLower(string(raw))
	for _, marker := range challengeBodies {
		if strings.Contains(lowerBody, marker) {
			return fmt.Sprintf("%q with %q", title, marker)
		}
	}
	return ""
}

// alternativeRoutes are the paths a site publishes for readers that are not
// browsers.
//
// llms.txt is a community convention, not a standard, with adoption around a
// tenth of documentation sites and rising: Anthropic, Cursor and Vercel publish
// one. It is worth a probe precisely when the normal route has failed, because
// it is served as text, needs no JavaScript, and is usually not behind the wall
// that blocked the page.
var alternativeRoutes = []string{"/llms-full.txt", "/llms.txt"}

// findAlternative probes the origin for a text route and returns its URL, or "".
//
// Only ever called on a failure, so the cost is paid by a request that already
// went wrong. A probe that 404s returns nothing rather than a guess: offering a
// URL that does not exist sends the caller to a second dead end and spends its
// trust.
func findAlternative(ctx context.Context, pageURL string) string {
	parsed, err := url.Parse(pageURL)
	if err != nil || parsed.Host == "" {
		return ""
	}
	client := &http.Client{Timeout: probeTimeout}
	for _, route := range alternativeRoutes {
		candidate := parsed.Scheme + "://" + parsed.Host + route
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, candidate, nil)
		if err != nil {
			continue
		}
		req.Header.Set("User-Agent", userAgent)
		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, probeSniff))
		_ = resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			continue
		}
		// A 200 is not enough. A single-page app answers every path with its
		// shell, so a status check alone would advertise an llms.txt on any
		// origin that has none, which is a second dead end wearing the clothes
		// of a fix. The file is markdown by convention, so anything that
		// announces or looks like HTML is the catch-all, not the route.
		if looksLikeHTML(resp.Header.Get("Content-Type"), body) {
			continue
		}
		return candidate
	}
	return ""
}

// probeSniff is how much of a probe response to read before judging it. Enough
// for a doctype and a first heading, and no more: this is a discard.
const probeSniff = 512

func looksLikeHTML(contentType string, body []byte) bool {
	if strings.Contains(strings.ToLower(contentType), "text/html") {
		return true
	}
	head := strings.ToLower(strings.TrimSpace(string(body)))
	return strings.HasPrefix(head, "<!doctype html") || strings.HasPrefix(head, "<html")
}

// Advise turns a dead end into a next step.
//
// No AI crawler runs JavaScript, this one included, so "try again" is never the
// answer. The two real routes are a tool that drives a browser and a text
// endpoint the site publishes itself, and this names whichever of them applies.
func Advise(ctx context.Context, pageURL, problem string) string {
	advice := problem + ". Read it with a browser-driving tool"
	if alternative := findAlternative(ctx, pageURL); alternative != "" {
		return advice + fmt.Sprintf(", or fetch %s, which this origin publishes as text", alternative)
	}
	return advice + ", or look for a server-rendered equivalent such as a raw or API URL"
}

// Client implements the Source and Guard ports over HTTP and the local
// filesystem.
type Client struct {
	// Timeout bounds one request. Zero means DefaultTimeout.
	Timeout time.Duration
}

// Read retrieves a source and reports what it turned out to be.
func (c Client) Read(ctx context.Context, source string) ([]byte, string, error) {
	return Read(ctx, source, c.Timeout)
}

// ChallengeReason names the bot wall a response is, or returns "".
func (c Client) ChallengeReason(raw []byte, doc domain.Document) string {
	return challengeReason(raw, doc)
}

// Advise turns a dead end into a next step.
func (c Client) Advise(ctx context.Context, pageURL, problem string) string {
	return Advise(ctx, pageURL, problem)
}
