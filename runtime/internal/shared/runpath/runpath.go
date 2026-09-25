// Package runpath resolves and allocates versioned docs and run directories.
//
// Layout: docs/<YYYY-MM-DD>/<slug>/<version>/ and
// .agent-state/runs/<YYYY-MM-DD>/<slug>/<version>/. Version numbers are global
// per slug. The current revision comes from the runs table when wired, else
// from scanning those versioned directories. A leftover .agent-state/run-index/
// file is only consulted for dual-write detection elsewhere, not as the
// source of truth for Resolve.
package runpath

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/validate"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/workspace"
)

// ErrNotFound means no runs-table row and no versioned directory for the slug.
var ErrNotFound = errors.New("run path not found")

// ResolveSQL looks up the latest (date, version) for a slug in the runs table.
// persistence sets this so Resolve still works after backfill removes
// run-index files; this package must not import persistence itself.
var ResolveSQL func(workspaceRoot, slug string) (Entry, bool)

// Entry is the current revision pointer for one slug.
type Entry struct {
	SchemaVersion int    `json:"schemaVersion"`
	Slug          string `json:"slug"`
	Date          string `json:"date"`
	Version       int    `json:"version"`
}

// IndexPath is the legacy JSON pointer path for one slug. Kept so dual-write
// and backfill can detect a leftover file; nothing writes this path any more.
func IndexPath(workspaceRoot, slug string) string {
	return filepath.Join(workspace.RunIndexDir(workspaceRoot), slug+".json")
}

// DocsDir resolves the current docs directory for a slug.
func DocsDir(workspaceRoot, slug string) (string, error) {
	entry, err := Resolve(workspaceRoot, slug)
	if err != nil {
		return "", err
	}
	return workspace.DocsDirAt(workspaceRoot, entry.Date, entry.Slug, entry.Version), nil
}

// RunDir resolves the current run/evidence directory for a slug.
func RunDir(workspaceRoot, slug string) (string, error) {
	entry, err := Resolve(workspaceRoot, slug)
	if err != nil {
		return "", err
	}
	return workspace.RunDirAt(workspaceRoot, entry.Date, entry.Slug, entry.Version), nil
}

// Resolve returns the current entry for a slug: the runs table when wired,
// else scan disk for the highest version under .agent-state/runs and docs/.
func Resolve(workspaceRoot, slug string) (Entry, error) {
	if !validate.Slug(slug) {
		return Entry{}, fmt.Errorf("slug %q is not usable", slug)
	}
	if ResolveSQL != nil {
		if entry, ok := ResolveSQL(workspaceRoot, slug); ok {
			return entry, nil
		}
	}
	entry, ok := scanHighest(workspaceRoot, slug)
	if !ok {
		return Entry{}, ErrNotFound
	}
	return entry, nil
}

// Allocate picks today's date and the next global version for the slug,
// creates the versioned run directory so Resolve can find it without a
// run-index file, and returns the entry. It does not create the docs dir or
// write a manifest; callers do that.
func Allocate(workspaceRoot, slug string, now time.Time) (Entry, error) {
	if !validate.Slug(slug) {
		return Entry{}, fmt.Errorf("slug %q is not usable", slug)
	}
	if now.IsZero() {
		now = time.Now()
	}
	date := now.Format("2006-01-02")
	next := 1
	if existing, err := Resolve(workspaceRoot, slug); err == nil {
		next = existing.Version + 1
	} else if !errors.Is(err, ErrNotFound) {
		return Entry{}, err
	}
	entry := Entry{Slug: slug, Date: date, Version: next}
	dir := workspace.RunDirAt(workspaceRoot, entry.Date, entry.Slug, entry.Version)
	if dir == "" {
		return Entry{}, fmt.Errorf("run directory for %s@%s/%d is empty", slug, date, next)
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return Entry{}, fmt.Errorf("create run directory: %w", err)
	}
	return entry, nil
}

