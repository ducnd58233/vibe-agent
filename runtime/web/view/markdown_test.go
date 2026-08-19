package view

import (
	"strings"
	"testing"
)

func TestRenderMarkdownFormatsCommonMark(t *testing.T) {
	html := string(RenderMarkdown("# Title\n\n- item\n\n```go\nfmt.Println(1)\n```"))
	if !strings.Contains(html, "<h1") {
		t.Fatalf("expected heading, got %s", html)
	}
	if !strings.Contains(html, "<li>") {
		t.Fatalf("expected list, got %s", html)
	}
	if !strings.Contains(html, "<pre>") || !strings.Contains(html, "<code") {
		t.Fatalf("expected fenced code, got %s", html)
	}
}

func TestRenderMarkdownDoesNotExecuteRawHTML(t *testing.T) {
	html := string(RenderMarkdown("hello <script>alert(1)</script>"))
	if strings.Contains(html, "<script>") {
		t.Fatalf("raw script must not survive, got %s", html)
	}
	if !strings.Contains(html, "alert(1)") {
		t.Fatalf("escaped text should remain visible, got %s", html)
	}
}

func TestRenderMarkdownDropsJavascriptHrefs(t *testing.T) {
	html := string(RenderMarkdown("[x](javascript:alert(1))"))
	if strings.Contains(strings.ToLower(html), "javascript:") {
		t.Fatalf("javascript href must not survive, got %s", html)
	}
}

func TestRenderMarkdownLinksFilePathsInCode(t *testing.T) {
	html := string(RenderMarkdown("See `docs/harness-improvement/SPEC.md` for details."))
	if !strings.Contains(html, `data-file-view="docs/harness-improvement/SPEC.md"`) {
		t.Fatalf("file path in backtick code should become a clickable link, got %s", html)
	}
	if !strings.Contains(html, "file-link") {
		t.Fatalf("missing file-link class, got %s", html)
	}
}

func TestRenderMarkdownDoesNotLinkNonFilePaths(t *testing.T) {
	html := string(RenderMarkdown("Use `go test ./...` to run tests."))
	if strings.Contains(html, "data-file-view") {
		t.Fatalf("non-file code should not become a file link, got %s", html)
	}
}
