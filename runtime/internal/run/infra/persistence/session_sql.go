package persistence

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/run/domain"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/workspace"
)

// ambientScope keys the workspace-level session log, written when no run is
// in flight. Run logs use "<date>/<slug>/<version>".
const ambientScope = "ambient"

// corruptSuffix marks a legacy session file this package could not parse.
// Renaming it keeps one bad line from failing every later append and leaves the
// file for a person to repair.
const corruptSuffix = ".corrupt"

type sessionLocation struct {
	WorkspaceRoot string
	Scope         string
}

// parseSessionPath recognises the two session logs that live in memory.db: one
// beside a run under .agent-state/runs/..., and the ambient log directly under
// .agent-state/. A session.ndjson anywhere else is an ordinary file.
func parseSessionPath(path string) (sessionLocation, bool) {
	cleaned := filepath.Clean(path)
	if filepath.Base(cleaned) != domain.SessionLogName {
		return sessionLocation{}, false
	}
	if loc, _, ok := parseRunPath(cleaned); ok {
		return sessionLocation{
			WorkspaceRoot: loc.WorkspaceRoot,
			Scope:         fmt.Sprintf("%s/%s/%d", loc.Date, loc.Slug, loc.Version),
		}, true
	}
	dir := filepath.Dir(cleaned)
	if filepath.Base(dir) == workspace.StateDirName {
		return sessionLocation{WorkspaceRoot: filepath.Dir(dir), Scope: ambientScope}, true
	}
	return sessionLocation{}, false
}

// IsSessionSQLPath reports whether a log path is stored in the session_events table.
func IsSessionSQLPath(path string) bool {
	_, ok := parseSessionPath(path)
	return ok
}

// appendSessionSQL stores one session gesture. Sequence is assigned inside one
// statement, so two hook processes appending at once cannot pick the same number.
func appendSessionSQL(path string, event *domain.Event) (bool, error) {
	loc, ok := parseSessionPath(path)
	if !ok {
		return false, nil
	}
	ctx := context.Background()
	db, err := openDB(ctx, loc.WorkspaceRoot)
	if err != nil {
		return false, err
	}
	defer func() { _ = db.Close() }()

	// A log written before this table existed is adopted first so sequence
	// numbers continue from it. A file that cannot be adopted is set aside, not
	// allowed to wedge every later append.
	if _, statErr := os.Stat(path); statErr == nil {
		_, _ = importSessionFile(ctx, db, loc.Scope, path)
	}

	payload := string(event.Payload)
	at := event.At.UTC().Format(time.RFC3339Nano)
	err = db.QueryRowContext(ctx, `
        INSERT INTO session_events (scope, sequence, type, at, payload, created_at)
        SELECT ?, COALESCE(MAX(sequence), 0) + 1, ?, ?, ?, ?
        FROM session_events WHERE scope = ?
        RETURNING sequence`,
		loc.Scope, string(event.Type), at, payload, at, loc.Scope).Scan(&event.Sequence)
	if err != nil {
		return false, fmt.Errorf("insert session_event: %w", err)
	}
	return true, nil
}

// readSessionSQL returns the stored session events. handled is false when the
// caller should read the file instead: not a database-backed path, no database
// yet, or a legacy file still on disk (it is authoritative until imported).
func readSessionSQL(path string) (events []domain.Event, handled bool, err error) {
	loc, ok := parseSessionPath(path)
	if !ok {
		return nil, false, nil
	}
	if _, err := os.Stat(path); err == nil {
		return nil, false, nil
	}
	dbPath := workspace.MemoryDBPath(loc.WorkspaceRoot)
	if _, err := os.Stat(dbPath); err != nil {
		if os.IsNotExist(err) {
			return nil, true, nil
		}
		return nil, false, err
	}
	ctx := context.Background()
	db, err := openDB(ctx, loc.WorkspaceRoot)
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = db.Close() }()

	rows, err := db.QueryContext(ctx, `
        SELECT sequence, type, at, payload FROM session_events
        WHERE scope = ? ORDER BY sequence ASC`, loc.Scope)
	if err != nil {
		return nil, false, fmt.Errorf("query session_events: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var (
			seq              int
			typ, at, payload string
		)
		if err := rows.Scan(&seq, &typ, &at, &payload); err != nil {
			return nil, false, err
		}
		event := domain.Event{Sequence: seq, Type: domain.EventType(typ), At: parseStoredTime(at)}
		if payload != "" {
			event.Payload = json.RawMessage(payload)
		}
		events = append(events, event)
	}
	return events, true, rows.Err()
}

func parseStoredTime(value string) time.Time {
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed.UTC()
		}
	}
	return time.Time{}
}

// importSessionFile moves one legacy session.ndjson into session_events and
// removes the file. Re-running is safe: rows already present are kept.
func importSessionFile(ctx context.Context, db *sql.DB, scope, path string) (int, error) {
	events, err := readEventsFileOnly(path)
	if err != nil {
		renamed := path + corruptSuffix
		if renameErr := os.Rename(path, renamed); renameErr == nil {
			return 0, fmt.Errorf("read %s: %w (renamed to %s for manual repair)", path, err, renamed)
		}
		return 0, fmt.Errorf("read %s: %w", path, err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	moved := 0
	for index, event := range events {
		sequence := event.Sequence
		if sequence < 1 {
			sequence = index + 1
		}
		at := event.At.UTC().Format(time.RFC3339Nano)
		if event.At.IsZero() {
			at = time.Now().UTC().Format(time.RFC3339Nano)
		}
		result, err := tx.ExecContext(ctx, `
            INSERT OR IGNORE INTO session_events (scope, sequence, type, at, payload, created_at)
            VALUES (?, ?, ?, ?, ?, ?)`,
			scope, sequence, string(event.Type), at, string(event.Payload), at)
		if err != nil {
			return 0, fmt.Errorf("insert session_event %d: %w", sequence, err)
		}
		if n, _ := result.RowsAffected(); n > 0 {
			moved++
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return moved, fmt.Errorf("remove %s: %w", path, err)
	}
	return moved, nil
}

// BackfillSessions moves every legacy session.ndjson (one per run directory,
// plus the ambient log) into session_events, then removes the files. Safe to
// re-run: a workspace with none left reports zero.
func BackfillSessions(ctx context.Context, workspaceRoot string) (int, error) {
	var files []string
	runsRoot := workspace.RunsDir(workspaceRoot)
	walkErr := filepath.WalkDir(runsRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) && path == runsRoot {
				return nil
			}
			return err
		}
		if !d.IsDir() && d.Name() == domain.SessionLogName {
			files = append(files, path)
		}
		return nil
	})
	if walkErr != nil {
		return 0, walkErr
	}
	ambient := filepath.Join(workspace.StateDir(workspaceRoot), domain.SessionLogName)
	if _, err := os.Stat(ambient); err == nil {
		files = append(files, ambient)
	}
	if len(files) == 0 {
		return 0, nil
	}

	db, err := openDB(ctx, workspaceRoot)
	if err != nil {
		return 0, fmt.Errorf("open session database: %w", err)
	}
	defer func() { _ = db.Close() }()

	total := 0
	for _, path := range files {
		loc, ok := parseSessionPath(path)
		if !ok {
			fmt.Fprintf(os.Stderr, "migrate sessions: skip non-run session log %s\n", path)
			continue
		}
		moved, err := importSessionFile(ctx, db, loc.Scope, path)
		total += moved
		if err != nil {
			return total, err
		}
	}
	return total, nil
}
