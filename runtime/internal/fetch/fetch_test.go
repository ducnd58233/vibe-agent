package fetch

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ducnd58233/vibe-agent/runtime/internal/fetch/app/usecases"
)

const page = `<html><head><title>Guide</title><script>var x=1</script></head>
<body><nav>menu</nav><main><h1>Guide</h1><p>The content.</p></main></body></html>`

func get(t *testing.T, root, source string, options Options) (Document, bool) {
	t.Helper()
	doc, cached, err := Get(t.Context(), root, source, options)
	if err != nil {
		t.Fatalf("Get(t.Context(), %s): %v", source, err)
	}
	return doc, cached
}

func TestGetExtractsAWebPage(t *testing.T) {
	var hits int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(page))
	}))
	defer server.Close()
	root := t.TempDir()

	doc, cached := get(t, root, server.URL, Options{})
	if cached {
		t.Error("the first fetch reported a cache hit")
	}
	if doc.Title != "Guide" || !strings.Contains(doc.Text, "The content.") {
		t.Errorf("bad extraction: %+v", doc)
	}
	if strings.Contains(doc.Text, "var x=1") || strings.Contains(doc.Text, "menu") {
		t.Errorf("machinery survived: %s", doc.Text)
	}

	// The reason this package exists: the second ask costs no request at all.
	doc2, cached2 := get(t, root, server.URL, Options{})
	if !cached2 {
		t.Error("the second fetch did not hit the cache")
	}
	if hits != 1 {
		t.Errorf("the server was asked %d times; a cached source must not be requested again", hits)
	}
	if doc2.Text != doc.Text {
		t.Error("the cached text differs from what was extracted")
	}
}

func TestRefreshBypassesTheCache(t *testing.T) {
	var hits int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		_, _ = w.Write([]byte(page))
	}))
	defer server.Close()
	root := t.TempDir()

	get(t, root, server.URL, Options{})
	if _, cached := get(t, root, server.URL, Options{Refresh: true}); cached {
		t.Error("--refresh served from cache")
	}
	if hits != 2 {
		t.Errorf("server hits = %d, want 2", hits)
	}
}

func TestGetReadsALocalFile(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "notes.md")
	if err := os.WriteFile(path, []byte("# Notes\n\nPlain markdown stays as it is.\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	doc, _ := get(t, root, path, Options{})
	if !strings.Contains(doc.Text, "Plain markdown stays as it is.") {
		t.Errorf("local file not read: %+v", doc)
	}
}

func TestGetExtractsALocalHTMLFile(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "saved.html")
	if err := os.WriteFile(path, []byte(page), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	doc, _ := get(t, root, path, Options{})
	if strings.Contains(doc.Text, "var x=1") {
		t.Errorf("a local page was not extracted: %s", doc.Text)
	}
}

// Emitting the bytes of a PDF as if they were text burns a large number of
// tokens on mojibake the agent cannot detect. Refusing it outright was the first
// answer and it was too blunt: a spec PDF is an ordinary thing to want. The
// document is retrieved and named, and its bytes stay out of the context window.
func TestABinaryDocumentIsNamedAndNotPrinted(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "spec.pdf")
	if err := os.WriteFile(path, []byte("%PDF-1.7\n%\xe2\xe3\xcf\xd3\nbinary"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	doc, _, err := Get(t.Context(), root, path, Options{})
	if err != nil {
		t.Fatalf("a PDF was refused: %v", err)
	}
	if doc.Status != StatusAsset {
		t.Errorf("status = %q, want asset", doc.Status)
	}
	if !strings.Contains(doc.Text, "spec.pdf") {
		t.Errorf("the description does not name the file:\n%s", doc.Text)
	}
	if strings.Contains(doc.Text, "%PDF-1.7") {
		t.Errorf("binary content leaked into the text:\n%s", doc.Text)
	}
}

func TestGetRefusesAMissingFile(t *testing.T) {
	root := t.TempDir()
	if _, _, err := Get(t.Context(), root, filepath.Join(root, "missing.md"), Options{}); err == nil {
		t.Error("a missing file was accepted")
	}
}

// The cache is workspace state and belongs with the rest of it.
func TestTheCacheLivesInTheWorkspaceStateDirectory(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(page))
	}))
	defer server.Close()
	root := t.TempDir()
	get(t, root, server.URL, Options{})

	dir := CacheDir(root)
	if !strings.Contains(filepath.ToSlash(dir), "/.agent-state/") {
		t.Errorf("cache directory %s is outside the workspace state directory", dir)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) == 0 {
		t.Fatalf("nothing cached in %s: %v", dir, err)
	}
}

// A page larger than the cap is a download, not a document. Reading it into
// memory to extract three paragraphs is the failure this avoids.
func TestGetRefusesAnOversizeResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("x", usecases.MaxDocumentBytes+1)))
	}))
	defer server.Close()

	if _, _, err := Get(t.Context(), t.TempDir(), server.URL, Options{}); err == nil {
		t.Error("an oversize response was accepted")
	}
}

func TestClipStaysWithinBudgetAndReportsTheRemainder(t *testing.T) {
	text := strings.TrimSuffix(strings.Repeat("a line of roughly forty characters here\n", 100), "\n")

	clipped, omitted := Clip(text, 50)

	if EstimateTokens(clipped) > 50 {
		t.Errorf("clip is %d tokens, over the 50 asked for", EstimateTokens(clipped))
	}
	if omitted == 0 {
		t.Error("a clipped document reported nothing omitted, so a reader would assume it saw everything")
	}
	if strings.HasSuffix(clipped, "of") || strings.HasSuffix(clipped, "roughl") {
		t.Errorf("clip cut mid-line: %q", clipped[max(0, len(clipped)-40):])
	}
}

func TestClipLeavesAShortDocumentWhole(t *testing.T) {
	text := "one\ntwo\nthree"
	clipped, omitted := Clip(text, 1000)
	if clipped != text || omitted != 0 {
		t.Errorf("a document inside the budget was altered: %q omitted=%d", clipped, omitted)
	}
}

func TestGetReportsTheHTTPStatusItWasRefusedWith(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	_, _, err := Get(t.Context(), t.TempDir(), server.URL, Options{})
	if err == nil || !strings.Contains(err.Error(), "404") {
		t.Errorf("a 404 was not reported as one: %v", err)
	}
}

func TestClipFromTailKeepsTheEnd(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 200; i++ {
		fmt.Fprintf(&b, "line-%03d\n", i)
	}
	text := b.String()
	clipped, omitted := ClipFrom(text, 20, "tail")
	if !strings.Contains(clipped, "line-199") {
		t.Fatalf("tail clip missing last line: %q", clipped)
	}
	if strings.Contains(clipped, "line-000") {
		t.Fatalf("tail clip kept the start: %q", clipped)
	}
	if omitted <= 0 {
		t.Fatal("omittedLines should count dropped leading lines")
	}
	head, _ := ClipFrom(text, 20, "head")
	legacy, _ := Clip(text, 20)
	if head != legacy {
		t.Fatal("ClipFrom head must match Clip")
	}
}
