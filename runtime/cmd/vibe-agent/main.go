// Command vibe-agent is the outer-loop control plane for the vibe-agent
// toolkit. It owns run state, graph transitions, and verification evidence.
//
// It does not own the inner loop. Claude Code, Codex, Cursor, and opencode keep
// their own model and tool loops; this binary decides what happens between
// their turns.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/state"
)

// version is overridden at build time with -ldflags "-X main.version=...".
var version = "dev"

const usage = `vibe-agent - outer-loop control plane

Usage:
  vibe-agent run start --slug <slug> --goal <text> [--graph <id>] [--max-transitions <n>]
  vibe-agent run status --slug <slug> [--json]
  vibe-agent version

Run state is written to tmp/<slug>/manifest.json with an append-only log at
tmp/<slug>/events.ndjson, both under the workspace root and both gitignored.

Global flags:
  --workspace <dir>   Workspace root (default: current directory)
`

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "vibe-agent: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		fmt.Print(usage)
		return nil
	}

	switch args[0] {
	case "run":
		return runCommand(args[1:])
	case "version":
		fmt.Println(version)
		return nil
	case "help", "-h", "--help":
		fmt.Print(usage)
		return nil
	default:
		return fmt.Errorf("unknown command %q; try `vibe-agent help`", args[0])
	}
}

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

func runStart(args []string) error {
	flags := flag.NewFlagSet("run start", flag.ContinueOnError)
	workspace := flags.String("workspace", ".", "workspace root")
	slug := flags.String("slug", "", "kebab-case slug for this goal (required)")
	goal := flags.String("goal", "", "one-line objective (required)")
	graph := flags.String("graph", "goal-delivery", "workflow graph id")
	maxTransitions := flags.Int("max-transitions", 50, "transition budget")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *slug == "" || *goal == "" {
		return fmt.Errorf("run start needs --slug and --goal")
	}

	root, err := filepath.Abs(*workspace)
	if err != nil {
		return fmt.Errorf("resolve workspace: %w", err)
	}

	manifest := state.ManifestPath(root, *slug)
	if _, err := os.Stat(manifest); err == nil {
		return fmt.Errorf("a run already exists at %s; use `run status` or remove it first", manifest)
	}

	current, err := state.NewRun(*slug, *goal, *graph, *maxTransitions, time.Now())
	if err != nil {
		return err
	}

	payload, err := json.Marshal(map[string]string{
		"goal":  current.Goal,
		"graph": current.GraphID,
	})
	if err != nil {
		return fmt.Errorf("encode start event: %w", err)
	}
	if _, err := state.AppendEvent(
		state.EventLogPath(root, *slug),
		state.Event{Type: "run_started", At: current.CreatedAt, Payload: payload},
	); err != nil {
		return err
	}

	if err := state.Save(manifest, current); err != nil {
		return err
	}

	fmt.Printf("started %s\n", current.RunID)
	fmt.Printf("  state    %s\n", manifest)
	fmt.Printf("  events   %s\n", state.EventLogPath(root, *slug))
	return nil
}

func runStatus(args []string) error {
	flags := flag.NewFlagSet("run status", flag.ContinueOnError)
	workspace := flags.String("workspace", ".", "workspace root")
	slug := flags.String("slug", "", "kebab-case slug for this goal (required)")
	asJSON := flags.Bool("json", false, "print the manifest as JSON")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *slug == "" {
		return fmt.Errorf("run status needs --slug")
	}

	root, err := filepath.Abs(*workspace)
	if err != nil {
		return fmt.Errorf("resolve workspace: %w", err)
	}

	current, err := state.Load(state.ManifestPath(root, *slug))
	if err != nil {
		return err
	}

	if *asJSON {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(current)
	}

	events, err := state.ReadEvents(state.EventLogPath(root, *slug))
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

func verdict(check state.Check) string {
	switch {
	case check.Skipped:
		return "skipped"
	case check.Passed:
		return "pass"
	default:
		return "fail"
	}
}

func orDash(value string) string {
	if value == "" {
		return "-"
	}
	return value
}

func orDashPtr(value *string) string {
	if value == nil {
		return "-"
	}
	return orDash(*value)
}
