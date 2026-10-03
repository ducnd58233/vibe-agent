package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/memory/domain"
)

// execer is the part of *sql.DB and *sql.Tx the ledger needs, so a mutation can
// log inside its own transaction. A second connection writing while that
// transaction is open would wait on its lock until the busy timeout.
type execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// logEvent appends one ledger line.
func logEvent(ctx context.Context, db execer, memoryID, action, from, to, actor, detail string, now time.Time) error {
	_, err := db.ExecContext(ctx, `
        INSERT INTO memory_events (memory_id, action, from_status, to_status, actor, detail, at)
        VALUES (?,?,?,?,?,?,?)`,
		memoryID, action, from, to, actor, detail, now.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("record memory event: %w", err)
	}
	return nil
}

// History returns a memory's ledger, oldest first.
func (s *Store) History(ctx context.Context, id string) ([]domain.LedgerEntry, error) {
	rows, err := s.db.QueryContext(ctx, `
        SELECT id, memory_id, action, from_status, to_status, actor, detail, at
        FROM memory_events WHERE memory_id = ? ORDER BY id ASC`, id)
	if err != nil {
		return nil, fmt.Errorf("read memory history: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var entries []domain.LedgerEntry
	for rows.Next() {
		var (
			entry domain.LedgerEntry
			at    string
		)
		if err := rows.Scan(&entry.ID, &entry.MemoryID, &entry.Action, &entry.FromStatus,
			&entry.ToStatus, &entry.Actor, &entry.Detail, &at); err != nil {
			return nil, err
		}
		entry.At = parseTime(at)
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

// Link records a typed edge between two existing memories. Re-adding the same
// edge is a no-op.
func (s *Store) Link(ctx context.Context, link domain.Link, now time.Time) error {
	if err := link.Validate(); err != nil {
		return err
	}
	for _, id := range []string{link.SrcID, link.DstID} {
		if _, err := s.Get(ctx, id); err != nil {
			return err
		}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	result, err := tx.ExecContext(ctx, `
        INSERT OR IGNORE INTO memory_links (src_id, dst_id, relation, created_by, created_at)
        VALUES (?,?,?,?,?)`,
		link.SrcID, link.DstID, string(link.Relation), link.CreatedBy, now.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("insert memory link: %w", err)
	}
	if added, _ := result.RowsAffected(); added > 0 {
		if err := logEvent(ctx, tx, link.SrcID, "link", "", "", link.CreatedBy,
			string(link.Relation)+" -> "+link.DstID, now); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Links returns every edge touching a memory, in either direction.
func (s *Store) Links(ctx context.Context, id string) ([]domain.Link, error) {
	rows, err := s.db.QueryContext(ctx, `
        SELECT src_id, dst_id, relation, created_by, created_at FROM memory_links
        WHERE src_id = ? OR dst_id = ? ORDER BY created_at ASC`, id, id)
	if err != nil {
		return nil, fmt.Errorf("read memory links: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var links []domain.Link
	for rows.Next() {
		var (
			link      domain.Link
			relation  string
			createdAt string
		)
		if err := rows.Scan(&link.SrcID, &link.DstID, &relation, &link.CreatedBy, &createdAt); err != nil {
			return nil, err
		}
		link.Relation = domain.Relation(relation)
		link.CreatedAt = parseTime(createdAt)
		links = append(links, link)
	}
	return links, rows.Err()
}

// neighbours returns the ids one expanding hop away from a memory.
func (s *Store) neighbours(ctx context.Context, id string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
        SELECT CASE WHEN src_id = ? THEN dst_id ELSE src_id END, relation
        FROM memory_links WHERE src_id = ? OR dst_id = ? ORDER BY created_at ASC`, id, id, id)
	if err != nil {
		return nil, fmt.Errorf("read memory neighbours: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var ids []string
	for rows.Next() {
		var other, relation string
		if err := rows.Scan(&other, &relation); err != nil {
			return nil, err
		}
		if domain.Relation(relation).Expands() {
			ids = append(ids, other)
		}
	}
	return ids, rows.Err()
}

// SearchSessions finds past session turns matching text, best match first.
//
// Session text was redacted when it was written, so a snippet cannot carry a
// secret the writer already removed. Returned text is still history, not
// authority: the same Disclaimer applies as for a memory.
func (s *Store) SearchSessions(ctx context.Context, text string, limit int) ([]domain.SessionHit, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, errors.New("session search needs a query")
	}
	if limit <= 0 {
		limit = DefaultLimit
	}
	rows, err := s.db.QueryContext(ctx, `
        SELECT e.scope, e.sequence, e.type, e.at,
               snippet(session_events_fts, 0, '[', ']', '...', 24)
        FROM session_events_fts f
        JOIN session_events e ON e.rowid = f.rowid
        WHERE session_events_fts MATCH ?
        ORDER BY bm25(session_events_fts) ASC, e.at DESC
        LIMIT ?`, ftsQuery(text), limit)
	if err != nil {
		return nil, fmt.Errorf("search sessions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var hits []domain.SessionHit
	for rows.Next() {
		var (
			hit domain.SessionHit
			at  string
		)
		if err := rows.Scan(&hit.Scope, &hit.Sequence, &hit.Type, &at, &hit.Snippet); err != nil {
			return nil, err
		}
		hit.At = parseTime(at)
		hits = append(hits, hit)
	}
	return hits, rows.Err()
}
