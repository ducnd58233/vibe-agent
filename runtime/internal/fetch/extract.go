// Package fetch turns a web page or a local document into the text an agent
// actually needs, and remembers it so the next session does not pay for it
// again.
//
// The saving is the point and it is measured, not assumed. HTML is mostly not
// content: scripts, stylesheets, navigation, and footers are the bulk of a
// typical page, and none of it answers the question that caused the fetch.
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
	"github.com/ducnd58233/vibe-agent/runtime/internal/fetch/domain"
	"net/url"
	"strings"

	readability "codeberg.org/readeck/go-readability/v2"
	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/base"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/commonmark"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/table"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// Document, Status and the status values live in the domain package now. These
// aliases keep the package's callers and tests compiling while the rest of the
// module moves behind them, so there is one definition rather than two.
type (
	Document = domain.Document
	Status   = domain.Status
)

const (
	StatusOK    = domain.StatusOK
	StatusEmpty = domain.StatusEmpty
	StatusThin  = domain.StatusThin
	StatusAsset = domain.StatusAsset
)

// minArticleChars is how much readable text an article has to hold before its
// boilerplate-stripped form is preferred over the whole page.
//
// Readability is tuned for articles. An API reference or a table of options is
// not article-shaped, and on those it can return a fragment or nothing at all.
// Falling back to the whole document costs some navigation and never costs the
// content, which is the right way round: a map of the page with some clutter
// beats a clean excerpt that dropped the part the reader came for.
const minArticleChars = 200

// ExtractHTML pulls the readable text out of a page.
func ExtractHTML(raw []byte) Document {
	return ExtractHTMLFrom(raw, "")
}

// ExtractHTMLFrom extracts a page, resolving relative links against pageURL.
//
// Three libraries, none of them hand-rolled, and that is the point. HTML is not
// a regular language: attribute values hold `>`, script bodies hold `<`,
// comments and CDATA nest, and browsers apply an error-correction algorithm to
// all of it. An earlier version of this file tokenized by hand and passed every
// test in this package while returning an empty body for the first real page it
// met, because JavaScript is full of `<` and one desync swallowed the document.
// A spec-compliant parser is not a convenience here, it is the difference
// between working and appearing to work.
//
//   - golang.org/x/net/html parses to a tree the way the WHATWG algorithm says,
//     including the error recovery real pages depend on.
//   - readability strips the navigation, header, footer, and aside that repeat
//     on every page of a site.
//   - html-to-markdown renders what is left, with tables and code blocks intact.
func ExtractHTMLFrom(raw []byte, pageURL string) Document {
	doc, err := html.Parse(bytes.NewReader(raw))
	if err != nil {
		// The parser recovers from malformed input rather than failing, so this
		// is a read error on a byte slice: effectively unreachable, and not a
		// reason to lose the page.
		return Document{OriginalBytes: len(raw), Empty: true}
	}

	title := textOf(findElement(doc, atom.Title))

	text := articleMarkdown(raw, pageURL)
	if len(strings.TrimSpace(text)) < minArticleChars {
		if whole := documentMarkdown(doc); len(whole) > len(text) {
			text = whole
		}
	}
	text = strings.TrimSpace(text)

	return Document{
		Title:         strings.TrimSpace(collapse(title)),
		Text:          text,
		OriginalBytes: len(raw),
		Status:        classify(text, len(raw)),
		Empty:         text == "",
	}
}

// articleMarkdown renders the boilerplate-stripped article, or "" if there is
// none to find.
func articleMarkdown(raw []byte, pageURL string) string {
	var base *url.URL
	if pageURL != "" {
		if parsed, err := url.Parse(pageURL); err == nil {
			base = parsed
		}
	}
	article, err := readability.FromReader(bytes.NewReader(raw), base)
	if err != nil || article.Node == nil {
		return ""
	}
	return convert(article.Node)
}

// documentMarkdown renders the whole page, minus the elements that never carry
// prose.
//
// The drop list is short on purpose. html-to-markdown already ignores script and
// style; these are the ones that survive it and repeat on every page of a site,
// so an agent reading ten pages pays for the same sidebar ten times.
func documentMarkdown(doc *html.Node) string {
	remove(doc, map[atom.Atom]bool{
		atom.Nav: true, atom.Footer: true, atom.Aside: true,
		atom.Script: true, atom.Style: true, atom.Noscript: true,
		atom.Svg: true, atom.Form: true, atom.Iframe: true, atom.Template: true,
	})
	return convert(doc)
}

