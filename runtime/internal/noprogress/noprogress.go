// Package noprogress flags a retried research/hypothesis artifact that is
// byte-identical, or nearly so, to the version that already failed once at
// results_eval. See AGENTS.md "Blocker vs. retry" and
// docs/2026-09-24/research-experiment-persistence/1/RESEARCH-2026-09-24.md:
// the graph's retry loop already works automatically, but nothing checked
// whether a resubmission actually tried something different, so a host
// agent could satisfy the graph mechanically while its content repeats a
// failed attempt. The comparison is a plain line-multiset difference, never
// a model self-report - AGENTS.md keeps no source for model assertion.
package noprogress

import "strings"

// MinChangedLines is the bar a resubmission must clear to count as a genuine
// revision. A fixed, non-judgment threshold, the same kind slop audit
// already uses elsewhere in this repo.
const MinChangedLines = 3

// Check reports whether current counts as progress against previous, and how
// many lines differ. An empty previous means this is the first visit, which
// is always allowed - there is nothing yet to have repeated.
func Check(previous, current string) (progressed bool, changedLines int) {
	if previous == "" {
		return true, 0
	}
	changed := changedLineCount(previous, current)
	return changed >= MinChangedLines, changed
}

// changedLineCount is a symmetric multiset difference over trimmed,
// non-blank lines: how many line-instances exist in one side and not the
// other. Deliberately not a sequence diff - reordering unchanged lines
// costs nothing, which is the right call for prose, and a real diff
// algorithm is more machinery than this one comparison needs.
func changedLineCount(previous, current string) int {
	previousLines := lineCounts(previous)
	currentLines := lineCounts(current)

	changed := 0
	for line, count := range previousLines {
		if extra := count - currentLines[line]; extra > 0 {
			changed += extra
		}
	}
	for line, count := range currentLines {
		if extra := count - previousLines[line]; extra > 0 {
			changed += extra
		}
	}
	return changed
}

func lineCounts(content string) map[string]int {
	counts := make(map[string]int)
	for line := range strings.SplitSeq(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		counts[trimmed]++
	}
	return counts
}
