package legacy

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/infra/database"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/workspace"
)

// corruptSuffix marks an older file that could not be parsed. Renaming it,
// rather than leaving it in place, keeps one bad line from failing every later
// migration identically; the file stays on disk for a person to repair.
const corruptSuffix = ".corrupt"

// openDB opens the workspace database, creating it: migrating is a write.
func openDB(ctx context.Context, workspaceRoot string) (*sql.DB, error) {
	path := workspace.MemoryDBPath(workspaceRoot)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, err
	}
	return database.Open(ctx, path)
}

// exists reports whether a path is present.
func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// jsonFiles lists the *.json files directly inside dir. A missing dir is empty.
func jsonFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read %s: %w", dir, err)
	}
	var out []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			out = append(out, filepath.Join(dir, entry.Name()))
		}
	}
	return out, nil
}

// findNamed walks root for files called name. A missing root is empty.
func findNamed(root, name string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) && path == root {
				return nil
			}
			return err
		}
		if !d.IsDir() && d.Name() == name {
			out = append(out, path)
		}
		return nil
	})
	return out, err
}

// readEventLog reads an NDJSON event log, the shape every older log used.
func readEventLog(path string) ([]state.Event, error) {
	file, err := os.Open(filepath.Clean(path))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer func() { _ = file.Close() }()

	var events []state.Event
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	line := 0
	for scanner.Scan() {
		line++
		text := scanner.Bytes()
		if len(text) == 0 {
			continue
		}
		var event state.Event
		if err := json.Unmarshal(text, &event); err != nil {
			return nil, fmt.Errorf("%s line %d is not valid JSON: %w", path, line, err)
		}
		events = append(events, event)
	}
	return events, scanner.Err()
}

// setAside renames an unparseable file out of the way and says so.
func setAside(path string, cause error) error {
	renamed := path + corruptSuffix
	if err := os.Rename(path, renamed); err == nil {
		return fmt.Errorf("%w (renamed to %s for manual repair)", cause, renamed)
	}
	return cause
}

// removeMoved deletes a file whose rows now exist.
func removeMoved(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove %s: %w", path, err)
	}
	return nil
}
