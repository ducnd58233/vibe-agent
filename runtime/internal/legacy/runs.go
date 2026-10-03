package legacy

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/validate"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/workspace"
	"github.com/ducnd58233/vibe-agent/runtime/internal/tasks"
)

// manifestName is a run's state file in .agent-state/runs/<date>/<slug>/<version>/,
// before the runs table.
const manifestName = "manifest.json"

var runSource = Source{
	Name: "runs",
	Into: "runs, run_events",
	Pending: func(root string) bool {
		manifests, _ := findNamed(workspace.RunsDir(root), manifestName)
		indexes, _ := jsonFiles(workspace.RunIndexDir(root))
		return len(manifests) > 0 || len(indexes) > 0
	},
	Migrate: migrateRuns,
}

// runPlace reads date, slug, and version back out of a manifest's path. A
// manifest elsewhere under runs/ (a backup, a tool's dump) is not a run.
func runPlace(root, manifest string) (date, slug string, version int, ok bool) {
	rel, err := filepath.Rel(workspace.RunsDir(root), filepath.Dir(manifest))
	if err != nil {
		return "", "", 0, false
	}
	parts := strings.Split(filepath.ToSlash(rel), "/")
	if len(parts) != 3 {
		return "", "", 0, false
	}
	version, err = strconv.Atoi(parts[2])
	if err != nil || version < 1 || !validate.Date(parts[0]) || !validate.Slug(parts[1]) {
		return "", "", 0, false
	}
	return parts[0], parts[1], version, true
}

func migrateRuns(ctx context.Context, root string) (int, error) {
	manifests, err := findNamed(workspace.RunsDir(root), manifestName)
	if err != nil {
		return 0, err
	}
	moved := 0
	for _, manifest := range manifests {
		date, slug, version, ok := runPlace(root, manifest)
		if !ok {
			fmt.Fprintf(os.Stderr, "migrate runs: skip non-run manifest %s\n", manifest)
			continue
		}
		raw, err := os.ReadFile(filepath.Clean(manifest))
		if err != nil {
			return moved, fmt.Errorf("read %s: %w", manifest, err)
		}
		var run state.Run
		if err := json.Unmarshal(raw, &run); err != nil || run.Validate() != nil {
			fmt.Fprintf(os.Stderr, "migrate runs: skip unreadable run manifest %s\n", manifest)
			continue
		}
		if run.Slug == "" {
			run.Slug = slug
		}
		eventsPath := filepath.Join(filepath.Dir(manifest), state.EventLogName)
		events, err := readEventLog(eventsPath)
		if err != nil {
			return moved, fmt.Errorf("read %s: %w", eventsPath, err)
		}
		if err := state.ImportRun(ctx, root, &run, date, version, events); err != nil {
			return moved, err
		}
		for _, path := range []string{manifest, eventsPath} {
			if err := removeMoved(path); err != nil {
				return moved, err
			}
		}
		moved++
	}

	// The run-index pointers named a slug's current revision; the runs table
	// answers that now, so they go once their runs are rows.
	indexes, err := jsonFiles(workspace.RunIndexDir(root))
	if err != nil {
		return moved, err
	}
	for _, index := range indexes {
		if err := removeMoved(index); err != nil {
			return moved, err
		}
	}
	return moved, nil
}

// taskListSource: docs/**/tasks-<date>.json (and the earlier tasks.json),
// before task_lists. TASKS-*.md prose is current and never touched.
var taskListSource = Source{
	Name: "task lists",
	Into: "task_lists",
	Pending: func(root string) bool {
		files, _ := taskListFiles(root)
		return len(files) > 0
	},
	Migrate: migrateTaskLists,
}

func taskListFiles(root string) ([]string, error) {
	var out []string
	all, err := findAll(filepath.Join(root, "docs"))
	if err != nil {
		return nil, err
	}
	for _, path := range all {
		name := filepath.Base(path)
		if name == tasks.FileName || (strings.HasPrefix(name, "tasks-") && strings.HasSuffix(name, ".json")) {
			out = append(out, path)
		}
	}
	return out, nil
}

func findAll(root string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) && path == root {
				return nil
			}
			return err
		}
		if !d.IsDir() {
			out = append(out, path)
		}
		return nil
	})
	return out, err
}

// migrateTaskLists stores each file through tasks.Save. A file that fails to
// read or parse (a pre-schemaVersion export, say) is skipped and left in place:
// one stale file from an unrelated slug must not hide every other slug's task
// list from the tasks verifier and doctor.
func migrateTaskLists(ctx context.Context, root string) (int, error) {
	files, err := taskListFiles(root)
	if err != nil {
		return 0, err
	}
	moved := 0
	for _, path := range files {
		raw, err := os.ReadFile(filepath.Clean(path))
		if err != nil {
			fmt.Fprintf(os.Stderr, "tasks: skipping %s, could not read: %v\n", path, err)
			continue
		}
		file, err := tasks.Parse(raw)
		if err != nil {
			fmt.Fprintf(os.Stderr, "tasks: skipping %s, could not parse: %v\n", path, err)
			continue
		}
		if err := tasks.Save(ctx, root, file); err != nil {
			return moved, fmt.Errorf("store %s: %w", path, err)
		}
		if err := removeMoved(path); err != nil {
			return moved, err
		}
		moved++
	}
	return moved, nil
}

// nodeReminderFileName is the marker the post-tool node reminder kept in
// .agent-state/ before agent_state. It was derived and disposable, so it is
// deleted rather than imported: losing it costs one repeated reminder.
const nodeReminderFileName = "cursor-node.json"

func nodeReminderPath(root string) string {
	return filepath.Join(workspace.StateDir(root), nodeReminderFileName)
}

var nodeReminderSource = Source{
	Name:    "node reminder marker",
	Into:    "agent_state (rebuilt on next use)",
	Pending: func(root string) bool { return exists(nodeReminderPath(root)) },
	Migrate: func(_ context.Context, root string) (int, error) {
		if !exists(nodeReminderPath(root)) {
			return 0, nil
		}
		return 1, removeMoved(nodeReminderPath(root))
	},
}
