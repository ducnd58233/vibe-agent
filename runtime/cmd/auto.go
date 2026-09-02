package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/auto"
	"github.com/ducnd58233/vibe-agent/runtime/internal/autoconfig"
	"github.com/ducnd58233/vibe-agent/runtime/internal/checkpoint"
	"github.com/ducnd58233/vibe-agent/runtime/internal/graph"
	"github.com/ducnd58233/vibe-agent/runtime/internal/graphroute"
	"github.com/ducnd58233/vibe-agent/runtime/internal/loop"
	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
	"github.com/ducnd58233/vibe-agent/runtime/internal/runstart"
)

// The auto command surface.
//
//	auto --goal "<text>"   start a run that walks the auto path
//	auto init              write the opt-in a workspace answers before merging
//	auto gate --slug <s>   test the artifact behind the gate the run sits at
//	auto merge --slug <s>  record the merge approval the opt-in file already gave
//
// What this command does not do is the work. The host coding agent still runs
// every agent node, exactly as it does under /goal, and the runtime still holds
// the evidence. Auto mode is a different route through the same graph, not a
// second implementation of it, which is why starting a run and answering a gate
// from a document are the only two things here.

// slugWords bounds a derived slug. Four is enough to tell two goals apart in a
// directory listing and short enough to type.
const slugWords = 4

func autoCommand(args []string) error {
	sub, rest := autoSubcommand(args)
	switch sub {
	case "init":
		return autoInitCommand(rest)
	case "gate":
		return autoGateCommand(rest)
	case "merge":
		return autoMergeCommand(rest)
	case "research":
		return autoStartCommand(rest, graphroute.WorkflowResearch)
	}
	return autoStartCommand(args, graphroute.WorkflowDelivery)
}

// autoSubcommand finds init/gate/merge/research after global flags.
func autoSubcommand(args []string) (string, []string) {
	i := skipCommandFlags(args)
	if i >= len(args) {
		return "", args
	}
	switch args[i] {
	case "init", "gate", "merge", "research":
		rest := append(append([]string{}, args[:i]...), args[i+1:]...)
		return args[i], rest
	default:
		return "", args
	}
}

// skipCommandFlags advances past --flag and --flag=value pairs and their values.
func skipCommandFlags(args []string) int {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			return i + 1
		}
		if !strings.HasPrefix(arg, "-") {
			return i
		}
		if strings.Contains(arg, "=") {
			continue
		}
		if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
			i++
		}
	}
	return len(args)
}

// autoInitCommand writes the opt-in template and says what to do with it.
func autoInitCommand(args []string) error {
	flags := newFlagSet("auto init")
	paths := addRootFlags(flags)
	if err := flags.Parse(args); err != nil {
		return err
	}
	workspaceRoot, _, err := paths.resolve()
	if err != nil {
		return err
	}

	path, err := autoconfig.Write(workspaceRoot)
	if err != nil {
		return err
	}

	fmt.Printf("wrote %s\n", path)
	fmt.Println("  auto mode will not merge until someone sets merge: true in it")
	fmt.Println("  the file is gitignored with the rest of .agent-state, so the answer is per checkout")
	return nil
}

