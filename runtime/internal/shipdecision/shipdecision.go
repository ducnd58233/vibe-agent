// Package shipdecision parses the file /ship writes at
// .agent-state/runs/<date>/<slug>/<version>/ship/DECISION.md.
//
// /ship's GO/NO-GO has always been a judgement call rendered as chat prose, which is
// exactly why the "ship" check in vibe-checks.yaml is verifier: human — there was never
// anything a file_assert could read. This package is the format a future auto-only
// verifier reads instead: a decision that came from real specialist work, not the
// orchestrating model asserting success. Parse is deliberately strict rather than
// lenient, because a check that defaults to pass on a shape it does not recognize is
// the failure mode this whole design exists to avoid.
package shipdecision

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ducnd58233/vibe-agent/runtime/internal/run/infra/persistence"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/workspace"
)

// Specialist is one review lane's own verdict, e.g. "code-reviewer -> PASS".
type Specialist struct {
	Name   string
	Passed bool
}

// Decision is one /ship run's outcome, parsed from a DECISION.md file.
type Decision struct {
	GO          bool
	Blockers    []string
	Specialists []Specialist
}

const (
	decisionPrefix   = "Ship Decision: "
	blockerPrefix    = "BLOCKER: "
	specialistPrefix = "Specialist: "
	specialistSep    = " -> "
)

// Parse reads a DECISION.md body and returns the decision it records.
//
// Exactly one "Ship Decision: GO" or "Ship Decision: NO-GO" line is required. A GO
// decision carrying a BLOCKER line, or a NO-GO decision carrying none, is malformed:
// the two must agree, or the file is not trustworthy evidence of either state.
func Parse(r io.Reader) (*Decision, error) {
	d := &Decision{}
	sawDecision := false

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r")
		if line == "" {
			continue
		}

		switch {
		case strings.HasPrefix(line, decisionPrefix):
			if sawDecision {
				return nil, fmt.Errorf("shipdecision: duplicate %q line", decisionPrefix)
			}
			verdict := strings.TrimPrefix(line, decisionPrefix)
			switch verdict {
			case "GO":
				d.GO = true
			case "NO-GO":
				d.GO = false
			default:
				return nil, fmt.Errorf("shipdecision: %q line must be GO or NO-GO, got %q", decisionPrefix, verdict)
			}
			sawDecision = true

		case strings.HasPrefix(line, blockerPrefix):
			text := strings.TrimPrefix(line, blockerPrefix)
			if text == "" {
				return nil, fmt.Errorf("shipdecision: empty %q line", blockerPrefix)
			}
			d.Blockers = append(d.Blockers, text)

		case strings.HasPrefix(line, specialistPrefix):
			rest := strings.TrimPrefix(line, specialistPrefix)
			name, verdict, ok := strings.Cut(rest, specialistSep)
			if !ok || name == "" {
				return nil, fmt.Errorf("shipdecision: malformed %q line %q", specialistPrefix, line)
			}
			var passed bool
			switch verdict {
			case "PASS":
				passed = true
			case "FAIL":
				passed = false
			default:
				return nil, fmt.Errorf("shipdecision: specialist verdict must be PASS or FAIL, got %q", verdict)
			}
			d.Specialists = append(d.Specialists, Specialist{Name: name, Passed: passed})

		default:
			return nil, fmt.Errorf("shipdecision: unrecognized line %q", line)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("shipdecision: reading decision: %w", err)
	}
	if !sawDecision {
		return nil, fmt.Errorf("shipdecision: missing %q line", decisionPrefix)
	}
	if d.GO && len(d.Blockers) > 0 {
		return nil, fmt.Errorf("shipdecision: GO decision carries %d blocker(s), must carry none", len(d.Blockers))
	}
	if !d.GO && len(d.Blockers) == 0 {
		return nil, fmt.Errorf("shipdecision: NO-GO decision carries no blockers, must carry at least one")
	}
	return d, nil
}

// Path is where /ship writes its decision for a run.
func Path(workspaceRoot, slug string) string {
	dir := persistence.RunDir(workspaceRoot, slug)
	if dir == "" {
		// No indexed run yet: point at a path under RunsDir that cannot exist
		// until Begin allocates, so Load returns os.ErrNotExist.
		return filepath.Join(workspace.RunsDir(workspaceRoot), "_unindexed", slug, "ship", "DECISION.md")
	}
	return filepath.Join(dir, "ship", "DECISION.md")
}

// Load reads and parses the decision file for a run.
//
// A missing file reports through the returned error like any other open
// failure (os.ErrNotExist) rather than a distinct sentinel. /ship simply not
// having run yet is not this package's concern to name specially; the caller
// deciding what a missing file means for a check is.
func Load(workspaceRoot, slug string) (*Decision, error) {
	f, err := os.Open(Path(workspaceRoot, slug))
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return Parse(f)
}
