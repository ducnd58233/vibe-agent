package harness

import "strings"

// Masking exists because a test for a parser, a linter, or a code generator
// states its input as source code, and that source starts lines with `func`,
// `def`, or `class` in column one. Read literally it breaks the test guard in
// both directions:
//
//   - a block ends at the first declaration inside the fixture, hiding every
//     assertion that follows it. That is a false positive on precisely the
//     tests that most need checking.
//   - an assertion quoted inside a fixture counts as a real one, so a test that
//     checks nothing passes because its own example says `assert`.
//
// Both go away by blanking the inside of literals and comments before anything
// else looks at the text. Byte length and newlines are preserved, so every
// offset and every reported line number still refers to the file a person will
// open.

// A delimiter is a comment's opener and closer. A closer of "\n" means the
// comment ends at the end of its line.
type delimiter struct {
	open  string
	close string
}

// A stringDelimiter is a string literal's syntax. escapes marks the literals
// where a backslash defers the closer; Go's raw string and Python's triple
// quote take a backslash as an ordinary byte.
type stringDelimiter struct {
	open    string
	close   string
	escapes bool
}

// literalSyntax is one language's comment and string forms.
type literalSyntax struct {
	comments []delimiter
	strings  []stringDelimiter
}

// maskLiterals returns text with the inside of every literal and comment
// blanked out.
//
// Openers are tried longest first, so Python's triple quote wins over its
// single one.
func maskLiterals(syntax literalSyntax, text string) string {
	comments := longestFirst(syntax.comments, func(d delimiter) string { return d.open })
	quotes := longestFirst(syntax.strings, func(d stringDelimiter) string { return d.open })

	var out strings.Builder
	out.Grow(len(text))

	for index := 0; index < len(text); {
		if next, ok := maskComment(&out, text, index, comments); ok {
			index = next
			continue
		}
		if next, ok := maskString(&out, text, index, quotes); ok {
			index = next
			continue
		}
		out.WriteByte(text[index])
		index++
	}
	return out.String()
}

// maskComment blanks a comment starting at index, and reports where it ended.
func maskComment(out *strings.Builder, text string, index int, comments []delimiter) (int, bool) {
	for _, comment := range comments {
		if !strings.HasPrefix(text[index:], comment.open) {
			continue
		}
		from := index + len(comment.open)
		stop := len(text)
		if end := strings.Index(text[from:], comment.close); end >= 0 {
			stop = from + end
			// A line comment keeps its newline, which is its own closer, so the
			// line count survives. A block comment's closer is blanked with the
			// rest, which costs nothing and keeps the length identical.
			if comment.close != "\n" {
				stop += len(comment.close)
			}
		}
		out.WriteString(comment.open)
		out.WriteString(blank(text[from:stop]))
		return stop, true
	}
	return index, false
}

// maskString blanks a string literal starting at index, and reports where it
// ended. The closer is written back verbatim so the text still parses.
func maskString(out *strings.Builder, text string, index int, quotes []stringDelimiter) (int, bool) {
	for _, quote := range quotes {
		if !strings.HasPrefix(text[index:], quote.open) {
			continue
		}
		from := index + len(quote.open)
		cursor := from
		for cursor < len(text) {
			if quote.escapes && text[cursor] == '\\' {
				cursor += 2
				continue
			}
			if strings.HasPrefix(text[cursor:], quote.close) {
				break
			}
			cursor++
		}
		if cursor > len(text) {
			cursor = len(text)
		}
		stop := min(cursor+len(quote.close), len(text))

		out.WriteString(quote.open)
		out.WriteString(blank(text[from:min(cursor, len(text))]))
		out.WriteString(text[min(cursor, len(text)):stop])
		return stop, true
	}
	return index, false
}

// blank replaces a span with spaces, keeping its newlines so lines still count.
func blank(span string) string {
	var out strings.Builder
	out.Grow(len(span))
	for index := 0; index < len(span); index++ {
		if span[index] == '\n' {
			out.WriteByte('\n')
			continue
		}
		out.WriteByte(' ')
	}
	return out.String()
}

// longestFirst orders delimiters so a longer opener is tried before a shorter
// one that prefixes it.
func longestFirst[T any](items []T, opener func(T) string) []T {
	ordered := make([]T, len(items))
	copy(ordered, items)
	for i := 1; i < len(ordered); i++ {
		for j := i; j > 0 && len(opener(ordered[j])) > len(opener(ordered[j-1])); j-- {
			ordered[j], ordered[j-1] = ordered[j-1], ordered[j]
		}
	}
	return ordered
}
