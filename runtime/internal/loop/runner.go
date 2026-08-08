// Package loop advances a run through its graph.
//
// This is the outer loop. It decides which node comes next from recorded
// evidence; it never runs a model and never touches a coding agent's own
// inner loop.
//
// The rule the package exists to enforce: a transition happens because a check
// was written by a process, a file, an API, or a person. Model output cannot
// move a run forward.
package loop

import (
	"fmt"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/graph"
	"github.com/ducnd58233/vibe-agent/runtime/internal/state"
)

// MaxBlockerAttempts is the stop rule from the delivery command: three cycles
// on the same blocker end the run and report root cause instead of retrying a
// fourth time.
const MaxBlockerAttempts = 3

// Outcome is what a node reported. Agent and human nodes supply it; verifier
// nodes have it derived from evidence.
type Outcome struct {
	// Result is a free-form label the graph can branch on through a guard with
	// source "result", for example "changes_requested".
	Result map[string]bool
	// Check is evidence to record before resolving the transition. Nil when the
	// node produces none.
	Check *NamedCheck
	// Blocker, when set, records a failure at this node for the stop rule.
	Blocker string
}

// NamedCheck is one check result plus the key it belongs under.
type NamedCheck struct {
	Name  string
	Check state.Check
}

// Transition is the decision the runner made.
type Transition struct {
	From     string
	To       string
	Via      string // guard that fired, or "" for the fallback
	Terminal bool
	Status   state.Status
}

// Runner advances runs. It holds no per-run state; everything lives in the
// manifest, so a fresh process resumes exactly where the last one stopped.
type Runner struct {
	Graph *graph.Graph
	Now   func() time.Time
}

// New builds a runner for a graph.
func New(g *graph.Graph) *Runner {
	return &Runner{Graph: g, Now: time.Now}
}

func (r *Runner) now() time.Time {
	if r.Now == nil {
		return time.Now().UTC()
	}
	return r.Now().UTC()
}

// Enter puts a run at the graph's initial node. It is separate from starting a
// run so `run start` can create state before a graph is chosen.
func (r *Runner) Enter(run *state.Run) error {
	if run.CurrentNode != "" {
		return fmt.Errorf("run is already at node %q", run.CurrentNode)
	}
	run.CurrentNode = r.Graph.Spec.Initial
	run.MaxTransitions = r.Graph.Spec.MaxTransitions
	run.GraphID = r.Graph.Metadata.ID
	run.UpdatedAt = r.now()
	return nil
}

// Advance applies an outcome and moves the run to the next node.
//
// Order matters: evidence is recorded first, then guards are resolved against
// the updated state. Resolving first would branch on stale state.
func (r *Runner) Advance(run *state.Run, outcome Outcome) (*Transition, error) {
	if run.Status != state.StatusRunning && run.Status != state.StatusAwaitingHuman {
		return nil, fmt.Errorf("run is %s and cannot advance", run.Status)
	}
	current, ok := r.Graph.Node(run.CurrentNode)
	if !ok {
		return nil, fmt.Errorf("run is at node %q, which the graph does not define", run.CurrentNode)
	}

	now := r.now()

	if outcome.Check != nil {
		if err := run.SetCheckAt(outcome.Check.Name, outcome.Check.Check, now); err != nil {
			return nil, err
		}
	}
	if current.Type == graph.NodeVerifier && outcome.Check == nil {
		return nil, fmt.Errorf("verifier node %q must report a check; nothing else may write %q", run.CurrentNode, current.Check)
	}
	if outcome.Check != nil && current.Check != "" && outcome.Check.Name != current.Check {
		return nil, fmt.Errorf("node %q writes check %q, not %q", run.CurrentNode, current.Check, outcome.Check.Name)
	}

	if outcome.Blocker != "" {
		if r.recordBlocker(run, run.CurrentNode, outcome.Blocker, now) >= MaxBlockerAttempts {
			run.Status = state.StatusFailed
			run.UpdatedAt = now
			return &Transition{
				From: run.CurrentNode, To: run.CurrentNode,
				Terminal: true, Status: state.StatusFailed,
			}, nil
		}
	}

	edge, guardName, err := r.resolve(run, outcome)
	if err != nil {
		return nil, err
	}

	run.Iteration++
	if run.Iteration > run.MaxTransitions {
		run.Status = state.StatusBudgetExceeded
		run.UpdatedAt = now
		return &Transition{
			From: run.CurrentNode, To: run.CurrentNode,
			Terminal: true, Status: state.StatusBudgetExceeded,
		}, nil
	}

	transition := &Transition{From: run.CurrentNode, To: edge.To, Via: guardName}
	run.CurrentNode = edge.To
	run.UpdatedAt = now

	next, _ := r.Graph.Node(edge.To)
	switch next.Type {
	case graph.NodeTerminal:
		transition.Terminal = true
		transition.Status = terminalStatus(next.Status)
		run.Status = transition.Status
	case graph.NodeHumanGate:
		run.Status = state.StatusAwaitingHuman
		transition.Status = state.StatusAwaitingHuman
	default:
		run.Status = state.StatusRunning
		transition.Status = state.StatusRunning
	}
	return transition, nil
}

