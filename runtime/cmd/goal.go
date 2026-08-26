package main

import (
	"github.com/ducnd58233/vibe-agent/runtime/internal/graphroute"
	"github.com/ducnd58233/vibe-agent/runtime/internal/runstart"
)

// goalCommand starts a delivery run with human gates.
//
// Users and slash commands pass the objective as plain text; the host agent
// must not ask for slug or graph.
func goalCommand(args []string) error {
	flags := newFlagSet("goal")
	paths := addRootFlags(flags)
	goal := flags.String("goal", "", "one-line objective (optional when passed as plain text)")
	slug := flags.String("slug", "", "run slug; derived from the objective when omitted")
	graphID := flags.String("graph", "", "workflow graph id (advanced; default from command)")
	if err := flags.Parse(args); err != nil {
		return err
	}
	text, err := goalFromFlags(flags, goal)
	if err != nil {
		return err
	}

	workspaceRoot, toolkitRoot, err := paths.resolve()
	if err != nil {
		return err
	}
	resolved, err := resolveStart(graphroute.CmdGoal, "", text, *slug, *graphID)
	if err != nil {
		return err
	}

	result, err := runstart.Start(runstart.Options{
		WorkspaceRoot: workspaceRoot,
		ToolkitRoot:   toolkitRoot,
		Resolved:      resolved,
	})
	if err != nil {
		return err
	}

	printStartedRun(result, resolved.Slug, "")
	return nil
}