// convert renders a parsed tree as markdown.
//
// Two settings differ from the defaults, and both are about who reads the
// output. Escaping exists so markdown round-trips through a renderer; nothing
// renders this, a model reads it, and `a &lt; b` costs three tokens to say `<`
// while making the text less like the page. The table plugin is not on by
// default and a settings table without it arrives as "timeout30s", which reads
// as one value and is two.
// mediaSources are elements whose src is the thing a reader wanted and which the
// markdown converter has no rule for, so it renders them as nothing.
var mediaSources = map[atom.Atom]bool{
	atom.Video: true, atom.Audio: true, atom.Source: true,
	atom.Embed: true, atom.Track: true,
}

// promoteMedia rewrites media elements as links so their URLs survive.
//
// A page whose point is a demo video converts to prose with the video silently
// absent, which is the same failure as a dropped table: the reader cannot tell
// anything is missing. A link is the smallest thing that keeps the URL, and the
// URL is what makes the file fetchable afterwards.
func promoteMedia(root *html.Node) {
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
			if child.Type != html.ElementNode || !mediaSources[child.DataAtom] {
				continue
			}
			src := attr(child, "src")
			if src == "" {
				continue
			}
			child.Type = html.ElementNode
			child.DataAtom = atom.A
			child.Data = "a"
			child.Attr = []html.Attribute{{Key: "href", Val: src}}
			if child.FirstChild == nil {
				child.AppendChild(&html.Node{Type: html.TextNode, Data: src})
			}
		}
	}
	walk(root)
}

func attr(node *html.Node, name string) string {
	for _, a := range node.Attr {
		if strings.EqualFold(a.Key, name) {
			return a.Val
		}
	}
	return ""
}

func convert(node *html.Node) string {
	promoteMedia(node)
	out, err := markdown().ConvertNode(node)
	if err != nil {
		return ""
	}
	// Decode entities that survive the conversion. A markdown renderer wants
	// `&lt;`, and nothing renders this: a model reads it, where `&lt;` costs
	// three tokens to say `<` and reads less like the page did. Safe on code
	// samples too, since an entity inside one stands for the character the
	// sample contains.
	return tidy(html.UnescapeString(string(out)))
}

// markdown builds the converter.
//
// The top-level helper takes a different option type and carries no table
// support, so the converter is assembled here: base and commonmark are what the
// helper would have given, table is the addition, and escaping is turned off.
func markdown() *converter.Converter {
	return converter.NewConverter(
		converter.WithEscapeMode(converter.EscapeModeDisabled),
		converter.WithPlugins(
			base.NewBasePlugin(),
			commonmark.NewCommonmarkPlugin(),
			table.NewTablePlugin(),
		),
	)
}

// remove deletes every matching element from the tree.
func remove(node *html.Node, drop map[atom.Atom]bool) {
	var doomed []*html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			if child.Type == html.ElementNode && drop[child.DataAtom] {
				doomed = append(doomed, child)
				continue
			}
			walk(child)
		}
	}
	walk(node)
	for _, n := range doomed {
		if n.Parent != nil {
			n.Parent.RemoveChild(n)
		}
	}
}

// findElement returns the first element of a kind, depth first.
func findElement(node *html.Node, want atom.Atom) *html.Node {
	if node.Type == html.ElementNode && node.DataAtom == want {
		return node
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if found := findElement(child, want); found != nil {
			return found
		}
	}
	return nil
}

// textOf returns an element's character data, concatenated.
func textOf(node *html.Node) string {
	if node == nil {
		return ""
	}
	var out strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			out.WriteString(n.Data)
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(node)
	return out.String()
}

// collapse reduces every run of whitespace to one space.
func collapse(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// tidy drops trailing whitespace and runs of blank lines.
//
// Fenced lines pass through untouched: the whitespace inside a code sample is
// content, and a code sample is often why the page was fetched.
func tidy(s string) string {
	lines := strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
	kept := make([]string, 0, len(lines))
	blank := false
	fenced := false
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			fenced = !fenced
			kept = append(kept, strings.TrimRight(line, " \t"))
			blank = false
			continue
		}
		if fenced {
			kept = append(kept, strings.TrimRight(line, " \t"))
			blank = false
			continue
		}
		trimmed := strings.TrimRight(line, " \t")
		if strings.TrimSpace(trimmed) == "" {
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
