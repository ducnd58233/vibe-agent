package main

import (
	"fmt"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/migrate"
)

func migrateCommand(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: vibe-agent migrate docs-tmp|state [--dry-run] [--workspace <dir>]")
	}
	switch args[0] {
	case "docs-tmp":
		return migrateDocsTmp(args[1:])
	case "state":
		return migrateStateCommand(args[1:])
	default:
		return fmt.Errorf("usage: vibe-agent migrate docs-tmp|state [--dry-run] [--workspace <dir>]")
	}
}

func migrateDocsTmp(args []string) error {
	flags := newFlagSet("migrate docs-tmp")
	paths := addRootFlags(flags)
	dryRun := flags.Bool("dry-run", false, "list planned moves without writing")
	if err := flags.Parse(args); err != nil {
		return err
	}
	workspaceRoot, _, err := paths.resolve()
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	plans, err := migrate.PlanWorkspace(workspaceRoot, now)
	if err != nil {
		return err
	}
	fmt.Print(migrate.FormatPlan(plans))
	if err := migrate.Apply(workspaceRoot, plans, migrate.Options{DryRun: *dryRun, Now: now}); err != nil {
		return err
	}
	if *dryRun {
		fmt.Println("dry-run: no files written")
	} else {
		fmt.Println("migrate: done")
	}
	return nil
}
