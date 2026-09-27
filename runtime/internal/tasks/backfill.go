package tasks

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Backfill moves every docs/**/tasks-*.json (and legacy tasks.json) into
// task_lists, then removes each file it moved. Safe to re-run: a workspace with
// nothing left under docs migrates zero rows rather than erroring. TASKS-*.md
// prose files are never touched.
//
// A file that fails to read or parse (a pre-schemaVersion export from an
// older toolkit build, for example) is skipped and left on disk rather than
// aborting the whole walk: one stale file from an unrelated slug must not
// hide every other slug's task list from the tasks verifier and doctor. Skip
// reasons print to stderr so a human notices without the run failing.
func Backfill(ctx context.Context, workspaceRoot string) (int, error) {
	docsRoot := filepath.Join(workspaceRoot, "docs")
	files, err := listLegacyTaskJSON(docsRoot)
	if err != nil {
		return 0, err
	}
	if len(files) == 0 {
		return 0, nil
	}

	db, err := openTaskListsDB(ctx, workspaceRoot)
	if err != nil {
		return 0, fmt.Errorf("open task_lists: %w", err)
	}
	defer func() { _ = db.Close() }()

	now := time.Now().UTC().Format(time.RFC3339)
	migrated := 0
	for _, path := range files {
		raw, err := os.ReadFile(filepath.Clean(path))
		if err != nil {
			fmt.Fprintf(os.Stderr, "tasks: skipping %s, could not read: %v\n", path, err)
			continue
		}
		file, err := Parse(raw)
		if err != nil {
			fmt.Fprintf(os.Stderr, "tasks: skipping %s, could not parse: %v\n", path, err)
			continue
		}
		id := rowID(file.Slug, file.Date, file.Version)
		_, err = db.ExecContext(ctx, `
            INSERT INTO task_lists (id, slug, date, version, body, created_by, created_at, updated_at)
            VALUES (?, ?, ?, ?, ?, '', ?, ?)
            ON CONFLICT(id) DO UPDATE SET
                body = excluded.body,
                updated_at = excluded.updated_at`,
			id, file.Slug, file.Date, file.Version, string(raw), now, now)
		if err != nil {
			return migrated, fmt.Errorf("insert task_lists %s: %w", id, err)
		}
		if err := os.Remove(path); err != nil {
			return migrated, fmt.Errorf("remove %s: %w", path, err)
		}
		migrated++
	}
	return migrated, nil
}

func listLegacyTaskJSON(docsRoot string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(docsRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) && path == docsRoot {
				return nil
			}
			return err
		}
		if d.IsDir() {
			return nil
		}
		name := d.Name()
		if name == FileName || (strings.HasPrefix(name, "tasks-") && strings.HasSuffix(name, ".json")) {
			out = append(out, path)
		}
		return nil
	})
	return out, err
}
