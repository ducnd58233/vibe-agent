// Package fetch turns a web page or a local document into the text an agent
// actually needs, and remembers it so the next session does not pay for it
// again.
//
// The saving is the point and it is measured, not assumed. HTML is mostly not
// content: scripts, stylesheets, navigation, and footers are the bulk of a
// typical page, and none of it answers the question that caused the fetch.
// Reported reductions for HTML to markdown run to 80-90%, and the test in this
// package holds the extractor to that on a synthetic page built to look like a
// real one.
//
// Two rules, both inherited from the packages around this one:
//
//   - Nothing here summarizes. Extraction deletes markup and boilerplate and
//     keeps the author's words verbatim, so what an agent reads is what the
//     page said. A summary would be model output presented as a source.
//   - The cache is content-addressed and lives in the workspace, beside the
//     memory database, under the same gitignored directory.
package fetch

import (
	"bytes"
	"html"
	"strings"
	"unicode"
)

// Document is extracted content and where it came from.
type Document struct {
	Source string `json:"source"`
	Title  string `json:"title,omitempty"`
	Text   string `json:"text"`
	// OriginalBytes is what arrived before extraction, so the saving can be
	// reported rather than claimed.
	OriginalBytes int `json:"originalBytes"`
}

// dropped elements contribute nothing a reader wants, and are the bulk of a
// page's bytes.
//
// nav, header, footer, and aside are the boilerplate that repeats on every page
// of a site: an agent reading ten pages of one documentation set pays for the
// same sidebar ten times. script and style are machinery. form and svg are
// markup with no prose in them.
var dropped = map[string]bool{
	"script": true, "style": true, "noscript": true, "svg": true,
	"nav": true, "footer": true, "aside": true, "form": true,
	"header": true, "template": true, "iframe": true, "canvas": true,
}

// rawText elements hold character data, not markup, and have to be skipped by
// searching for their closing tag rather than by tokenizing what is inside.
//
// This is the HTML spec's rule and it is not a nicety. JavaScript is full of
// `<`: comparisons, generics, JSX. A tokenizer that treats `if (a < b) { ... }`
// as the start of a tag resolves it at some later `>` and can step straight over
// the real `</script>`. One desync in a page with a megabyte of script swallows
// the entire document, which is what this did on the first real page it met:
// title extracted, body empty.
var rawText = map[string]bool{
	"script": true, "style": true, "noscript": true, "textarea": true, "title": true,
}

// blocks end a line when they open or close, so prose does not run together.
var blocks = map[string]bool{
	"p": true, "div": true, "section": true, "article": true, "main": true,
	"ul": true, "ol": true, "table": true, "tr": true, "br": true, "hr": true,
	"blockquote": true, "pre": true, "figure": true, "dl": true, "dt": true,
	"dd": true, "h1": true, "h2": true, "h3": true, "h4": true, "h5": true,
	"h6": true, "li": true,
}

// headings map to the markdown level of the same depth. Markdown is kept
// because it is the cheapest structure that survives: a heading costs one or two
// characters and tells a reader where they are.
var headings = map[string]string{
	"h1": "# ", "h2": "## ", "h3": "### ",
	"h4": "#### ", "h5": "##### ", "h6": "###### ",
}

// ExtractHTML pulls the readable text out of a page.
//
// A tokenizer, not a parser. Text extraction needs to know where tags start and
// stop and nothing about the tree they form, so this walks the bytes and keeps a
// depth counter for the elements being skipped. The same tradeoff the repomap
// package makes with regexes over tree-sitter, and the same reason: a DOM parser
// is a dependency, and this module's whole dependency set is a SQLite driver and
// a YAML reader.
//
// Malformed markup degrades into slightly noisier text rather than an error. A
// page that fails to close a tag is common, and refusing it would be refusing
// most of the web.
func ExtractHTML(raw []byte) Document {
	var out strings.Builder
	var title string

	// skip counts how deep inside a dropped element the cursor is, so a <script>
	// containing "</div>" cannot end the skip early.
	skip := 0
	skipName := ""
	inTitle := false
	// pre is the one element whose whitespace is content. A code sample is
	// usually why a page was fetched at all, and squeezing it turns a program
	// into one long line.
	pre := 0

	index, size := 0, len(raw)
	for index < size {
		next := bytes.IndexByte(raw[index:], '<')
		if next < 0 {
			if skip == 0 {
				out.WriteString(characters(raw[index:], pre > 0))
			}
			break
		}
		if skip == 0 && next > 0 {
			chunk := characters(raw[index:index+next], pre > 0)
			if inTitle {
				title += chunk
			} else {
				out.WriteString(chunk)
			}
		}
		index += next

		// A comment or doctype ends at its own terminator, not at the next '>'.
		if bytes.HasPrefix(raw[index:], []byte("<!--")) {
			if end := bytes.Index(raw[index:], []byte("-->")); end >= 0 {
				index += end + 3
				continue
			}
			break
		}
		end := bytes.IndexByte(raw[index:], '>')
		if end < 0 {
			break
		}
		tag := string(raw[index+1 : index+end])
		index += end + 1

		closing := strings.HasPrefix(tag, "/")
		name := elementName(tag)
		if name == "" {
			continue
		}

		// Raw text first, and regardless of whether anything is being skipped:
		// its content is character data, so the only way out is its closing tag.
		if rawText[name] && !closing && !strings.HasSuffix(tag, "/") {
			body, after := rawTextBody(raw, index, name)
			index = after
			if name == "title" && skip == 0 {
				title += characters(body, false)
			}
			continue
		}

		switch {
		case dropped[name]:
			if closing {
				if skip > 0 && name == skipName {
					skip--
					if skip == 0 {
						skipName = ""
					}
				}
			} else if !strings.HasSuffix(tag, "/") {
				if skip == 0 {
					skipName = name
				}
				if skipName == name {
					skip++
				}
			}
			continue
		case skip > 0:
			continue
		}

		if name == "pre" {
			// Fenced, so the renderer and the reader both know the whitespace
			// inside is deliberate, and so tidy can leave those lines alone.
			if closing {
				if pre > 0 {
					pre--
				}
				out.WriteString("\n```\n")
			} else {
				pre++
				out.WriteString("\n```\n")
			}
			continue
		}
		if prefix, ok := headings[name]; ok && !closing {
			out.WriteString("\n" + prefix)
			continue
		}
		if name == "li" && !closing {
			out.WriteString("\n- ")
			continue
		}
		if blocks[name] {
			out.WriteString("\n")
		}
	}

	return Document{
		Title:         strings.TrimSpace(collapse(title)),
		Text:          tidy(out.String()),
		OriginalBytes: len(raw),
	}
}

