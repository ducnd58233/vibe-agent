package verifier

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

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

// Verify checks the run's recorded ship result before it reads the file.
//
// release/REVIEW.md is written by the host, including the row that says /ship
// passed. On the auto path a NO-GO still reaches this node, so without this a
// file could overrule the ship check the run already recorded as failed. The
// recorded check is evidence; the row is a claim about it.
func (Release) Verify(_ context.Context, req Request) (Result, error) {
	if req.Slug == "" {
		return Result{}, errors.New("release verifier needs a slug")
	}
	manifest := state.ManifestPath(req.WorkspaceRoot, req.Slug)
	run, err := state.Load(manifest)
	if errors.Is(err, os.ErrNotExist) {
		return failResult(relativeTo(req.WorkspaceRoot, manifest),
			"no run state to read the ship check from; release readiness needs it", time.Now().UTC()), nil
	}
	if err != nil {
		return Result{}, fmt.Errorf("read run state: %w", err)
	}
	if ship, ok := run.Checks["ship"]; !ok || !ship.Passed {
		return failResult(relativeTo(req.WorkspaceRoot, manifest),
			"ship check has not passed in run state; a release file cannot overrule it", time.Now().UTC()), nil
	}
	return verifyReviewFile(req, "release", ReleaseReviewPath(req.WorkspaceRoot, req.Slug),
		"REVIEW.md missing; write release readiness review before verify",
		"release")
}
