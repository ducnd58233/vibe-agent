package fetch

import (
	"strings"
	"testing"
)

func TestExtractHTMLKeepsProseAndDropsMachinery(t *testing.T) {
	raw := `<!doctype html>
<html><head>
<title>Rate limiting</title>
<style>.nav { color: red }</style>
<script>window.analytics = {track: function(){}};</script>
</head>
<body>
<nav><a href="/">Home</a><a href="/docs">Docs</a></nav>
<main>
<h1>Rate limiting</h1>
<p>Requests are capped at <code>100/minute</code> per key.</p>
<ul><li>Burst is 20</li><li>Window is 60s</li></ul>
</main>
<footer>Copyright 2026</footer>
</body></html>`

	doc := ExtractHTML([]byte(raw))

	if doc.Title != "Rate limiting" {
		t.Errorf("title = %q", doc.Title)
	}
	for _, want := range []string{"# Rate limiting", "100/minute", "- Burst is 20", "- Window is 60s"} {
		if !strings.Contains(doc.Text, want) {
			t.Errorf("extract dropped %q:\n%s", want, doc.Text)
		}
	}
	// Script and style bodies are the largest thing on most pages and none of it
	// is content. Nav and footer are the boilerplate that repeats on every page
	// of a site, so an agent reading ten pages pays for it ten times.
	for _, unwanted := range []string{"window.analytics", "color: red", "Copyright 2026", "Docs"} {
		if strings.Contains(doc.Text, unwanted) {
			t.Errorf("extract kept %q:\n%s", unwanted, doc.Text)
		}
	}
}

func TestExtractHTMLDecodesEntities(t *testing.T) {
	doc := ExtractHTML([]byte(`<p>a &lt; b &amp;&amp; c &gt; d &quot;quoted&quot; &#39;x&#39; &nbsp;end</p>`))
	for _, want := range []string{"a < b && c > d", `"quoted"`, "'x'"} {
		if !strings.Contains(doc.Text, want) {
			t.Errorf("entity not decoded, want %q in:\n%s", want, doc.Text)
		}
	}
	if strings.Contains(doc.Text, "&") && !strings.Contains(doc.Text, "&&") {
		t.Errorf("a raw entity survived:\n%s", doc.Text)
	}
}

// Links keep their target.
//
// An earlier version dropped hrefs to save tokens, and the saving was real: a
// documentation page carries hundreds of them. It was still the wrong trade. The
// most common thing to do with a fetched page is follow a link out of it, and an
// agent that cannot see where a link goes has to guess a URL, which is how a
// confident wrong answer gets made. Cost is bounded and paid once per link;
// guessing is unbounded.
func TestExtractHTMLKeepsLinksAndTheirTargets(t *testing.T) {
	doc := ExtractHTML([]byte(
		`<main><p>See <a href="https://example.com/guide">the guide</a> first.</p></main>`))
	if !strings.Contains(doc.Text, "the guide") {
		t.Errorf("link text lost:\n%s", doc.Text)
	}
	if !strings.Contains(doc.Text, "https://example.com/guide") {
		t.Errorf("link target lost, so the page cannot be followed:\n%s", doc.Text)
	}
}

func TestExtractHTMLSeparatesBlocks(t *testing.T) {
	doc := ExtractHTML([]byte(`<main><p>First para.</p><p>Second para.</p><h2>Heading</h2></main>`))
	if strings.Contains(doc.Text, "First para.Second") {
		t.Errorf("blocks were run together:\n%s", doc.Text)
	}
	if !strings.Contains(doc.Text, "## Heading") {
		t.Errorf("h2 is not a markdown heading:\n%s", doc.Text)
	}
}

// A page with no <main> is the common case. Falling back to the body rather
// than returning nothing is what makes this usable on real sites.
func TestExtractHTMLFallsBackToTheBody(t *testing.T) {
	doc := ExtractHTML([]byte(`<html><body><div><p>Only content here.</p></div></body></html>`))
	if !strings.Contains(doc.Text, "Only content here.") {
		t.Errorf("body content lost when there is no main:\n%s", doc.Text)
	}
}

func TestExtractHTMLCollapsesWhitespace(t *testing.T) {
	doc := ExtractHTML([]byte("<main><p>lots\n\n\n   of      space</p></main>"))
	if strings.Contains(doc.Text, "   ") || strings.Contains(doc.Text, "\n\n\n") {
		t.Errorf("whitespace not collapsed: %q", doc.Text)
	}
	if !strings.Contains(doc.Text, "lots of space") {
		t.Errorf("collapsing lost words: %q", doc.Text)
	}
}

// The bug that got past every other test here, found on the first real page.
//
// Script bodies are character data, and JavaScript is full of `<`. A tokenizer
// that reads `if (a < b) {` as a tag resolves it at some later `>` and can step
// over the real </script>, after which the rest of the document is inside a
// skip that never ends. On a documentation page with a megabyte of script the
// result was a correct title and an empty body.
func TestExtractHTMLSurvivesLessThanInsideScript(t *testing.T) {
	raw := `<html><head><title>T</title></head><body>
<script>
for (let i = 0; i < items.length; i++) { if (a<b && c>d) render(); }
const el = <div className="x">jsx</div>;
</script>
<main><p>The content after the script.</p></main>
</body></html>`

	doc := ExtractHTML([]byte(raw))

	if !strings.Contains(doc.Text, "The content after the script.") {
		t.Errorf("everything after a script with `<` in it was swallowed:\n%q", doc.Text)
	}
	if strings.Contains(doc.Text, "items.length") || strings.Contains(doc.Text, "className") {
		t.Errorf("script body leaked into the text:\n%q", doc.Text)
	}
}

