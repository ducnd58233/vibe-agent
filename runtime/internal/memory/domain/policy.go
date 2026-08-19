package domain

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/redact"
)

// Verdict is what the policy filter decided about a candidate.
type Verdict string

const (
	VerdictStore  Verdict = "store"
	VerdictMerge  Verdict = "merge"
	VerdictReject Verdict = "reject"
)

// Decision is the filter's answer plus why.
type Decision struct {
	Verdict Verdict
	Reason  string
	// MergeInto is the existing record id when Verdict is merge.
	MergeInto string
}

// secretPatterns catch candidates that would put a credential in a database
// that gets read back into a model's context. Detection is deliberately broad:
// a false positive costs one rejected memory, a false negative leaks a secret.
var secretPatterns = joinSecretPatterns(
	redact.LiteralPatterns(),
	redact.ContextualPatterns(),
)

func joinSecretPatterns(groups ...[]*regexp.Regexp) []*regexp.Regexp {
	var n int
	for _, g := range groups {
		n += len(g)
	}
	out := make([]*regexp.Regexp, 0, n)
	for _, g := range groups {
		out = append(out, g...)
	}
	return out
}

// hedgePatterns catch candidates that record a guess rather than an
// observation. "Probably needs Redis" is not worth remembering; it is worth
// checking.
var hedgePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\b(probably|maybe|might|perhaps|seems? to|i think|possibly|could be|not sure|appears to)\b`),
}

// transientPatterns catch task state that belongs in a run manifest, not in
// durable memory. Storing it makes the next run act on a finished task.
var transientPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\b(currently (working|running)|for now|temporar(y|ily)|todo|fixme|wip|in progress|next step is|about to)\b`),
	regexp.MustCompile(`(?i)\bon branch\s+\S+`),
}

// Filter decides whether a candidate becomes a stored memory.
//
// The failure mode this exists to prevent is the one most agent frameworks
// have: saving everything, then retrieving noise, secrets, and stale guesses
// into the next run's context.
type Filter struct {
	// Existing is consulted for duplicates. Nil means no merge detection.
	Existing []Record
}

// Decide applies the policy to a candidate.
func (f Filter) Decide(candidate Record) Decision {
	if err := candidate.Validate(); err != nil {
		return Decision{VerdictReject, err.Error(), ""}
	}

	// A candidate arriving as confirmed is a caller trying to skip the gate.
	if candidate.Status == StatusConfirmed {
		return Decision{VerdictReject, "a candidate may only be proposed; confirmation comes from a verifier result or a human event", ""}
	}

	haystack := candidate.Content + " " + strings.Join(candidate.Evidence, " ")

	for _, pattern := range secretPatterns {
		if pattern.MatchString(haystack) {
			return Decision{VerdictReject, "looks like a credential; secrets are never stored in memory", ""}
		}
	}
	for _, pattern := range transientPatterns {
		if pattern.MatchString(candidate.Content) {
			return Decision{VerdictReject, "reads as task state, which belongs in the run manifest rather than durable memory", ""}
		}
	}
	for _, pattern := range hedgePatterns {
		if pattern.MatchString(candidate.Content) {
			return Decision{VerdictReject, "reads as a guess rather than an observation; record what was observed", ""}
		}
	}
	if !hasSubstance(candidate.Evidence) {
		return Decision{VerdictReject, "evidence is too thin to support the claim", ""}
	}

	for _, existing := range f.Existing {
		if existing.WorkspaceID != candidate.WorkspaceID || existing.Kind != candidate.Kind {
			continue
		}
		if existing.Status == StatusStale || existing.Status == StatusRejected {
			continue
		}
		if normalize(existing.Content) == normalize(candidate.Content) {
			return Decision{VerdictMerge, "duplicates an existing memory", existing.ID}
		}
	}

	return Decision{VerdictStore, "evidence-backed and reusable", ""}
}

// hasSubstance rejects evidence that is technically present but says nothing.
func hasSubstance(evidence []string) bool {
	for _, item := range evidence {
		if len(strings.TrimSpace(item)) >= 12 {
			return true
		}
	}
	return false
}

func normalize(text string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimRight(text, ". "))), " ")
}

// PromotionThreshold is how many successful reuses make a memory worth turning
// into a durable rule.
const PromotionThreshold = 3

// Promotion is a suggestion that a memory has earned a permanent home.
type Promotion struct {
	Record Record
	// Target is where the rule should go.
	Target string
	Reason string
}

// ProposePromotions finds memories reused often enough to belong in a reviewed,
// versioned rule instead of a database.
//
// It proposes. It does not write. A self-editing rule file is exactly the kind
// of unreviewed autonomy this design constrains.
func ProposePromotions(records []Record) []Promotion {
	var promotions []Promotion
	for _, record := range records {
		if record.Status != StatusConfirmed || record.UsedCount < PromotionThreshold {
			continue
		}
		promotions = append(promotions, Promotion{
			Record: record,
			Target: promotionTarget(record.Kind),
			Reason: fmt.Sprintf("used successfully %d times; a durable rule is reviewable and versioned, a memory row is not", record.UsedCount),
		})
	}
	return promotions
}

func promotionTarget(kind Kind) string {
	switch kind {
	case KindSemantic:
		return "AGENTS.md or a stack profile, plus a deterministic preflight check where one is possible"
	case KindPreference, KindCorrection:
		return "AGENTS.md conventions, or the workspace CLAUDE.local.md when it is personal"
	default:
		return "AGENTS.md"
	}
}
