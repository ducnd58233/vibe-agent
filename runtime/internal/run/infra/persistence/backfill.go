package persistence

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/run/domain"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/workspace"
)

// Backfill moves every .agent-state/runs/**/manifest.json (and its sibling
// events.ndjson) into runs and run_events, then removes those files and every
// .agent-state/run-index/*.json pointer. Evidence subdirectories under each
// run are left alone. Safe to re-run: a workspace with nothing left migrates
// zero rows rather than erroring.
func Backfill(ctx context.Context, workspaceRoot string) (int, error) {
	manifests, err := listLegacyManifests(workspaceRoot)
	if err != nil {
		return 0, err
	}

	db, err := openDB(ctx, workspaceRoot)
	if err != nil {
		return 0, fmt.Errorf("open runs database: %w", err)
	}
	defer func() { _ = db.Close() }()

	migrated := 0
	for _, manifestPath := range manifests {
		loc, _, ok := parseRunPath(manifestPath)
		if !ok {
			// Consumer trees sometimes keep unrelated manifest.json files under
			// runs/ (vault backups, tooling dumps). Those are not versioned run
			// paths; skipping keeps migrate state usable on real workspaces.
			fmt.Fprintf(os.Stderr, "migrate runs: skip non-run manifest %s\n", manifestPath)
			continue
		}
		run, err := loadRunFromFile(manifestPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "migrate runs: skip unreadable run manifest %s: %v\n", manifestPath, err)
			continue
		}
		if err := upsertRun(ctx, db, run, loc); err != nil {
			return migrated, err
		}
		// Dual-write may already have inserted events; the file is the
		// source of truth for this one-time move.
		if _, err := db.ExecContext(ctx, `DELETE FROM run_events WHERE run_id = ?`, run.RunID); err != nil {
			return migrated, fmt.Errorf("clear run_events for %s: %w", run.RunID, err)
		}

		eventsPath := filepath.Join(filepath.Dir(manifestPath), domain.EventLogName)
		events, err := readEventsFileOnly(eventsPath)
		if err != nil {
			return migrated, fmt.Errorf("read %s: %w", eventsPath, err)
		}
		for _, event := range events {
			payload := ""
			if len(event.Payload) > 0 {
				payload = string(event.Payload)
			}
			at := time.Now().UTC().Format(time.RFC3339)
			if !event.At.IsZero() {
				at = event.At.UTC().Format(time.RFC3339)
			}
			_, err = db.ExecContext(ctx, `
                INSERT INTO run_events (
                    run_id, sequence, type, node, at, payload,
                    created_by, reviewed_by_agents, created_at, updated_at)
                VALUES (?, ?, ?, ?, ?, ?, '', '', ?, ?)`,
				run.RunID, event.Sequence, string(event.Type), event.Node, at, payload, at, at)
			if err != nil {
				return migrated, fmt.Errorf("insert event %d for %s: %w", event.Sequence, run.RunID, err)
			}
		}

		if err := os.Remove(manifestPath); err != nil {
			return migrated, fmt.Errorf("remove %s: %w", manifestPath, err)
		}
		if _, err := os.Stat(eventsPath); err == nil {
			if err := os.Remove(eventsPath); err != nil {
				return migrated, fmt.Errorf("remove %s: %w", eventsPath, err)
			}
		} else if !os.IsNotExist(err) {
			return migrated, err
		}
		migrated++
	}

	if err := removeRunIndexFiles(workspaceRoot); err != nil {
		return migrated, err
	}
	return migrated, nil
}

func listLegacyManifests(workspaceRoot string) ([]string, error) {
	root := workspace.RunsDir(workspaceRoot)
	var out []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) && path == root {
				return nil
			}
			return err
		}
		if d.IsDir() || d.Name() != "manifest.json" {
			return nil
		}
		out = append(out, path)
		return nil
	})
	return out, err
}

func loadRunFromFile(path string) (*domain.Run, error) {
	raw, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, err
	}
	return decodeRunBody(string(raw))
}

func readEventsFileOnly(path string) ([]domain.Event, error) {
	file, err := os.Open(filepath.Clean(path))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
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
		return nil, err
	}
	return events, nil
}

func removeRunIndexFiles(workspaceRoot string) error {
	dir := workspace.RunIndexDir(workspaceRoot)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("list run index: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("remove %s: %w", path, err)
		}
	}
	return nil
}
