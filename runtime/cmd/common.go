package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
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
//
// The workspace default is empty rather than ".", so that "not given" and
// "given as the current directory" stay distinguishable. Only the first of
// those may be discovered from; the second is an instruction.
func addRootFlags(flags *flag.FlagSet) *rootFlags {
	return &rootFlags{
		workspace: flags.String("workspace", "", "workspace root (default: discovered by walking up from the current directory)"),
		toolkit:   flags.String("toolkit", "", "toolkit root holding .ai-agents (default: workspace root)"),
	}
}

// discoverWorkspace walks up from the current directory looking for the marker
// that says a directory is the root of a checkout.
//
// This exists because only one of the four hosts publishes a project-directory
// variable a hook command can pass. Claude Code has ${CLAUDE_PROJECT_DIR};
// Cursor and Codex publish none and do not document the working directory their
// hook processes run in. Defaulting to the current directory therefore made the
// workspace a guess on three hosts out of four, and a wrong guess is silent:
// every hook still runs, still exits 0, and reads run state and memory from a
// directory that has none. The control plane then reports no runs and no
// memories while appearing perfectly wired, which is exactly how it looks when
// it is broken.
//
// Walking up fixes the realistic failure, which is a hook running in a
// subdirectory of the project rather than somewhere unrelated. A cwd outside
// the checkout entirely still cannot be recovered from, and nothing here
// pretends otherwise: discovery falls back to the current directory, the same
// answer as before.
//
// Two passes, over two kinds of marker, because they behave differently when
// nested.
//
// Structural markers mark a checkout or a toolkit mount. A repository has one
// .git, and the nearest one is the answer git itself gives, so these are
// searched first and the nearest match wins.
//
// Rules files are the fallback, and they are the reason the passes are
// separate: AGENTS.md, CLAUDE.md and CURSOR.md are all allowed to nest, one per
// subdirectory. Searching them in the same pass would let a hook running in
// packages/web stop at packages/web/CLAUDE.md and call that the workspace, with
// the real root two levels up. Consulting them only when no structural marker
// exists anywhere in the chain keeps that from happening to a git repository,
// which is nearly all of them, while still finding a workspace that is not one.
// GlobalToolkitDir is deliberately absent from structuralMarkers, though it
// looks like it belongs. It marks a toolkit, not a workspace, and the two are
// separate for the reason rootFlags is two flags rather than one. Including it
// was tried and caught by a test: scripts/install-global creates ~/.vibe-agent,
// so on any machine that has run it, a hook firing from anywhere under the home
// directory outside a checkout resolved the workspace to the home directory
// itself. discoverToolkit already consults that path, in the pass where it
// means something.
var (
	structuralMarkers = []string{".git", ".ai-agents"}
	rulesMarkers      = []string{"AGENTS.md", "CLAUDE.md", "CURSOR.md"}
)

func discoverWorkspace() (string, error) {
	start, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("resolve current directory: %w", err)
	}
	if root, found := nearestAncestorWith(start, structuralMarkers); found {
		return root, nil
	}
	if root, found := nearestAncestorWith(start, rulesMarkers); found {
		return root, nil
	}
	return start, nil
}

// nearestAncestorWith returns the closest directory at or above start holding
// any of the markers.
func nearestAncestorWith(start string, markers []string) (string, bool) {
	for dir := start; ; {
		for _, marker := range markers {
			if _, err := os.Stat(filepath.Join(dir, marker)); err == nil {
				return dir, true
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

// resolve turns the flags into absolute paths.
func (r *rootFlags) resolve() (workspaceRoot, toolkitRoot string, err error) {
	given := *r.workspace
	if given == "" {
		if given, err = discoverWorkspace(); err != nil {
			return "", "", err
		}
	}
	workspaceRoot, err = filepath.Abs(given)
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
