package main

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/graph"
	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
)

// The verbs that end a run or restart one.
//
// Without them a wedged run had no supported exit. The only route was editing
// manifest.json by hand, which pre-tool-use correctly refuses, so the refusal
// and the absence of a verb left a run with nowhere to go. Two runs in this
// workspace sat that way for days.
//
// Both writing verbs demand a reason. A run that stopped and a run someone
// stopped look identical in the manifest a week later, and the difference is
// the whole reason anyone reads the log.

// resumableStatus reports whether a run has stopped in a way a person can
// deliberately undo. A finished run is not one of them: restarting it would
// reopen work its own evidence says is complete.
func resumableStatus(status state.Status) bool {
	switch status {
	case state.StatusFailed, state.StatusBudgetExceeded, state.StatusCancelled:
		return true
	default:
		return false
	}
}

// runList reports every run under the workspace.
//
// It replaces reading doctor output to find out what is in flight, which meant
// the only list of runs was a side effect of a health check.
func runList(args []string) error {
	flags := newFlagSet("run list")
	paths := addRootFlags(flags)
	status := flags.String("status", "", "only runs with this status")
	if err := flags.Parse(args); err != nil {
		return err
	}

	workspaceRoot, _, err := paths.resolve()
	if err != nil {
		return err
	}

	slugs, err := state.List(workspaceRoot)
	if err != nil {
		return err
	}

	now := time.Now()
	shown := 0
	for _, slug := range slugs {
		current, err := state.Load(state.ManifestPath(workspaceRoot, slug))
		if err != nil {
			continue
		}
		if *status != "" && string(current.Status) != *status {
			continue
		}
		if shown == 0 {
			fmt.Printf("%-34s %-16s %-18s %-9s %s\n", "SLUG", "STATUS", "NODE", "ITERATION", "IDLE")
		}
		shown++
		fmt.Printf("%-34s %-16s %-18s %-9s %s\n",
			slug,
			current.Status,
			orDash(current.CurrentNode),
			fmt.Sprintf("%d/%d", current.Iteration, current.MaxTransitions),
			idleFor(now, current.UpdatedAt),
		)
	}
	if shown == 0 {
		if *status != "" {
			fmt.Printf("no runs with status %q\n", *status)
			return nil
		}
		fmt.Println("no runs in this workspace")
		return nil
	}
	fmt.Printf("\n%d run(s)\n", shown)
	return nil
}

// idleFor renders how long a run has sat untouched, in the coarsest unit that
// still says something. A run idle for eleven days is the fact; the minutes are
// noise.
func idleFor(now, updated time.Time) string {
	if updated.IsZero() {
		return "-"
	}
	elapsed := now.Sub(updated)
	switch {
	case elapsed < time.Minute:
		return "just now"
	case elapsed < time.Hour:
		return fmt.Sprintf("%dm", int(elapsed.Minutes()))
	case elapsed < 24*time.Hour:
		return fmt.Sprintf("%dh", int(elapsed.Hours()))
	default:
		return fmt.Sprintf("%dd", int(elapsed.Hours()/24))
	}
}

