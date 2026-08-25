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

// ExpectationReviewFile is the basename under expectation/ in the run directory.
const ExpectationReviewFile = "REVIEW.md"

// Expectation reads .agent-state/runs/.../expectation/REVIEW.md.
//
// Passed only when status is pass and every table result cell is pass. Missing
// or malformed files fail with file_assert so auto mode can reopen plan without
// a judge LLM as evidence.
type Expectation struct{}

func (Expectation) Kind() string { return "expectation" }

// ExpectationReviewPath is where the host agent must keep REVIEW.md.
func ExpectationReviewPath(workspaceRoot, slug string) string {
	dir := state.RunDir(workspaceRoot, slug)
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "expectation", ExpectationReviewFile)
}

func (Expectation) Verify(_ context.Context, req Request) (Result, error) {
	return verifyReviewFile(req, "expectation", ExpectationReviewPath(req.WorkspaceRoot, req.Slug),
		"REVIEW.md missing; write expectation review before verify",
		"expectation")
}

func verifyReviewFile(req Request, label, path, missingMsg, summaryNoun string) (Result, error) {
	if req.Slug == "" {
		return Result{}, fmt.Errorf("%s verifier needs a slug", label)
	}
	if path == "" {
		return Result{}, fmt.Errorf("no run directory for slug %q", req.Slug)
	}

	relative := relativeTo(req.WorkspaceRoot, path)
	raw, err := os.ReadFile(filepath.Clean(path))
	if errors.Is(err, os.ErrNotExist) {
		return failResult(relative, missingMsg, time.Now().UTC()), nil
	}
	if err != nil {
		return Result{}, fmt.Errorf("read %s: %w", relative, err)
	}

	parsed, parseErr := parseReviewMarkdown(string(raw))
	now := time.Now().UTC()
	if parseErr != "" {
		return failResult(relative, fmt.Sprintf("REVIEW.md has %s", parseErr), now), nil
	}

	if parsed.failed() {
		summary := fmt.Sprintf("%s miss (status=%s, fail_rows=%d, attempt=%d)", summaryNoun, parsed.Status, parsed.FailRows, parsed.Attempt)
		if parsed.Attempt > SoftAttemptCap {
			summary += "; soft attempt cap exceeded, stop and ask a person"
		}
		return failResult(relative, summary, now), nil
	}

	return Result{
		Check: state.Check{
			Passed: true,
			Source: state.SourceFileAssert,
			Ref:    relative,
			At:     now,
		},
		Summary: fmt.Sprintf("%s review passed (attempt=%d)", summaryNoun, parsed.Attempt),
		Detail:  string(raw),
	}, nil
}
