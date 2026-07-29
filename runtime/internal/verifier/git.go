package verifier

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/state"
)

// GitExpectation is what a git verifier should assert. Zero values mean
// "do not care", so a node can check the tree is clean without pinning a branch.
type GitExpectation struct {
	// CleanTree requires no uncommitted changes.
	CleanTree bool
	// Branch, when set, requires that branch to be checked out.
	Branch string
	// NotBranch, when set, requires that branch not to be checked out. Used to
	// keep work off main.
	NotBranch string
}

// Observation is what git reported.
type Observation struct {
	Branch string
	Head   string
	Clean  bool
}

// Git observes repository state.
//
// It reports; it does not act. Nothing here commits, pushes, or checks out, so
// a verifier can never move the repository while claiming to measure it.
type Git struct{}

func (Git) Kind() string { return "git" }

func (g Git) Verify(ctx context.Context, req Request) (Result, error) {
	observation, err := Observe(ctx, req.WorkspaceRoot)
	if err != nil {
		return Result{}, err
	}

	var failures []string
	if req.Expect.CleanTree && !observation.Clean {
		failures = append(failures, "working tree has uncommitted changes")
	}
	if req.Expect.Branch != "" && observation.Branch != req.Expect.Branch {
		failures = append(failures, fmt.Sprintf("on branch %q, want %q", observation.Branch, req.Expect.Branch))
	}
	if req.Expect.NotBranch != "" && observation.Branch == req.Expect.NotBranch {
		failures = append(failures, fmt.Sprintf("on branch %q, which this step must not run on", observation.Branch))
	}

	summary := fmt.Sprintf("branch %s at %s", observation.Branch, shortSHA(observation.Head))
	if observation.Clean {
		summary += ", clean"
	} else {
		summary += ", dirty"
	}
	if len(failures) > 0 {
		summary += " - " + strings.Join(failures, "; ")
	}

	return Result{
		Check: state.Check{
			Passed: len(failures) == 0,
			Source: state.SourceFileAssert,
			Ref:    observation.Head,
			At:     time.Now().UTC(),
		},
		Summary: summary,
		Detail:  summary,
	}, nil
}

// Observe reads branch, head, and cleanliness from a repository.
func Observe(ctx context.Context, root string) (Observation, error) {
	branch, err := git(ctx, root, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return Observation{}, fmt.Errorf("read branch: %w", err)
	}
	head, err := git(ctx, root, "rev-parse", "HEAD")
	if err != nil {
		return Observation{}, fmt.Errorf("read head: %w", err)
	}
	status, err := git(ctx, root, "status", "--porcelain")
	if err != nil {
		return Observation{}, fmt.Errorf("read status: %w", err)
	}
	return Observation{Branch: branch, Head: head, Clean: status == ""}, nil
}

func git(ctx context.Context, root string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = root
	var out, errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(errBuf.String()))
	}
	return strings.TrimSpace(out.String()), nil
}

func shortSHA(sha string) string {
	if len(sha) > 8 {
		return sha[:8]
	}
	return sha
}
