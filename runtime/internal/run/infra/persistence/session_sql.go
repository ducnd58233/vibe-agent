package persistence

import (
	"context"
	"encoding/json"
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
// path is not a database-backed one and the caller should read the file. A
// session.ndjson left by an older build is not read here; the legacy module
// imports it (vibe-agent migrate state) and doctor reports one that remains.
func readSessionSQL(path string) (events []domain.Event, handled bool, err error) {
	loc, ok := parseSessionPath(path)
	if !ok {
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

// ImportSessionEvents stores events under the session log path names, keeping
// their sequence numbers. Events already present for a sequence are kept, so
// importing the same events twice changes nothing. It returns how many were
// new. A path that is not database-backed is refused.
func ImportSessionEvents(ctx context.Context, path string, events []domain.Event) (int, error) {
	loc, ok := parseSessionPath(path)
	if !ok {
		return 0, fmt.Errorf("%s is not a session log the database stores", path)
	}
	db, err := openDB(ctx, loc.WorkspaceRoot)
	if err != nil {
		return 0, err
	}
	defer func() { _ = db.Close() }()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	stored := 0
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
			loc.Scope, sequence, string(event.Type), at, string(event.Payload), at)
		if err != nil {
			return stored, fmt.Errorf("insert session_event %d: %w", sequence, err)
		}
		if n, _ := result.RowsAffected(); n > 0 {
			stored++
		}
	}
	return stored, tx.Commit()
}
