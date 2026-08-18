package view

import (
	"bytes"
	"html"
	"html/template"
	"sync"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	mdhtml "github.com/yuin/goldmark/renderer/html"
)

var (
	markdownOnce sync.Once
	markdown     goldmark.Markdown
)

func markdownEngine() goldmark.Markdown {
	markdownOnce.Do(func() {
		// Default goldmark leaves WithUnsafe off, so raw HTML and javascript:
		// URLs are not rendered. See github.com/yuin/goldmark README Security.
		markdown = goldmark.New(
			goldmark.WithExtensions(extension.GFM),
			goldmark.WithRendererOptions(mdhtml.WithHardWraps()),
		)
	})
	return markdown
}

// RenderMarkdown turns CommonMark / GFM into HTML for Chat and transcript rows.
func RenderMarkdown(src string) template.HTML {
	if src == "" {
		return ""
	}
	var buf bytes.Buffer
	if err := markdownEngine().Convert([]byte(src), &buf); err != nil {
		return template.HTML(html.EscapeString(src)) //nolint:gosec // G203 escaped fallback
	}
	return template.HTML(buf.String()) //nolint:gosec // G203 goldmark WithUnsafe is off
}

func markdownBody(role, body string) template.HTML {
	switch role {
	case "user", "assistant", "question":
		return RenderMarkdown(body)
	default:
		return ""
	}
}
