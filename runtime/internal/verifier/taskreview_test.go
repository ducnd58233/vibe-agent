package verifier

import "testing"

func TestTaskReviewReadsItsOwnFile(t *testing.T) {
	root := t.TempDir()
	body := `# Task review
status: pass
attempt: 1

| AC id | Spec reference | Observed evidence path | result |
|-------|----------------|------------------------|--------|
| AC1 | SPEC | drafts/1.md | pass |
`
	allocateNamedReview(t, root, "tr-pass", "task", TaskReviewFile, body)
	result, err := TaskReview{}.Verify(t.Context(), Request{Slug: "tr-pass", WorkspaceRoot: root})
	if err != nil || !result.Check.Passed {
		t.Fatalf("pass: %v %s", err, result.Summary)
	}

	// The expectation file is a different file and must not satisfy this one.
	allocateNamedReview(t, root, "tr-other", "expectation", ExpectationReviewFile, body)
	result, err = TaskReview{}.Verify(t.Context(), Request{Slug: "tr-other", WorkspaceRoot: root})
	if err != nil || result.Check.Passed {
		t.Fatalf("an expectation file satisfied the task review: %v %s", err, result.Summary)
	}
}

func TestTaskReviewFailsOnAFailRow(t *testing.T) {
	root := t.TempDir()
	body := `status: fail
attempt: 1

| AC id | Spec reference | Observed evidence path | result |
|---|---|---|---|
| AC1 | SPEC | x | fail |
`
	allocateNamedReview(t, root, "tr-fail", "task", TaskReviewFile, body)
	result, _ := TaskReview{}.Verify(t.Context(), Request{Slug: "tr-fail", WorkspaceRoot: root})
	if result.Check.Passed {
		t.Fatal("a failed row passed")
	}
}
