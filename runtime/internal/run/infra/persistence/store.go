// Package persistence reads and writes a run's manifest and event log.
//
// The manifest is rewritten atomically and the log is only ever appended to, so
// a crash leaves a readable run rather than half of one.
package persistence

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/run/domain"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/runpath"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/validate"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/workspace"
)

// RunsDirName is the workspace directory holding every run's state and evidence.
const RunsDirName = workspace.RunsDirName

// RunsDir is where every run's directory sits.
func RunsDir(workspaceRoot string) string { return workspace.RunsDir(workspaceRoot) }

// RunDir is where a slug's state and evidence live.
// Prefer the indexed versioned path under .agent-state/runs; if that revision
// still sits in legacy tmp/, use it until migrate moves the tree. Fall back to
// the legacy flat path so unmigrated workspaces keep loading.
func RunDir(workspaceRoot, slug string) string {
	entry, err := runpath.Resolve(workspaceRoot, slug)
	if err == nil {
		modern := workspace.RunDirAt(workspaceRoot, entry.Date, entry.Slug, entry.Version)
		if manifestReadable(filepath.Join(modern, "manifest.json")) {
			return modern
		}
		legacy := workspace.LegacyRunDirAt(workspaceRoot, entry.Date, entry.Slug, entry.Version)
		if manifestReadable(filepath.Join(legacy, "manifest.json")) {
			return legacy
		}
		return modern
	}
	return workspace.RunDir(workspaceRoot, slug)
}

func manifestReadable(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// ManifestPath is the run-state file for a slug.
func ManifestPath(workspaceRoot, slug string) string {
	return filepath.Join(RunDir(workspaceRoot, slug), "manifest.json")
}

// Load reads and validates a manifest.
func Load(path string) (*domain.Run, error) {
	raw, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("read run state: %w", err)
	}
	var run domain.Run
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&run); err != nil {
		return nil, fmt.Errorf("parse run state %s: %w", path, err)
	}
	if err := run.Validate(); err != nil {
		return nil, fmt.Errorf("run state %s is invalid: %w", path, err)
	}
	return &run, nil
}

// Save writes the manifest atomically: a temp file in the same directory, then
// a rename. A crash mid-write leaves the previous manifest intact rather than a
// truncated one.
func Save(path string, run *domain.Run) error {
	if err := run.Validate(); err != nil {
		return fmt.Errorf("refusing to save invalid run state: %w", err)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("create run directory: %w", err)
	}

	encoded, err := json.MarshalIndent(run, "", "  ")
	if err != nil {
		return fmt.Errorf("encode run state: %w", err)
	}
	encoded = append(encoded, '\n')

	temp, err := os.CreateTemp(dir, ".manifest-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp manifest: %w", err)
	}
	tempName := temp.Name()
	defer func() { _ = os.Remove(tempName) }()

	if _, err := temp.Write(encoded); err != nil {
		_ = temp.Close()
		return fmt.Errorf("write temp manifest: %w", err)
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return fmt.Errorf("sync temp manifest: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close temp manifest: %w", err)
	}
	if err := os.Rename(tempName, path); err != nil {
		return fmt.Errorf("replace manifest: %w", err)
	}
	return nil
}

// EventLogPath is the event log for a slug.
func EventLogPath(workspaceRoot, slug string) string {
	return filepath.Join(RunDir(workspaceRoot, slug), domain.EventLogName)
}

// AppendEvent adds one line to the log and returns the stored event, including
// the sequence number it was given.
func AppendEvent(path string, event domain.Event) (domain.Event, error) {
	if event.Type == "" {
		return domain.Event{}, errors.New("event type must not be empty")
	}
	if event.At.IsZero() {
		event.At = time.Now()
	}
	event.At = event.At.UTC()

	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return domain.Event{}, fmt.Errorf("create run directory: %w", err)
	}

	existing, err := countLines(path)
	if err != nil {
		return domain.Event{}, err
	}
	event.Sequence = existing + 1

	encoded, err := json.Marshal(event)
	if err != nil {
		return domain.Event{}, fmt.Errorf("encode event: %w", err)
	}

	file, err := os.OpenFile(filepath.Clean(path), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return domain.Event{}, fmt.Errorf("open event log: %w", err)
	}
	defer func() { _ = file.Close() }()

	if _, err := file.Write(append(encoded, '\n')); err != nil {
		return domain.Event{}, fmt.Errorf("append event: %w", err)
	}
	if err := file.Sync(); err != nil {
		return domain.Event{}, fmt.Errorf("sync event log: %w", err)
	}
	return event, nil
}

