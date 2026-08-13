package fetch

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A one-pixel PNG. Real bytes, so the content-type sniff and the size report are
// measuring something rather than agreeing with a fixture.
var pngBytes = []byte{
	0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a,
	0x00, 0x00, 0x00, 0x0d, 'I', 'H', 'D', 'R',
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4, 0x89,
}

// An illustration is a legitimate thing to want from a page. Refusing it left
// the runtime unable to help at all, when the useful move is the same one it
// makes everywhere else: keep the payload out of context and hand back a handle.
func TestAnImageIsSavedAndHandedBackAsAPath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(pngBytes)
	}))
	defer server.Close()
	root := t.TempDir()

	doc, _, err := Get(root, server.URL+"/assets/diagram.png", Options{})
	if err != nil {
		t.Fatalf("an image was refused: %v", err)
	}
	if doc.Status != StatusAsset {
		t.Errorf("status = %q, want %q", doc.Status, StatusAsset)
	}
	if doc.LocalPath == "" {
		t.Fatal("no path was returned, so nothing can open the image")
	}
	saved, err := os.ReadFile(doc.LocalPath)
	if err != nil {
		t.Fatalf("read saved asset: %v", err)
	}
	if len(saved) != len(pngBytes) {
		t.Errorf("saved %d bytes, fetched %d", len(saved), len(pngBytes))
	}
	if filepath.Ext(doc.LocalPath) != ".png" {
		t.Errorf("saved as %s; the extension is what tells a reader how to open it",
			doc.LocalPath)
	}
	// The bytes must not be in the text. That is the whole point: an image in
	// context is tens of thousands of tokens of nothing a model can read.
	if strings.Contains(doc.Text, "PNG") || len(doc.Text) > 400 {
		t.Errorf("binary content leaked into the text: %.120q", doc.Text)
	}
	if !strings.Contains(doc.Text, doc.LocalPath) {
		t.Errorf("the text does not name the path a reader has to open:\n%s", doc.Text)
	}
}

// The type comes from the response, not the URL. A CDN serving an image from a
// path with no extension is ordinary.
func TestAnExtensionlessAssetIsNamedFromItsContentType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(pngBytes)
	}))
	defer server.Close()

	doc, _, err := Get(t.TempDir(), server.URL+"/cdn/8f2a1c", Options{})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if doc.Status != StatusAsset {
		t.Fatalf("status = %q, want asset", doc.Status)
	}
	if filepath.Ext(doc.LocalPath) != ".png" {
		t.Errorf("saved as %s, want a .png named from image/png", doc.LocalPath)
	}
}

func TestALocalBinaryFileIsReportedWithoutBeingCopied(t *testing.T) {
	root := t.TempDir()
	original := filepath.Join(root, "shot.png")
	if err := os.WriteFile(original, pngBytes, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	doc, _, err := Get(root, original, Options{})
	if err != nil {
		t.Fatalf("a local image was refused: %v", err)
	}
	if doc.Status != StatusAsset {
		t.Errorf("status = %q, want asset", doc.Status)
	}
	// It is already on disk. Copying it into the cache would double the bytes
	// and give the reader two paths for one file.
	if doc.LocalPath != original {
		t.Errorf("local path = %s, want the file itself at %s", doc.LocalPath, original)
	}
}

// Media elements carry the URL a reader came for, and the markdown converter has
// no rule for them, so without help a video is silently absent from the page.
func TestMediaSourcesSurviveExtraction(t *testing.T) {
	body := []byte(`<html><body><main>
<p>A paragraph long enough that readability keeps this as an article rather than
falling back, which is a different code path and would not prove the same thing
about how media elements are handled during conversion to markdown.</p>
<video src="/media/demo.mp4" controls></video>
<audio><source src="/media/talk.mp3" type="audio/mpeg"></audio>
</main></body></html>`)

	text := ExtractHTMLFrom(body, "https://example.com/docs/page").Text

	for _, want := range []string{"https://example.com/media/demo.mp4", "https://example.com/media/talk.mp3"} {
		if !strings.Contains(text, want) {
			t.Errorf("media source %s is missing, so it cannot be fetched:\n%s", want, text)
		}
	}
}
