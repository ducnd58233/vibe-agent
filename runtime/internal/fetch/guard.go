package fetch

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// Status is what a caller needs to know before trusting the text.
//
// A field rather than prose, because the caller that most needs this is a
// program deciding whether to answer from the document. Prose on stderr is for
// the person; this is for the branch.
type Status string

const (
	// StatusOK means the page carried prose and nothing suggests it is not the
	// page that was asked for.
	StatusOK Status = "ok"
	// StatusEmpty means the page parsed and held no readable text at all. Almost
	// always a client-rendered shell.
	StatusEmpty Status = "empty"
	// StatusThin means there is text, and too little of it relative to the
	// markup and script around it to be the content. A navigation bar and a
	// footer with nothing between them lands here.
	StatusThin Status = "thin"
	// StatusAsset means the source was not text and is now a file on disk. Text
	// holds the path and the description; LocalPath holds the path alone.
	StatusAsset Status = "asset"
)

// thinTextRatio is the share of a page's bytes that has to survive extraction
// before the result is treated as the content.
//
// Derived from the shape of the failure rather than tuned: a real article keeps
// a few percent of a heavy page, and a shell keeps a fraction of one. The
// threshold sits between, and the consequence of it being wrong in either
// direction is a warning, never a refusal or a dropped document.
const thinTextRatio = 0.005

// minRealChars is the floor below which a ratio means nothing. A small page that
// is genuinely short must not be called thin because its markup is efficient.
const minRealChars = 400

// classify decides what a caller may do with an extracted document.
func classify(text string, originalBytes int) Status {
	if strings.TrimSpace(text) == "" {
		return StatusEmpty
	}
	if len(text) >= minRealChars {
		return StatusOK
	}
	if originalBytes > 0 && float64(len(text))/float64(originalBytes) < thinTextRatio {
		return StatusThin
	}
	return StatusOK
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
func challengeReason(raw []byte, doc Document) string {
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
		resp.Body.Close()
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
