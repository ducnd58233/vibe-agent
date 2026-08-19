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

func TestRenderMarkdownLinksBareFilenameWithKnownExtension(t *testing.T) {
	html := string(RenderMarkdown("See `loop-and-graph-engineering.md` in references."))
	if !strings.Contains(html, `data-file-view="loop-and-graph-engineering.md"`) {
		t.Fatalf("bare filename with linguist-known extension should link, got %s", html)
	}
}

func TestRenderMarkdownStylesTaskListItems(t *testing.T) {
	html := string(RenderMarkdown("- [ ] first\n- [x] second"))
	if !strings.Contains(html, `class="task-list-item"`) {
		t.Fatalf("task list item class missing, got %s", html)
	}
	if !strings.Contains(html, `class="task-list-item-checkbox"`) {
		t.Fatalf("task list checkbox class missing, got %s", html)
	}
}

func TestRenderMarkdownLinksLocalMarkdownAnchors(t *testing.T) {
	html := string(RenderMarkdown("See [spec](docs/harness-improvement/SPEC.md) for details."))
	if !strings.Contains(html, `data-file-view="docs/harness-improvement/SPEC.md"`) {
		t.Fatalf("local markdown anchor should open file viewer, got %s", html)
	}
	if strings.Contains(html, `href="docs/harness-improvement/SPEC.md"`) {
		t.Fatalf("local href should be replaced with file viewer trigger, got %s", html)
	}
}

func TestRenderMarkdownOpensExternalLinksInNewTab(t *testing.T) {
	html := string(RenderMarkdown("Read [LangChain](https://www.langchain.com/blog/the-art-of-loop-engineering) for context."))
	if !strings.Contains(html, `href="https://www.langchain.com/blog/the-art-of-loop-engineering"`) {
		t.Fatalf("external href should remain, got %s", html)
	}
	if !strings.Contains(html, `target="_blank"`) || !strings.Contains(html, `rel="noopener noreferrer"`) {
		t.Fatalf("external link should open in a new tab, got %s", html)
	}
	if strings.Contains(html, "data-file-view") {
		t.Fatalf("external link must not become a file viewer trigger, got %s", html)
	}
}
