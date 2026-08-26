package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/ducnd58233/vibe-agent/runtime/internal/checkpoint"
	"github.com/ducnd58233/vibe-agent/runtime/internal/graph"
)

// verifyCommand produces a verifier node's evidence and checkpoints it.
//
// This is the command that made internal/verifier reachable. Before it, the
// package was written, tested, and never called: every check entered run state
// through `checkpoint --source <string>`, which records what kind of evidence a
// check claims to be without anything having produced it.
//
// There is deliberately no --passed flag. The verdict is the verifier's, and
// what it runs comes from the workspace's check plan, so neither the outcome nor
// the command is the caller's to choose.
func verifyCommand(args []string) error {
	flags := newFlagSet("verify")
	paths := addRootFlags(flags)
	slug := flags.String("slug", "", "goal slug (required)")
	only := flags.String("check", "", "fail unless the current node writes this check")
	dryRun := flags.Bool("dry-run", false, "print what would run without running it")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *slug == "" {
		return fmt.Errorf("verify needs --slug")
	}

	workspaceRoot, toolkitRoot, err := paths.resolve()
	if err != nil {
		return err
	}
	request := checkpoint.VerifyRequest{
		WorkspaceRoot: workspaceRoot,
		GraphDir:      graph.DefaultDir(toolkitRoot),
		Slug:          *slug,
		Check:         *only,
	}

	if *dryRun {
		resolved, err := checkpoint.Resolve(request)
		if err != nil {
			return err
		}
		if resolved.Skipped() {
			fmt.Printf("would skip %s at node %s\n", resolved.Check, resolved.Node)
			fmt.Printf("  reason     %s\n", resolved.SkipReason)
			return nil
		}
		fmt.Printf("would verify %s at node %s\n", resolved.Check, resolved.Node)
		fmt.Printf("  verifier   %s\n", resolved.Kind)
		fmt.Printf("  plan       %s\n", resolved.PlanPath)
		if line := commandLine(resolved); line != "" {
			fmt.Printf("  command    %s\n", line)
		}
		if len(resolved.Entry.Paths) > 0 {
			fmt.Printf("  paths      %s\n", strings.Join(resolved.Entry.Paths, ", "))
		}
		fmt.Printf("  timeout    %s\n", resolved.Timeout)
		return nil
	}

	// Progress goes to stderr so stdout stays the result. A verifier command can
	// run for an hour, and a host with nothing to show would look hung.
	fmt.Fprintf(os.Stderr, "verifying %s\n", *slug)

	result, err := checkpoint.Verify(context.Background(), request)
	if err != nil {
		return err
	}

	if result.Kind == "" {
		// Nothing ran, so naming a verifier would be wrong. The reason is the
		// evidence here, and it has to be visible: a skip that reads like a pass is
		// the failure this whole path exists to prevent.
		fmt.Printf("%s skipped\n", result.Check)
	} else {
		fmt.Printf("%s %s (%s verifier)\n", result.Check, verdict(result.Verifier.Check), result.Kind)
	}
	fmt.Printf("  evidence   %s\n", result.Verifier.Summary)
	if result.Verifier.LogPath != "" {
		fmt.Printf("  log        %s\n", result.Verifier.LogPath)
	}

	applied := result.Applied
	if applied.Duplicate {
		fmt.Printf("%s\n", checkpoint.DuplicateReplayCLI())
	} else {
		via := applied.Transition.Via
		if via == "" {
			via = "(fallback)"
		}
		fmt.Printf("  %s -> %s via %s\n", applied.Transition.From, applied.Transition.To, via)
		if applied.Transition.Skipped {
			// A gate nobody answered is worth a line of its own. Run state
			// records it as skipped rather than passed, and so does this.
			fmt.Printf("  gate       skipped, not approved: the graph declares this gate skippable and the run's flags say so\n")
		}
	}
	fmt.Printf("  status     %s\n", applied.Run.Status)
	if node, ok := applied.Graph.Node(applied.Run.CurrentNode); ok {
		if applied.Duplicate || !applied.Transition.Terminal {
			fmt.Printf("  next       [%s] %s\n", node.Type, node.Description)
		}
	}

	// A failing check is not a failing command. The run recorded the failure and
	// routed on it, which is the loop working; exiting non-zero here would make a
	// host treat a correctly handled failure as a broken tool call.
	return nil
}

func commandLine(resolved *checkpoint.Plan) string {
	if resolved.Entry.Command == "" {
		return ""
	}
	return strings.TrimSpace(resolved.Entry.Command + " " + strings.Join(resolved.Entry.Args, " "))
}
