package main

import (
	"fmt"
	"os"

	"github.com/ducnd58233/vibe-agent/runtime/internal/harness"
)

// hookCommand handles one lifecycle hook from a host.
//
// Event names are vendor-neutral here; each client config maps its own event to
// one of these. That keeps the per-tool config a thin adapter rather than a
// place behavior diverges.
func hookCommand(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("hook needs an event: session-start, user-prompt-submit, stop, subagent-stop")
	}
	event := harness.Event(args[0])

	flags := newFlagSet("hook")
	paths := addRootFlags(flags)
	client := flags.String("client", "claude", "host: claude or cursor")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	workspaceRoot, toolkitRoot, err := paths.resolve()
	if err != nil {
		return err
	}

	return harness.Run(harness.Request{
		Event:         event,
		Client:        harness.Client(*client),
		WorkspaceRoot: workspaceRoot,
		ToolkitRoot:   toolkitRoot,
		Stdin:         os.Stdin,
	}, os.Stdout)
}
