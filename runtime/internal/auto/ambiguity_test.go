package auto

import (
	"strings"
	"testing"

	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/validate"
)

// The rule is a test on the document, not a judgement about it. These cases are
// the documentation of what it does and does not catch.
func TestScanFindsWhatAnAuthorParked(t *testing.T) {
	cases := []struct {
		name     string
		document string
		want     string
	}{
		{
			name: "a populated open questions section",
			document: strings.Join([]string{
				"# Spec", "## Open questions", "", "- Which store backs the queue?",
			}, "\n"),
			want: RuleOpenQuestion,
		},
		{
			name:     "a placeholder in the prose",
			document: "The retry ceiling is TBD.",
			want:     RulePlaceholder,
		},
		{
			name:     "an unresolved marker",
			document: "Ownership of the migration is UNRESOLVED.",
			want:     RulePlaceholder,
		},
		{
			name:     "a row of question marks",
			document: "| timeout | ??? |",
			want:     RulePlaceholder,
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			found := Scan(testCase.document)
			if len(found) == 0 {
				t.Fatalf("nothing found in:\n%s", testCase.document)
			}
			if found[0].Rule != testCase.want {
				t.Errorf("rule = %q, want %q", found[0].Rule, testCase.want)
			}
			if found[0].Line == 0 || found[0].Text == "" {
				t.Errorf("a finding has to say where and what: %+v", found[0])
			}
		})
	}
}

// A rule that fires on ordinary prose stops being used. These must stay quiet.
func TestScanLeavesASettledDocumentAlone(t *testing.T) {
	document := strings.Join([]string{
		"# Spec",
		"## Open questions",
		"",
		"- None.",
		"",
		"## Assumptions",
		"",
		"- The queue is at-least-once, which the design already handles.",
		"",
		"## Non-goals",
		"",
		"- Rewriting the scheduler. That was decided against and is not open.",
	}, "\n")

	if found := Scan(document); len(found) != 0 {
		t.Errorf("a settled spec was called ambiguous:\n%s", Report(found))
	}
}

// The section ends at the next heading, or every list item after it counts.
func TestTheOpenQuestionsSectionEndsAtTheNextHeading(t *testing.T) {
	document := strings.Join([]string{
		"## Open questions", "", "- None.", "",
		"## Tasks", "", "- Build the thing", "- Test the thing",
	}, "\n")

	if found := Scan(document); len(found) != 0 {
		t.Errorf("list items outside the section were counted:\n%s", Report(found))
	}
}

func TestReportSaysWhereAndWhichRule(t *testing.T) {
	report := Report(Scan("line one\nThe ceiling is TBD."))
	for _, want := range []string{"line 2", RulePlaceholder, "TBD"} {
		if !strings.Contains(report, want) {
			t.Errorf("report does not contain %q:\n%s", want, report)
		}
	}
}

func TestSlugifyProducesSomethingRunStateAccepts(t *testing.T) {
	for _, goal := range []string{
		"Add a retry ceiling to the webhook dispatcher",
		"FIX: the CI cache key (issue #412)",
		"migrate   the   queue",
	} {
		slug := Slugify(goal, 4)
		if slug == "" {
			t.Errorf("%q produced no slug", goal)
			continue
		}
		if !validate.Slug(slug) {
			t.Errorf("%q produced %q, which run state would refuse", goal, slug)
		}
		if strings.Count(slug, "-") > 3 {
			t.Errorf("%q produced %q, longer than the bound", goal, slug)
		}
	}
}

func TestSlugifyDropsTheWordsEveryGoalShares(t *testing.T) {
	if slug := Slugify("Add a retry ceiling to the dispatcher", 4); strings.HasPrefix(slug, "add-a-") {
		t.Errorf("slug = %q, want the filler words dropped", slug)
	}
}

// A goal with nothing usable in it fails here rather than three calls later.
func TestSlugifyReturnsNothingForAGoalWithNoWords(t *testing.T) {
	if slug := Slugify("!!! ??? ...", 4); slug != "" {
		t.Errorf("slug = %q, want empty", slug)
	}
}
