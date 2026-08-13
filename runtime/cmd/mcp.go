package main

import (
	"context"
	"fmt"
	"os"

	"github.com/ducnd58233/vibe-agent/runtime/internal/mcp"
	"github.com/ducnd58233/vibe-agent/runtime/internal/memory"
)

// mcpCommand serves the control plane over stdio.
//
// This is the fallback surface, for Codex and opencode, which have no hook
// system. An MCP tool call is model-decided, so it is best effort by nature;
// Claude and Cursor get the same capabilities through hooks, which always fire.
func mcpCommand(args []string) error {
	if len(args) == 0 || args[0] != "serve" {
		return fmt.Errorf("mcp needs the serve subcommand")
	}

	flags := newFlagSet("mcp serve")
	paths := addRootFlags(flags)
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	workspaceRoot, toolkitRoot, err := paths.resolve()
	if err != nil {
		return err
	}

	store, err := memory.Open(context.Background(), workspaceRoot)
	if err != nil {
		return err
	}
	defer func() { _ = store.Close() }()

	server := mcp.NewServer(version, mcp.Deps{
		WorkspaceRoot: workspaceRoot,
		ToolkitRoot:   toolkitRoot,
		// The workspace path is the workspace identity: memories never cross
		// repositories, and two checkouts of the same repo are two workspaces.
		WorkspaceID: workspaceRoot,
		Memory:      store,
	})
	return server.Serve(os.Stdin, os.Stdout)
}
