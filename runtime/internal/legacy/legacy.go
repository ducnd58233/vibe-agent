// Package legacy brings a workspace written by an older vibe-agent up to the
// current layout, and is the only package that knows what the older layouts
// were.
//
// Every other module stores state one way, in its tables in memory.db. The
// shapes that came before (JSON cache files, NDJSON logs, run manifests on disk,
// run-index pointers, per-host marker files) live here alone: how to find them,
// how to read them, and that they are deleted once their rows exist. A module
// that wants state imported exposes an Import function on its current storage
// and never learns where the state came from.
//
// docstmp is the one migration that is not about state: it moves flat docs/ and
// a workspace-root tmp/ tree into the versioned layout, and it has its own
// command (vibe-agent migrate docs-tmp) because it rewrites tracked files.
package legacy

import (
	"context"
	"errors"
)

// Source is one older layout and how to bring it forward.
type Source struct {
	// Name is what the CLI prints.
	Name string
	// Into names where the state now lives, for the report.
	Into string
	// Pending reports whether this workspace still holds the older layout.
	// It reads; it never creates.
	Pending func(workspaceRoot string) bool
	// Migrate moves what it finds and deletes what it moved. Safe to repeat: a
	// workspace with nothing left reports zero.
	Migrate func(ctx context.Context, workspaceRoot string) (int, error)
}

// Result is what one source moved.
type Result struct {
	Source Source
	Moved  int
}

// Sources lists every older layout this build can bring forward, in the order
// they are migrated.
func Sources() []Source {
	return []Source{
		fetchCacheSource,
		sddCacheSource,
		ambientJournalSource,
		taskListSource,
		runSource,
		sessionSource,
		nodeReminderSource,
	}
}

// Migrate brings every older layout forward. It runs each source in turn and
// joins their errors, so one unreadable layout does not keep the rest of a
// workspace on the old format; what moved before a failure is reported.
func Migrate(ctx context.Context, workspaceRoot string) ([]Result, error) {
	var (
		results []Result
		errs    []error
	)
	for _, source := range Sources() {
		moved, err := source.Migrate(ctx, workspaceRoot)
		results = append(results, Result{Source: source, Moved: moved})
		if err != nil {
			errs = append(errs, err)
		}
	}
	return results, errors.Join(errs...)
}

// Pending names every older layout still present in the workspace.
func Pending(workspaceRoot string) []string {
	var names []string
	for _, source := range Sources() {
		if source.Pending(workspaceRoot) {
			names = append(names, source.Name)
		}
	}
	return names
}
