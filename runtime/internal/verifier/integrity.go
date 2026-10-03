package verifier

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

// Integrity kinds. A run either reports a number from data it did not tune on,
// or says in writing why that question does not apply to it.
const (
	integrityHeldOut       = "held_out_eval"
	integrityNotApplicable = "not_applicable"
)

// integrityPrefix marks a problem the loop cannot fix by trying again. Looping
// back to hypothesis after one of these re-scores the reported split, which is
// the mechanism that turns a held-out set into a tuning set.
const integrityPrefix = "INTEGRITY: "

// Integrity is the evaluation record an author writes beside the metrics. It is
// self-reported, so it does not stop deliberate fraud; it stops the failures
// nobody notices, by forcing the facts that expose them into a file the
// verifier reads: which split drove choices, which split is reported, how many
// times that split was scored, and how far apart the two numbers are allowed to
// be. Values are declared in the approved PLAN before the run.
type Integrity struct {
	Kind string `json:"kind"`
	// Reason is required for not_applicable and ignored otherwise.
	Reason string `json:"reason,omitempty"`
	// SelectionSplit is where hyperparameters, prompts, features, checkpoints,
	// and thresholds were chosen (usually validation or cross-validation).
	SelectionSplit string `json:"selectionSplit,omitempty"`
	// ReportedSplit is the held-out split the headline metrics come from.
	ReportedSplit string `json:"reportedSplit,omitempty"`
	// SelectionMetrics holds the same metrics measured on SelectionSplit.
	SelectionMetrics map[string]float64 `json:"selectionMetrics,omitempty"`
	// MaxGap is the largest accepted absolute difference per gated metric.
	MaxGap map[string]float64 `json:"maxGap,omitempty"`
	// Trials is how many configurations were tried to reach this run, counting
	// failures. It is recorded so a reader can discount the best-of-N.
	Trials int `json:"trials,omitempty"`
	// ReportedSplitEvaluations is how many times ReportedSplit was scored over
	// the whole project. A held-out split scored twice is no longer held out.
	ReportedSplitEvaluations int `json:"reportedSplitEvaluations,omitempty"`
}

// checkIntegrity returns one problem per violated rule. An empty result means
// the record is internally consistent, not that the experiment is sound.
func checkIntegrity(doc MetricsDocument) []string {
	in := doc.Integrity
	if in == nil {
		return []string{integrityPrefix + "METRICS.json has no integrity block; declare kind " +
			integrityHeldOut + ", or " + integrityNotApplicable + " with a reason"}
	}
	switch in.Kind {
	case integrityNotApplicable:
		if strings.TrimSpace(in.Reason) == "" {
			return []string{integrityPrefix + integrityNotApplicable + " needs a reason"}
		}
		return nil
	case integrityHeldOut:
		return checkHeldOut(doc, in)
	default:
		return []string{fmt.Sprintf("%sintegrity kind %q is not %s or %s",
			integrityPrefix, in.Kind, integrityHeldOut, integrityNotApplicable)}
	}
}

func checkHeldOut(doc MetricsDocument, in *Integrity) []string {
	var problems []string
	add := func(format string, args ...any) {
		problems = append(problems, integrityPrefix+fmt.Sprintf(format, args...))
	}

	selection := strings.ToLower(strings.TrimSpace(in.SelectionSplit))
	reported := strings.ToLower(strings.TrimSpace(in.ReportedSplit))
	switch {
	case selection == "" || reported == "":
		add("selectionSplit and reportedSplit must both be named")
	case selection == reported:
		add("choices were made on %q and the result is reported on %q; a number measured on the data it was tuned on is not a held-out result", in.SelectionSplit, in.ReportedSplit)
	}
	if in.Trials < 1 {
		add("trials must count every configuration tried, at least 1")
	}
	switch {
	case in.ReportedSplitEvaluations < 1:
		add("reportedSplitEvaluations must be at least 1")
	case in.ReportedSplitEvaluations > 1:
		add("%q was scored %d times, so it is no longer held out; re-split, or call it a selection split and report on a fresh held-out set",
			in.ReportedSplit, in.ReportedSplitEvaluations)
	}

	names := make([]string, 0, len(doc.Thresholds))
	for name := range doc.Thresholds {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		problems = append(problems, checkGap(name, doc, in)...)
	}
	return problems
}

func checkGap(name string, doc MetricsDocument, in *Integrity) []string {
	chosen, ok := in.SelectionMetrics[name]
	if !ok || math.IsNaN(chosen) || math.IsInf(chosen, 0) {
		return []string{integrityPrefix + fmt.Sprintf("%s has no finite selectionMetrics value to compare against", name)}
	}
	limit, ok := in.MaxGap[name]
	if !ok || math.IsNaN(limit) || limit < 0 {
		return []string{integrityPrefix + fmt.Sprintf("%s has no maxGap; declare the accepted gap in the PLAN before the run", name)}
	}
	actual, recorded := doc.Metrics[name]
	if !recorded {
		return nil // already reported as "not recorded" by the threshold pass
	}
	gap := math.Abs(chosen - actual)
	if gap <= limit {
		return nil
	}
	return []string{integrityPrefix + fmt.Sprintf(
		"%s is %g on %q and %g on %q, a gap of %g against an accepted %g; %s",
		name, chosen, in.SelectionSplit, actual, in.ReportedSplit, gap, limit,
		gapCause(doc.Thresholds[name], chosen, actual))}
}

// gapCause names the likelier explanation. The direction comes from the
// threshold operator: >= and > mean higher is better, < and <= mean lower is.
func gapCause(threshold MetricThreshold, chosen, actual float64) string {
	higherIsBetter := true
	switch threshold.Op {
	case "<", "<=", "lt", "lte":
		higherIsBetter = false
	case "==", "eq":
		return "check for leakage into the selection split and for splits that are not comparable"
	}
	worse := actual < chosen
	if !higherIsBetter {
		worse = actual > chosen
	}
	if worse {
		return "suspect leakage into the selection split, or overfitting to it; do not tune against the reported split to close the gap"
	}
	return "better on the held-out split than on the selection split; suspect leakage into the held-out split or a split that is not comparable"
}