// Style bodies have the same property, and a media query is the common way in.
func TestExtractHTMLSurvivesLessThanInsideStyle(t *testing.T) {
	raw := `<html><body><style>@media (max-width: 40rem) { .a > .b { top: 0 } }</style>
<main><p>Still here.</p></main></body></html>`
	doc := ExtractHTML([]byte(raw))
	if !strings.Contains(doc.Text, "Still here.") {
		t.Errorf("content after a style block was lost:\n%q", doc.Text)
	}
}

// A page that never closes its script is malformed and common. Treating the
// remainder as script is the safe direction: the alternative reads minified
// JavaScript out as prose, at full token price.
func TestExtractHTMLTreatsAnUnclosedScriptAsScript(t *testing.T) {
	doc := ExtractHTML([]byte(`<body><main><p>Before.</p></main><script>var a = 1;`))
	if !strings.Contains(doc.Text, "Before.") {
		t.Errorf("content before an unclosed script was lost:\n%q", doc.Text)
	}
	if strings.Contains(doc.Text, "var a = 1") {
		t.Errorf("an unclosed script was read as prose:\n%q", doc.Text)
	}
}

// Running table cells together is worse than dropping the table. Documentation
// is full of settings tables, and "timeout30s" reads as a value a reader can act
// on while being two facts glued together.
func TestExtractHTMLSeparatesTableCells(t *testing.T) {
	doc := ExtractHTML([]byte(`<main><table>
<tr><th>Setting</th><th>Default</th></tr>
<tr><td>timeout</td><td>30s</td></tr>
</table></main>`))

	for _, glued := range []string{"SettingDefault", "timeout30s"} {
		if strings.Contains(doc.Text, glued) {
			t.Errorf("table cells run together as %q:\n%s", glued, doc.Text)
		}
	}
	for _, want := range []string{"Setting", "Default", "timeout", "30s"} {
		if !strings.Contains(doc.Text, want) {
			t.Errorf("table lost %q:\n%s", want, doc.Text)
		}
	}
	// One row per line, so a reader can tell which value belongs to which key.
	if !strings.Contains(doc.Text, "timeout | 30s") {
		t.Errorf("a row is not one line:\n%s", doc.Text)
	}
}

// An agent cannot tell a literal from prose once the marking is gone, and a
// flag name is exactly the thing it will copy into a command.
func TestExtractHTMLMarksInlineCode(t *testing.T) {
	doc := ExtractHTML([]byte(`<main><p>Pass <code>--budget</code> to raise it.</p></main>`))
	if !strings.Contains(doc.Text, "`--budget`") {
		t.Errorf("inline code lost its marking:\n%s", doc.Text)
	}
}

// The silent failure that matters most. A client-rendered page has a title, a
// script, and no prose, so extraction succeeds and returns nothing. Reported as
// a 99% saving it looks like a win; the agent then answers from an empty
// document.
func TestExtractHTMLReportsAPageThatCarriedNoText(t *testing.T) {
	doc := ExtractHTML([]byte(
		`<html><head><title>App</title></head><body><div id="root"></div><script>renderApp()</script></body></html>`))

	if doc.Text != "" {
		t.Fatalf("this fixture is supposed to extract to nothing, got %q", doc.Text)
	}
	if !doc.Empty {
		t.Error("a page that yielded no text does not say so, so a caller cannot tell " +
			"an empty page from a page it failed to read")
	}
}

func TestExtractHTMLDoesNotCryEmptyOnARealPage(t *testing.T) {
	doc := ExtractHTML([]byte(`<main><p>There is prose here.</p></main>`))
	if doc.Empty {
		t.Errorf("a page with content was reported as empty: %q", doc.Text)
	}
}

// The whole justification for this package. If extraction does not actually
// shrink a realistic page, nothing here is worth its complexity.
func TestExtractHTMLShrinksARealisticPage(t *testing.T) {
	var page strings.Builder
	page.WriteString("<html><head><title>Doc</title><style>")
	for range 200 {
		page.WriteString(".cls-x { margin: 0 auto; padding: 1rem; color: #333 }\n")
	}
	page.WriteString("</style><script>")
	for range 200 {
		page.WriteString("function handler(e) { return e.preventDefault(); }\n")
	}
	page.WriteString("</script></head><body><nav>")
	for range 50 {
		page.WriteString(`<a href="/some/nav/link">Navigation entry</a>`)
	}
	page.WriteString("</nav><main><h1>Doc</h1><p>The one paragraph that matters.</p></main></body></html>")

	raw := page.String()
	doc := ExtractHTML([]byte(raw))

	if !strings.Contains(doc.Text, "The one paragraph that matters.") {
		t.Fatalf("extraction lost the content:\n%s", doc.Text)
	}
	ratio := float64(len(doc.Text)) / float64(len(raw))
	if ratio > 0.05 {
		t.Errorf("extract kept %.1f%% of the page; the research this is built on reports 80-90%% reductions",
			ratio*100)
	}
}
