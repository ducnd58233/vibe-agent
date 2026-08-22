// Package verifier produces the evidence transitions depend on.
//
// Every result carries a provenance that is not the model: a process exit code,
// a file assertion, or a git observation. That is the whole point. A verifier
// that returned "the agent says it passed" would defeat the design.
package verifier

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
)

// relativeTo renders a path for a reader, relative to the workspace where that
// is possible and absolute where it is not.
//
// No error: a path outside the workspace root simply has no relative form, and
// reporting the absolute one answers the question that was asked.
func relativeTo(workspaceRoot, path string) string {
	relative, err := filepath.Rel(workspaceRoot, path)
	if err != nil {
		return path
	}
	return relative
}

// Request is what to verify and where to put the evidence.
type Request struct {
	// Check is the run-state key this result will be written under.
	Check string
	// WorkspaceRoot anchors relative paths and command execution.
	WorkspaceRoot string
	// Slug selects the .agent-state/runs/.../ evidence directory.
	Slug string
	// Timeout bounds the work. Zero means the verifier's own default.
	Timeout time.Duration

	// Command verifier
	Command string
	Args    []string
	// LogDir is the subdirectory under .agent-state/runs/.../ for captured output, for
	// example "unit" or "e2e".
	LogDir string

	// Files verifier
	Paths []string

	// Git verifier
	Expect GitExpectation

	// Screen verifier
	Screen *ScreenSpec
}

// Result is one verification outcome plus a human-readable account of it.
type Result struct {
	Check   state.Check
	Summary string
	// Detail is the captured output or assertion trace, already written to
	// .agent-state/runs/.../ when the verifier produces a log.
	Detail  string
	LogPath string
}

// Verifier turns a request into evidence.
type Verifier interface {
	// Kind is the graph's verifier name: command, files, git, screen, tasks,
	// shipdecision, or reviewbots.
	Kind() string
	Verify(ctx context.Context, req Request) (Result, error)
}

// Registry maps graph verifier names to implementations.
type Registry map[string]Verifier

// Default is the set every runtime ships with.
func Default() Registry {
	return Registry{
		"command":      Command{},
		"files":        Files{},
		"git":          Git{},
		"screen":       Screen{},
		"tasks":        Tasks{},
		"experiment":   Experiment{},
		"shipdecision": ShipDecision{},
		"reviewbots":   ReviewBots{},
	}
}

// Get returns a verifier by the name a graph node used.
func (r Registry) Get(kind string) (Verifier, error) {
	verifier, ok := r[kind]
	if !ok {
		return nil, fmt.Errorf("no verifier %q; known verifiers are %s", kind, strings.Join(r.Kinds(), ", "))
	}
	return verifier, nil
}

// Kinds lists the registered verifier names in a stable order.
func (r Registry) Kinds() []string {
	kinds := make([]string, 0, len(r))
	for kind := range r {
		kinds = append(kinds, kind)
	}
	sort.Strings(kinds)
	return kinds
}

// Skipped builds the result for a node the graph told the runtime to skip.
// A skipped check is recorded as skipped, never as passed.
func Skipped(reason string, now time.Time) Result {
	return Result{
		Check: state.Check{
			Passed:  false,
			Skipped: true,
			Source:  state.SourceFileAssert,
			At:      now.UTC(),
		},
		Summary: "skipped: " + reason,
	}
}
