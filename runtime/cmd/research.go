package main

import (
	"github.com/ducnd58233/vibe-agent/runtime/internal/graphroute"
	"github.com/ducnd58233/vibe-agent/runtime/internal/runstart"
)

// researchCommand starts a researcher-delivery run.
func researchCommand(args []string) error {
	flags := newFlagSet("research")
	paths := addRootFlags(flags)
	goal := flags.String("goal", "", "research topic (optional when passed as plain text)")
	slug := flags.String("slug", "", "run slug; derived from the topic when omitted")
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
	resolved, err := resolveStart(graphroute.CmdResearch, graphroute.WorkflowResearch, text, *slug, "")
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