// Begin starts a new revision for a slug: refuses if one already exists, then
// Allocate returns the entry. Callers create documentation directories by
// saving into DocsDirAt / RunDirAt.
//
// Also refuses a slug that differs only in letter case from an existing one.
// Versioned docs/ and .agent-state/runs/ directories sit on a case-preserving
// but case-insensitive filesystem on Windows and macOS, where "MyFeature" and
// "myfeature" would silently alias the same files. The check runs on every
// platform so behavior does not depend on which OS created the run.
func Begin(workspaceRoot, slug string, now time.Time) (Entry, error) {
	if !validate.Slug(slug) {
		return Entry{}, fmt.Errorf("slug %q is not usable", slug)
	}
	if entry, err := Resolve(workspaceRoot, slug); err == nil {
		return Entry{}, fmt.Errorf("a run already exists for %q at %s", slug,
			workspace.RunDirAt(workspaceRoot, entry.Date, entry.Slug, entry.Version))
	} else if !errors.Is(err, ErrNotFound) {
		return Entry{}, err
	}
	existing, err := ExistingSlugs(workspaceRoot)
	if err != nil {
		return Entry{}, err
	}
	for _, other := range existing {
		if other != slug && strings.EqualFold(other, slug) {
			return Entry{}, fmt.Errorf("slug %q differs only in case from existing slug %q; "+
				"case-insensitive filesystems would alias their files", slug, other)
		}
	}
	return Allocate(workspaceRoot, slug, now)
}

// ExistingSlugs lists every slug this workspace has a record of, from leftover
// run-index pointers (pre-backfill) and from scanning the versioned docs/ and
// .agent-state/runs/ directories. Raw names as found on disk, not filtered by
// validate.Slug: a case-collision check needs to see everything that could
// alias, not just what a fresh slug would itself be allowed to be.
func ExistingSlugs(workspaceRoot string) ([]string, error) {
	seen := map[string]bool{}

	indexDir := workspace.RunIndexDir(workspaceRoot)
	entries, err := os.ReadDir(indexDir)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("list run index: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		seen[strings.TrimSuffix(entry.Name(), ".json")] = true
	}

	roots := []string{
		workspace.RunsDir(workspaceRoot),
		filepath.Join(workspaceRoot, workspace.DocsDirName),
	}
	for _, root := range roots {
		dates, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, dateEnt := range dates {
			if !dateEnt.IsDir() || !validate.Date(dateEnt.Name()) {
				continue
			}
			slugEntries, err := os.ReadDir(filepath.Join(root, dateEnt.Name()))
			if err != nil {
				continue
			}
			for _, slugEnt := range slugEntries {
				if !slugEnt.IsDir() {
					continue
				}
				seen[slugEnt.Name()] = true
			}
		}
	}

	out := make([]string, 0, len(seen))
	for slug := range seen {
		out = append(out, slug)
	}
	return out, nil
}

func scanHighest(workspaceRoot, slug string) (Entry, bool) {
	best := Entry{Slug: slug, Version: 0}
	found := false
	roots := []string{
		workspace.RunsDir(workspaceRoot),
		filepath.Join(workspaceRoot, workspace.DocsDirName),
	}
	for _, root := range roots {
		dates, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, dateEnt := range dates {
			if !dateEnt.IsDir() || !validate.Date(dateEnt.Name()) {
				continue
			}
			slugDir := filepath.Join(root, dateEnt.Name(), slug)
			versions, err := os.ReadDir(slugDir)
			if err != nil {
				continue
			}
			for _, verEnt := range versions {
				if !verEnt.IsDir() {
					continue
				}
				n, err := strconv.Atoi(verEnt.Name())
				if err != nil || n < 1 {
					continue
				}
				if !found || n > best.Version {
					best.Date = dateEnt.Name()
					best.Version = n
					found = true
				}
			}
		}
	}
	return best, found
}
