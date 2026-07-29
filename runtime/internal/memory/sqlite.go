package memory

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

	// Pure Go SQLite. No cgo, so the binary cross-compiles and users need
	// neither a C toolchain nor a SQLite install.
	_ "modernc.org/sqlite"
)

// StateDirName is the workspace-local directory holding the memory database.
// It is gitignored: memory is per-workspace state, not a versioned asset.
const StateDirName = ".agent-state"

// DBPath is the memory database for a workspace.
func DBPath(workspaceRoot string) string {
	return filepath.Join(workspaceRoot, StateDirName, "memory.db")
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
func Open(workspaceRoot string) (*Store, error) {
	path := DBPath(workspaceRoot)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create state directory: %w", err)
	}
	return OpenAt(path)
}

// OpenAt opens a database at an explicit path. Use ":memory:" in tests.
func OpenAt(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open memory database: %w", err)
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("apply memory schema: %w", err)
	}
	return &Store{db: db}, nil
}

// Close releases the database.
func (s *Store) Close() error { return s.db.Close() }

// Propose runs the policy filter and stores a surviving candidate as proposed.
//
// This is the only write path model output can reach, and it cannot produce a
// confirmed record.
func (s *Store) Propose(ctx context.Context, candidate Record, now time.Time) (Record, Decision, error) {
	existing, err := s.List(ctx, candidate.WorkspaceID)
	if err != nil {
		return Record{}, Decision{}, err
	}

	if candidate.Status == "" {
		candidate.Status = StatusProposed
	}
	decision := Filter{Existing: existing}.Decide(candidate)

	switch decision.Verdict {
	case VerdictReject:
		return Record{}, decision, nil
	case VerdictMerge:
		merged, err := s.mergeEvidence(ctx, decision.MergeInto, candidate, now)
		return merged, decision, err
	}

	candidate.ID = newID()
	candidate.Status = StatusProposed
	candidate.CreatedAt = now.UTC()
	candidate.UpdatedAt = now.UTC()
	if err := s.insert(ctx, candidate); err != nil {
		return Record{}, decision, err
	}
	return candidate, decision, nil
}

// Confirm promotes a proposed memory. The caller must supply real provenance,
// which is why this takes a source rather than trusting the record's own.
func (s *Store) Confirm(ctx context.Context, id string, source SourceType, ref string, now time.Time) (Record, error) {
	if !source.valid() {
		return Record{}, fmt.Errorf("confirmation needs real provenance; %q is not a source", source)
	}
	record, err := s.Get(ctx, id)
	if err != nil {
		return Record{}, err
	}
	if record.Status == StatusStale || record.Status == StatusRejected {
		return Record{}, fmt.Errorf("memory %s is %s and cannot be confirmed", id, record.Status)
	}

	record.Status = StatusConfirmed
	record.SourceType = source
	if ref != "" {
		record.SourceRef = ref
	}
	record.UpdatedAt = now.UTC()

	if err := s.update(ctx, record); err != nil {
		return Record{}, err
	}
	// Confirming a superseding memory retires the one it replaces, so retrieval
	// never returns both sides of a contradiction.
	if record.SupersedesID != "" {
		if err := s.SetStatus(ctx, record.SupersedesID, StatusStale, now); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return Record{}, err
		}
	}
	return record, nil
}

// SetStatus changes a memory's status, for example retiring a stale fact.
func (s *Store) SetStatus(ctx context.Context, id string, status Status, now time.Time) error {
	if !status.valid() {
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
func (s *Store) Get(ctx context.Context, id string) (Record, error) {
	rows, err := s.db.QueryContext(ctx, selectColumns+` WHERE id = ?`, id)
	if err != nil {
		return Record{}, fmt.Errorf("read memory: %w", err)
	}
	defer rows.Close()
	records, err := scanRecords(rows)
	if err != nil {
		return Record{}, err
	}
	if len(records) == 0 {
		return Record{}, fmt.Errorf("no memory %s: %w", id, sql.ErrNoRows)
	}
	return records[0], nil
}

// List returns every memory for a workspace, newest first.
func (s *Store) List(ctx context.Context, workspaceID string) ([]Record, error) {
	rows, err := s.db.QueryContext(ctx,
		selectColumns+` WHERE workspace_id = ? ORDER BY created_at DESC`, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list memories: %w", err)
	}
	defer rows.Close()
	return scanRecords(rows)
}

func (s *Store) insert(ctx context.Context, record Record) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
        INSERT INTO memories (id, workspace_id, kind, content, tags, confidence,
            status, source_type, source_ref, evidence, supersedes_id, used_count,
            expires_at, created_at, updated_at)
        VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		record.ID, record.WorkspaceID, string(record.Kind), record.Content,
		strings.Join(record.Tags, " "), record.Confidence, string(record.Status),
		string(record.SourceType), record.SourceRef, strings.Join(record.Evidence, "\n"),
		record.SupersedesID, record.UsedCount, formatTimePtr(record.ExpiresAt),
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

func (s *Store) update(ctx context.Context, record Record) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
        UPDATE memories SET kind=?, content=?, tags=?, confidence=?, status=?,
            source_type=?, source_ref=?, evidence=?, supersedes_id=?,
            used_count=?, expires_at=?, updated_at=?
        WHERE id=?`,
		string(record.Kind), record.Content, strings.Join(record.Tags, " "),
		record.Confidence, string(record.Status), string(record.SourceType),
		record.SourceRef, strings.Join(record.Evidence, "\n"), record.SupersedesID,
		record.UsedCount, formatTimePtr(record.ExpiresAt),
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
func (s *Store) mergeEvidence(ctx context.Context, id string, candidate Record, now time.Time) (Record, error) {
	existing, err := s.Get(ctx, id)
	if err != nil {
		return Record{}, err
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
		return Record{}, err
	}
	return existing, nil
}

const selectColumns = `
SELECT id, workspace_id, kind, content, tags, confidence, status, source_type,
       source_ref, evidence, supersedes_id, used_count, expires_at,
       created_at, updated_at
FROM memories`

func scanRecords(rows *sql.Rows) ([]Record, error) {
	var records []Record
	for rows.Next() {
		var (
			record       Record
			tags         string
			sourceRef    sql.NullString
			evidence     string
			supersedesID sql.NullString
			expiresAt    sql.NullString
			createdAt    string
			updatedAt    string
			kind         string
			status       string
			sourceType   string
		)
		if err := rows.Scan(&record.ID, &record.WorkspaceID, &kind, &record.Content,
			&tags, &record.Confidence, &status, &sourceType, &sourceRef, &evidence,
			&supersedesID, &record.UsedCount, &expiresAt, &createdAt, &updatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan memory: %w", err)
		}
		record.Kind = Kind(kind)
		record.Status = Status(status)
		record.SourceType = SourceType(sourceType)
		record.SourceRef = sourceRef.String
		record.SupersedesID = supersedesID.String
		if tags != "" {
			record.Tags = strings.Fields(tags)
		}
		if evidence != "" {
			record.Evidence = strings.Split(evidence, "\n")
		}
		record.ExpiresAt = parseTimePtr(expiresAt)
		record.CreatedAt = parseTime(createdAt)
		record.UpdatedAt = parseTime(updatedAt)
		records = append(records, record)
	}
	return records, rows.Err()
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