// autoStartCommand turns one objective into a run on the auto path.
//
// The objective may follow the command as plain text. Slug and graph are derived;
// host agents must not ask the user for them.
func autoStartCommand(args []string, workflow graphroute.Workflow) error {
	flags := newFlagSet("auto")
	paths := addRootFlags(flags)
	goal := flags.String("goal", "", "one-line objective (optional when passed as plain text)")
	slug := flags.String("slug", "", "run slug; derived from the objective when omitted")
	graphID := flags.String("graph", "", "workflow graph id (advanced; default from command)")
	source := flags.String("task-source", "", "where the goal text came from, when it came from a task tracker")
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

	// The opt-in is read before anything starts, so a workspace that has not
	// answered finds out now rather than at the merge it was going to refuse.
	config, found, err := autoconfig.Load(workspaceRoot)
	if err != nil {
		return err
	}
	if !found {
		path, writeErr := autoconfig.Write(workspaceRoot)
		if writeErr != nil {
			return writeErr
		}
		return fmt.Errorf("this workspace has not answered the auto opt-in.\n"+
			"  wrote %s\n"+
			"  answer it and run this again; nothing starts until someone has", path)
	}

	resolved, err := resolveStart(graphroute.CmdAuto, workflow, text, *slug, *graphID)
	if err != nil {
		return err
	}

	result, err := runstart.Start(runstart.Options{
		WorkspaceRoot: workspaceRoot,
		ToolkitRoot:   toolkitRoot,
		Resolved:      resolved,
		Auto:          true,
		TaskSource:    *source,
		TokenBudget:   config.Spec.Budgets.Tokens,
		WallclockSec:  config.Spec.Budgets.WallclockSeconds,
	})
	if err != nil {
		return err
	}
	current := result.Run

	fmt.Printf("started %s on the auto path\n", current.RunID)
	fmt.Printf("  slug     %s\n", resolved.Slug)
	fmt.Printf("  graph    %s\n", current.GraphID)
	fmt.Printf("  node     %s\n", current.CurrentNode)
	fmt.Printf("  merge    %s\n", mergeLine(config))
	fmt.Printf("  state    %s\n", result.Manifest)
	switch workflow {
	case graphroute.WorkflowResearch:
		fmt.Println("  auto research walks literature through writeup; call vibe_checkpoint after each artifact")
		fmt.Println("  do not ask the human for next steps until status is done or a gate document leaves something open")
	default:
		fmt.Println("  gates skip when SPEC, PLAN, and TASKS have no open markers; vibe_checkpoint chains past them on the auto path")
	}
	return nil
}

func mergeLine(config *autoconfig.Config) string {
	if config.MayMerge() {
		return "opted in; auto may merge a pull request that is green on every other count"
	}
	return "not opted in; auto stops at a green pull request"
}

// autoGateCommand answers the gate a run sits at, from what the document says.
//
// It sets the flag only when the document says nothing is open. An empty result
// is not a promise the document is complete - it is the most a text search can
// claim - which is why the gate it opens records skipped rather than approved.
func autoGateCommand(args []string) error {
	flags := newFlagSet("auto gate")
	paths := addRootFlags(flags)
	slug := flags.String("slug", "", "goal slug (required)")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *slug == "" {
		return fmt.Errorf("auto gate needs --slug")
	}

	workspaceRoot, toolkitRoot, err := paths.resolve()
	if err != nil {
		return err
	}
	current, err := state.Load(state.ManifestPath(workspaceRoot, *slug))
	if err != nil {
		return err
	}
	if !current.Flags["auto"] {
		return fmt.Errorf("run %q is not on the auto path; a gate outside auto mode is answered by a person", *slug)
	}

	spec, ok := auto.GateSpecFor(current.CurrentNode)
	if !ok {
		return fmt.Errorf("run %q is at node %q, which is not a gate this command answers; it answers %s",
			*slug, current.CurrentNode, strings.Join(auto.GateNodeNames(), " and "))
	}

	findings, _, err := auto.ScanGateDocuments(workspaceRoot, *slug, current.CurrentNode, current.Date)
	if err != nil {
		return fmt.Errorf("read gate documents: %w", err)
	}
	if len(findings) > 0 {
		fmt.Printf("%s stays closed: the document leaves %d thing(s) open\n", current.CurrentNode, len(findings))
		fmt.Println(auto.Report(findings))
		fmt.Println("  a person decides these. Settle them in the document and run this again, or")
		fmt.Printf("  record the approval yourself: checkpoint --check %s --source human_event --passed\n", spec.Check)
		return nil
	}

	loaded, err := graph.LoadByID(graph.DefaultDir(toolkitRoot), current.GraphID)
	if err != nil {
		return err
	}
	answer, err := auto.TryAnswerGate(workspaceRoot, loaded, current)
	if err != nil {
		return err
	}
	if !answer.Answered {
		return fmt.Errorf("the documents look settled but the gate did not open; run `run status --slug %s`", *slug)
	}
	if err := state.Save(state.ManifestPath(workspaceRoot, *slug), current); err != nil {
		return err
	}
	// TryAnswerGate settled the gate on the manifest; Advance walks past it the
	// same way vibe_checkpoint does after an artifact node.
	result, err := checkpoint.Apply(checkpoint.Request{
		WorkspaceRoot: workspaceRoot,
		GraphDir:      graph.DefaultDir(toolkitRoot),
		Slug:          *slug,
		Outcome:       loop.Outcome{},
	})
	if err != nil {
		return err
	}

	fmt.Printf("%s = true\n", spec.Flag)
	fmt.Printf("  %s declares nothing open, so the gate skips rather than waits\n", strings.Join(spec.Files, " and "))
	fmt.Println("  it records skipped, not approved: nobody was asked, and run state keeps the difference")
	if node, ok := loaded.Node(current.CurrentNode); ok {
		fmt.Printf("  gate     %s\n", node.Description)
	}
	if result.Transition != nil {
		fmt.Printf("  node     %s\n", result.Run.CurrentNode)
	}
	return nil
}

