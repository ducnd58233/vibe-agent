package main

import (
	"flag"
	"fmt"
	"strings"

	"github.com/ducnd58233/vibe-agent/runtime/internal/graphroute"
	"github.com/ducnd58233/vibe-agent/runtime/internal/runstart"
)

// goalFromFlags reads --goal or positional text after flag parsing.
func goalFromFlags(flags *flag.FlagSet, goalFlag *string) (string, error) {
	if trimmed := strings.TrimSpace(*goalFlag); trimmed != "" {
		return trimmed, nil
	}
	rest := flags.Args()
	if len(rest) == 0 {
		return "", fmt.Errorf("need a one-line objective as the command text")
	}
	return strings.TrimSpace(strings.Join(rest, " ")), nil
}

func resolveStart(cmd graphroute.Command, workflow graphroute.Workflow, goal, slug, graphOverride string) (graphroute.Resolved, error) {
	return graphroute.Params{
		Command:       cmd,
		Workflow:      workflow,
		Goal:          goal,
		Slug:          slug,
		GraphOverride: graphOverride,
		SlugWords:     slugWords,
	}.Resolve()
}

// printStartedRun writes the standard lines after runstart.Start succeeds.
// eventsPath is printed when non-empty (run start only).
func printStartedRun(result runstart.Result, slug, eventsPath string) {
	fmt.Printf("started %s\n", result.Run.RunID)
	fmt.Printf("  graph    %s\n", result.Run.GraphID)
	fmt.Printf("  slug     %s\n", slug)
	fmt.Printf("  node     %s\n", result.Run.CurrentNode)
	fmt.Printf("  state    %s\n", result.Manifest)
	if eventsPath != "" {
		fmt.Printf("  events   %s\n", eventsPath)
	}
}