// rawTextBody returns a raw-text element's content and the offset just past its
// closing tag.
//
// An unclosed one runs to the end of the document, which is what a browser does
// with it too. Returning the remainder as body rather than resuming the scan is
// the safe direction: the alternative reads a minified script as prose.
func rawTextBody(raw []byte, from int, name string) (body []byte, after int) {
	closer := []byte("</" + name)
	end := indexFold(raw[from:], closer)
	if end < 0 {
		return raw[from:], len(raw)
	}
	body = raw[from : from+end]
	rest := from + end
	if gt := bytes.IndexByte(raw[rest:], '>'); gt >= 0 {
		return body, rest + gt + 1
	}
	return body, len(raw)
}

// indexFold finds a needle without regard to case, which tag names need: a page
// writing </SCRIPT> is unusual and legal.
//
// Scans rather than lowercasing a copy. The obvious version allocates the whole
// remaining document per raw-text element, and a page with a hundred scripts
// then costs a hundred copies of itself.
func indexFold(haystack, needle []byte) int {
	if len(needle) == 0 || len(haystack) < len(needle) {
		return -1
	}
	first := lower(needle[0])
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if lower(haystack[i]) != first {
			continue
		}
		match := true
		for j := 1; j < len(needle); j++ {
			if lower(haystack[i+j]) != lower(needle[j]) {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

func lower(b byte) byte {
	if b >= 'A' && b <= 'Z' {
		return b + ('a' - 'A')
	}
	return b
}

// elementName returns the lowercased tag name, without the slash or attributes.
func elementName(tag string) string {
	tag = strings.TrimPrefix(tag, "/")
	tag = strings.TrimLeft(tag, "!?")
	cut := strings.IndexAny(tag, " \t\r\n/>")
	if cut >= 0 {
		tag = tag[:cut]
	}
	return strings.ToLower(strings.TrimSpace(tag))
}

// characters decodes entities in a run of character data, and squeezes its
// whitespace unless it came from a pre block.
//
// Newlines inside a text node are source formatting, not line breaks: HTML says
// where a line ends with tags, and only pre says otherwise. Squeezing here
// rather than later is what keeps a paragraph that was wrapped in the source
// from arriving as several lines. The single space is kept at both ends, since
// dropping it would join the words either side of a tag.
func characters(raw []byte, preformatted bool) string {
	decoded := html.UnescapeString(string(raw))
	if preformatted {
		return decoded
	}
	return squeeze(decoded)
}

// squeeze reduces every whitespace run to one space, ends included.
func squeeze(s string) string {
	var out strings.Builder
	space := false
	for _, r := range s {
		// unicode.IsSpace says no to the non-breaking space, which is what
		// &nbsp; decodes to and what a page indenting with it would leave on
		// every line.
		if unicode.IsSpace(r) || r == ' ' {
			space = true
			continue
		}
		if space {
			out.WriteByte(' ')
			space = false
		}
		out.WriteRune(r)
	}
	if space {
		out.WriteByte(' ')
	}
	return out.String()
}

// collapse reduces every run of whitespace to one space.
//
// Source formatting is invisible to a reader and is not free: indentation on a
// deeply nested page is a real share of its bytes.
func collapse(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// tidy collapses whitespace inside lines and blank runs between them.
//
// Fenced lines pass through untouched. That is what makes the pre handling above
// mean anything: squeezing a code sample here would undo it one step later.
func tidy(s string) string {
	lines := strings.Split(s, "\n")
	kept := make([]string, 0, len(lines))
	blank := false
	fenced := false
	for _, line := range lines {
		if strings.TrimSpace(line) == "```" {
			fenced = !fenced
			kept = append(kept, "```")
			blank = false
			continue
		}
		if fenced {
			kept = append(kept, strings.TrimRight(line, " \t\r"))
			blank = false
			continue
		}
		trimmed := collapse(line)
		if trimmed == "" {
			// One blank line separates blocks; more is just page structure.
			if !blank && len(kept) > 0 {
				kept = append(kept, "")
			}
			blank = true
			continue
		}
		blank = false
		kept = append(kept, trimmed)
	}
	return strings.TrimSpace(strings.Join(kept, "\n"))
}
