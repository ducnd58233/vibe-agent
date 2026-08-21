// Package runpath resolves and allocates versioned docs/tmp directories.
//
// Layout: {docs,tmp}/<YYYY-MM-DD>/<slug>/<version>/. Version numbers are global
// per slug. The current revision is recorded under
// .agent-state/run-index/<slug>.json so CLI and web do not scan on every call.
//
// These helpers never fall back to the legacy flat {docs,tmp}/<slug>/ tree.
package runpath

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
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
// the highest version under tmp/ and docs/. Never reads the flat <slug>/ layout.
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
// the index, and returns the entry. It does not create the docs/tmp directories.
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

func scanHighest(workspaceRoot, slug string) (Entry, bool) {
	best := Entry{Slug: slug, Version: 0}
	found := false
	for _, rootName := range []string{workspace.RunsDirName, workspace.DocsDirName} {
		root := filepath.Join(workspaceRoot, rootName)
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
