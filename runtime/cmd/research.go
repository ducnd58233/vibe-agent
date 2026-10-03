package main

import (
	"github.com/ducnd58233/vibe-agent/runtime/internal/graphroute"
	"github.com/ducnd58233/vibe-agent/runtime/internal/runstart"
)

// startNamed starts a run for a command that names its own workflow, so there is
// nothing to read from the objective.
func startNamed(name string, command graphroute.Command, workflow graphroute.Workflow, args []string) error {
	flags := newFlagSet(name)
	paths := addRootFlags(flags)
	goal := flags.String("goal", "", "objective (optional when passed as plain text)")
	slug := flags.String("slug", "", "run slug; derived from the objective when omitted")
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
	resolved, err := resolveStart(command, workflow, text, *slug, "")
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

	printStartedRun(result, resolved.Slug, resolved.Reason, "")
	return nil
}

// researchCommand starts a researcher-delivery run.
func researchCommand(args []string) error {
	return startNamed("research", graphroute.CmdResearch, graphroute.WorkflowResearch, args)
}

// experimentCommand starts the same researcher graph for an experiment
// objective. The graph already holds the experiment loop; the command exists so
// the word a person says is the word that works.
func experimentCommand(args []string) error {
	return startNamed("experiment", graphroute.CmdExperiment, graphroute.WorkflowExperiment, args)
}

// taskCommand starts a task-delivery run: a non-code deliverable with a person
// approving the delivery.
func taskCommand(args []string) error {
	return startNamed("task", graphroute.CmdTask, graphroute.WorkflowTask, args)
}

// tutorCommand starts a study-delivery run for one learner.
func tutorCommand(args []string) error {
	return startNamed("tutor", graphroute.CmdTutor, graphroute.WorkflowTutor, args)
}