// runResume re-enters a run that stopped.
//
// The budget is raised by an explicit increment rather than being cleared. A
// run that exceeded its transitions did so for a reason, and handing it an
// unlimited one turns the stop rule off permanently instead of extending it
// once.
func runResume(args []string) error {
	flags := newFlagSet("run resume")
	paths := addRootFlags(flags)
	slug := flags.String("slug", "", "goal slug (required)")
	reason := flags.String("reason", "", "why this run should continue (required)")
	budget := flags.Int("budget", 0, "raise the budget that stopped the run by this much: transitions, tokens, or seconds")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *slug == "" {
		return fmt.Errorf("run resume needs --slug")
	}
	if *reason == "" {
		return fmt.Errorf("run resume needs --reason; a run restarted without one is indistinguishable from a run that never stopped")
	}
	if *budget < 0 {
		return fmt.Errorf("--budget raises the transition limit, so it cannot be negative")
	}

	workspaceRoot, toolkitRoot, err := paths.resolve()
	if err != nil {
		return err
	}
	manifest := state.ManifestPath(workspaceRoot, *slug)
	current, err := state.Load(manifest)
	if err != nil {
		return err
	}

	if !resumableStatus(current.Status) {
		return fmt.Errorf("run is %s, which is not a stop a resume can undo; resumable states are failed, budget_exceeded, and cancelled",
			current.Status)
	}
	if current.Status == state.StatusBudgetExceeded && *budget <= 0 {
		return fmt.Errorf("this run stopped on its transition budget, so resuming it needs --budget to say by how much")
	}

	previous := current.Status
	raised, unit := raiseBudget(current, *budget)

	// The blocker count is what ended the run. Leaving it in place would end
	// the resumed run on its first failure rather than after three.
	current.Blockers = nil

	current.Status = state.StatusRunning
	if loaded, loadErr := graph.LoadByID(graph.DefaultDir(toolkitRoot), current.GraphID); loadErr == nil {
		if node, ok := loaded.Node(current.CurrentNode); ok && node.Type == graph.NodeHumanGate {
			current.Status = state.StatusAwaitingHuman
		}
	}
	current.UpdatedAt = time.Now().UTC()

	payload, err := json.Marshal(map[string]any{
		"reason":         *reason,
		"from":           string(previous),
		"budgetRaisedBy": raised,
		"budgetUnit":     unit,
	})
	if err != nil {
		return fmt.Errorf("encode resume event: %w", err)
	}
	if _, err := state.AppendEvent(state.EventLogPath(workspaceRoot, *slug),
		state.Event{Type: "run_resumed", Node: current.CurrentNode, At: current.UpdatedAt, Payload: payload},
	); err != nil {
		return err
	}
	if err := state.Save(manifest, current); err != nil {
		return err
	}

	fmt.Printf("resumed %s\n", current.RunID)
	fmt.Printf("  was        %s\n", previous)
	fmt.Printf("  status     %s\n", current.Status)
	fmt.Printf("  node       %s\n", orDash(current.CurrentNode))
	fmt.Printf("  iteration  %d/%d\n", current.Iteration, current.MaxTransitions)
	if raised > 0 {
		fmt.Printf("  raised     %s by %d\n", unit, raised)
	}
	return nil
}

// runAbort closes a run that should not continue.
func runAbort(args []string) error {
	flags := newFlagSet("run abort")
	paths := addRootFlags(flags)
	slug := flags.String("slug", "", "goal slug (required)")
	reason := flags.String("reason", "", "why this run is being closed (required)")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *slug == "" {
		return fmt.Errorf("run abort needs --slug")
	}
	if *reason == "" {
		return fmt.Errorf("run abort needs --reason; the reason is the only thing that separates an abandoned run from a decided one")
	}

	workspaceRoot, _, err := paths.resolve()
	if err != nil {
		return err
	}
	manifest := state.ManifestPath(workspaceRoot, *slug)
	current, err := state.Load(manifest)
	if err != nil {
		return err
	}

	if current.Status == state.StatusDone {
		return fmt.Errorf("run is done; aborting it would overwrite a completion its own evidence records")
	}
	if current.Status == state.StatusCancelled {
		return fmt.Errorf("run is already cancelled")
	}

	previous := current.Status
	current.Status = state.StatusCancelled
	current.UpdatedAt = time.Now().UTC()

	payload, err := json.Marshal(map[string]any{"reason": *reason, "from": string(previous)})
	if err != nil {
		return fmt.Errorf("encode abort event: %w", err)
	}
	if _, err := state.AppendEvent(state.EventLogPath(workspaceRoot, *slug),
		state.Event{Type: "run_aborted", Node: current.CurrentNode, At: current.UpdatedAt, Payload: payload},
	); err != nil {
		return err
	}
	if err := state.Save(manifest, current); err != nil {
		return err
	}

	fmt.Printf("aborted %s\n", current.RunID)
	fmt.Printf("  was        %s\n", previous)
	fmt.Printf("  status     %s\n", current.Status)
	fmt.Printf("  node       %s\n", orDash(current.CurrentNode))
	fmt.Printf("  reason     %s\n", *reason)
	return nil
}

