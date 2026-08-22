package verifier

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
)

const metricsFileName = "METRICS.json"

// MetricThreshold names one numeric success criterion.
type MetricThreshold struct {
	Op    string  `json:"op"`
	Value float64 `json:"value"`
}

// MetricsDocument is written under experiment/ after a run finishes.
type MetricsDocument struct {
	Metrics    map[string]float64         `json:"metrics"`
	Thresholds map[string]MetricThreshold `json:"thresholds"`
}

// Results reads experiment/METRICS.json and experiment/STATUS.md together.
//
// Passed is true only when STATUS is done and every threshold in METRICS is
// met. Failed runs and missing metrics fail the check so the graph can loop
// back to hypothesis without model assertion.
type Results struct{}

func (Results) Kind() string { return "results" }

func metricsPath(workspaceRoot, slug string) string {
	dir := state.RunDir(workspaceRoot, slug)
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "experiment", metricsFileName)
}

func (Results) Verify(_ context.Context, req Request) (Result, error) {
	if req.Slug == "" {
		return Result{}, errors.New("results verifier needs a slug")
	}
	statusPath := ExperimentStatusPath(req.WorkspaceRoot, req.Slug)
	statusRaw, err := os.ReadFile(filepath.Clean(statusPath))
	if errors.Is(err, os.ErrNotExist) {
		return failResult(statusPath, "STATUS.md missing; experiment not finished", time.Now().UTC()), nil
	}
	if err != nil {
		return Result{}, fmt.Errorf("read status: %w", err)
	}
	status, ok := parseExperimentStatus(string(statusRaw))
	if !ok {
		return failResult(statusPath, "STATUS.md has no terminal status line", time.Now().UTC()), nil
	}
	if status == ExperimentRunning {
		return failResult(statusPath, "experiment still running", time.Now().UTC()), nil
	}
	if status == ExperimentFailed {
		return failResult(statusPath, "experiment failed; refine and rerun", time.Now().UTC()), nil
	}

	path := metricsPath(req.WorkspaceRoot, req.Slug)
	relative := relativeTo(req.WorkspaceRoot, path)
	raw, err := os.ReadFile(filepath.Clean(path))
	if errors.Is(err, os.ErrNotExist) {
		return failResult(relative, "METRICS.json missing after a done experiment", time.Now().UTC()), nil
	}
	if err != nil {
		return Result{}, fmt.Errorf("read %s: %w", relative, err)
	}

	var doc MetricsDocument
	if decodeErr := json.Unmarshal(raw, &doc); decodeErr != nil {
		return Result{}, fmt.Errorf("parse %s: %w", relative, decodeErr)
	}
	if len(doc.Thresholds) == 0 {
		return failResult(relative, "METRICS.json declares no thresholds", time.Now().UTC()), nil
	}
	if doc.Metrics == nil {
		doc.Metrics = map[string]float64{}
	}

	var problems []string
	for name, threshold := range doc.Thresholds {
		actual, recorded := doc.Metrics[name]
		if !recorded {
			problems = append(problems, fmt.Sprintf("%s not recorded", name))
			continue
		}
		if !compareMetric(actual, threshold) {
			problems = append(problems, fmt.Sprintf("%s=%g does not satisfy %s %g", name, actual, threshold.Op, threshold.Value))
		}
	}
	now := time.Now().UTC()
	if len(problems) > 0 {
		return failResult(relative, strings.Join(problems, "; "), now), nil
	}
	return Result{
		Check: state.Check{
			Passed: true,
			Source: state.SourceFileAssert,
			Ref:    relative,
			At:     now,
		},
		Summary: "experiment metrics meet every declared threshold",
		Detail:  string(raw),
	}, nil
}

func compareMetric(actual float64, threshold MetricThreshold) bool {
	switch threshold.Op {
	case ">=", "gte":
		return actual >= threshold.Value
	case "<=", "lte":
		return actual <= threshold.Value
	case ">", "gt":
		return actual > threshold.Value
	case "<", "lt":
		return actual < threshold.Value
	case "==", "eq":
		return actual == threshold.Value || (math.IsNaN(actual) && math.IsNaN(threshold.Value))
	default:
		return false
	}
}

func failResult(ref, summary string, at time.Time) Result {
	return Result{
		Check: state.Check{
			Passed: false,
			Source: state.SourceFileAssert,
			Ref:    ref,
			At:     at,
		},
		Summary: summary,
	}
}
