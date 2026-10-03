package main

import (
	"context"
	"fmt"

	"github.com/ducnd58233/vibe-agent/runtime/internal/legacy"
)

// migrateStateCommand brings state an older build kept in files into the
// workspace database. Which layouts exist, and how each is read, belongs to the
// legacy module; this only reports. Safe to re-run: a layout with nothing left
// reports zero.
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
	results, err := legacy.Migrate(context.Background(), workspaceRoot)
	for _, result := range results {
		fmt.Printf("%s: %d moved to %s\n", result.Source.Name, result.Moved, result.Source.Into)
	}
	if err != nil {
		return fmt.Errorf("migrate state: %w", err)
	}
	return nil
}
