package calc

import (
	"strings"
	"testing"
)

func TestCheckMarkdownAcceptsTrueLines(t *testing.T) {
	doc := "# Title\n\nText with `1 + 1 => 3` inline is ignored.\n\n```calc\n# a comment\n(1250 - 1000) / 1000 * 100 => 25\n1000 / 7 => ~142.86\n1000 / 7 => ~142.857\n2.5 * 2 => 5 [half_up]\n0.5 => ~1 [half_up]\n0.5 => ~0\n-2.5 => ~-2\n-2.5 => ~-3 [half_up]\ntodate(date(\"2026-10-03\") + 7) => 2026-10-10\n0.1 + 0.2 => 0.3\n\n```\n\n```text\nnot a calc block => nonsense\n```\n"
	blocks, lines, issues := CheckMarkdown([]byte(doc))
	if blocks != 1 || lines != 10 {
		t.Errorf("blocks, lines = %d, %d; want 1, 10", blocks, lines)
	}
	for _, issue := range issues {
		t.Errorf("unexpected issue: %s", issue)
	}
}

func TestCheckMarkdownCatchesWrongLines(t *testing.T) {
	cases := []struct {
		line string
		want string
	}{
		{"7 / 2 => 3", "the result is 3.5"},
		{"25.4 => 25", "the result is 25.4"},
		{"1 / 3 => 0.3333", "write ~"},
		{"1 / 3 => ~0.334", "rounds to 0.333"},
		{"1000 / 7 => ~142.85", "rounds to 142.86"},
		{"2.5 => ~3", "rounds to 2"},
		{"2.5 => ~2 [half_up]", "rounds to 3"},
		{"1 + => 2", "ends where a value"},
		{"no arrow here", `no "=>"`},
		{"=> 3", "both sides"},
		{"3 =>", "both sides"},
		{"10 => 10%", "plain decimal"},
		{"1000 => 1,000", "plain decimal"},
		{"5 => 5 USD", "plain decimal"},
		{`date("2026-10-03") => 2026-10-03`, "wrong"},
		{`todate(date("2026-10-03")) => 2026-10-04`, "wrong"},
		{`1 => 2026-10-03`, "wrong"},
		{`todate(1) => 5`, "is a date"},
		{"sqrt(2) => 1.4142", "write ~"},
		{"1 => ~1 [bogus]", "unknown rounding mode"},
	}
	for _, tc := range cases {
		doc := "```calc\n" + tc.line + "\n```\n"
		_, _, issues := CheckMarkdown([]byte(doc))
		if len(issues) != 1 {
			t.Errorf("%q: %d issues, want 1", tc.line, len(issues))
			continue
		}
		if !strings.Contains(issues[0].Message, tc.want) {
			t.Errorf("%q: message %q does not contain %q", tc.line, issues[0].Message, tc.want)
		}
		if issues[0].Line != 2 {
			t.Errorf("%q: line = %d, want 2", tc.line, issues[0].Line)
		}
	}
}

func TestCheckMarkdownReportsTheRightLineAndKeepsGoing(t *testing.T) {
	doc := "intro\n\n```calc\n1 + 1 => 2\n2 + 2 => 5\n3 + 3 => 7\n```\n"
	_, _, issues := CheckMarkdown([]byte(doc))
	if len(issues) != 2 || issues[0].Line != 5 || issues[1].Line != 6 {
		t.Errorf("issues = %v", issues)
	}
}

func TestCheckMarkdownIgnoresADocumentWithNoCalcBlock(t *testing.T) {
	blocks, lines, issues := CheckMarkdown([]byte("# Notes\n\n```go\nx := 1 => 2\n```\n"))
	if blocks != 0 || lines != 0 || len(issues) != 0 {
		t.Errorf("got %d, %d, %v", blocks, lines, issues)
	}
}

// A four-backtick block shows a calc block as text. The inner fence must not
// end it, and a real calc block after it must still be checked.
func TestCheckMarkdownFollowsCommonMarkFences(t *testing.T) {
	doc := "````markdown\n```calc\n1 + 1 => 3\n```\n````\n\nprose\n\n```calc\n2 + 2 => 5\n```\n"
	blocks, lines, issues := CheckMarkdown([]byte(doc))
	if blocks != 1 || lines != 1 {
		t.Errorf("blocks, lines = %d, %d; want 1, 1 (only the real block)", blocks, lines)
	}
	if len(issues) != 1 || issues[0].Line != 10 {
		t.Errorf("issues = %v, want one at line 10", issues)
	}
}

func TestCheckMarkdownHandlesTildesAndIndentedFences(t *testing.T) {
	doc := "~~~calc\n1 + 1 => 3\n~~~\n\n   ```calc\n   2 + 2 => 4\n   ```\n"
	// Tilde fences carry an info string too, but only backtick calc blocks are
	// the documented form, so the tilde block is still read as calc.
	blocks, lines, issues := CheckMarkdown([]byte(doc))
	if blocks != 2 || lines != 2 || len(issues) != 1 || issues[0].Line != 2 {
		t.Errorf("blocks %d, lines %d, issues %v", blocks, lines, issues)
	}
}

func TestCheckMarkdownIgnoresAnInlineSpanThatStartsALine(t *testing.T) {
	blocks, _, _ := CheckMarkdown([]byte("```calc``` is how it is written\n"))
	if blocks != 0 {
		t.Errorf("an inline span opened a block")
	}
}

func TestCheckMarkdownAnUnclosedBlockRunsToTheEnd(t *testing.T) {
	_, lines, issues := CheckMarkdown([]byte("```calc\n1 + 1 => 3\n"))
	if lines != 1 || len(issues) != 1 {
		t.Errorf("lines %d issues %v", lines, issues)
	}
}