// SkipReason reports whether a node's declared skip condition holds, and why.
//
// The condition was parsed and validated from the day skipWhen existed, and
// never read. A graph could therefore declare that a step was conditional and
// the runtime would run it unconditionally: of the two states the graph
// described, only one was real. This is the missing half.
//
// It answers for the node the caller names rather than the current one, so a
// caller can ask before entering a node as well as on arrival.
func (r *Runner) SkipReason(run *state.Run, nodeID string) (string, bool) {
	node, ok := r.Graph.Node(nodeID)
	if !ok || node.SkipWhen == "" {
		return "", false
	}

	name, negated := node.SkipCondition()
	// A guard that fails to resolve is not a reason to skip. Skipping on an
	// unresolvable condition would turn a graph bug into a silently dropped
	// verification, which is the more expensive of the two failures.
	value, err := r.evaluate(run, Outcome{}, name)
	if err != nil {
		return "", false
	}
	if value == negated {
		return "", false
	}

	condition := name
	if negated {
		condition = "!" + name
	}
	return fmt.Sprintf("the graph declares this node skipped when %s", condition), true
}

// resolve picks the edge to follow. Conditional edges are tried in order, then
// the fallback. A node with neither a matching guard nor a fallback is a graph
// bug, which the validator rejects, so reaching that here means the graph
// changed under a running run.
func (r *Runner) resolve(run *state.Run, outcome Outcome) (graph.Edge, string, error) {
	for _, edge := range r.Graph.OutgoingEdges(run.CurrentNode) {
		if edge.When == "" {
			return edge, "", nil
		}
		name, negated := edge.Negated()
		value, err := r.evaluate(run, outcome, name)
		if err != nil {
			return graph.Edge{}, "", err
		}
		if value != negated {
			return edge, edge.When, nil
		}
	}
	return graph.Edge{}, "", fmt.Errorf("no edge leaves node %q for the current state; the graph and the run have diverged", run.CurrentNode)
}

// evaluate resolves one guard to a boolean. A guard reading a check that has
// not been written yet is false, not an error: the run simply has not got there.
func (r *Runner) evaluate(run *state.Run, outcome Outcome, name string) (bool, error) {
	guard, ok := r.Graph.Guard(name)
	if !ok {
		return false, fmt.Errorf("guard %q is not declared in graph %q", name, r.Graph.Metadata.ID)
	}

	switch guard.Source {
	case graph.GuardFlag, "":
		return run.Flags[guard.Key()], nil
	case graph.GuardCheck:
		check, recorded := run.Checks[guard.Key()]
		if !recorded {
			return false, nil
		}
		// A skipped check satisfies a guard only where the graph opted in. This
		// used to be unconditional, which erased the distinction the state schema
		// keeps: any check that got skipped opened its own gate, whatever the
		// graph intended.
		return check.Passed || (check.Skipped && guard.AcceptsSkipped), nil
	case graph.GuardResult:
		return outcome.Result[guard.Name], nil
	case graph.GuardRuntime:
		return r.runtimeGuard(run, guard.Key()), nil
	default:
		return false, fmt.Errorf("guard %q has unknown source %q", name, guard.Source)
	}
}

// runtimeGuard answers the booleans the runner maintains itself rather than any
// node producing.
func (r *Runner) runtimeGuard(run *state.Run, key string) bool {
	switch key {
	case "blocked":
		for _, blocker := range run.Blockers {
			if blocker.Attempts >= MaxBlockerAttempts {
				return true
			}
		}
		return false
	case "budget_exceeded":
		return run.Iteration >= run.MaxTransitions
	default:
		return false
	}
}

// recordBlocker counts repeated failures at the same node with the same reason,
// and returns the new attempt count.
func (r *Runner) recordBlocker(run *state.Run, node, reason string, now time.Time) int {
	for i := range run.Blockers {
		if run.Blockers[i].Node == node && run.Blockers[i].Reason == reason {
			run.Blockers[i].Attempts++
			run.Blockers[i].At = now
			return run.Blockers[i].Attempts
		}
	}
	run.Blockers = append(run.Blockers, state.Blocker{
		Node: node, Reason: reason, Attempts: 1, At: now,
	})
	return 1
}

func terminalStatus(status graph.TerminalStatus) state.Status {
	switch status {
	case graph.TerminalDone:
		return state.StatusDone
	case graph.TerminalFailed:
		return state.StatusFailed
	case graph.TerminalCancelled:
		return state.StatusCancelled
	default:
		return state.StatusFailed
	}
}
