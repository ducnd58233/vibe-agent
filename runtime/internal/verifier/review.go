package verifier

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
)

// ReviewFile is the basename under review/ in the run directory.
const ReviewFile = "REVIEW.md"

// reviewAxes are the five axes /review already documents
// (.ai-agents/commands/review.md), each matched on a word boundary so
// "incorrectness" does not count as "correctness". Presence, not quality, is
// what this verifier checks: grading the review itself would just move the
// same self-report problem this node exists to close up one level, to a judge
// LLM.
var reviewAxes = []struct {
	name    string
	pattern *regexp.Regexp
}{
	{"correctness", regexp.MustCompile(`(?i)\bcorrectness\b`)},
	{"readability", regexp.MustCompile(`(?i)\breadability\b`)},
	{"architecture", regexp.MustCompile(`(?i)\barchitecture\b`)},
	{"security", regexp.MustCompile(`(?i)\bsecurity\b`)},
	{"performance", regexp.MustCompile(`(?i)\bperformance\b`)},
}

// Review reads .agent-state/runs/.../review/REVIEW.md on the auto path before
// experiment_run. Missing or incomplete files fail with file_assert.
type Review struct{}

func (Review) Kind() string { return "review" }

// ReviewPath is where the host agent must keep REVIEW.md.
func ReviewPath(workspaceRoot, slug string) string {
	dir := state.RunDir(workspaceRoot, slug)
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "review", ReviewFile)
}

func (Review) Verify(_ context.Context, req Request) (Result, error) {
	if req.Slug == "" {
		return Result{}, fmt.Errorf("review verifier needs a slug")
	}
	path := ReviewPath(req.WorkspaceRoot, req.Slug)
	if path == "" {
		return Result{}, fmt.Errorf("no run directory for slug %q", req.Slug)
	}

	relative := relativeTo(req.WorkspaceRoot, path)
	raw, err := os.ReadFile(filepath.Clean(path))
	now := time.Now().UTC()
	if errors.Is(err, os.ErrNotExist) {
		return failResult(relative, "REVIEW.md missing; write the five-axis review before verify", now), nil
	}
	if err != nil {
		return Result{}, fmt.Errorf("read %s: %w", relative, err)
	}

	body := string(raw)
	var missing []string
	for _, axis := range reviewAxes {
		if !axis.pattern.MatchString(body) {
			missing = append(missing, axis.name)
		}
	}
	if len(missing) > 0 {
		return failResult(relative, fmt.Sprintf("REVIEW.md missing axis: %s", strings.Join(missing, ", ")), now), nil
	}

	return Result{
		Check: state.Check{
			Passed: true,
			Source: state.SourceFileAssert,
			Ref:    relative,
			At:     now,
		},
		Summary: "review passed (all five axes present)",
		Detail:  string(raw),
	}, nil
}
