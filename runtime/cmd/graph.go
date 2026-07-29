package main

import (
	"fmt"
	"path/filepath"

	"github.com/ducnd58233/vibe-agent/runtime/internal/graph"
)

// graphCommand validates workflow graphs without needing Python or a JSON
// Schema validator, so the same checks run wherever the binary does.
func graphCommand(args []string) error {
	if len(args) == 0 || args[0] != "validate" {
		return fmt.Errorf("graph needs the validate subcommand")
	}

	flags := newFlagSet("graph validate")
	toolkit := flags.String("toolkit", ".", "toolkit root holding .ai-agents")
	graphID := flags.String("graph", "", "validate one graph by id instead of all")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}

	toolkitRoot, err := filepath.Abs(*toolkit)
	if err != nil {
		return fmt.Errorf("resolve toolkit: %w", err)
	}
	dir := graph.DefaultDir(toolkitRoot)

	if *graphID != "" {
		loaded, err := graph.LoadByID(dir, *graphID)
		if err != nil {
			return err
		}
		printGraphSummary(loaded)
		return nil
	}

	graphs, err := graph.LoadDir(dir)
	if err != nil {
		return err
	}
	for _, loaded := range graphs {
		printGraphSummary(loaded)
	}

	noun := "graph"
	if len(graphs) != 1 {
		noun = "graphs"
	}
	fmt.Printf("%d %s validated\n", len(graphs), noun)
	return nil
}

func printGraphSummary(loaded *graph.Graph) {
	fmt.Printf("ok %s: %d nodes, %d edges, %d guards\n",
		loaded.Metadata.ID, len(loaded.Spec.Nodes), len(loaded.Spec.Edges), len(loaded.Spec.Guards))
}
