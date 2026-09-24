package checkpoint

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ducnd58233/vibe-agent/runtime/internal/noprogress"
	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
	// Aliased: this package's own test file already declares a workspace(t)
	// helper, and the two names would collide in the same package.
	sharedworkspace "github.com/ducnd58233/vibe-agent/runtime/internal/shared/workspace"
)

// noProgressTargets names, per graph, the one artifact-node whose retried
// content this checks for a genuine revision, and the output filename
// template (only ${date} varies; slug/version are the run's own). Hardcoded
// to these two rather than generalized from the graph's own outputs: field,
// which the runtime does not otherwise resolve - building a template
// interpolator for one caller would be infrastructure nobody else needs yet.
var noProgressTargets = map[string]struct{ node, filename string }{
	"goal-delivery":       {node: "auto_research", filename: "RESEARCH-${date}.md"},
	"researcher-delivery": {node: "hypothesis", filename: "HYPOTHESIS-${date}.md"},
}

// checkProgress refuses to advance past this delivery's research-retry node
// when the artifact it produced repeats, or nearly repeats, the version
// already on disk from a prior visit. See AGENTS.md "Blocker vs. retry" and
// package noprogress. A run whose graph or current node is not one of the
// two targets, or that has no versioned docs tree yet, is unaffected.
func checkProgress(workspaceRoot string, run *state.Run) error {
	target, ok := noProgressTargets[run.GraphID]
	if !ok || target.node != run.CurrentNode || run.Date == "" || run.Version < 1 {
		return nil
	}

	artifactPath := filepath.Join(
		sharedworkspace.DocsDirAt(workspaceRoot, run.Date, run.Slug, run.Version),
		strings.ReplaceAll(target.filename, "${date}", run.Date),
	)
	current, err := os.ReadFile(filepath.Clean(artifactPath))
	if errors.Is(err, os.ErrNotExist) {
		return nil // nothing written yet is a different failure, not this check's job
	}
	if err != nil {
		return fmt.Errorf("read %s: %w", artifactPath, err)
	}

	// run.CurrentNode is ordinarily a graph node id, but it comes from a
	// manifest on disk, not a compile-time constant, so a value like
	// "../../../outside" must not reach a write. os.Root gives kernel-enforced
	// containment under runDir specifically - not merely somewhere in the
	// wider workspace, which a plain filepath.Rel check against workspaceRoot
	// would still allow (confirmed by trying it: such a value escaped runDir
	// into an unrelated sibling directory while staying inside the workspace).
	runDir := sharedworkspace.RunDirAt(workspaceRoot, run.Date, run.Slug, run.Version)
	if err := os.MkdirAll(runDir, 0o750); err != nil {
		return fmt.Errorf("prepare run directory: %w", err)
	}
	root, err := os.OpenRoot(runDir)
	if err != nil {
		return fmt.Errorf("open run directory: %w", err)
	}
	defer func() { _ = root.Close() }()

	snapshotName := filepath.Join(run.CurrentNode, "snapshot.md")
	previous, err := root.ReadFile(snapshotName)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read %s: %w", snapshotName, err)
	}

	progressed, changed := noprogress.Check(string(previous), string(current))
	if !progressed {
		return fmt.Errorf(
			"%s at %s repeats the version already tried once (%d line(s) differ, need at least %d): "+
				"revise the approach before retrying, do not resubmit the same content",
			filepath.Base(artifactPath), run.CurrentNode, changed, noprogress.MinChangedLines)
	}

	if err := root.MkdirAll(run.CurrentNode, 0o750); err != nil {
		return fmt.Errorf("prepare no-progress snapshot dir: %w", err)
	}
	if err := root.WriteFile(snapshotName, current, 0o600); err != nil {
		return fmt.Errorf("write no-progress snapshot: %w", err)
	}
	return nil
}
