package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/auto"
	"github.com/ducnd58233/vibe-agent/runtime/internal/autoconfig"
	"github.com/ducnd58233/vibe-agent/runtime/internal/graph"
	"github.com/ducnd58233/vibe-agent/runtime/internal/loop"
	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/validate"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/workspace"
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
	if len(args) > 0 {
		switch args[0] {
		case "init":
			return autoInitCommand(args[1:])
		case "gate":
			return autoGateCommand(args[1:])
		case "merge":
			return autoMergeCommand(args[1:])
		}
	}
	return autoStartCommand(args)
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
// The auto flag is set before the run enters the graph rather than through
// `run flag`, because the first node is a gate that flag decides. Setting it
// afterwards would mean the run parked waiting for a person and then had the
// reason it was waiting taken away, which is a different thing from never
// having waited.
func autoStartCommand(args []string) error {
	flags := newFlagSet("auto")
	paths := addRootFlags(flags)
	goal := flags.String("goal", "", "one-line objective (required)")
	slug := flags.String("slug", "", "run slug; derived from the goal when omitted")
	graphID := flags.String("graph", "goal-delivery", "workflow graph id")
	source := flags.String("task-source", "", "where the goal text came from, when it came from a task tracker")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *goal == "" {
		return fmt.Errorf("auto needs --goal; one prompt is the whole input")
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

	name := *slug
	if name == "" {
		name = auto.Slugify(*goal, slugWords)
	}
	if !validate.Slug(name) {
		return fmt.Errorf("%q is not a usable slug; pass --slug", name)
	}

	loaded, err := graph.LoadByID(graph.DefaultDir(toolkitRoot), *graphID)
	if err != nil {
		return err
	}

	manifest := state.ManifestPath(workspaceRoot, name)
	if _, statErr := os.Stat(manifest); statErr == nil {
		return fmt.Errorf("a run already exists at %s; use `run status` or pass a different --slug", manifest)
	}

	// Text from a task tracker is a description of work someone filed, not an
	// instruction addressed to this process. It is wrapped where it enters, so
	// everything downstream reads it already marked.
	recorded := *goal
	if *source != "" {
		recorded = auto.Task(*source, *goal)
	}

	now := time.Now()
	current, err := state.NewRun(name, recorded, loaded.Metadata.ID, loaded.Spec.MaxTransitions, now)
	if err != nil {
		return err
	}
	current.TokenBudget = config.Spec.Budgets.Tokens
	current.WallclockSeconds = config.Spec.Budgets.WallclockSeconds
	if err := current.SetFlagAt("auto", true, now); err != nil {
		return err
	}
	if err := loop.New(loaded).Enter(current); err != nil {
		return err
	}

	payload, err := json.Marshal(map[string]any{
		"goal": current.Goal, "graph": current.GraphID, "auto": true, "taskSource": *source,
	})
	if err != nil {
		return fmt.Errorf("encode start event: %w", err)
	}
	if _, err := state.AppendEvent(state.EventLogPath(workspaceRoot, name),
		state.Event{Type: "run_started", Node: current.CurrentNode, At: current.CreatedAt, Payload: payload},
	); err != nil {
		return err
	}
	if err := state.Save(manifest, current); err != nil {
		return err
	}

	fmt.Printf("started %s on the auto path\n", current.RunID)
	fmt.Printf("  slug     %s\n", name)
	fmt.Printf("  node     %s\n", current.CurrentNode)
	fmt.Printf("  merge    %s\n", mergeLine(config))
	fmt.Printf("  state    %s\n", manifest)
	fmt.Println("  the spec and plan gates still hold until `auto gate` finds nothing open in the document")
	return nil
}

func mergeLine(config *autoconfig.Config) string {
	if config.MayMerge() {
		return "opted in; auto may merge a pull request that is green on every other count"
	}
	return "not opted in; auto stops at a green pull request"
}

// gate names the document a skippable gate is a judgement about, the flag that
// says the judgement came out clean, and the check a person writes instead.
//
// A table rather than a switch because the pairing is data: adding a gate means
// adding a row, and a row with no document is a gate nothing can answer.
type gate struct {
	flag  string
	check string
	files []string
}

var gateArtifact = map[string]gate{
	"approve_spec": {flag: "spec_unambiguous", check: "spec_approved", files: []string{"SPEC.md"}},
	"approve_plan": {flag: "plan_unambiguous", check: "plan_approved", files: []string{"PLAN.md", "TASKS.md"}},
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

	answerable, ok := gateArtifact[current.CurrentNode]
	if !ok {
		return fmt.Errorf("run %q is at node %q, which is not a gate this command answers; it answers %s",
			*slug, current.CurrentNode, strings.Join(gateNames(), " and "))
	}

	var findings []auto.Ambiguity
	for _, file := range answerable.files {
		path := filepath.Join(workspace.DocsDir(workspaceRoot, *slug), file)
		document, readErr := os.ReadFile(filepath.Clean(path))
		if readErr != nil {
			return fmt.Errorf("read %s: %w", path, readErr)
		}
		findings = append(findings, auto.Scan(string(document))...)
	}

	if len(findings) > 0 {
		fmt.Printf("%s stays closed: the document leaves %d thing(s) open\n", current.CurrentNode, len(findings))
		fmt.Println(auto.Report(findings))
		fmt.Println("  a person decides these. Settle them in the document and run this again, or")
		fmt.Printf("  record the approval yourself: checkpoint --check %s --source human_event --passed\n", answerable.check)
		return nil
	}

	loaded, err := graph.LoadByID(graph.DefaultDir(toolkitRoot), current.GraphID)
	if err != nil {
		return err
	}
	now := time.Now()
	if err := current.SetFlagAt(answerable.flag, true, now); err != nil {
		return err
	}
	payload, err := json.Marshal(map[string]any{
		"flag": answerable.flag, "value": true,
		"note": "auto gate found nothing open in " + strings.Join(answerable.files, ", "),
	})
	if err != nil {
		return fmt.Errorf("encode flag event: %w", err)
	}
	if _, err := state.AppendEvent(state.EventLogPath(workspaceRoot, *slug),
		state.Event{Type: "flag_set", Node: current.CurrentNode, At: current.UpdatedAt, Payload: payload},
	); err != nil {
		return err
	}
	// The flag arrived while the run was already parked at the gate, so the
	// gate has to be asked again. enterGate answers on arrival, and arrival was
	// the only moment that existed before a document could answer one.
	skipped, err := loop.New(loaded).SettleGate(current)
	if err != nil {
		return err
	}
	if err := state.Save(state.ManifestPath(workspaceRoot, *slug), current); err != nil {
		return err
	}
	if !skipped {
		return fmt.Errorf("the flag is set but the gate did not open; run `run status --slug %s`", *slug)
	}

	fmt.Printf("%s = true\n", answerable.flag)
	fmt.Printf("  %s declares nothing open, so the gate skips rather than waits\n", strings.Join(answerable.files, " and "))
	fmt.Println("  it records skipped, not approved: nobody was asked, and run state keeps the difference")
	if node, ok := loaded.Node(current.CurrentNode); ok {
		fmt.Printf("  gate     %s\n", node.Description)
	}
	return nil
}

func gateNames() []string {
	names := make([]string, 0, len(gateArtifact))
	for id := range gateArtifact {
		names = append(names, id)
	}
	sort.Strings(names)
	return names
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

	config, found, err := autoconfig.Load(workspaceRoot)
	if err != nil {
		return err
	}
	if !found || !config.MayMerge() {
		return fmt.Errorf("this workspace has not opted into auto-merge, so auto stops at a green pull request.\n"+
			"  set merge: true in %s, or merge it yourself", autoconfig.Path(workspaceRoot))
	}

	loaded, err := graph.LoadByID(graph.DefaultDir(toolkitRoot), current.GraphID)
	if err != nil {
		return err
	}
	now := time.Now()
	if err := current.SetCheckAt("merge_approved", state.Check{
		Passed: true,
		Source: state.SourceFileAssert,
		Ref:    approvalRef(workspaceRoot, config),
		At:     now,
	}, now); err != nil {
		return err
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
		"key": "auto-merge-" + *slug, "approval": approvalRef(workspaceRoot, config),
	})
	if err != nil {
		return fmt.Errorf("encode transition: %w", err)
	}
	if _, err := state.AppendEvent(state.EventLogPath(workspaceRoot, *slug),
		state.Event{Type: "transition", Node: transition.To, At: current.UpdatedAt, Payload: transitionPayload},
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

// approvalRef is what the merge approval records as its evidence.
//
// The path alone said where the answer lived, not what it was. The opt-in file
// is gitignored, so there is no diff to fall back on: a manifest that names a
// path reads as "a person answered yes" while the file can say something else
// by the time anyone opens it. A run was aborted on exactly that uncertainty,
// and the answer turned out to be legitimate - which is the point, because
// nothing in the record could say so either way.
//
// So the reference carries the answer and a fingerprint of the bytes it was
// read from, on one line somebody can read.
func approvalRef(workspaceRoot string, config *autoconfig.Config) string {
	return fmt.Sprintf("%s merge=%t sha256=%s",
		autoconfig.Path(workspaceRoot), config.MayMerge(), config.Digest())
}
