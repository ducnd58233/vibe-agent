package verifier

import (
	"context"
	"path/filepath"

	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
)

// TaskReviewFile is the basename under task/ in the run directory.
const TaskReviewFile = "REVIEW.md"

// TaskReview reads .agent-state/runs/.../task/REVIEW.md for a non-code deliverable:
// one row per acceptance row in the SPEC, each with an evidence path and a pass
// or fail.
//
// It is its own verifier, and its own file, rather than a second use of
// expectation. The goal graph's auto path already writes expectation/REVIEW.md
// for the code under review, and a delivery segment on the same run would have
// read and overwritten that file.
type TaskReview struct{}

func (TaskReview) Kind() string { return "taskreview" }

// TaskReviewPath is where the host agent must keep the task REVIEW.md.
func TaskReviewPath(workspaceRoot, slug string) string {
	dir := state.RunDir(workspaceRoot, slug)
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "task", TaskReviewFile)
}

func (TaskReview) Verify(_ context.Context, req Request) (Result, error) {
	return verifyReviewFile(req, "taskreview", TaskReviewPath(req.WorkspaceRoot, req.Slug),
		"REVIEW.md missing; write the task review before verify",
		"task")
}
