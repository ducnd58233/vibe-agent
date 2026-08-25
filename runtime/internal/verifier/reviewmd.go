package verifier

import (
	"regexp"
	"strconv"
	"strings"
)

// SoftAttemptCap is the documented host stop after consecutive review misses.
// The verifier still fails above this; the host must not keep refining forever.
const SoftAttemptCap = 2

var (
	reviewStatusLine  = regexp.MustCompile(`(?im)^\s*status:\s*(pass|fail)\s*$`)
	reviewAttemptLine = regexp.MustCompile(`(?im)^\s*attempt:\s*(\d+)\s*$`)
	reviewResultCell  = regexp.MustCompile(`(?i)\|\s*(pass|fail)\s*\|\s*$`)
)

// reviewMarkdown is the shared contract for expectation/release/bug_hunt REVIEW files.
type reviewMarkdown struct {
	Status   string
	Attempt  int
	FailRows int
}

func parseReviewMarkdown(body string) (reviewMarkdown, string) {
	statusMatch := reviewStatusLine.FindStringSubmatch(body)
	if statusMatch == nil {
		return reviewMarkdown{}, "no status: pass|fail line"
	}
	out := reviewMarkdown{
		Status:  strings.ToLower(statusMatch[1]),
		Attempt: 1,
	}
	if attemptMatch := reviewAttemptLine.FindStringSubmatch(body); attemptMatch != nil {
		if n, convErr := strconv.Atoi(attemptMatch[1]); convErr == nil && n > 0 {
			out.Attempt = n
		}
	}
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "|") {
			continue
		}
		if strings.Contains(trimmed, "---") {
			continue
		}
		cell := reviewResultCell.FindStringSubmatch(trimmed)
		if cell == nil {
			continue
		}
		if strings.EqualFold(cell[1], "fail") {
			out.FailRows++
		}
	}
	return out, ""
}

func (r reviewMarkdown) failed() bool {
	return r.Status == "fail" || r.FailRows > 0
}
