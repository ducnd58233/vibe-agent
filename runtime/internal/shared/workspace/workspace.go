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

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/validate"
)

const (
	// StateDirName holds state derived from a checkout: caches, databases, and
	// run evidence. Gitignored.
	StateDirName = ".agent-state"

	// RunsDirName is the subdirectory under StateDirName for one directory per
	// run: manifest, event log, evidence. Kept as its own folder so caches
	// (fetch, sdd-cache, memory) are not mixed with evidence a person reads.
	// Layout: .agent-state/runs/<date>/<slug>/<version>/.
	RunsDirName = "runs"

	// SDDCacheDirName holds the source-driven WebFetch cache. It used to sit
	// under .claude/, hardcoded in the two Python hooks, so a Cursor or
	// opencode session wrote its cache into another host's directory and
	// nothing reported it. Derived state has one home.
	SDDCacheDirName = "sdd-cache"

	// DocsDirName holds written deliverables: specs, plans, task lists.
	// Tracked or ignored by the workspace's own choice, unlike StateDirName,
	// which is always ignored. Layout is docs/<date>/<slug>/<version>/.
	DocsDirName = "docs"

	// RunIndexDirName holds one JSON pointer per slug under StateDirName so
	// CLI and web can resolve the current (date, version) without scanning.
	RunIndexDirName = "run-index"

	// FetchCacheDirName holds extracted WebFetch documents beside other caches.
	FetchCacheDirName = "fetch"

	// EnvSDDCacheDir hands the resolved cache directory to the hook scripts.
	// They cannot import this constant, so the runtime passes it instead of
	// each side keeping its own copy of the layout.
	EnvSDDCacheDir = "VIBE_SDD_CACHE_DIR"
)

// StateDir is where derived state lives for a workspace.
func StateDir(workspaceRoot string) string {
	return filepath.Join(workspaceRoot, StateDirName)
}

// RunsDir is where every run's versioned directory sits.
func RunsDir(workspaceRoot string) string {
	return filepath.Join(StateDir(workspaceRoot), RunsDirName)
}

// SDDCacheDir is where the WebFetch cache lives for a workspace.
func SDDCacheDir(workspaceRoot string) string {
	return filepath.Join(StateDir(workspaceRoot), SDDCacheDirName)
}

// FetchCacheDir is where fetched page text and assets are stored.
func FetchCacheDir(workspaceRoot string) string {
	return filepath.Join(StateDir(workspaceRoot), FetchCacheDirName)
}

// RunIndexDir is where per-slug current-revision pointers live.
func RunIndexDir(workspaceRoot string) string {
	return filepath.Join(StateDir(workspaceRoot), RunIndexDirName)
}

// DocsDirAt is the versioned docs directory for one revision.
// date must be YYYY-MM-DD; version must be >= 1. Invalid inputs return "".
func DocsDirAt(workspaceRoot, date, slug string, version int) string {
	if err := CheckRevision(date, version); err != nil {
		return ""
	}
	return filepath.Join(workspaceRoot, DocsDirName, date, slug, strconv.Itoa(version))
}

// RunDirAt is the versioned run/evidence directory for one revision.
// date must be YYYY-MM-DD; version must be >= 1. Invalid inputs return "".
func RunDirAt(workspaceRoot, date, slug string, version int) string {
	if err := CheckRevision(date, version); err != nil {
		return ""
	}
	return filepath.Join(RunsDir(workspaceRoot), date, slug, strconv.Itoa(version))
}

// CheckRevision reports whether date and version are usable in a path segment.
func CheckRevision(date string, version int) error {
	if !validate.Date(date) {
		return fmt.Errorf("date %q is not YYYY-MM-DD", date)
	}
	if version < 1 {
		return fmt.Errorf("version %d must be >= 1", version)
	}
	return nil
}

// DocsArtifact is the dated basename for a docs deliverable.
// stem is SPEC, PLAN, TASKS, tasks, RESEARCH, and so on.
// Markdown stems get .md; the stem "tasks" gets .json.
func DocsArtifact(stem, date string) (string, error) {
	if stem == "" {
		return "", fmt.Errorf("artifact stem is empty")
	}
	if !validate.Date(date) {
		return "", fmt.Errorf("date %q is not YYYY-MM-DD", date)
	}
	ext := ".md"
	if stem == "tasks" {
		ext = ".json"
	}
	return stem + "-" + date + ext, nil
}

// DocsDir is the legacy flat docs path (docs/<slug>/). Prefer DocsDirAt.
// Kept for docs migrate only; new work uses DocsDirAt.
func DocsDir(workspaceRoot, slug string) string {
	return filepath.Join(workspaceRoot, DocsDirName, slug)
}

// PresentBasenames returns each name that exists as a direct child of root.
// Used by MCP status and session-start hooks so the file list cannot drift.
func PresentBasenames(root string, names ...string) []string {
	var present []string
	for _, name := range names {
		if _, err := os.Stat(filepath.Join(root, name)); err == nil {
			present = append(present, name)
		}
	}
	return present
}
