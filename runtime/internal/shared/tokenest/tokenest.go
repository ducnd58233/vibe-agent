// Package tokenest estimates and bounds what text costs to read.
//
// Budgets across the runtime (fetch clipping, the repo map, memory recall,
// replayed session prefixes) share one estimate so "400 tokens" means the same
// thing everywhere. It is an approximation, named as one: being off costs a clip
// slightly early or late, never a wrong answer.
package tokenest

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// CharsPerToken is the ratio budgets are estimated with.
const CharsPerToken = 4

// Estimate approximates what a string costs to read.
func Estimate(text string) int {
	return (len(text) + CharsPerToken - 1) / CharsPerToken
}

// TruncateWords cuts text to at most maxChars bytes at a word boundary and marks
// the cut with an ellipsis. Text already within the limit is returned unchanged.
func TruncateWords(text string, maxChars int) string {
	text = strings.TrimSpace(text)
	if maxChars <= 0 || len(text) <= maxChars {
		return text
	}
	cut := maxChars
	for cut > 0 && !utf8.RuneStart(text[cut]) {
		cut--
	}
	head := text[:cut]
	if space := strings.LastIndexAny(head, " \t\n"); space > maxChars/2 {
		head = head[:space]
	}
	return strings.TrimRight(head, " \t\n.,;:") + "..."
}

// HeadTail keeps the start and the end of text and drops the middle, saying how
// much was dropped. Capping each observation at a stable size, rather than
// summarizing it, is what recent agent-context studies found cuts cost without
// losing task success: it never rewrites what came before it, and the two ends
// of a log or a message are where the question and the conclusion usually are.
func HeadTail(text string, maxChars int) string {
	if maxChars <= 0 || len(text) <= maxChars {
		return text
	}
	marker := func(omitted int) string { return fmt.Sprintf("\n[... %d chars omitted ...]\n", omitted) }
	budget := maxChars - len(marker(len(text)))
	if budget < 2 {
		return TruncateWords(text, maxChars)
	}
	headLen := budget * 6 / 10
	tailLen := budget - headLen
	for headLen > 0 && !utf8.RuneStart(text[headLen]) {
		headLen--
	}
	tailStart := len(text) - tailLen
	for tailStart < len(text) && !utf8.RuneStart(text[tailStart]) {
		tailStart++
	}
	return text[:headLen] + marker(tailStart-headLen) + text[tailStart:]
}
