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
)

// StateDir is where derived state lives for a workspace.
func StateDir(workspaceRoot string) string {
	return filepath.Join(workspaceRoot, StateDirName)
}

// RunsDir is where every run's directory sits.
func RunsDir(workspaceRoot string) string {
	return filepath.Join(workspaceRoot, RunsDirName)
}

// RunDir is one run's directory, by slug.
func RunDir(workspaceRoot, slug string) string {
	return filepath.Join(RunsDir(workspaceRoot), slug)
}
