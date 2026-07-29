package main

import (
	"flag"
	"fmt"
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

// resolve turns the flags into absolute paths. The toolkit falls back to the
// workspace rather than erroring, since standalone use is the common case.
func (r *rootFlags) resolve() (workspaceRoot, toolkitRoot string, err error) {
	workspaceRoot, err = filepath.Abs(*r.workspace)
	if err != nil {
		return "", "", fmt.Errorf("resolve workspace: %w", err)
	}
	if *r.toolkit == "" {
		return workspaceRoot, workspaceRoot, nil
	}
	toolkitRoot, err = filepath.Abs(*r.toolkit)
	if err != nil {
		return "", "", fmt.Errorf("resolve toolkit: %w", err)
	}
	return workspaceRoot, toolkitRoot, nil
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
