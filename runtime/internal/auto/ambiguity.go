// Package auto holds the parts of auto mode that are decisions rather than
// plumbing: when a document still needs a person, and how text from somewhere
// else enters a prompt.
//
// Both live here rather than in cmd because both are rules, and a rule that
// only exists inside a command handler cannot be tested without running the
// command.
package auto

import (
	"fmt"
	"regexp"
	"strings"
)

// Ambiguity is one reason a document still needs a person.
type Ambiguity struct {
	// Rule names which of the two tests below fired, so a report can say what
	// was found rather than only that something was.
	Rule string
	Line int
	Text string
}

// The two rules, and why these two.
//
// Auto mode has one content-based stop, and it has to be a test rather than a
// judgement, or the model is deciding whether the model may proceed. So the
// question is not "is this spec good" but "does this spec say, in writing, that
// something is undecided". A document that says so is the author asking for
// someone; a document that does not is not evidence that nothing is missing,
// and the toolkit does not claim it is.
//
// Both rules match what a person actually writes when they park a decision.
// Neither infers anything.
const (
	// RuleOpenQuestion is a populated "Open questions" section.
	RuleOpenQuestion = "open-question"
	// RulePlaceholder is a marker left in the prose.
	RulePlaceholder = "placeholder"
)

var (
	openQuestionsHeading = regexp.MustCompile(`(?i)^#{1,6}\s+open\s+questions?\b`)
	anyHeading           = regexp.MustCompile(`^#{1,6}\s+`)
	listItem             = regexp.MustCompile(`^\s*(?:[-*+]|\d+[.)])\s+(.*\S)`)
	// TBD and friends. Word-bounded so "unresolved" inside a sentence about
	// something else does not fire, and case-insensitive because nobody is
	// consistent about it.
	placeholder = regexp.MustCompile(`(?i)\b(TBD|UNRESOLVED|TO\s?BE\s?DECIDED)\b|\?{3,}`)
	// "None", "n/a", and "none yet" under an Open questions heading are the
	// author saying there are none. Treating them as questions would mean a
	// spec could never close the section it was asked to fill in.
	noneItem = regexp.MustCompile(`(?i)^(none|n/?a|nothing|none\s+yet|no\s+open\s+questions?)\.?$`)
)

// Scan reports every place a document says a decision is still open.
//
// An empty result is not a promise the document is complete. It means the
// document does not say otherwise, which is the most a text search can
// honestly claim, and it is why the gate this feeds is skipped rather than
// recorded as approved.
func Scan(document string) []Ambiguity {
	var found []Ambiguity
	inOpenQuestions := false

	for index, line := range strings.Split(document, "\n") {
		number := index + 1
		trimmed := strings.TrimSpace(line)

		switch {
		case openQuestionsHeading.MatchString(trimmed):
			inOpenQuestions = true
			continue
		case anyHeading.MatchString(trimmed):
			inOpenQuestions = false
		}

		if inOpenQuestions {
			if item := listItem.FindStringSubmatch(line); item != nil && !noneItem.MatchString(item[1]) {
				found = append(found, Ambiguity{Rule: RuleOpenQuestion, Line: number, Text: item[1]})
				continue
			}
		}
		if match := placeholder.FindString(line); match != "" {
			found = append(found, Ambiguity{Rule: RulePlaceholder, Line: number, Text: trimmed})
		}
	}
	return found
}

// Report renders findings for a person reading a terminal.
func Report(findings []Ambiguity) string {
	lines := make([]string, 0, len(findings))
	for _, finding := range findings {
		lines = append(lines, fmt.Sprintf("  line %d [%s] %s", finding.Line, finding.Rule, finding.Text))
	}
	return strings.Join(lines, "\n")
}

// Slugify turns a one-line objective into a run slug.
//
// A slug is derived rather than asked for so that one prompt is genuinely all
// auto mode needs. It is bounded at a handful of words because a slug names a
// directory a person will type, and it is checked by the same validator run
// state uses, so a goal that cannot produce a usable one fails here rather than
// three calls later.
func Slugify(goal string, words int) string {
	var kept []string
	for _, word := range strings.FieldsFunc(strings.ToLower(goal), func(r rune) bool {
		return (r < 'a' || r > 'z') && (r < '0' || r > '9')
	}) {
		if skipWord[word] {
			continue
		}
		kept = append(kept, word)
		if len(kept) == words {
			break
		}
	}
	return strings.Join(kept, "-")
}

// Words carried by almost every objective, which make one slug look like the
// next. Dropping them is cosmetic and deliberately short of a stopword list:
// removing more would start losing what the goal was about.
var skipWord = map[string]bool{
	"a": true, "an": true, "the": true, "to": true, "of": true,
	"for": true, "and": true, "in": true, "on": true, "with": true,
}
