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
	"strings"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/graph"
	"github.com/ducnd58233/vibe-agent/runtime/internal/harness"
	"github.com/ducnd58233/vibe-agent/runtime/internal/loop"
	"github.com/ducnd58233/vibe-agent/runtime/internal/mcp"
	"github.com/ducnd58233/vibe-agent/runtime/internal/memory"
	"github.com/ducnd58233/vibe-agent/runtime/internal/state"
)

// version is overridden at build time with -ldflags "-X main.version=...".
var version = "dev"

const usage = `vibe-agent - outer-loop control plane

Usage:
  vibe-agent run start --slug <slug> --goal <text> [--graph <id>]
  vibe-agent run status --slug <slug> [--json]
  vibe-agent checkpoint --slug <slug> --check <name> --source <source> [--passed|--failed|--skipped]
  vibe-agent graph validate [--graph <id>]
  vibe-agent mcp serve
  vibe-agent hook <session-start|user-prompt-submit|stop|subagent-stop> [--client claude|cursor]
  vibe-agent doctor
  vibe-agent version

Run state is written to tmp/<slug>/manifest.json with an append-only log at
tmp/<slug>/events.ndjson, both under the workspace root and both gitignored.

Evidence sources: exit_code, file_assert, ci_api, human_event. There is no
source for model assertion, so nothing can mark its own work complete.

Global flags:
  --workspace <dir>   Workspace root (default: current directory)
  --toolkit <dir>     Toolkit root holding .ai-agents (default: workspace root)
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
	case "checkpoint":
		return checkpointCommand(args[1:])
	case "graph":
		return graphCommand(args[1:])
	case "mcp":
		return mcpCommand(args[1:])
	case "hook":
		return hookCommand(args[1:])
	case "doctor":
		return doctorCommand(args[1:])
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

// roots resolves the workspace and toolkit directories. The toolkit defaults to
// the workspace, which is right when the toolkit is used standalone, and is
// overridden when it is mounted as a submodule.
func roots(workspace, toolkit string) (string, string, error) {
	workspaceRoot, err := filepath.Abs(workspace)
	if err != nil {
		return "", "", fmt.Errorf("resolve workspace: %w", err)
	}
	if toolkit == "" {
		return workspaceRoot, workspaceRoot, nil
	}
	toolkitRoot, err := filepath.Abs(toolkit)
	if err != nil {
		return "", "", fmt.Errorf("resolve toolkit: %w", err)
	}
	return workspaceRoot, toolkitRoot, nil
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
	toolkit := flags.String("toolkit", "", "toolkit root holding .ai-agents")
	slug := flags.String("slug", "", "kebab-case slug for this goal (required)")
	goal := flags.String("goal", "", "one-line objective (required)")
	graphID := flags.String("graph", "goal-delivery", "workflow graph id")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *slug == "" || *goal == "" {
		return fmt.Errorf("run start needs --slug and --goal")
	}

	workspaceRoot, toolkitRoot, err := roots(*workspace, *toolkit)
	if err != nil {
		return err
	}

	loaded, err := graph.LoadByID(graph.DefaultDir(toolkitRoot), *graphID)
	if err != nil {
		return err
	}

	manifest := state.ManifestPath(workspaceRoot, *slug)
	if _, err := os.Stat(manifest); err == nil {
		return fmt.Errorf("a run already exists at %s; use `run status` or remove it first", manifest)
	}

	current, err := state.NewRun(*slug, *goal, loaded.Metadata.ID, loaded.Spec.MaxTransitions, time.Now())
	if err != nil {
		return err
	}
	runner := loop.New(loaded)
	if err := runner.Enter(current); err != nil {
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

func runStatus(args []string) error {
	flags := flag.NewFlagSet("run status", flag.ContinueOnError)
	workspace := flags.String("workspace", ".", "workspace root")
	toolkit := flags.String("toolkit", "", "toolkit root holding .ai-agents")
	slug := flags.String("slug", "", "kebab-case slug for this goal (required)")
	asJSON := flags.Bool("json", false, "print the manifest as JSON")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *slug == "" {
		return fmt.Errorf("run status needs --slug")
	}

	workspaceRoot, toolkitRoot, err := roots(*workspace, *toolkit)
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

func checkpointCommand(args []string) error {
	flags := flag.NewFlagSet("checkpoint", flag.ContinueOnError)
	workspace := flags.String("workspace", ".", "workspace root")
	toolkit := flags.String("toolkit", "", "toolkit root holding .ai-agents")
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

	workspaceRoot, toolkitRoot, err := roots(*workspace, *toolkit)
	if err != nil {
		return err
	}

	manifest := state.ManifestPath(workspaceRoot, *slug)
	current, err := state.Load(manifest)
	if err != nil {
		return err
	}
	loaded, err := graph.LoadByID(graph.DefaultDir(toolkitRoot), current.GraphID)
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

	runner := loop.New(loaded)
	from := current.CurrentNode
	transition, err := runner.Advance(current, outcome)
	if err != nil {
		return err
	}

	payload, _ := json.Marshal(map[string]any{"from": from, "to": transition.To, "via": transition.Via})
	if _, err := state.AppendEvent(state.EventLogPath(workspaceRoot, *slug),
		state.Event{Type: "transition", Node: transition.To, Payload: payload, At: time.Now()},
	); err != nil {
		return err
	}
	if err := state.Save(manifest, current); err != nil {
		return err
	}

	via := transition.Via
	if via == "" {
		via = "(fallback)"
	}
	fmt.Printf("%s -> %s via %s\n", transition.From, transition.To, via)
	fmt.Printf("  status     %s\n", current.Status)
	fmt.Printf("  iteration  %d/%d\n", current.Iteration, current.MaxTransitions)
	if node, ok := loaded.Node(current.CurrentNode); ok && !transition.Terminal {
		fmt.Printf("  next       [%s] %s\n", node.Type, node.Description)
	}
	return nil
}

func graphCommand(args []string) error {
	if len(args) == 0 || args[0] != "validate" {
		return fmt.Errorf("graph needs the validate subcommand")
	}
	flags := flag.NewFlagSet("graph validate", flag.ContinueOnError)
	toolkit := flags.String("toolkit", ".", "toolkit root holding .ai-agents")
	graphID := flags.String("graph", "", "validate one graph by id instead of all")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	toolkitRoot, err := filepath.Abs(*toolkit)
	if err != nil {
		return err
	}
	dir := graph.DefaultDir(toolkitRoot)

	if *graphID != "" {
		loaded, err := graph.LoadByID(dir, *graphID)
		if err != nil {
			return err
		}
		fmt.Printf("ok %s: %d nodes, %d edges, %d guards\n",
			loaded.Metadata.ID, len(loaded.Spec.Nodes), len(loaded.Spec.Edges), len(loaded.Spec.Guards))
		return nil
	}

	graphs, err := graph.LoadDir(dir)
	if err != nil {
		return err
	}
	for id, loaded := range graphs {
		fmt.Printf("ok %s: %d nodes, %d edges, %d guards\n",
			id, len(loaded.Spec.Nodes), len(loaded.Spec.Edges), len(loaded.Spec.Guards))
	}
	fmt.Printf("%d graphs validated\n", len(graphs))
	return nil
}

func mcpCommand(args []string) error {
	if len(args) == 0 || args[0] != "serve" {
		return fmt.Errorf("mcp needs the serve subcommand")
	}
	flags := flag.NewFlagSet("mcp serve", flag.ContinueOnError)
	workspace := flags.String("workspace", ".", "workspace root")
	toolkit := flags.String("toolkit", "", "toolkit root holding .ai-agents")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	workspaceRoot, toolkitRoot, err := roots(*workspace, *toolkit)
	if err != nil {
		return err
	}

	store, err := memory.Open(workspaceRoot)
	if err != nil {
		return err
	}
	defer store.Close()

	server := mcp.NewServer(version, mcp.Deps{
		WorkspaceRoot: workspaceRoot,
		ToolkitRoot:   toolkitRoot,
		WorkspaceID:   workspaceRoot,
		Memory:        store,
	})
	return server.Serve(os.Stdin, os.Stdout)
}

func hookCommand(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("hook needs an event: session-start, user-prompt-submit, stop, subagent-stop")
	}
	event := harness.Event(args[0])

	flags := flag.NewFlagSet("hook", flag.ContinueOnError)
	workspace := flags.String("workspace", ".", "workspace root")
	toolkit := flags.String("toolkit", "", "toolkit root holding .ai-agents")
	client := flags.String("client", "claude", "host: claude or cursor")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	workspaceRoot, toolkitRoot, err := roots(*workspace, *toolkit)
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

func doctorCommand(args []string) error {
	flags := flag.NewFlagSet("doctor", flag.ContinueOnError)
	workspace := flags.String("workspace", ".", "workspace root")
	toolkit := flags.String("toolkit", "", "toolkit root holding .ai-agents")
	if err := flags.Parse(args); err != nil {
		return err
	}
	workspaceRoot, toolkitRoot, err := roots(*workspace, *toolkit)
	if err != nil {
		return err
	}

	problems := 0
	report := func(label string, ok bool, detail string) {
		if ok {
			fmt.Printf("  ok    %s\n", label)
			return
		}
		problems++
		fmt.Printf("  FAIL  %s: %s\n", label, detail)
	}

	fmt.Printf("vibe-agent %s\n", version)
	fmt.Printf("  workspace %s\n", workspaceRoot)
	fmt.Printf("  toolkit   %s\n\n", toolkitRoot)

	graphs, err := graph.LoadDir(graph.DefaultDir(toolkitRoot))
	if err != nil {
		report("workflow graphs load and validate", false, err.Error())
	} else {
		report(fmt.Sprintf("workflow graphs load and validate (%d)", len(graphs)), true, "")
	}

	store, err := memory.Open(workspaceRoot)
	if err != nil {
		report("memory database opens", false, err.Error())
	} else {
		report("memory database opens", true, "")
		store.Close()
	}

	entries, err := os.ReadDir(filepath.Join(workspaceRoot, "tmp"))
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			path := state.ManifestPath(workspaceRoot, entry.Name())
			if _, statErr := os.Stat(path); statErr != nil {
				continue
			}
			if _, loadErr := state.Load(path); loadErr != nil {
				report("run state "+entry.Name()+" is valid", false, loadErr.Error())
			} else {
				report("run state "+entry.Name()+" is valid", true, "")
			}
		}
	}

	gitignore, err := os.ReadFile(filepath.Join(workspaceRoot, ".gitignore"))
	if err == nil {
		content := string(gitignore)
		report("tmp/ is gitignored", strings.Contains(content, "/tmp/"),
			"add /tmp/ so run evidence is not committed")
		report(memory.StateDirName+"/ is gitignored", strings.Contains(content, "/"+memory.StateDirName+"/"),
			"add /"+memory.StateDirName+"/ so the memory database is not committed")
	}

	fmt.Println()
	if problems > 0 {
		return fmt.Errorf("doctor found %d problems", problems)
	}
	fmt.Println("doctor: OK")
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
