package fetch

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// A Cloudflare interstitial. The dangerous part is the 200: nothing in the
// status line says this is not the page, and it carries enough prose to look
// like content.
//
// Signatures per Cloudflare's own challenge-page documentation and the
// third-party writeups of them: the title, the "verifying you are human" line,
// and the challenge-platform script path.
const challengePage = `<!DOCTYPE html><html><head><title>Just a moment...</title>
<script src="/cdn-cgi/challenge-platform/h/b/orchestrate/chl_page/v1"></script></head>
<body><div class="main-wrapper"><h1>example.com</h1>
<p>Verifying you are human. This may take a few seconds.</p>
<p>example.com needs to review the security of your connection before proceeding.</p>
</div></body></html>`

func TestAChallengePageIsRefusedRatherThanReturned(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(challengePage))
	}))
	defer server.Close()

	_, _, err := Get(t.TempDir(), server.URL, Options{})
	if err == nil {
		t.Fatal("a bot check answered with 200 was accepted as the page")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "bot check") {
		t.Errorf("the refusal does not name what happened: %v", err)
	}
}

// The detector has to be narrow. A page about Cloudflare, or one that uses the
// word "verifying", is still a page.
func TestOrdinaryProseIsNotMistakenForAChallenge(t *testing.T) {
	cases := []string{
		`<html><head><title>Verifying webhook signatures</title></head><body><main>
<p>Verifying you are human is what a CAPTCHA does. This guide explains how to
verify a webhook signature instead, with a shared secret and a timing-safe
comparison. Cloudflare is one provider that signs its webhooks this way.</p>
</main></body></html>`,
		`<html><head><title>Just a moment in time: date handling</title></head><body><main>
<p>Timestamps are the usual source of flaky tests. This page covers clock
skew, monotonic time, and why wall-clock comparisons drift.</p></main></body></html>`,
	}
	for _, page := range cases {
		if reason := challengeReason([]byte(page), ExtractHTML([]byte(page))); reason != "" {
			t.Errorf("ordinary prose flagged as %q:\n%s", reason, page[:80])
		}
	}
}

// A shell with a nav and a footer is not empty, and it is not the page either.
// Reporting it as content is how an agent answers from a menu.
func TestAnUnderRenderedShellIsReported(t *testing.T) {
	var page strings.Builder
	page.WriteString(`<html><head><title>Dashboard</title></head><body>`)
	page.WriteString(`<div id="root"><nav><a href="/a">Home</a><a href="/b">Docs</a></nav></div>`)
	for range 40 {
		page.WriteString(`<script src="/static/chunk.js"></script>`)
		page.WriteString(`<script>window.__DATA__={"a":1,"b":2,"c":3,"d":4,"e":5};</script>`)
	}
	page.WriteString(`</body></html>`)

	doc := ExtractHTML([]byte(page.String()))
	if doc.Status != StatusThin {
		t.Errorf("status = %q, want %q for a page that is markup and scripts with no prose",
			doc.Status, StatusThin)
	}
}

func TestARealPageIsStatusOK(t *testing.T) {
	var page strings.Builder
	page.WriteString(`<html><head><title>Guide</title></head><body><main><h1>Guide</h1>`)
	for range 12 {
		page.WriteString(`<p>A paragraph of real documentation prose that a reader came here for, ` +
			`long enough to be worth the request that fetched it.</p>`)
	}
	page.WriteString(`</main></body></html>`)

	doc := ExtractHTML([]byte(page.String()))
	if doc.Status != StatusOK {
		t.Errorf("status = %q, want ok:\n%.200s", doc.Status, doc.Text)
	}
}

func TestAnEmptyPageIsStatusEmpty(t *testing.T) {
	doc := ExtractHTML([]byte(
		`<html><head><title>App</title></head><body><div id="root"></div></body></html>`))
	if doc.Status != StatusEmpty {
		t.Errorf("status = %q, want %q", doc.Status, StatusEmpty)
	}
	if !doc.Empty {
		t.Error("Empty and Status disagree")
	}
}

// The actionable half. Telling an agent to "use a browser" is a dead end when it
// has no browser; naming a text route that exists on the same host is not.
func TestAnAlternativeIsNamedWhenTheOriginPublishesOne(t *testing.T) {
	// A catch-all that answers everything with the wall, which is the shape that
	// makes a status-only probe lie: /llms-full.txt returns 200 here and is not
	// a text route.
	var mux http.ServeMux
	mux.HandleFunc("/llms.txt", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("# Docs\n\n- [Guide](/guide)\n"))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(challengePage))
	})
	server := httptest.NewServer(&mux)
	defer server.Close()

	_, _, err := Get(t.TempDir(), server.URL+"/blocked", Options{})
	if err == nil {
		t.Fatal("the challenge was accepted")
	}
	if !strings.Contains(err.Error(), "/llms.txt") {
		t.Errorf("a published llms.txt on the same origin was not offered: %v", err)
	}
}

func TestNoAlternativeIsInventedWhenTheOriginHasNone(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "llms.txt") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(challengePage))
	}))
	defer server.Close()

	_, _, err := Get(t.TempDir(), server.URL, Options{})
	if err == nil {
		t.Fatal("the challenge was accepted")
	}
	if strings.Contains(err.Error(), "llms.txt") {
		t.Errorf("an llms.txt that does not exist was offered: %v", err)
	}
}
