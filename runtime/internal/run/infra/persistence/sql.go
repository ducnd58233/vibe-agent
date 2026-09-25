package persistence

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/run/domain"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/infra/database"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/runpath"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/validate"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/workspace"
)

func init() {
	// runpath must not import this package; Resolve asks here when the
	// run-index file is gone after backfill.
	runpath.ResolveSQL = func(workspaceRoot, slug string) (runpath.Entry, bool) {
		entry, ok, err := LatestEntry(workspaceRoot, slug)
		if err != nil || !ok {
			return runpath.Entry{}, false
		}
		return entry, true
	}
}

// openDB opens the shared workspace database. Schema is applied by
// database.Open from runtime/migrations. Caller closes the connection.
func openDB(ctx context.Context, workspaceRoot string) (*sql.DB, error) {
	path := workspace.MemoryDBPath(workspaceRoot)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, err
	}
	return database.Open(ctx, path)
}

// runLocation is one versioned run directory under .agent-state/runs/.
type runLocation struct {
	WorkspaceRoot string
	Date          string
	Slug          string
	Version       int
}

// parseRunPath reads slug/date/version/workspace from a manifest or events path
// under .agent-state/runs/<date>/<slug>/<version>/.
func parseRunPath(path string) (runLocation, string, bool) {
	cleaned := filepath.Clean(path)
	base := filepath.Base(cleaned)
	dir := filepath.Dir(cleaned)
	versionStr := filepath.Base(dir)
	slug := filepath.Base(filepath.Dir(dir))
	date := filepath.Base(filepath.Dir(filepath.Dir(dir)))
	runsSeg := filepath.Base(filepath.Dir(filepath.Dir(filepath.Dir(dir))))
	stateSeg := filepath.Base(filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(dir)))))
	workspaceRoot := filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(dir)))))

	version, err := strconv.Atoi(versionStr)
	if err != nil || version < 1 {
		return runLocation{}, "", false
	}
	if runsSeg != workspace.RunsDirName || stateSeg != workspace.StateDirName {
		return runLocation{}, "", false
	}
	if !validate.Date(date) || !validate.Slug(slug) {
		return runLocation{}, "", false
	}
	return runLocation{
		WorkspaceRoot: workspaceRoot,
		Date:          date,
		Slug:          slug,
		Version:       version,
	}, base, true
}

// LatestEntry returns the highest-version run for a slug from the runs table.
// ok is false when the slug has no row, or when memory.db is not there yet
// (a read must not create the database).
func LatestEntry(workspaceRoot, slug string) (runpath.Entry, bool, error) {
	if !validate.Slug(slug) {
		return runpath.Entry{}, false, fmt.Errorf("slug %q is not usable", slug)
	}
	ctx := context.Background()
	dbPath := workspace.MemoryDBPath(workspaceRoot)
	if _, err := os.Stat(dbPath); err != nil {
		if os.IsNotExist(err) {
			return runpath.Entry{}, false, nil
		}
		return runpath.Entry{}, false, err
	}
	db, err := openDB(ctx, workspaceRoot)
	if err != nil {
		return runpath.Entry{}, false, err
	}
	defer func() { _ = db.Close() }()

	var date string
	var version int
	err = db.QueryRowContext(ctx, `
        SELECT date, version FROM runs WHERE slug = ?
        ORDER BY version DESC LIMIT 1`, slug).Scan(&date, &version)
	if errors.Is(err, sql.ErrNoRows) {
		return runpath.Entry{}, false, nil
	}
	if err != nil {
		return runpath.Entry{}, false, err
	}
	return runpath.Entry{Slug: slug, Date: date, Version: version}, true, nil
}

func listSlugsFromDB(workspaceRoot string, seen map[string]bool) error {
	ctx := context.Background()
	path := workspace.MemoryDBPath(workspaceRoot)
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	db, err := openDB(ctx, workspaceRoot)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	rows, err := db.QueryContext(ctx, `SELECT DISTINCT slug FROM runs`)
	if err != nil {
		return fmt.Errorf("list runs slugs: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var slug string
		if err := rows.Scan(&slug); err != nil {
			return err
		}
		if validate.Slug(slug) {
			seen[slug] = true
		}
	}
	return rows.Err()
}

func loadRunFromDB(ctx context.Context, db *sql.DB, loc runLocation) (*domain.Run, error) {
	var body string
	err := db.QueryRowContext(ctx, `
        SELECT body FROM runs WHERE slug = ? AND date = ? AND version = ?`,
		loc.Slug, loc.Date, loc.Version).Scan(&body)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, sql.ErrNoRows
	}
	if err != nil {
		return nil, err
	}
	return decodeRunBody(body)
}

