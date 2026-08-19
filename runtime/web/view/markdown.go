package view

import (
	"bytes"
	"html"
	"html/template"
	"regexp"
	"strings"
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
	out := highlightRefs(buf.String())
	return template.HTML(out) //nolint:gosec // G203 goldmark WithUnsafe is off
}

var refRe = regexp.MustCompile(`(^|\s)(/[a-z][a-z0-9-]*)|(^|\s)(@[^\s<]+)`)

// highlightRefs wraps /command and @file references with <mark> tags,
// skipping content inside <code> and <pre> blocks.
func highlightRefs(s string) string {
	var out strings.Builder
	out.Grow(len(s) + len(s)/10)
	skip := 0
	for i := 0; i < len(s); {
		if s[i] == '<' {
			end := strings.IndexByte(s[i:], '>')
			if end < 0 {
				out.WriteString(s[i:])
				break
			}
			tag := s[i : i+end+1]
			out.WriteString(tag)
			i += end + 1
			lower := strings.ToLower(tag)
			if strings.HasPrefix(lower, "<code") || strings.HasPrefix(lower, "<pre") {
				skip++
			} else if strings.HasPrefix(lower, "</code") || strings.HasPrefix(lower, "</pre") {
				if skip > 0 {
					skip--
				}
			}
			continue
		}
		if skip > 0 {
			next := strings.IndexByte(s[i:], '<')
			if next < 0 {
				out.WriteString(s[i:])
				break
			}
			out.WriteString(s[i : i+next])
			i += next
			continue
		}
		next := strings.IndexByte(s[i:], '<')
		var chunk string
		if next < 0 {
			chunk = s[i:]
			i = len(s)
		} else {
			chunk = s[i : i+next]
			i += next
		}
		replaced := refRe.ReplaceAllStringFunc(chunk, func(m string) string {
			loc := refRe.FindStringIndex(m)
			if loc == nil {
				return m
			}
			lead := ""
			ref := m
			if len(m) > 0 && (m[0] == ' ' || m[0] == '\n' || m[0] == '\t' || m[0] == '\r') {
				lead = m[:1]
				ref = m[1:]
			}
			return lead + `<mark class="composer-ref">` + ref + `</mark>`
		})
		out.WriteString(replaced)
	}
	return out.String()
}

func markdownBody(role, body string) template.HTML {
	switch role {
	case "user", "assistant", "question":
		return RenderMarkdown(body)
	default:
		return ""
	}
}
