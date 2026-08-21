package tasks

import (
	"strings"
	"testing"
)

func TestAcceptanceIncompleteRequiresCheckedBoxes(t *testing.T) {
	prose := `
## T1: Example  [done]

**Description:** demo

**Acceptance criteria:**
- [x] first
- [ ] second

**Verification:** go test
`
	if reason := AcceptanceIncomplete(prose, "T1"); reason == "" {
		t.Fatal("an open acceptance box did not block done")
	}
}

func TestAcceptanceIncompleteAllowsFullyChecked(t *testing.T) {
	prose := `
## T1: Example  [done]

**Acceptance criteria:**
- [x] first
- [X] second
`
	if reason := AcceptanceIncomplete(prose, "T1"); reason != "" {
		t.Fatalf("fully checked AC still incomplete: %s", reason)
	}
}

func TestRemainingAgainstProseKeepsUncheckedDone(t *testing.T) {
	file := &File{
		Tasks: []Task{
			{ID: "T1", Title: "a", Status: StatusDone},
			{ID: "T2", Title: "b", Status: StatusCanceled},
		},
	}
	prose := `
## T1: a  [done]
**Acceptance criteria:**
- [ ] still open
## T2: b  [canceled]
`
	remaining := file.RemainingAgainstProse(prose)
	if len(remaining) != 1 || remaining[0].ID != "T1" {
		t.Fatalf("remaining = %+v, want only T1", remaining)
	}
}

func TestRemainingAgainstProseTreatsMissingProseAsIncompleteDone(t *testing.T) {
	file := &File{
		Tasks: []Task{{ID: "T1", Title: "a", Status: StatusDone}},
	}
	if remaining := file.RemainingAgainstProse(""); len(remaining) != 1 {
		t.Fatalf("done without prose settled; remaining=%+v", remaining)
	}
}

func TestTaskSectionFindsHashTwoAndThree(t *testing.T) {
	prose := "### T1: Title  [queued]\n\nbody\n\n## T2: Other\n"
	got := TaskSection(prose, "T1")
	if got == "" || !strings.Contains(got, "body") {
		t.Fatalf("section = %q", got)
	}
}