func decodeRunBody(body string) (*domain.Run, error) {
	var run domain.Run
	if err := json.Unmarshal([]byte(body), &run); err != nil {
		return nil, fmt.Errorf("parse run body: %w", err)
	}
	if err := run.Validate(); err != nil {
		return nil, fmt.Errorf("run state is invalid: %w", err)
	}
	return &run, nil
}

func upsertRun(ctx context.Context, db *sql.DB, run *domain.Run, loc runLocation) error {
	body, err := json.Marshal(run)
	if err != nil {
		return fmt.Errorf("encode run state: %w", err)
	}
	date := run.Date
	version := run.Version
	if date == "" {
		date = loc.Date
	}
	if version < 1 {
		version = loc.Version
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err = db.ExecContext(ctx, `
        INSERT INTO runs (
            run_id, slug, date, version, graph_id, current_node, status,
            iteration, max_transitions, token_budget, wallclock_seconds,
            tokens_used, stopped_by, body, created_by, created_at, updated_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, '', ?, ?)
        ON CONFLICT(run_id) DO UPDATE SET
            slug = excluded.slug,
            date = excluded.date,
            version = excluded.version,
            graph_id = excluded.graph_id,
            current_node = excluded.current_node,
            status = excluded.status,
            iteration = excluded.iteration,
            max_transitions = excluded.max_transitions,
            token_budget = excluded.token_budget,
            wallclock_seconds = excluded.wallclock_seconds,
            tokens_used = excluded.tokens_used,
            stopped_by = excluded.stopped_by,
            body = excluded.body,
            updated_at = excluded.updated_at`,
		run.RunID, run.Slug, date, version, run.GraphID, run.CurrentNode, string(run.Status),
		run.Iteration, run.MaxTransitions, run.TokenBudget, run.WallclockSeconds,
		run.TokensUsed, run.StoppedBy, string(body), now, now)
	if err != nil {
		return fmt.Errorf("upsert run %s: %w", run.RunID, err)
	}
	return nil
}

func saveRunSQL(path string, run *domain.Run) error {
	loc, base, ok := parseRunPath(path)
	if !ok || base != "manifest.json" {
		return nil
	}
	ctx := context.Background()
	db, err := openDB(ctx, loc.WorkspaceRoot)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()
	return upsertRun(ctx, db, run, loc)
}

func loadRunSQL(path string) (*domain.Run, bool, error) {
	loc, base, ok := parseRunPath(path)
	if !ok || base != "manifest.json" {
		return nil, false, nil
	}
	ctx := context.Background()
	dbPath := workspace.MemoryDBPath(loc.WorkspaceRoot)
	if _, err := os.Stat(dbPath); err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	db, err := openDB(ctx, loc.WorkspaceRoot)
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = db.Close() }()

	run, err := loadRunFromDB(ctx, db, loc)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return run, true, nil
}

