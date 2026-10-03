package persistence

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// PruneOptions chooses what Prune may remove. Both limits are opt-in by being
// non-zero: Prune never decides on its own how much history is too much.
type PruneOptions struct {
	Now time.Time
	// ExpiredFor removes memories whose expiry passed at least this long ago. An
	// expired memory is already excluded from retrieval, so this reclaims space
	// and index size without changing any answer. Zero skips memories.
	ExpiredFor time.Duration
	// SessionsOlderThan removes session events older than this. Zero keeps them.
	SessionsOlderThan time.Duration
	// DryRun counts without deleting.
	DryRun bool
}

// PruneResult reports what was (or, on a dry run, would be) removed.
type PruneResult struct {
	Memories int `json:"memories"`
	Links    int `json:"links"`
	Sessions int `json:"sessions"`
}

// Prune removes memories that expired and session history past a retention
// window, then compacts the indexes.
//
// Only memories that carry an expiry are ever removed: that is the author saying
// the fact had a shelf life ("this command fails" is true about a moment). A
// memory with no expiry is closed with Invalidate, never deleted. The ledger
// keeps its lines for a pruned memory and gains one saying it was pruned, so the
// history still explains why the row is gone.
func (s *Store) Prune(ctx context.Context, opts PruneOptions) (PruneResult, error) {
	var result PruneResult
	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}
	now = now.UTC()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return result, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if opts.ExpiredFor > 0 {
		cutoff := now.Add(-opts.ExpiredFor).Format(ExpiryLayout)
		ids, err := queryStrings(ctx, tx, `SELECT id FROM memories WHERE expires_at IS NOT NULL AND expires_at < ?`, cutoff)
		if err != nil {
			return result, err
		}
		result.Memories = len(ids)
		for _, id := range ids {
			if opts.DryRun {
				continue
			}
			links, err := tx.ExecContext(ctx, `DELETE FROM memory_links WHERE src_id = ? OR dst_id = ?`, id, id)
			if err != nil {
				return result, fmt.Errorf("prune links: %w", err)
			}
			n, _ := links.RowsAffected()
			result.Links += int(n)
			for _, statement := range []string{
				`DELETE FROM memory_exposures WHERE memory_id = ?`,
				`DELETE FROM memories_fts WHERE memory_id = ?`,
				`DELETE FROM memories WHERE id = ?`,
			} {
				if _, err := tx.ExecContext(ctx, statement, id); err != nil {
					return result, fmt.Errorf("prune memory %s: %w", id, err)
				}
			}
			if err := logEvent(ctx, tx, id, "prune", "", "", "", "expired before "+cutoff, now); err != nil {
				return result, err
			}
		}
	}

	if opts.SessionsOlderThan > 0 {
		cutoff := now.Add(-opts.SessionsOlderThan).Format(time.RFC3339Nano)
		if opts.DryRun {
			err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM session_events WHERE at < ?`, cutoff).Scan(&result.Sessions)
		} else {
			var deleted sql.Result
			deleted, err = tx.ExecContext(ctx, `DELETE FROM session_events WHERE at < ?`, cutoff)
			if err == nil {
				n, _ := deleted.RowsAffected()
				result.Sessions = int(n)
			}
		}
		if err != nil {
			return result, fmt.Errorf("prune sessions: %w", err)
		}
	}

	if opts.DryRun {
		return result, nil
	}
	if err := tx.Commit(); err != nil {
		return result, err
	}
	s.compact(ctx)
	return result, nil
}

// compact merges the full-text segments and truncates the write-ahead log, so a
// prune actually shrinks what is on disk. Best effort: a failure here leaves a
// correct, merely larger, database.
func (s *Store) compact(ctx context.Context) {
	for _, statement := range []string{
		`INSERT INTO memories_fts (memories_fts) VALUES ('optimize')`,
		`INSERT INTO session_events_fts (session_events_fts) VALUES ('optimize')`,
		`PRAGMA optimize`,
		`PRAGMA wal_checkpoint(TRUNCATE)`,
	} {
		_, _ = s.db.ExecContext(ctx, statement)
	}
}

func queryStrings(ctx context.Context, tx *sql.Tx, query string, args ...any) ([]string, error) {
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return nil, err
		}
		out = append(out, value)
	}
	return out, rows.Err()
}
