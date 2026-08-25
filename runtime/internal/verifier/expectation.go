package verifier

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
)

// ExpectationReviewFile is the basename under expectation/ in the run directory.
const ExpectationReviewFile = "REVIEW.md"

// SoftAttemptCap is the documented host stop after consecutive misses.
// The verifier still fails above this; the graph routes to plan. The host must
// not keep refining forever: after this many fails, stop and ask a person.
const SoftAttemptCap = 2

var (
	expectationStatusLine  = regexp.MustCompile(`(?im)^\s*status:\s*(pass|fail)\s*$`)
	expectationAttemptLine = regexp.MustCompile(`(?im)^\s*attempt:\s*(\d+)\s*$`)
	// Table row ending in a result cell. Last non-empty cell must be pass or fail.
	expectationResultCell = regexp.MustCompile(`(?i)\|\s*(pass|fail)\s*\|\s*$`)
)

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
	if req.Slug == "" {
		return Result{}, errors.New("expectation verifier needs a slug")
	}
	path := ExpectationReviewPath(req.WorkspaceRoot, req.Slug)
	if path == "" {
		return Result{}, fmt.Errorf("no run directory for slug %q", req.Slug)
	}

	relative := relativeTo(req.WorkspaceRoot, path)
	raw, err := os.ReadFile(filepath.Clean(path))
	if errors.Is(err, os.ErrNotExist) {
		return failResult(relative, "REVIEW.md missing; write expectation review before verify", time.Now().UTC()), nil
	}
	if err != nil {
		return Result{}, fmt.Errorf("read %s: %w", relative, err)
	}

	body := string(raw)
	statusMatch := expectationStatusLine.FindStringSubmatch(body)
	if statusMatch == nil {
		return failResult(relative, "REVIEW.md has no status: pass|fail line", time.Now().UTC()), nil
	}
	status := strings.ToLower(statusMatch[1])

	attempt := 1
	if attemptMatch := expectationAttemptLine.FindStringSubmatch(body); attemptMatch != nil {
		if n, convErr := strconv.Atoi(attemptMatch[1]); convErr == nil && n > 0 {
			attempt = n
		}
	}

	var failRows int
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "|") {
			continue
		}
		// Skip markdown separator rows.
		if strings.Contains(trimmed, "---") {
			continue
		}
		cell := expectationResultCell.FindStringSubmatch(trimmed)
		if cell == nil {
			continue
		}
		if strings.EqualFold(cell[1], "fail") {
			failRows++
		}
	}

	now := time.Now().UTC()
	if status == "fail" || failRows > 0 {
		summary := fmt.Sprintf("expectation miss (status=%s, fail_rows=%d, attempt=%d)", status, failRows, attempt)
		if attempt > SoftAttemptCap {
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
		Summary: fmt.Sprintf("expectation review passed (attempt=%d)", attempt),
		Detail:  body,
	}, nil
}
