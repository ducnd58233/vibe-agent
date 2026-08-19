package persistence

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base32"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/memory/domain"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/infra/database"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/workspace"
)

// dbFileName is this store's file inside the workspace state directory.
// It is gitignored: memory is per-workspace state, not a versioned asset.
const dbFileName = "memory.db"

// DBPath is the memory database for a workspace.
func DBPath(workspaceRoot string) string {
	return filepath.Join(workspace.StateDir(workspaceRoot), dbFileName)
}

const schema = `
CREATE TABLE IF NOT EXISTS memories (
    id            TEXT PRIMARY KEY,
    workspace_id  TEXT NOT NULL,
    kind          TEXT NOT NULL,
    content       TEXT NOT NULL,
    tags          TEXT NOT NULL DEFAULT '',
    confidence    REAL NOT NULL,
    status        TEXT NOT NULL,
    source_type   TEXT NOT NULL,
    source_ref    TEXT,
    evidence      TEXT NOT NULL,
    supersedes_id TEXT,
    used_count    INTEGER NOT NULL DEFAULT 0,
    expires_at    TEXT,
    valid_from    TEXT NOT NULL DEFAULT '',
    valid_to      TEXT,
    created_at    TEXT NOT NULL,
    updated_at    TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS memories_workspace_status
    ON memories(workspace_id, status, kind);

CREATE VIRTUAL TABLE IF NOT EXISTS memories_fts USING fts5(
    memory_id UNINDEXED,
    content,
    tags
);
`

// Store is the memory database.
type Store struct {
	db *sql.DB
}

// Open creates or opens the memory database for a workspace.
//
// The context covers the schema work Open does: creating tables and adding the
// columns an older database is missing. A caller that gave up waiting should not
// leave a migration running behind it.
func Open(ctx context.Context, workspaceRoot string) (*Store, error) {
	path := DBPath(workspaceRoot)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, fmt.Errorf("create state directory: %w", err)
	}
	return OpenAt(ctx, path)
}

