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

	_, _, err := Get(t.Context(), t.TempDir(), server.URL, Options{})
	if err == nil {
		t.Fatal("a bot check answered with 200 was accepted as the page")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "bot check") {
		t.Errorf("the refusal does not name what happened: %v", err)
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

	_, _, err := Get(t.Context(), t.TempDir(), server.URL+"/blocked", Options{})
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

	_, _, err := Get(t.Context(), t.TempDir(), server.URL, Options{})
	if err == nil {
		t.Fatal("the challenge was accepted")
	}
	if strings.Contains(err.Error(), "llms.txt") {
		t.Errorf("an llms.txt that does not exist was offered: %v", err)
	}
}