// autoMergeCommand records the merge approval a workspace already gave.
//
// This is the one boundary the toolkit otherwise states without exception, so
// it is worth being exact about what it does and does not claim.
//
// It does not record a human_event. Nobody is being asked at this moment, and
// writing one would be the model recording a person's answer on their behalf -
// the thing auto mode is specifically not allowed to do. The evidence is
// file_assert, and the reference is the path of the file someone edited: a
// person did answer this, once, in a diff that can be read.
//
// The check is recorded as passed rather than skipped, and that is deliberate
// too. pre-tool-use refuses `gh pr merge` unless an active run has *passed*
// merge_approved, and teaching that gate to accept a skip would open it to any
// misconfigured skip condition anywhere in the graph. A workspace that has not
// opted in gets a refusal here instead, which is where the refusal belongs.
func autoMergeCommand(args []string) error {
	flags := newFlagSet("auto merge")
	paths := addRootFlags(flags)
	slug := flags.String("slug", "", "goal slug (required)")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *slug == "" {
		return fmt.Errorf("auto merge needs --slug")
	}

	workspaceRoot, toolkitRoot, err := paths.resolve()
	if err != nil {
		return err
	}
	current, err := state.Load(state.ManifestPath(workspaceRoot, *slug))
	if err != nil {
		return err
	}
	if !current.Flags["auto"] {
		return fmt.Errorf("run %q is not on the auto path; a person approves a merge outside auto mode", *slug)
	}
	if current.CurrentNode != "approve_merge" {
		return fmt.Errorf("run %q is at node %q, not approve_merge; the gate precedes the merge and never follows it",
			*slug, current.CurrentNode)
	}

	loaded, err := graph.LoadByID(graph.DefaultDir(toolkitRoot), current.GraphID)
	if err != nil {
		return err
	}
	ref, err := auto.ApproveMerge(workspaceRoot, current, time.Now().UTC())
	if err != nil {
		return err
	}
	if ref == "" {
		return fmt.Errorf("this workspace has not opted into auto-merge, so auto stops at a green pull request.\n"+
			"  set merge: true in %s, or merge it yourself", autoconfig.Path(workspaceRoot))
	}
	transition, err := loop.New(loaded).Advance(current, loop.Outcome{})
	if err != nil {
		return err
	}

	// The journal is the account of how a run got where it is. Every other path
	// that moves a run appends one, and this moved a run past the gate standing
	// in front of an irreversible step: the transition least worth leaving out.
	transitionPayload, err := json.Marshal(map[string]any{
		"from": transition.From, "to": transition.To, "via": transition.Via,
		"key": "auto-merge-" + *slug, "approval": ref,
	})
	if err != nil {
		return fmt.Errorf("encode transition: %w", err)
	}
	if _, err := state.AppendRunEvent(state.EventLogPath(workspaceRoot, *slug),
		state.Event{Type: state.EventTransition, Node: transition.To, At: current.UpdatedAt, Payload: transitionPayload},
	); err != nil {
		return err
	}
	if err := state.Save(state.ManifestPath(workspaceRoot, *slug), current); err != nil {
		return err
	}

	fmt.Printf("%s -> %s\n", transition.From, transition.To)
	fmt.Printf("  approval %s\n", autoconfig.Path(workspaceRoot))
	fmt.Println("  recorded as file_assert, not human_event: nobody was asked now, and someone answered once in a diff")
	return nil
}
