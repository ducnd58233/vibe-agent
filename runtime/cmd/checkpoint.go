package main

import (
	"fmt"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/checkpoint"
	"github.com/ducnd58233/vibe-agent/runtime/internal/graph"
	"github.com/ducnd58233/vibe-agent/runtime/internal/loop"
	"github.com/ducnd58233/vibe-agent/runtime/internal/state"
)

// checkpointCommand records evidence for the current node and advances the graph.
//
// This is where the provenance rule is enforced at the process boundary.
// --source is required with a check, and its allowed values are exactly the
// four that come from outside a model.
func checkpointCommand(args []string) error {
	flags := newFlagSet("checkpoint")
	paths := addRootFlags(flags)
	slug := flags.String("slug", "", "goal slug (required)")
	checkName := flags.String("check", "", "check key this node writes")
	source := flags.String("source", "", "evidence source: exit_code, file_assert, ci_api, human_event")
	ref := flags.String("ref", "", "pointer to the evidence, for example a log path")
	passed := flags.Bool("passed", false, "record a pass")
	failed := flags.Bool("failed", false, "record a failure")
	skipped := flags.Bool("skipped", false, "record that the check did not run")
	blocker := flags.String("blocker", "", "record a blocker at this node")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *slug == "" {
		return fmt.Errorf("checkpoint needs --slug")
	}
	if *passed && *failed {
		return fmt.Errorf("choose one of --passed or --failed")
	}

	workspaceRoot, toolkitRoot, err := paths.resolve()
	if err != nil {
		return err
	}

	outcome := loop.Outcome{Blocker: *blocker}
	if *checkName != "" {
		if *source == "" {
			return fmt.Errorf("a check needs --source; evidence without provenance is not evidence")
		}
		outcome.Check = &loop.NamedCheck{
			Name: *checkName,
			Check: state.Check{
				Passed:  *passed,
				Skipped: *skipped,
				Source:  state.CheckSource(*source),
				Ref:     *ref,
				At:      time.Now(),
			},
		}
	}

	result, err := checkpoint.Apply(checkpoint.Request{
		WorkspaceRoot: workspaceRoot,
		GraphDir:      graph.DefaultDir(toolkitRoot),
		Slug:          *slug,
		Outcome:       outcome,
	})
	if err != nil {
		return err
	}

	current := result.Run
	if result.Duplicate {
		// Said plainly rather than silently: a caller retrying a checkpoint is
		// usually a caller that thinks the first one failed.
		fmt.Printf("Already recorded. This exact evidence was the last checkpoint, so nothing advanced.\n")
	} else {
		via := result.Transition.Via
		if via == "" {
			via = "(fallback)"
		}
		fmt.Printf("%s -> %s via %s\n", result.Transition.From, result.Transition.To, via)
	}
	fmt.Printf("  status     %s\n", current.Status)
	fmt.Printf("  iteration  %d/%d\n", current.Iteration, current.MaxTransitions)
	if node, ok := result.Graph.Node(current.CurrentNode); ok {
		if result.Duplicate || !result.Transition.Terminal {
			fmt.Printf("  next       [%s] %s\n", node.Type, node.Description)
		}
	}
	return nil
}
