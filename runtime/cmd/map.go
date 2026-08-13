package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/ducnd58233/vibe-agent/runtime/internal/repomap"
)

// defaultBudget is how much of a session's context a map may take.
//
// Two thousand tokens is roughly a percent of a large window: enough for a few
// hundred files with their declarations, and small enough that injecting it
// costs less than one avoided read of one medium file.
const defaultBudget = 2000

// mapCommand prints the workspace's structure, reading only what changed.
//
// It answers the question that otherwise costs an agent a directory walk and a
// dozen file reads: what is here, and where. The saving is not subtle. Reading
// this repository's Go sources to learn the same thing is on the order of a
// hundred thousand tokens; the map is two thousand, and the second run of it
// reads nothing from disk at all.
func mapCommand(args []string) error {
	flags := newFlagSet("map")
	paths := addRootFlags(flags)
	budget := flags.Int("budget", defaultBudget, "approximate token budget for the rendered map")
	asJSON := flags.Bool("json", false, "emit the full index as JSON, ignoring the budget")
	refresh := flags.Bool("refresh", false, "discard the cache and re-read every file")
	if err := flags.Parse(args); err != nil {
		return err
	}
	workspaceRoot, _, err := paths.resolve()
	if err != nil {
		return err
	}

	if *refresh {
		if err := os.Remove(repomap.CachePath(workspaceRoot)); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("discard cache: %w", err)
		}
	}

	result, err := repomap.Build(context.Background(), workspaceRoot)
	if err != nil {
		return err
	}

	if *asJSON {
		// Sorted, not ranked: a consumer that wants the ranking has Inbound on
		// every entry, and a tree is what a diff of two runs should show.
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(repomap.Sorted(result))
	}

	fmt.Print(repomap.Render(result, *budget))
	return nil
}
