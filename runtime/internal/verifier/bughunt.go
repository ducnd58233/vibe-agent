package verifier

import (
	"context"
	"path/filepath"

	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
)

// BugHuntFindingsFile is the basename under bug_hunt/ in the run directory.
const BugHuntFindingsFile = "FINDINGS.md"

// BugHunt reads .agent-state/runs/.../bug_hunt/FINDINGS.md on the auto path after e2e.
type BugHunt struct{}

func (BugHunt) Kind() string { return "bughunt" }

// BugHuntFindingsPath is where the host agent must keep FINDINGS.md.
func BugHuntFindingsPath(workspaceRoot, slug string) string {
	dir := state.RunDir(workspaceRoot, slug)
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "bug_hunt", BugHuntFindingsFile)
}

func (BugHunt) Verify(_ context.Context, req Request) (Result, error) {
	return verifyReviewFile(req, "bughunt", BugHuntFindingsPath(req.WorkspaceRoot, req.Slug),
		"FINDINGS.md missing; write bug-hunt findings before verify",
		"bug_hunt")
}
