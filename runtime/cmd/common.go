package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ducnd58233/vibe-agent/runtime/internal/state"
)

// rootFlags holds the two path flags every command shares.
//
// The workspace is where run state, evidence, and memory live. The toolkit is
// where .ai-agents sits. They are the same directory when the toolkit is used
// standalone and different when it is mounted as a submodule, which is why they
// are two flags rather than one.
type rootFlags struct {
	workspace *string
	toolkit   *string
}

// addRootFlags registers the shared pair on a flag set.
func addRootFlags(flags *flag.FlagSet) *rootFlags {
	return &rootFlags{
		workspace: flags.String("workspace", ".", "workspace root"),
		toolkit:   flags.String("toolkit", "", "toolkit root holding .ai-agents (default: workspace root)"),
	}
}

// resolve turns the flags into absolute paths.
func (r *rootFlags) resolve() (workspaceRoot, toolkitRoot string, err error) {
	workspaceRoot, err = filepath.Abs(*r.workspace)
	if err != nil {
		return "", "", fmt.Errorf("resolve workspace: %w", err)
	}
	if *r.toolkit == "" {
		return workspaceRoot, discoverToolkit(workspaceRoot), nil
	}
	toolkitRoot, err = filepath.Abs(*r.toolkit)
	if err != nil {
		return "", "", fmt.Errorf("resolve toolkit: %w", err)
	}
	return workspaceRoot, toolkitRoot, nil
}

// discoverToolkit finds the directory holding .ai-agents when --toolkit was not
// given.
//
// Standalone, that is the workspace itself. Mounted as a submodule it is a
// subdirectory, .vibe-agent by the convention in AGENTS.md. Probing for it
// keeps host hook configs identical in both layouts: a hook that only passes
// --workspace still finds the graphs, instead of silently losing them and
// degrading to a less useful message.
//
// One level deep only. A toolkit buried deeper is unusual enough to deserve an
// explicit --toolkit.
func discoverToolkit(workspaceRoot string) string {
	if holdsAssets(workspaceRoot) {
		return workspaceRoot
	}

	entries, err := os.ReadDir(workspaceRoot)
	if err != nil {
		return workspaceRoot
	}
	for _, entry := range entries {
		if !entry.IsDir() || entry.Name() == ".git" || entry.Name() == "node_modules" {
			continue
		}
		candidate := filepath.Join(workspaceRoot, entry.Name())
		if holdsAssets(candidate) {
			return candidate
		}
	}

	// Fall back to the workspace so any later error names a path the caller
	// recognises rather than an empty string.
	return workspaceRoot
}

func holdsAssets(dir string) bool {
	info, err := os.Stat(filepath.Join(dir, ".ai-agents"))
	return err == nil && info.IsDir()
}

// newFlagSet builds a flag set that reports usage errors through the returned
// error rather than exiting, so main can print them in one place.
func newFlagSet(name string) *flag.FlagSet {
	return flag.NewFlagSet(name, flag.ContinueOnError)
}

// verdict renders a check for humans. Skipped is printed distinctly from failed
// on purpose: a check that did not run is not a check that failed.
func verdict(check state.Check) string {
	switch {
	case check.Skipped:
		return "skipped"
	case check.Passed:
		return "pass"
	default:
		return "fail"
	}
}

func orDash(value string) string {
	if value == "" {
		return "-"
	}
	return value
}

func orDashPtr(value *string) string {
	if value == nil {
		return "-"
	}
	return orDash(*value)
}