// OpenAt opens a database at an explicit path. Use ":memory:" in tests.
func OpenAt(ctx context.Context, path string) (*Store, error) {
	db, err := database.Open(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("open memory database: %w", err)
	}
	if _, err := db.ExecContext(ctx, schema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("apply memory schema: %w", err)
	}
	if err := migrate(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

// migrate adds columns to a database created by an earlier version.
//
// CREATE TABLE IF NOT EXISTS does nothing to a table that already exists, so a
// workspace that has been storing memories since before the validity interval
// would otherwise fail every query against the new columns.
func migrate(ctx context.Context, db *sql.DB) error {
	rows, err := db.QueryContext(ctx, `PRAGMA table_info(memories)`)
	if err != nil {
		return fmt.Errorf("inspect memory schema: %w", err)
	}
	defer func() { _ = rows.Close() }()
	present := map[string]bool{}
	for rows.Next() {
		var (
			index      int
			name       string
			columnType string
			notNull    int
			preset     sql.NullString
			primary    int
		)
		if err := rows.Scan(&index, &name, &columnType, &notNull, &preset, &primary); err != nil {
			return fmt.Errorf("read memory schema: %w", err)
		}
		present[name] = true
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read memory schema: %w", err)
	}

	for _, column := range []struct{ name, definition string }{
		{"valid_from", `valid_from TEXT NOT NULL DEFAULT ''`},
		{"valid_to", `valid_to TEXT`},
	} {
		if present[column.name] {
			continue
		}
		if _, err := db.ExecContext(ctx, `ALTER TABLE memories ADD COLUMN `+column.definition); err != nil {
			return fmt.Errorf("add memory column %s: %w", column.name, err)
		}
	}

	// Rows written before the interval existed were true from when they were
	// recorded, which is the closest honest answer available.
	_, err = db.ExecContext(ctx, `UPDATE memories SET valid_from = created_at WHERE valid_from = ''`)
	if err != nil {
		return fmt.Errorf("backfill memory validity: %w", err)
	}
	return nil
}

// Close releases the database.
func (s *Store) Close() error { return s.db.Close() }

// Propose runs the policy filter and stores a surviving candidate as proposed.
//
// This is the only write path model output can reach, and it cannot produce a
// confirmed record.
func (s *Store) Propose(ctx context.Context, candidate domain.Record, now time.Time) (domain.Record, domain.Decision, error) {
	existing, err := s.List(ctx, candidate.WorkspaceID)
	if err != nil {
		return domain.Record{}, domain.Decision{}, err
	}

	if candidate.Status == "" {
		candidate.Status = domain.StatusProposed
	}
	decision := domain.Filter{Existing: existing}.Decide(candidate)

	switch decision.Verdict {
	case domain.VerdictReject:
		return domain.Record{}, decision, nil
	case domain.VerdictMerge:
		merged, err := s.mergeEvidence(ctx, decision.MergeInto, candidate, now)
		return merged, decision, err
	}

	candidate.ID = newID()
	candidate.Status = domain.StatusProposed
	candidate.CreatedAt = now.UTC()
	candidate.UpdatedAt = now.UTC()
	// A candidate that does not say when its fact started being true started
	// being true when it was observed.
	if candidate.ValidFrom.IsZero() {
		candidate.ValidFrom = now.UTC()
	}
	if err := s.insert(ctx, candidate); err != nil {
		return domain.Record{}, decision, err
	}
	return candidate, decision, nil
}

// Confirm promotes a proposed memory. The caller must supply real provenance,
// which is why this takes a source rather than trusting the record's own.
func (s *Store) Confirm(ctx context.Context, id string, source domain.SourceType, ref string, now time.Time) (domain.Record, error) {
	if !source.Valid() {
		return domain.Record{}, fmt.Errorf("confirmation needs real provenance; %q is not a source", source)
	}
	record, err := s.Get(ctx, id)
	if err != nil {
		return domain.Record{}, err
	}
	if record.Status == domain.StatusStale || record.Status == domain.StatusRejected {
		return domain.Record{}, fmt.Errorf("memory %s is %s and cannot be confirmed", id, record.Status)
	}

	record.Status = domain.StatusConfirmed
	record.SourceType = source
	if ref != "" {
		record.SourceRef = ref
	}
	record.UpdatedAt = now.UTC()

	if err := s.update(ctx, record); err != nil {
		return domain.Record{}, err
	}
	// Confirming a superseding memory closes the one it replaces, so retrieval
	// never returns both sides of a contradiction.
	//
	// It closes at this record's ValidFrom rather than at "now": the old fact
	// stopped being true when the new one started, which is not always the
	// moment somebody got around to recording it.
	if record.SupersedesID != "" {
		if err := s.Invalidate(ctx, record.SupersedesID, record.ValidFrom); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return domain.Record{}, err
		}
	}
	return record, nil
}

// Invalidate closes a memory's validity interval and marks it stale.
//
// Nothing is deleted. A fact that stopped being true is different from one that
// was never recorded, and only the first can explain a decision made while it
// still held.
func (s *Store) Invalidate(ctx context.Context, id string, at time.Time) error {
	result, err := s.db.ExecContext(ctx,
		`UPDATE memories SET valid_to = ?, status = ?, updated_at = ?
         WHERE id = ? AND valid_to IS NULL`,
		at.UTC().Format(ExpiryLayout), string(domain.StatusStale),
		at.UTC().Format(time.RFC3339Nano), id)
	if err != nil {
		return fmt.Errorf("invalidate memory: %w", err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// SetStatus changes a memory's status, for example retiring a stale fact.
func (s *Store) SetStatus(ctx context.Context, id string, status domain.Status, now time.Time) error {
	if !status.Valid() {
		return fmt.Errorf("status %q is not known", status)
	}
	result, err := s.db.ExecContext(ctx,
		`UPDATE memories SET status = ?, updated_at = ? WHERE id = ?`,
		string(status), now.UTC().Format(time.RFC3339Nano), id)
	if err != nil {
		return fmt.Errorf("update memory status: %w", err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// RecordUse counts a successful reuse, which feeds promotion proposals.
func (s *Store) RecordUse(ctx context.Context, id string, now time.Time) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE memories SET used_count = used_count + 1, updated_at = ? WHERE id = ?`,
		now.UTC().Format(time.RFC3339Nano), id)
	if err != nil {
		return fmt.Errorf("record memory use: %w", err)
	}
	return nil
}

// Get returns one memory.
func (s *Store) Get(ctx context.Context, id string) (domain.Record, error) {
	rows, err := s.db.QueryContext(ctx, selectColumns+` WHERE id = ?`, id)
	if err != nil {
		return domain.Record{}, fmt.Errorf("read memory: %w", err)
	}
	defer func() { _ = rows.Close() }()
	records, err := scanRecords(rows)
	if err != nil {
		return domain.Record{}, err
	}
	if len(records) == 0 {
		return domain.Record{}, fmt.Errorf("no memory %s: %w", id, sql.ErrNoRows)
	}
	return records[0], nil
}

// List returns every memory for a workspace, newest first.
func (s *Store) List(ctx context.Context, workspaceID string) ([]domain.Record, error) {
	rows, err := s.db.QueryContext(ctx,
		selectColumns+` WHERE workspace_id = ? ORDER BY created_at DESC`, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list memories: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanRecords(rows)
}

func (s *Store) insert(ctx context.Context, record domain.Record) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `
        INSERT INTO memories (id, workspace_id, kind, content, tags, confidence,
            status, source_type, source_ref, evidence, supersedes_id, used_count,
            expires_at, valid_from, valid_to, created_at, updated_at)
        VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		record.ID, record.WorkspaceID, string(record.Kind), record.Content,
		strings.Join(record.Tags, " "), record.Confidence, string(record.Status),
		string(record.SourceType), record.SourceRef, strings.Join(record.Evidence, "\n"),
		record.SupersedesID, record.UsedCount, formatTimePtr(record.ExpiresAt),
		record.ValidFrom.UTC().Format(ExpiryLayout), formatTimePtr(record.ValidTo),
		record.CreatedAt.Format(time.RFC3339Nano), record.UpdatedAt.Format(time.RFC3339Nano),
	); err != nil {
		return fmt.Errorf("insert memory: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO memories_fts (memory_id, content, tags) VALUES (?,?,?)`,
		record.ID, record.Content, strings.Join(record.Tags, " "),
	); err != nil {
		return fmt.Errorf("index memory: %w", err)
	}
	return tx.Commit()
}

func (s *Store) update(ctx context.Context, record domain.Record) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `
        UPDATE memories SET kind=?, content=?, tags=?, confidence=?, status=?,
            source_type=?, source_ref=?, evidence=?, supersedes_id=?,
            used_count=?, expires_at=?, valid_from=?, valid_to=?, updated_at=?
        WHERE id=?`,
		string(record.Kind), record.Content, strings.Join(record.Tags, " "),
		record.Confidence, string(record.Status), string(record.SourceType),
		record.SourceRef, strings.Join(record.Evidence, "\n"), record.SupersedesID,
		record.UsedCount, formatTimePtr(record.ExpiresAt),
		record.ValidFrom.UTC().Format(ExpiryLayout), formatTimePtr(record.ValidTo),
		record.UpdatedAt.Format(time.RFC3339Nano), record.ID,
	); err != nil {
		return fmt.Errorf("update memory: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE memories_fts SET content=?, tags=? WHERE memory_id=?`,
		record.Content, strings.Join(record.Tags, " "), record.ID,
	); err != nil {
		return fmt.Errorf("reindex memory: %w", err)
	}
	return tx.Commit()
}

// mergeEvidence folds a duplicate candidate's evidence into the record it
// duplicates, rather than storing the same claim twice.
func (s *Store) mergeEvidence(ctx context.Context, id string, candidate domain.Record, now time.Time) (domain.Record, error) {
	existing, err := s.Get(ctx, id)
	if err != nil {
		return domain.Record{}, err
	}
	seen := map[string]bool{}
	for _, item := range existing.Evidence {
		seen[item] = true
	}
	for _, item := range candidate.Evidence {
		if !seen[item] {
			existing.Evidence = append(existing.Evidence, item)
			seen[item] = true
		}
	}
	if candidate.Confidence > existing.Confidence {
		existing.Confidence = candidate.Confidence
	}
	existing.UpdatedAt = now.UTC()
	if err := s.update(ctx, existing); err != nil {
		return domain.Record{}, err
	}
	return existing, nil
}

const selectColumns = `
SELECT id, workspace_id, kind, content, tags, confidence, status, source_type,
       source_ref, evidence, supersedes_id, used_count, expires_at,
       valid_from, valid_to, created_at, updated_at
FROM memories`

func scanRecords(rows *sql.Rows) ([]domain.Record, error) {
	var records []domain.Record
	for rows.Next() {
		record, err := scanRecord(rows.Scan)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

// scanRecord reads the shared column list. Search adds a score column of its
// own, so it passes a scan function that consumes one more value rather than
// duplicating this list and letting the two drift.
func scanRecord(scan func(...any) error, extra ...any) (domain.Record, error) {
	var (
		record       domain.Record
		tags         string
		sourceRef    sql.NullString
		evidence     string
		supersedesID sql.NullString
		expiresAt    sql.NullString
		validFrom    string
		validTo      sql.NullString
		createdAt    string
		updatedAt    string
		kind         string
		status       string
		sourceType   string
	)
	targets := append([]any{&record.ID, &record.WorkspaceID, &kind, &record.Content,
		&tags, &record.Confidence, &status, &sourceType, &sourceRef, &evidence,
		&supersedesID, &record.UsedCount, &expiresAt, &validFrom, &validTo,
		&createdAt, &updatedAt}, extra...)

	if err := scan(targets...); err != nil {
		return domain.Record{}, fmt.Errorf("scan memory: %w", err)
	}
	record.Kind = domain.Kind(kind)
	record.Status = domain.Status(status)
	record.SourceType = domain.SourceType(sourceType)
	record.SourceRef = sourceRef.String
	record.SupersedesID = supersedesID.String
	if tags != "" {
		record.Tags = strings.Fields(tags)
	}
	if evidence != "" {
		record.Evidence = strings.Split(evidence, "\n")
	}
	record.ExpiresAt = parseTimePtr(expiresAt)
	record.ValidFrom = parseTime(validFrom)
	record.ValidTo = parseTimePtr(validTo)
	record.CreatedAt = parseTime(createdAt)
	record.UpdatedAt = parseTime(updatedAt)
	return record, nil
}

var idEncoding = base32.NewEncoding("0123456789ABCDEFGHJKMNPQRSTVWXYZ").WithPadding(base32.NoPadding)

func newID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		// crypto/rand failing means the process cannot generate ids at all.
		panic(fmt.Sprintf("memory: cannot generate id: %v", err))
	}
	return "mem_" + idEncoding.EncodeToString(buf)[:26]
}

// ExpiryLayout is fixed-width UTC RFC3339 at second precision, so expiry
// comparisons are plain lexicographic string comparisons in SQL. Nanosecond
// precision would make the strings variable width and break that ordering.
const ExpiryLayout = "2006-01-02T15:04:05Z"

func formatTimePtr(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.UTC().Format(ExpiryLayout)
}

func parseTime(value string) time.Time {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}
	}
	return parsed.UTC()
}

func parseTimePtr(value sql.NullString) *time.Time {
	if !value.Valid || value.String == "" {
		return nil
	}
	if parsed, err := time.Parse(ExpiryLayout, value.String); err == nil {
		utc := parsed.UTC()
		return &utc
	}
	parsed := parseTime(value.String)
	if parsed.IsZero() {
		return nil
	}
	return &parsed
}
