package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ducnd58233/vibe-agent/runtime/internal/docsrouter"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/workspace"
)

// docsCommand dispatches docs/ subcommands.
func docsCommand(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("docs needs a subcommand: router")
	}
	switch args[0] {
	case "router":
		return docsRouter(args[1:])
	default:
		return fmt.Errorf("unknown docs subcommand %q; try router", args[0])
	}
}

// docsRouter regenerates docs/ROUTER.md from this workspace's run-index and
// docs/ tree. Safe to run any time; a no-op regeneration produces an
// identical file, so it is not a deliverable a person needs to review as a
// diff each time.
func docsRouter(args []string) error {
	flags := newFlagSet("docs router")
	paths := addRootFlags(flags)
	if err := flags.Parse(args); err != nil {
		return err
	}
	workspaceRoot, _, err := paths.resolve()
	if err != nil {
		return err
	}
	content, err := docsrouter.Generate(workspaceRoot)
	if err != nil {
		return err
	}
	path := filepath.Join(workspaceRoot, workspace.DocsDirName, "ROUTER.md")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	fmt.Printf("wrote %s\n", path)
	return nil
}
