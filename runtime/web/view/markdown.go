// Package view renders session UI models. markdown.go converts GFM source to
// HTML for chat and .md file viewer content; workspace file I/O lives in
// internal/web/app/files.go.
package view

import (
	"bytes"
	"html"
	"html/template"
	"regexp"
	"strings"
	"sync"

	"github.com/ducnd58233/vibe-agent/runtime/internal/web/domain"
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
	out = linkFilePathsInCode(out)
	out = linkLocalMarkdownAnchors(out)
	out = styleExternalMarkdownAnchors(out)
	out = styleTaskListItems(out)
	return template.HTML(out) //nolint:gosec // G203 goldmark WithUnsafe is off
}

var refRe = regexp.MustCompile(`(^|\s)(/[a-z][a-z0-9-]*)|(^|\s)(@[^\s<]+)`)
var codeSpanRe = regexp.MustCompile(`<code>([^<]+)</code>`)
var taskListItemRe = regexp.MustCompile(`<li>\s*(<input[^>]*type="checkbox"[^>]*>)`)
var mdLocalAnchorRe = regexp.MustCompile(`<a href="([^"]+)"`)

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

// linkFilePathsInCode wraps file paths inside <code> tags with a clickable
// element that opens the file viewer dialog.
func linkFilePathsInCode(s string) string {
	return codeSpanRe.ReplaceAllStringFunc(s, func(m string) string {
		sub := codeSpanRe.FindStringSubmatch(m)
		if len(sub) < 2 {
			return m
		}
		path := strings.TrimSpace(sub[1])
		if !domain.LooksLikeFileRef(path) {
			return m
		}
		return `<code><a href="#" data-file-view="` + html.EscapeString(path) + `" class="file-link">` + html.EscapeString(path) + `</a></code>`
	})
}

// linkLocalMarkdownAnchors turns relative markdown links into file-viewer
// triggers so cross-references can navigate inside the viewer dialog.
func linkLocalMarkdownAnchors(s string) string {
	return mdLocalAnchorRe.ReplaceAllStringFunc(s, func(m string) string {
		sub := mdLocalAnchorRe.FindStringSubmatch(m)
		if len(sub) < 2 {
			return m
		}
		href := strings.TrimSpace(sub[1])
		if href == "" || strings.HasPrefix(href, "#") {
			return m
		}
		lower := strings.ToLower(href)
		if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") ||
			strings.HasPrefix(lower, "mailto:") || strings.HasPrefix(lower, "javascript:") {
			return m
		}
		if strings.Contains(m, "data-file-view") {
			return m
		}
		path := href
		if idx := strings.IndexAny(path, "#?"); idx >= 0 {
			path = path[:idx]
		}
		if path == "" {
			return m
		}
		escaped := html.EscapeString(path)
		return `<a href="#" data-file-view="` + escaped + `" class="file-link"`
	})
}

// styleExternalMarkdownAnchors keeps http(s) links in the session UI and opens
// them in a new tab instead of navigating the control-plane page away.
func styleExternalMarkdownAnchors(s string) string {
	return mdLocalAnchorRe.ReplaceAllStringFunc(s, func(m string) string {
		sub := mdLocalAnchorRe.FindStringSubmatch(m)
		if len(sub) < 2 {
			return m
		}
		href := strings.TrimSpace(sub[1])
		lower := strings.ToLower(href)
		if !strings.HasPrefix(lower, "http://") && !strings.HasPrefix(lower, "https://") {
			return m
		}
		if strings.Contains(m, `target="`) {
			return m
		}
		return strings.Replace(m, "<a href=", `<a target="_blank" rel="noopener noreferrer" href=`, 1)
	})
}

// styleTaskListItems annotates GFM task-list HTML so stylesheet rules can
// remove bullet markers and align checkboxes consistently across browsers.
func styleTaskListItems(s string) string {
	return taskListItemRe.ReplaceAllStringFunc(s, func(m string) string {
		sub := taskListItemRe.FindStringSubmatch(m)
		if len(sub) < 2 {
			return m
		}
		input := addClassToTag(sub[1], "task-list-item-checkbox")
		return `<li class="task-list-item">` + input
	})
}

func addClassToTag(tag, className string) string {
	if strings.Contains(tag, `class="`) {
		return strings.Replace(tag, `class="`, `class="`+className+` `, 1)
	}
	return strings.Replace(tag, ">", ` class="`+className+`">`, 1)
}

func markdownBody(role, body string) template.HTML {
	switch role {
	case "user", "assistant", "question":
		return RenderMarkdown(body)
	default:
		return ""
	}
}
