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

	RuleMissingEvaluationProtocol = "missing-evaluation-protocol"
	RuleMissingDataTerms          = "missing-data-terms"
)

var (
	applicabilityHeading = regexp.MustCompile(`(?i)^#{1,6}\s+applicability\b`)
	refineHeading        = regexp.MustCompile(`(?i)^#{1,6}\s+refine\b`)
	mermaidFence         = regexp.MustCompile("(?m)^```mermaid\\s*$")

	evaluationProtocolHeading = regexp.MustCompile(`(?i)^#{1,6}\s+evaluation\s+protocol\b`)
	dataTermsHeading          = regexp.MustCompile(`(?i)^#{1,6}\s+data\s+and\s+terms\b`)
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

// RequireEvaluationProtocol reports when an experiment PLAN does not commit, in
// writing and before the run, to how the result will be judged and where its
// data came from. The sections must exist and say something; whether what they
// say is sound is for the human gate and the results verifier. A PLAN with no
// data and no model to judge may say so in a sentence, but a bare "N/A" does not
// count, so the author has to state why.
func RequireEvaluationProtocol(document string) []Ambiguity {
	var found []Ambiguity
	if !sectionHasContent(document, evaluationProtocolHeading) {
		found = append(found, Ambiguity{
			Rule: RuleMissingEvaluationProtocol,
			Line: 1,
			Text: "experiment PLAN needs an Evaluation protocol section: splits, what is tuned on which split, frozen metric and thresholds, trial budget",
		})
	}
	if !sectionHasContent(document, dataTermsHeading) {
		found = append(found, Ambiguity{
			Rule: RuleMissingDataTerms,
			Line: 1,
			Text: "experiment PLAN needs a Data and terms section: source, license, terms of use, consent or PII, and model-provider terms for any model output reused as data",
		})
	}
	return found
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
