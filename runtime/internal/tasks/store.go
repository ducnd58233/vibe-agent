package tasks

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/infra/database"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/workspace"
)

// rowID is the primary key for one task list revision: slug, date, and version
// joined so a single TEXT key stays unique without a composite PK.
func rowID(slug, date string, version int) string {
	return fmt.Sprintf("%s/%s/%d", slug, date, version)
}

func openTaskListsDB(ctx context.Context, workspaceRoot string) (*sql.DB, error) {
	path := workspace.MemoryDBPath(workspaceRoot)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, err
	}
	return database.Open(ctx, path)
}

func loadBodyFromDB(ctx context.Context, db *sql.DB, slug, date string, version int) ([]byte, error) {
	var body string
	err := db.QueryRowContext(ctx, `
        SELECT body FROM task_lists WHERE id = ?`, rowID(slug, date, version)).Scan(&body)
	if err == sql.ErrNoRows {
		return nil, os.ErrNotExist
	}
	if err != nil {
		return nil, err
	}
	return []byte(body), nil
}

func loadLatestBodyFromDB(ctx context.Context, db *sql.DB, slug string) ([]byte, error) {
	var body string
	err := db.QueryRowContext(ctx, `
        SELECT body FROM task_lists WHERE slug = ?
        ORDER BY date DESC, version DESC LIMIT 1`, slug).Scan(&body)
	if err == sql.ErrNoRows {
		return nil, os.ErrNotExist
	}
	if err != nil {
		return nil, err
	}
	return []byte(body), nil
}

// Save writes a task list into task_lists. Callers that used to Edit a
// tasks-*.json file use this instead once the file mirror is gone.
func Save(workspaceRoot string, file *File) error {
	if file == nil {
		return fmt.Errorf("save task list: file is nil")
	}
	raw, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return fmt.Errorf("encode task list: %w", err)
	}
	raw = append(raw, '\n')
	ctx := context.Background()
	db, err := openTaskListsDB(ctx, workspaceRoot)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	now := time.Now().UTC().Format(time.RFC3339)
	id := rowID(file.Slug, file.Date, file.Version)
	_, err = db.ExecContext(ctx, `
        INSERT INTO task_lists (id, slug, date, version, body, created_by, created_at, updated_at)
        VALUES (?, ?, ?, ?, ?, '', ?, ?)
        ON CONFLICT(id) DO UPDATE SET
            body = excluded.body,
            updated_at = excluded.updated_at`,
		id, file.Slug, file.Date, file.Version, string(raw), now, now)
	if err != nil {
		return fmt.Errorf("upsert task_lists %s: %w", id, err)
	}
	return nil
}

// HasTaskList reports whether a slug has a task list in the database.
func HasTaskList(workspaceRoot, slug string) bool {
	_, err := Load(workspaceRoot, slug)
	return err == nil
}
