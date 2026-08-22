package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ducnd58233/vibe-agent/runtime/internal/verifier"
)

var mcpExperimentStatusLine = regexp.MustCompile(`(?im)^\s*status:\s*(running|done|failed)\s*$`)

// experimentStatus is vibe_experiment_status. It reports host-written STATUS.md
// and states plainly that sandboxed GPU execution is not provided in-process.
func experimentStatus(deps Deps, raw json.RawMessage) (any, error) {
	var args struct {
		Slug string `json:"slug"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, fmt.Errorf("vibe_experiment_status: %w", err)
	}
	if args.Slug == "" {
		return nil, fmt.Errorf("vibe_experiment_status needs slug")
	}

	path := verifier.ExperimentStatusPath(deps.WorkspaceRoot, args.Slug)
	out := map[string]any{
		"slug":        args.Slug,
		"path":        relativeIfPresent(deps.WorkspaceRoot, path),
		"computePort": "host_or_ci",
		"sandboxNote": "In-process GPU or container sandbox is not provided. " +
			"Run experiments on the host or in CI and keep STATUS.md updated.",
	}
	if path == "" {
		out["status"] = "no_run"
		out["error"] = "no run directory for this slug"
		return out, nil
	}

	rawStatus, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		if os.IsNotExist(err) {
			out["status"] = "missing"
			out["detail"] = "STATUS.md not written yet"
			return out, nil
		}
		return nil, fmt.Errorf("vibe_experiment_status: %w", err)
	}
	body := string(rawStatus)
	out["body"] = body
	if match := mcpExperimentStatusLine.FindStringSubmatch(body); match != nil {
		out["status"] = strings.ToLower(match[1])
	} else {
		out["status"] = "unparsed"
	}
	return out, nil
}
