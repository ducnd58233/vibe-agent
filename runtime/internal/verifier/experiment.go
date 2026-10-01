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

// ExperimentStatusFile is the basename under experiment/ in the run directory.
const ExperimentStatusFile = "STATUS.md"

// ExperimentStatus values written by the host agent running the experiment.
const (
	ExperimentRunning = "running"
	ExperimentDone    = "done"
	ExperimentFailed  = "failed"
)

// Judgement values the host states once an experiment reaches a terminal
// status, comparing what it observed against the hypothesis or assumption the
// experiment was testing. NotApplicable is a first-class value, not a
// loophole: a quick reproduce-and-fix cycle has nothing to score against a
// hypothesis, and forcing one would manufacture a false comparison.
const (
	JudgementConfirmed     = "confirmed"
	JudgementRefuted       = "refuted"
	JudgementInconclusive  = "inconclusive"
	JudgementNotApplicable = "not_applicable"
)

var experimentStatusLine = regexp.MustCompile(`(?im)^\s*status:\s*(running|done|failed)\s*$`)

var judgementLine = regexp.MustCompile(`(?im)^\s*judgement:\s*(\S+)\s*$`)

func validJudgement(value string) bool {
	switch value {
	case JudgementConfirmed, JudgementRefuted, JudgementInconclusive, JudgementNotApplicable:
		return true
	default:
		return false
	}
}

// Experiment reads .agent-state/runs/.../experiment/STATUS.md.
//
// Passed is true only for terminal statuses (done or failed). A running or
// missing file fails the check so the researcher-delivery monitor edge loops
// back to experiment_run. That is continuous monitoring without inventing a
// new evidence source.
type Experiment struct{}

func (Experiment) Kind() string { return "experiment" }

// ExperimentStatusPath is where the host agent must keep STATUS.md.
func ExperimentStatusPath(workspaceRoot, slug string) string {
	dir := state.RunDir(workspaceRoot, slug)
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "experiment", ExperimentStatusFile)
}

func (Experiment) Verify(_ context.Context, req Request) (Result, error) {
	if req.Slug == "" {
		return Result{}, errors.New("experiment verifier needs a slug")
	}
	path := ExperimentStatusPath(req.WorkspaceRoot, req.Slug)
	if path == "" {
		return Result{}, fmt.Errorf("no run directory for slug %q", req.Slug)
	}

	relative := relativeTo(req.WorkspaceRoot, path)
	raw, err := os.ReadFile(filepath.Clean(path))
	if errors.Is(err, os.ErrNotExist) {
		return Result{
			Check: state.Check{
				Passed: false,
				Source: state.SourceFileAssert,
				Ref:    relative,
				At:     time.Now().UTC(),
			},
			Summary: "STATUS.md not written yet; experiment still pending",
			Detail:  "missing " + relative,
		}, nil
	}
	if err != nil {
		return Result{}, fmt.Errorf("read %s: %w", relative, err)
	}

	status, ok := parseExperimentStatus(string(raw))
	if !ok {
		return Result{
			Check: state.Check{
				Passed: false,
				Source: state.SourceFileAssert,
				Ref:    relative,
				At:     time.Now().UTC(),
			},
			Summary: "STATUS.md has no status: running|done|failed line",
			Detail:  string(raw),
		}, nil
	}

	terminal := status == ExperimentDone || status == ExperimentFailed
	if !terminal {
		return Result{
			Check: state.Check{
				Passed: false,
				Source: state.SourceFileAssert,
				Ref:    relative + " status=" + status,
				At:     time.Now().UTC(),
			},
			Summary: fmt.Sprintf("experiment status %s", status),
			Detail:  strings.TrimSpace(string(raw)),
		}, nil
	}

	judgement, ok := parseJudgement(string(raw))
	passed := ok && validJudgement(judgement)
	summary := fmt.Sprintf("experiment status %s, judgement %s", status, judgement)
	switch {
	case !ok:
		summary = fmt.Sprintf("experiment status %s has no judgement: line; state confirmed, refuted, inconclusive, or not_applicable", status)
	case !passed:
		summary = fmt.Sprintf("experiment status %s has an unrecognized judgement %q; use confirmed, refuted, inconclusive, or not_applicable", status, judgement)
	}
	return Result{
		Check: state.Check{
			Passed: passed,
			Source: state.SourceFileAssert,
			Ref:    relative + " status=" + status + " judgement=" + judgement,
			At:     time.Now().UTC(),
		},
		Summary: summary,
		Detail:  strings.TrimSpace(string(raw)),
	}, nil
}

func parseJudgement(body string) (string, bool) {
	match := judgementLine.FindStringSubmatch(body)
	if match == nil {
		return "", false
	}
	return strings.ToLower(match[1]), true
}

func parseExperimentStatus(body string) (string, bool) {
	match := experimentStatusLine.FindStringSubmatch(body)
	if match == nil {
		return "", false
	}
	return strings.ToLower(match[1]), true
}
