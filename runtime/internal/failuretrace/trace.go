// Package failuretrace parses host-written failure/TRACE.md files.
//
// The control plane does not invent root causes. It only checks that a TRACE
// the host claimed to write carries the fields the refine loop needs.
package failuretrace

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// RequiredKeys are the field labels a TRACE.md body must set (key: value lines
// or a markdown table row is not required; simple "key: value" lines are enough).
var RequiredKeys = []string{
	"run_id",
	"slug",
	"failed_node",
	"failure_class",
	"symptom",
	"events_ref",
	"refine_target",
}

// Trace is the subset the refine loop reads.
type Trace struct {
	RunID        string
	Slug         string
	FailedNode   string
	FailureClass string
	Symptom      string
	EventsRef    string
	RefineTarget string
}

// Parse reads key: value lines from a TRACE.md body.
func Parse(r io.Reader) (Trace, error) {
	found := map[string]string{}
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "|") {
			continue
		}
		key, val, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(strings.ToLower(key))
		val = strings.TrimSpace(val)
		if val == "" {
			continue
		}
		found[key] = val
	}
	if err := scanner.Err(); err != nil {
		return Trace{}, err
	}
	var missing []string
	for _, key := range RequiredKeys {
		if found[key] == "" {
			missing = append(missing, key)
		}
	}
	if len(missing) > 0 {
		return Trace{}, fmt.Errorf("failure TRACE missing fields: %s", strings.Join(missing, ", "))
	}
	return Trace{
		RunID:        found["run_id"],
		Slug:         found["slug"],
		FailedNode:   found["failed_node"],
		FailureClass: found["failure_class"],
		Symptom:      found["symptom"],
		EventsRef:    found["events_ref"],
		RefineTarget: found["refine_target"],
	}, nil
}

// DefaultRefineTarget maps a FailureClass to refine_target.
func DefaultRefineTarget(failureClass string) string {
	switch strings.ToLower(strings.TrimSpace(failureClass)) {
	case "test", "model":
		return "build"
	case "ambiguity":
		return "plan"
	case "tool", "permission", "context":
		return "retry"
	default:
		return ""
	}
}
