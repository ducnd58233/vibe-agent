package harness

import (
	_ "embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// The starter plan is a commented template rather than a working one. An init
// that silently changed what runs would be a worse default than no init.
//
//go:embed guards-starter.yaml
var starterGuardPlan []byte

// GuardSummary is one guard as a person needs to see it: what it reads, what it
// looks for, and whether it is on.
type GuardSummary struct {
	Name     string
	Disabled bool
	Applies  string
	Checks   []string
}

// Guards reports the rules a workspace runs, so someone can see what they are
// customising before they customise it.
//
// The error is the plan's, when a workspace has one that does not load. Callers
// should show it rather than fall back quietly: at the command line, unlike in
// a hook, there is a person waiting for an answer.
func Guards(workspaceRoot string) ([]GuardSummary, error) {
	sets, err := activeRules(Request{WorkspaceRoot: workspaceRoot})
	if err != nil {
		return nil, err
	}

	summaries := make([]GuardSummary, 0, len(sets))
	for _, set := range sets {
		summary := GuardSummary{
			Name:     set.Name,
			Disabled: set.Disabled,
			Applies:  set.AppliesTo.describe(),
		}
		for _, rule := range set.Line {
			summary.Checks = append(summary.Checks, rule.ID)
		}
		for _, rule := range set.Density {
			summary.Checks = append(summary.Checks, rule.ID)
		}
		if set.inspect != nil && len(summary.Checks) == 0 {
			summary.Checks = append(summary.Checks, "(built-in analysis)")
		}
		summaries = append(summaries, summary)
	}
	return summaries, nil
}

// describe renders a selector the way someone would read it aloud.
func (s selector) describe() string {
	var parts []string
	if len(s.Types) > 0 {
		parts = append(parts, "types "+strings.Join(s.Types, ", "))
	}
	if len(s.Languages) > 0 {
		parts = append(parts, "languages "+strings.Join(s.Languages, ", "))
	}
	if len(s.Extensions) > 0 {
		parts = append(parts, "extensions "+strings.Join(s.Extensions, ", "))
	}
	if len(parts) == 0 {
		return "nothing"
	}
	return strings.Join(parts, "; ")
}

// ErrGuardPlanExists is returned when init would overwrite a plan someone
// already wrote.
var ErrGuardPlanExists = errors.New("a guard plan is already here")

// InitGuardPlan writes the starter plan into a workspace and returns its path.
//
// It refuses to overwrite unless told to, because the file it would replace is
// the one holding a repository's own decisions.
func InitGuardPlan(workspaceRoot string, force bool) (string, error) {
	path := filepath.Join(workspaceRoot, filepath.FromSlash(ConsumerGuardPlan))

	if _, err := os.Stat(path); err == nil && !force {
		return path, ErrGuardPlanExists
	} else if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return path, err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return path, err
	}
	if err := os.WriteFile(path, starterGuardPlan, 0o600); err != nil {
		return path, err
	}
	return path, nil
}

// StarterPlanLoads proves the template this command writes is a plan the loader
// accepts. A scaffold that fails the first time it is uncommented would teach
// people the feature is broken.
func StarterPlanLoads() error {
	if _, err := parseGuardPlan(starterGuardPlan); err != nil {
		return fmt.Errorf("starter guard plan is not loadable: %w", err)
	}
	return nil
}
