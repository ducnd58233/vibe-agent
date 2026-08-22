package auto

import (
	"regexp"
	"strings"
)

// Structural rules for researcher-delivery gates. These are still tests on the
// document, not judgements: a missing heading or fence is something the author
// left out in writing.
const (
	RuleMissingApplicability = "missing-applicability"
	RuleMissingRefine        = "missing-refine"
	RuleMissingMermaid       = "missing-mermaid"
)

var (
	applicabilityHeading = regexp.MustCompile(`(?i)^#{1,6}\s+applicability\b`)
	refineHeading        = regexp.MustCompile(`(?i)^#{1,6}\s+refine\b`)
	mermaidFence         = regexp.MustCompile("(?m)^```mermaid\\s*$")
)

// RequireApplicability reports when RESEARCH lacks Applicability, Refine, or a
// Mermaid fence. Empty result means the structural tests passed, not that the
// research is good.
func RequireApplicability(document string) []Ambiguity {
	var found []Ambiguity
	if !sectionHasContent(document, applicabilityHeading) {
		found = append(found, Ambiguity{
			Rule: RuleMissingApplicability,
			Line: 1,
			Text: "RESEARCH needs an Applicability section that maps sources to this topic",
		})
	}
	if !sectionHasContent(document, refineHeading) {
		found = append(found, Ambiguity{
			Rule: RuleMissingRefine,
			Line: 1,
			Text: "RESEARCH needs a Refine section stating what to change before experiments",
		})
	}
	if !mermaidFence.MatchString(document) {
		found = append(found, Ambiguity{
			Rule: RuleMissingMermaid,
			Line: 1,
			Text: "RESEARCH needs a fenced ```mermaid diagram",
		})
	}
	return found
}

// RequireExperimentDiagram reports when an experiment PLAN lacks a Mermaid fence.
func RequireExperimentDiagram(document string) []Ambiguity {
	if mermaidFence.MatchString(document) {
		return nil
	}
	return []Ambiguity{{
		Rule: RuleMissingMermaid,
		Line: 1,
		Text: "experiment PLAN needs a fenced ```mermaid setup diagram",
	}}
}

func sectionHasContent(document string, heading *regexp.Regexp) bool {
	inSection := false
	for _, line := range strings.Split(document, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case heading.MatchString(trimmed):
			inSection = true
			continue
		case anyHeading.MatchString(trimmed):
			if inSection {
				return false
			}
		}
		if inSection && trimmed != "" && !noneItem.MatchString(trimmed) {
			return true
		}
	}
	return false
}
