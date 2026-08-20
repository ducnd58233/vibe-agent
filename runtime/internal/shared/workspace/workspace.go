// Package workspace names the directories every module writes into.
//
// It exists because two modules were sharing a constant through an import that
// meant nothing else: internal/fetch depended on internal/memory for the string
// ".agent-state", which reads as "fetching a page needs the memory store" and is
// false. A name several modules must agree on belongs to neither of them.
//
// Nothing here does I/O. These are path answers, so the package sits below every
// module and depends on nothing.
package workspace

import "path/filepath"

const (
	// StateDirName holds state derived from a checkout: caches and databases
	// that can be deleted and rebuilt. Gitignored.
	StateDirName = ".agent-state"

	// RunsDirName holds one directory per run: manifest, event log, evidence.
	// Gitignored, and kept apart from StateDirName because a run's record is
	// evidence a person reads, not a cache a tool rebuilds.
	RunsDirName = "tmp"

	// SDDCacheDirName holds the source-driven WebFetch cache. It used to sit
	// under .claude/, hardcoded in the two Python hooks, so a Cursor or
	// opencode session wrote its cache into another host's directory and
	// nothing reported it. Derived state has one home.
	SDDCacheDirName = "sdd-cache"

	// DocsDirName holds one directory per slug: the spec, plan, task list, and
	// research a run produced. Tracked or ignored by the workspace's own
	// choice, unlike the two above, which are always ignored.
	DocsDirName = "docs"

	// EnvSDDCacheDir hands the resolved cache directory to the hook scripts.
	// They cannot import this constant, so the runtime passes it instead of
	// each side keeping its own copy of the layout.
	EnvSDDCacheDir = "VIBE_SDD_CACHE_DIR"
)

// StateDir is where derived state lives for a workspace.
func StateDir(workspaceRoot string) string {
	return filepath.Join(workspaceRoot, StateDirName)
}

// RunsDir is where every run's directory sits.
func RunsDir(workspaceRoot string) string {
	return filepath.Join(workspaceRoot, RunsDirName)
}

// SDDCacheDir is where the WebFetch cache lives for a workspace.
func SDDCacheDir(workspaceRoot string) string {
	return filepath.Join(StateDir(workspaceRoot), SDDCacheDirName)
}

// DocsDir is where a slug's written deliverables live.
func DocsDir(workspaceRoot, slug string) string {
	return filepath.Join(workspaceRoot, DocsDirName, slug)
}

// RunDir is one run's directory, by slug.
func RunDir(workspaceRoot, slug string) string {
	return filepath.Join(RunsDir(workspaceRoot), slug)
}