func resolveRunID(ctx context.Context, db *sql.DB, loc runLocation) (string, error) {
	var runID string
	err := db.QueryRowContext(ctx, `
        SELECT run_id FROM runs WHERE slug = ? AND date = ? AND version = ?`,
		loc.Slug, loc.Date, loc.Version).Scan(&runID)
	if err == nil {
		return runID, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	manifest := filepath.Join(workspace.RunDirAt(loc.WorkspaceRoot, loc.Date, loc.Slug, loc.Version), "manifest.json")
	raw, readErr := os.ReadFile(filepath.Clean(manifest))
	if readErr != nil {
		return "", sql.ErrNoRows
	}
	run, decodeErr := decodeRunBody(string(raw))
	if decodeErr != nil {
		return "", decodeErr
	}
	if err := upsertRun(ctx, db, run, loc); err != nil {
		return "", err
	}
	return run.RunID, nil
}

func nextEventSequence(ctx context.Context, db *sql.DB, runID, filePath string) (int, error) {
	var maxSeq sql.NullInt64
	if err := db.QueryRowContext(ctx,
		`SELECT MAX(sequence) FROM run_events WHERE run_id = ?`, runID).Scan(&maxSeq); err != nil {
		return 0, err
	}
	fileCount, err := countLines(filePath)
	if err != nil {
		return 0, err
	}
	sqlCount := 0
	if maxSeq.Valid {
		sqlCount = int(maxSeq.Int64)
	}
	if fileCount > sqlCount {
		return fileCount + 1, nil
	}
	return sqlCount + 1, nil
}

func appendEventSQL(path string, event *domain.Event) (bool, error) {
	loc, base, ok := parseRunPath(path)
	if !ok || base != domain.EventLogName {
		return false, nil
	}
	ctx := context.Background()
	db, err := openDB(ctx, loc.WorkspaceRoot)
	if err != nil {
		return false, err
	}
	defer func() { _ = db.Close() }()

	runID, err := resolveRunID(ctx, db, loc)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	seq, err := nextEventSequence(ctx, db, runID, path)
	if err != nil {
		return false, err
	}
	event.Sequence = seq

	payload := ""
	if len(event.Payload) > 0 {
		payload = string(event.Payload)
	}
	now := event.At.UTC().Format(time.RFC3339)
	_, err = db.ExecContext(ctx, `
        INSERT INTO run_events (
            run_id, sequence, type, node, at, payload,
            created_by, reviewed_by_agents, created_at, updated_at)
        VALUES (?, ?, ?, ?, ?, ?, '', '', ?, ?)`,
		runID, event.Sequence, string(event.Type), event.Node, now, payload, now, now)
	if err != nil {
		return false, fmt.Errorf("insert run_event: %w", err)
	}
	return true, nil
}

func readEventsSQL(path string) ([]domain.Event, bool, error) {
	loc, base, ok := parseRunPath(path)
	if !ok || base != domain.EventLogName {
		return nil, false, nil
	}
	ctx := context.Background()
	dbPath := workspace.MemoryDBPath(loc.WorkspaceRoot)
	if _, err := os.Stat(dbPath); err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	db, err := openDB(ctx, loc.WorkspaceRoot)
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = db.Close() }()

	var runID string
	err = db.QueryRowContext(ctx, `
        SELECT run_id FROM runs WHERE slug = ? AND date = ? AND version = ?`,
		loc.Slug, loc.Date, loc.Version).Scan(&runID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}

	rows, err := db.QueryContext(ctx, `
        SELECT sequence, type, node, at, payload FROM run_events
        WHERE run_id = ? ORDER BY sequence ASC`, runID)
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = rows.Close() }()

	var events []domain.Event
	for rows.Next() {
		var (
			seq           int
			typ, node, at string
			payload       string
		)
		if err := rows.Scan(&seq, &typ, &node, &at, &payload); err != nil {
			return nil, false, err
		}
		parsedAt, err := time.Parse(time.RFC3339, at)
		if err != nil {
			parsedAt, err = time.Parse(time.RFC3339Nano, at)
			if err != nil {
				return nil, false, fmt.Errorf("parse event at %q: %w", at, err)
			}
		}
		ev := domain.Event{
			Sequence: seq,
			Type:     domain.EventType(typ),
			Node:     node,
			At:       parsedAt,
		}
		if payload != "" {
			ev.Payload = json.RawMessage(payload)
		}
		events = append(events, ev)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	if len(events) == 0 {
		return nil, false, nil
	}
	return events, true, nil
}

// deleteRunSQLRow removes one runs row and its events. Tests use this to force
// a file-only Load after forging a manifest on disk.
func deleteRunSQLRow(workspaceRoot, runID string) error {
	ctx := context.Background()
	db, err := openDB(ctx, workspaceRoot)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()
	if _, err := db.ExecContext(ctx, `DELETE FROM run_events WHERE run_id = ?`, runID); err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, `DELETE FROM runs WHERE run_id = ?`, runID)
	return err
}
