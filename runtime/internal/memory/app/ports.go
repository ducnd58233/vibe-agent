// Package app declares what a caller needs from the memory module.
//
// The interface is here, with the code that depends on it, and satisfied under
// infra. That direction is what lets the SQLite store be swapped, faked in a
// test, or replaced entirely without any caller changing.
package app

import (
	"context"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/memory/domain"
)

// Store is the memory repository: one table's worth of records, and the four
// statuses a record can hold.
//
// Propose returns the policy's decision alongside the stored record, because a
// rejected candidate is an answer rather than an error: the filter did its job.
type Store interface {
	Propose(ctx context.Context, candidate domain.Record, now time.Time) (domain.Record, domain.Decision, error)
	Confirm(ctx context.Context, id string, source domain.SourceType, ref string, now time.Time) (domain.Record, error)
	Invalidate(ctx context.Context, id string, now time.Time) error
	SetStatus(ctx context.Context, id string, status domain.Status, now time.Time) error
	RecordUse(ctx context.Context, id string, now time.Time) error
	Get(ctx context.Context, id string) (domain.Record, error)
	List(ctx context.Context, workspaceID string) ([]domain.Record, error)
	Close() error
}
