package persistence

import (
	"context"
	"fmt"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/memory/domain"
)

// RecordExposures notes that memories were put in front of the model while runs
// were in flight. Nothing is credited here: exposure is half of the evidence,
// and CreditExposures supplies the other half.
//
// A memory already exposed and not yet credited in a run is left as it is, so
// repeating it every prompt costs nothing and earns nothing extra.
func (s *Store) RecordExposures(ctx context.Context, memoryIDs, runIDs []string, now time.Time) error {
	if len(memoryIDs) == 0 || len(runIDs) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	at := now.UTC().Format(time.RFC3339Nano)
	for _, runID := range runIDs {
		for _, memoryID := range memoryIDs {
			if _, err := tx.ExecContext(ctx, `
                INSERT OR IGNORE INTO memory_exposures (memory_id, run_id, exposed_at)
                VALUES (?, ?, ?)`, memoryID, runID, at); err != nil {
				return fmt.Errorf("record exposure: %w", err)
			}
		}
	}
	return tx.Commit()
}

// CreditExposures turns a run's open exposures into reuse, because the run just
// recorded verified success. Each memory exposed in the run before at earns one
// use and a ledger line naming the evidence; every one of those exposures is
// then closed, so the same exposure cannot be credited twice.
//
// Only a memory still held (confirmed, interval open) earns the use. One that
// was closed or contradicted after it was shown is not rewarded for a success
// it may have been wrong about, but its exposure is still consumed.
//
// It returns the ids that were credited.
func (s *Store) CreditExposures(ctx context.Context, runID, evidence string, at time.Time) ([]string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	stamp := at.UTC().Format(time.RFC3339Nano)
	rows, err := tx.QueryContext(ctx, `
        SELECT DISTINCT e.memory_id
        FROM memory_exposures e JOIN memories m ON m.id = e.memory_id
        WHERE e.run_id = ? AND e.credited_at IS NULL AND e.exposed_at <= ?
          AND m.status = ? AND m.valid_to IS NULL
        ORDER BY e.memory_id`, runID, stamp, string(domain.StatusConfirmed))
	if err != nil {
		return nil, fmt.Errorf("read exposures: %w", err)
	}
	var credited []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return nil, err
		}
		credited = append(credited, id)
	}
	_ = rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for _, id := range credited {
		if _, err := tx.ExecContext(ctx, `UPDATE memories SET used_count = used_count + 1 WHERE id = ?`, id); err != nil {
			return nil, fmt.Errorf("credit memory: %w", err)
		}
		if err := logEvent(ctx, tx, id, "use", "", "", "verifier", evidence, at); err != nil {
			return nil, err
		}
	}
	if _, err := tx.ExecContext(ctx, `
        UPDATE memory_exposures SET credited_at = ?
        WHERE run_id = ? AND credited_at IS NULL AND exposed_at <= ?`, stamp, runID, stamp); err != nil {
		return nil, fmt.Errorf("close exposures: %w", err)
	}
	return credited, tx.Commit()
}
