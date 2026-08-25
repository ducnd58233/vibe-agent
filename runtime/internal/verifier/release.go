package verifier

import (
	"context"
	"path/filepath"

	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
)

// ReleaseReviewFile is the basename under release/ in the run directory.
const ReleaseReviewFile = "REVIEW.md"

// Release reads .agent-state/runs/.../release/REVIEW.md on the auto path after ship.
type Release struct{}

func (Release) Kind() string { return "release" }

// ReleaseReviewPath is where the host agent must keep release REVIEW.md.
func ReleaseReviewPath(workspaceRoot, slug string) string {
	dir := state.RunDir(workspaceRoot, slug)
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "release", ReleaseReviewFile)
}

func (Release) Verify(_ context.Context, req Request) (Result, error) {
	return verifyReviewFile(req, "release", ReleaseReviewPath(req.WorkspaceRoot, req.Slug),
		"REVIEW.md missing; write release readiness review before verify",
		"release")
}
