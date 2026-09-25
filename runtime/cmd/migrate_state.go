package main

import (
	"context"
	"fmt"

	fetchpersistence "github.com/ducnd58233/vibe-agent/runtime/internal/fetch/infra/persistence"
	"github.com/ducnd58233/vibe-agent/runtime/internal/harness"
)

// migrateStateCommand moves agent-only, machine-consumed state that used to
// live in files into the shared workspace database. Safe to re-run: a
// location with nothing left to migrate reports zero and does not error.
//
// One backfill call per subsystem as each lands (SPEC
// docs/2026-09-25/migrate-agent-only-machine); this file grows a case as
// later tasks add their own table.
func migrateStateCommand(args []string) error {
	flags := newFlagSet("migrate state")
	paths := addRootFlags(flags)
	if err := flags.Parse(args); err != nil {
		return err
	}
	workspaceRoot, _, err := paths.resolve()
	if err != nil {
		return err
	}
	ctx := context.Background()

	fetchMigrated, err := fetchpersistence.Backfill(ctx, workspaceRoot)
	if err != nil {
		return fmt.Errorf("migrate fetch cache: %w", err)
	}
	fmt.Printf("fetch cache: %d row(s) moved to fetch_cache\n", fetchMigrated)

	sddMigrated, err := harness.SDDCacheBackfill(ctx, workspaceRoot)
	if err != nil {
		return fmt.Errorf("migrate sdd-cache: %w", err)
	}
	fmt.Printf("sdd-cache: %d row(s) moved to sdd_cache\n", sddMigrated)

	return nil
}
