// Package agentstate keeps small keyed state for hooks in memory.db's
// agent_state table.
//
// It exists so a hook that needs to remember one value between calls, such as
// "which node did I last announce", does not invent a JSON file under
// .agent-state/. Values are opaque strings; callers own their encoding.
package agentstate

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/infra/database"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/workspace"
)

// Get returns the stored value. ok is false when the key, or the database
// itself, does not exist: a read never creates memory.db.
func Get(ctx context.Context, workspaceRoot, namespace, key string) (value string, ok bool, err error) {
	path := workspace.MemoryDBPath(workspaceRoot)
	if _, statErr := os.Stat(path); statErr != nil {
		if errors.Is(statErr, os.ErrNotExist) {
			return "", false, nil
		}
		return "", false, statErr
	}
	db, err := database.Open(ctx, path)
	if err != nil {
		return "", false, err
	}
	defer func() { _ = db.Close() }()

	err = db.QueryRowContext(ctx,
		`SELECT value FROM agent_state WHERE namespace = ? AND key = ?`, namespace, key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("read agent_state: %w", err)
	}
	return value, true, nil
}

// Set stores a value, creating the database on first use.
func Set(ctx context.Context, workspaceRoot, namespace, key, value string) error {
	path := workspace.MemoryDBPath(workspaceRoot)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("create state directory: %w", err)
	}
	db, err := database.Open(ctx, path)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	_, err = db.ExecContext(ctx, `
        INSERT INTO agent_state (namespace, key, value, updated_at) VALUES (?, ?, ?, ?)
        ON CONFLICT(namespace, key) DO UPDATE SET
            value = excluded.value, updated_at = excluded.updated_at`,
		namespace, key, value, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("write agent_state: %w", err)
	}
	return nil
}

// Delete removes a key. Absent keys and databases are not errors.
func Delete(ctx context.Context, workspaceRoot, namespace, key string) error {
	path := workspace.MemoryDBPath(workspaceRoot)
	if _, err := os.Stat(path); err != nil {
		return nil
	}
	db, err := database.Open(ctx, path)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()
	_, err = db.ExecContext(ctx, `DELETE FROM agent_state WHERE namespace = ? AND key = ?`, namespace, key)
	return err
}