// raiseBudget extends whichever limit stopped the run, and reports what it did.
//
// Raising the transition count on a run that ran out of wallclock would leave it
// stopping again on the next advance, having reported a resume that changed
// nothing. StoppedBy is what makes the right budget knowable.
func raiseBudget(run *state.Run, by int) (int, string) {
	if by <= 0 {
		return 0, ""
	}
	switch run.StoppedBy {
	case "tokens":
		run.TokenBudget += by
		return by, "token budget"
	case "wallclock":
		run.WallclockSeconds += by
		return by, "wallclock seconds"
	default:
		run.MaxTransitions += by
		return by, "transitions"
	}
}

// runExtend raises a healthy run's transition ceiling.
//
// A separate verb from resume, and the separation is the point. Resume undoes a
// stop: it clears the blocker count, restores a status, and refuses a run that
// has not stopped. A run at 99 of 100 with eight tasks left has not stopped, and
// the only supported way to give it room was to let it break first and then undo
// the break. That is a worse record of what happened as well as a worse
// experience: the manifest ends up saying a run failed and was resumed, when it
// was neither.
//
// Transitions only. A run heading for a token or wallclock ceiling is a
// different need, and nothing has shown that need yet; adding the knob now would
// be the shape this audit exists to find.
func runExtend(args []string) error {
	flags := newFlagSet("run extend")
	paths := addRootFlags(flags)
	slug := flags.String("slug", "", "goal slug (required)")
	reason := flags.String("reason", "", "why this run needs more room (required)")
	budget := flags.Int("budget", 0, "raise the transition ceiling by this many (required)")
	if err := flags.Parse(args); err != nil {
		return err
	}
	switch {
	case *slug == "":
		return fmt.Errorf("run extend needs --slug")
	case *reason == "":
		return fmt.Errorf("run extend needs --reason; a ceiling raised without one is a ceiling nobody chose")
	case *budget <= 0:
		return fmt.Errorf("run extend needs --budget to say by how many transitions")
	}

	workspaceRoot, _, err := paths.resolve()
	if err != nil {
		return err
	}
	manifest := state.ManifestPath(workspaceRoot, *slug)
	current, err := state.Load(manifest)
	if err != nil {
		return err
	}

	// A stopped run is resume's job. Sending it here would skip the blocker
	// clearing and the status restore, and leave a run that looks extended and
	// is still stopped.
	if resumableStatus(current.Status) {
		return fmt.Errorf("run is %s, which is a stop; use `run resume --slug %s --reason ... --budget %d`",
			current.Status, *slug, *budget)
	}

	before := current.MaxTransitions
	current.MaxTransitions += *budget
	current.UpdatedAt = time.Now().UTC()

	payload, err := json.Marshal(map[string]any{
		"reason": *reason, "from": before, "to": current.MaxTransitions,
	})
	if err != nil {
		return fmt.Errorf("encode extend event: %w", err)
	}
	if _, err := state.AppendEvent(state.EventLogPath(workspaceRoot, *slug),
		state.Event{Type: "run_extended", Node: current.CurrentNode, At: current.UpdatedAt, Payload: payload},
	); err != nil {
		return err
	}
	if err := state.Save(manifest, current); err != nil {
		return err
	}

	fmt.Printf("extended %s\n", *slug)
	fmt.Printf("  transitions %d -> %d\n", before, current.MaxTransitions)
	fmt.Printf("  iteration   %d/%d\n", current.Iteration, current.MaxTransitions)
	fmt.Printf("  reason      %s\n", *reason)
	return nil
}
