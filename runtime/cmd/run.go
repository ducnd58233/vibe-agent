package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/graph"
	"github.com/ducnd58233/vibe-agent/runtime/internal/loop"
	"github.com/ducnd58233/vibe-agent/runtime/internal/state"
)

func runCommand(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("run needs a subcommand: start or status")
	}
	switch args[0] {
	case "start":
		return runStart(args[1:])
	case "status":
		return runStatus(args[1:])
	default:
		return fmt.Errorf("unknown run subcommand %q; try start or status", args[0])
	}
}

// runStart creates a run and places it at the graph's initial node.
func runStart(args []string) error {
	flags := newFlagSet("run start")
	paths := addRootFlags(flags)
	slug := flags.String("slug", "", "kebab-case slug for this goal (required)")
	goal := flags.String("goal", "", "one-line objective (required)")
	graphID := flags.String("graph", "goal-delivery", "workflow graph id")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *slug == "" || *goal == "" {
		return fmt.Errorf("run start needs --slug and --goal")
	}

	workspaceRoot, toolkitRoot, err := paths.resolve()
	if err != nil {
		return err
	}

	loaded, err := graph.LoadByID(graph.DefaultDir(toolkitRoot), *graphID)
	if err != nil {
		return err
	}

	// Refuse rather than overwrite: a second start would silently discard the
	// evidence the first one recorded.
	manifest := state.ManifestPath(workspaceRoot, *slug)
	if _, err := os.Stat(manifest); err == nil {
		return fmt.Errorf("a run already exists at %s; use `run status` or remove it first", manifest)
	}

	current, err := state.NewRun(*slug, *goal, loaded.Metadata.ID, loaded.Spec.MaxTransitions, time.Now())
	if err != nil {
		return err
	}
	if err := loop.New(loaded).Enter(current); err != nil {
		return err
	}

	payload, err := json.Marshal(map[string]string{"goal": current.Goal, "graph": current.GraphID})
	if err != nil {
		return fmt.Errorf("encode start event: %w", err)
	}
	if _, err := state.AppendEvent(state.EventLogPath(workspaceRoot, *slug),
		state.Event{Type: "run_started", Node: current.CurrentNode, At: current.CreatedAt, Payload: payload},
	); err != nil {
		return err
	}
	if err := state.Save(manifest, current); err != nil {
		return err
	}

	fmt.Printf("started %s\n", current.RunID)
	fmt.Printf("  graph    %s\n", current.GraphID)
	fmt.Printf("  node     %s\n", current.CurrentNode)
	fmt.Printf("  state    %s\n", manifest)
	fmt.Printf("  events   %s\n", state.EventLogPath(workspaceRoot, *slug))
	return nil
}

// runStatus reports where a run stands and what would complete the current node.
func runStatus(args []string) error {
	flags := newFlagSet("run status")
	paths := addRootFlags(flags)
	slug := flags.String("slug", "", "kebab-case slug for this goal (required)")
	asJSON := flags.Bool("json", false, "print the manifest as JSON")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *slug == "" {
		return fmt.Errorf("run status needs --slug")
	}

	workspaceRoot, toolkitRoot, err := paths.resolve()
	if err != nil {
		return err
	}

	current, err := state.Load(state.ManifestPath(workspaceRoot, *slug))
	if err != nil {
		return err
	}

	if *asJSON {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(current)
	}

	events, err := state.ReadEvents(state.EventLogPath(workspaceRoot, *slug))
	if err != nil {
		return err
	}

	fmt.Printf("%s\n", current.RunID)
	fmt.Printf("  graph      %s\n", current.GraphID)
	fmt.Printf("  status     %s\n", current.Status)
	fmt.Printf("  node       %s\n", orDash(current.CurrentNode))
	fmt.Printf("  iteration  %d/%d\n", current.Iteration, current.MaxTransitions)
	fmt.Printf("  branch     %s\n", orDashPtr(current.Branch))
	fmt.Printf("  events     %d\n", len(events))

	// The graph is a nicety here, not a requirement: status still reports state
	// when the graph is missing, which is what you want while debugging one.
	if loaded, err := graph.LoadByID(graph.DefaultDir(toolkitRoot), current.GraphID); err == nil {
		if node, ok := loaded.Node(current.CurrentNode); ok {
			fmt.Printf("  next       [%s] %s\n", node.Type, node.Description)
			if node.Type == graph.NodeHumanGate {
				fmt.Printf("             ask: %s\n", node.Prompt)
			}
		}
	}

	if len(current.Checks) == 0 {
		fmt.Printf("  checks     none recorded\n")
	} else {
		fmt.Printf("  checks\n")
		for name, check := range current.Checks {
			fmt.Printf("    %-18s %-8s %s\n", name, verdict(check), check.Source)
		}
	}
	for _, blocker := range current.Blockers {
		fmt.Printf("  blocker    %s at %s (attempt %d)\n", blocker.Reason, blocker.Node, blocker.Attempts)
	}
	return nil
}
