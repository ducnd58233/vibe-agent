// Package runpath resolves and allocates versioned docs and run directories.
//
// Layout: docs/<YYYY-MM-DD>/<slug>/<version>/ and
// .agent-state/runs/<YYYY-MM-DD>/<slug>/<version>/. Version numbers are global
// per slug. The current revision is recorded under
// .agent-state/run-index/<slug>.json so CLI and web do not scan on every call.
package runpath

import (
	"encoding/json"
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

const indexSchemaVersion = 1

// ErrNotFound means no index entry and no versioned directory for the slug.
var ErrNotFound = errors.New("run path not found")

// Entry is the current revision pointer for one slug.
type Entry struct {
	SchemaVersion int    `json:"schemaVersion"`
	Slug          string `json:"slug"`
	Date          string `json:"date"`
	Version       int    `json:"version"`
}

// IndexPath is the JSON file for one slug's current revision.
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

// SaveIndex writes the current-revision pointer for a slug.
func SaveIndex(workspaceRoot string, entry Entry) error {
	if !validate.Slug(entry.Slug) {
		return fmt.Errorf("slug %q is not usable", entry.Slug)
	}
	if err := workspace.CheckRevision(entry.Date, entry.Version); err != nil {
		return err
	}
	entry.SchemaVersion = indexSchemaVersion
	dir := workspace.RunIndexDir(workspaceRoot)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("create run index dir: %w", err)
	}
	raw, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return fmt.Errorf("encode run index: %w", err)
	}
	path := IndexPath(workspaceRoot, entry.Slug)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(raw, '\n'), 0o600); err != nil {
		return fmt.Errorf("write run index: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("replace run index: %w", err)
	}
	return nil
}

// LoadIndex reads the pointer file. Missing file returns ErrNotFound.
func LoadIndex(workspaceRoot, slug string) (Entry, error) {
	raw, err := os.ReadFile(IndexPath(workspaceRoot, slug))
	if err != nil {
		if os.IsNotExist(err) {
			return Entry{}, ErrNotFound
		}
		return Entry{}, fmt.Errorf("read run index: %w", err)
	}
	var entry Entry
	if err := json.Unmarshal(raw, &entry); err != nil {
		return Entry{}, fmt.Errorf("parse run index: %w", err)
	}
	if entry.Slug == "" {
		entry.Slug = slug
	}
	if err := workspace.CheckRevision(entry.Date, entry.Version); err != nil {
		return Entry{}, err
	}
	return entry, nil
}

// Resolve returns the current entry for a slug: index first, else scan disk for
// the highest version under .agent-state/runs and docs/.
func Resolve(workspaceRoot, slug string) (Entry, error) {
	if !validate.Slug(slug) {
		return Entry{}, fmt.Errorf("slug %q is not usable", slug)
	}
	if entry, err := LoadIndex(workspaceRoot, slug); err == nil {
		return entry, nil
	} else if !errors.Is(err, ErrNotFound) {
		return Entry{}, err
	}
	entry, ok := scanHighest(workspaceRoot, slug)
	if !ok {
		return Entry{}, ErrNotFound
	}
	return entry, nil
}

// Allocate picks today's date and the next global version for the slug, writes
// the index, and returns the entry. It does not create the docs or runs dirs.
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
	entry := Entry{SchemaVersion: indexSchemaVersion, Slug: slug, Date: date, Version: next}
	if err := SaveIndex(workspaceRoot, entry); err != nil {
		return Entry{}, err
	}
	return entry, nil
}

// Begin starts a new revision for a slug: refuses if one already exists, then
// Allocate writes the index and returns the entry. Callers create directories by
// saving the manifest into RunDirAt.
//
// Also refuses a slug that differs only in letter case from an existing one.
// .agent-state/run-index/ and the versioned docs/ and .agent-state/runs/
// directories sit on a case-preserving but case-insensitive filesystem on
// Windows and macOS, where "MyFeature" and "myfeature" would silently alias
// the same files. The check runs on every platform so behavior does not
// depend on which OS created the run.
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

// ExistingSlugs lists every slug this workspace has a record of, from the
// run-index pointers and from scanning the versioned docs/ and
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