// ReadEvents returns every event in the log. A missing log is not an error.
func ReadEvents(path string) ([]domain.Event, error) {
	file, err := os.Open(filepath.Clean(path))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("open event log: %w", err)
	}
	defer func() { _ = file.Close() }()

	var events []domain.Event
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	line := 0
	for scanner.Scan() {
		line++
		text := scanner.Bytes()
		if len(text) == 0 {
			continue
		}
		var event domain.Event
		if err := json.Unmarshal(text, &event); err != nil {
			return nil, fmt.Errorf("event log %s line %d is not valid JSON: %w", path, line, err)
		}
		events = append(events, event)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read event log: %w", err)
	}
	return events, nil
}

func countLines(path string) (int, error) {
	file, err := os.Open(filepath.Clean(path))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return 0, nil
		}
		return 0, fmt.Errorf("open event log: %w", err)
	}
	defer func() { _ = file.Close() }()

	count := 0
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		if len(scanner.Bytes()) > 0 {
			count++
		}
	}
	if err := scanner.Err(); err != nil {
		return 0, fmt.Errorf("read event log: %w", err)
	}
	return count, nil
}

// PrepareStart allocates a new versioned revision for the slug and creates the
// matching docs directory. Callers must set Date and Version on the Run from
// the returned entry before Save.
func PrepareStart(workspaceRoot, slug string, now time.Time) (runpath.Entry, error) {
	entry, err := runpath.Begin(workspaceRoot, slug, now)
	if err != nil {
		return runpath.Entry{}, err
	}
	docs := workspace.DocsDirAt(workspaceRoot, entry.Date, entry.Slug, entry.Version)
	if err := os.MkdirAll(docs, 0o750); err != nil {
		return runpath.Entry{}, fmt.Errorf("create docs directory: %w", err)
	}
	return entry, nil
}

// List returns slugs that have a readable manifest. Versioned trees,
// run-index pointers, and legacy flat dirs are all considered.
func List(workspaceRoot string) ([]string, error) {
	seen := map[string]bool{}

	indexDir := workspace.RunIndexDir(workspaceRoot)
	if entries, err := os.ReadDir(indexDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
				continue
			}
			slug := entry.Name()[:len(entry.Name())-len(".json")]
			if !validate.Slug(slug) {
				continue
			}
			if _, err := Load(ManifestPath(workspaceRoot, slug)); err == nil {
				seen[slug] = true
			}
		}
	}

	if err := walkVersionedRuns(workspaceRoot, seen); err != nil {
		return nil, err
	}

	// Legacy flat tmp/<slug>/manifest.json until migrate.
	entries, err := os.ReadDir(workspace.LegacyRunsDir(workspaceRoot))
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("list legacy runs: %w", err)
		}
	} else {
		for _, entry := range entries {
			if !entry.IsDir() || validate.Date(entry.Name()) {
				continue
			}
			slug := entry.Name()
			if !validate.Slug(slug) || seen[slug] {
				continue
			}
			flat := filepath.Join(workspace.LegacyRunsDir(workspaceRoot), slug, "manifest.json")
			if _, err := Load(flat); err != nil {
				continue
			}
			seen[slug] = true
		}
	}

	slugs := make([]string, 0, len(seen))
	for slug := range seen {
		slugs = append(slugs, slug)
	}
	sort.Strings(slugs)
	return slugs, nil
}

func walkVersionedRuns(workspaceRoot string, seen map[string]bool) error {
	for _, root := range []string{RunsDir(workspaceRoot), workspace.LegacyRunsDir(workspaceRoot)} {
		if err := walkVersionedRunRoot(workspaceRoot, root, seen); err != nil {
			return err
		}
	}
	return nil
}

func walkVersionedRunRoot(workspaceRoot, root string, seen map[string]bool) error {
	dates, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("list runs under %s: %w", root, err)
	}
	for _, dateEnt := range dates {
		if !dateEnt.IsDir() || !validate.Date(dateEnt.Name()) {
			continue
		}
		slugs, err := os.ReadDir(filepath.Join(root, dateEnt.Name()))
		if err != nil {
			continue
		}
		for _, slugEnt := range slugs {
			if !slugEnt.IsDir() || !validate.Slug(slugEnt.Name()) {
				continue
			}
			slug := slugEnt.Name()
			if seen[slug] {
				continue
			}
			if _, err := Load(ManifestPath(workspaceRoot, slug)); err == nil {
				seen[slug] = true
			}
		}
	}
	return nil
}
