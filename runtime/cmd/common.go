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

// ToolkitEnv names the environment variable that overrides toolkit discovery.
const ToolkitEnv = "VIBE_AGENT_TOOLKIT"

// GlobalToolkitDir is the per-user toolkit location scripts/install-global
// populates. Placing a link to .ai-agents here is what lets a repository use
// the toolkit without vendoring it.
const GlobalToolkitDir = ".vibe-agent"

// discoverToolkit finds the directory holding .ai-agents when --toolkit was not
// given.
//
// Order, most specific first:
//
//  1. the workspace itself, when the toolkit is used standalone
//  2. a subdirectory one level down, .vibe-agent by the convention in
//     AGENTS.md, which is the submodule layout
//  3. $VIBE_AGENT_TOOLKIT
//  4. ~/.vibe-agent, written by scripts/install-global
//
// Local before global is deliberate and matches the precedence rule in
// AGENTS.md: a repository that ships its own assets means them, and a
// machine-wide install is the fallback. Someone who needs to override a
// vendored toolkit has --toolkit, which beats all of this.
//
// The global steps exist because the toolkit supplies only graphs and hook
// wiring, and neither is repository-specific. Without them a repository that
// wants the control plane has to vendor the whole toolkit to get two files, and
// `doctor` fails on a workspace that is otherwise correctly set up.
//
// One level deep only. A toolkit buried deeper is unusual enough to deserve an
// explicit --toolkit.
func discoverToolkit(workspaceRoot string) string {
	if holdsAssets(workspaceRoot) {
		return workspaceRoot
	}

	if entries, err := os.ReadDir(workspaceRoot); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() || entry.Name() == ".git" || entry.Name() == "node_modules" {
				continue
			}
			candidate := filepath.Join(workspaceRoot, entry.Name())
			if holdsAssets(candidate) {
				return candidate
			}
		}
	}

	if env := os.Getenv(ToolkitEnv); env != "" {
		if abs, err := filepath.Abs(env); err == nil && holdsAssets(abs) {
			return abs
		}
	}

	if home, err := os.UserHomeDir(); err == nil {
		candidate := filepath.Join(home, GlobalToolkitDir)
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
