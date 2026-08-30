// Package checkpoint records evidence and advances a run, once.
//
// The sequence it owns, load the run, apply the outcome, append the event, save
// the manifest, used to exist twice: once in the CLI and once in the MCP tool.
// Two copies of a write path is two places for the ordering to drift, and the
// ordering matters, so it lives here instead.
//
// The "once" is the other reason. A checkpoint is a side effect on durable
// state, and the callers are retry-prone by nature: a tool call that times out
// after the write, a shell command run twice because the first looked stuck. A
// replayed checkpoint would advance the graph a second time and burn an
// iteration for evidence that was already recorded, which is the failure
// durable execution systems answer with an idempotency key. This does the same.
package checkpoint

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/checkplan"
	"github.com/ducnd58233/vibe-agent/runtime/internal/graph"
	"github.com/ducnd58233/vibe-agent/runtime/internal/loop"
	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
	"github.com/ducnd58233/vibe-agent/runtime/internal/verifier"
)

// origin is who produced the evidence in a request.
//
// The distinction is not bureaucratic. state.CheckSource records what *kind* of
// evidence a check claims to be, and any caller can type one of the four allowed
// strings. origin records whether this process watched it happen.
//
// It is unexported, and so is the Request field holding it, which is the point:
// no package outside this one can grant itself the runtime's authority. Verify
// sets it after a verifier has returned, and Apply may set it only for a skip
// derived from the tracked check plan in an opted-in auto run.
type origin string

const (
	// originCaller is evidence that arrived as arguments: a CLI flag or an MCP
	// tool parameter. It is a claim about a result, not the result.
	//
	// It is the zero value on purpose, so a new surface that forgets to say
	// where its evidence came from is treated as the untrusted case.
	originCaller origin = ""
	// originRuntime is evidence a verifier in this process produced.
	originRuntime origin = "runtime"
)

// silence the unused warning for the zero-value name, which documents the
// default even though nothing needs to write it.
var _ = originCaller

// Request is one checkpoint.
type Request struct {
	WorkspaceRoot string
	// GraphDir holds the workflow graphs, normally graph.DefaultDir(toolkit).
	GraphDir string
	Slug     string
	Outcome  loop.Outcome
	// origin says whether the runtime produced this evidence or a caller
	// supplied it. Unexported so only Verify can claim the former.
	origin origin
	// Now is injectable so tests do not depend on the clock.
	Now time.Time
	// TokensUsed is what the session log reports so far. Filled by the caller
	// that has the log, because this package must not reach into another
	// module's persistence to find one.
	TokensUsed int
}

func (r Request) now() time.Time {
	if r.Now.IsZero() {
		return time.Now().UTC()
	}
	return r.Now.UTC()
}

// Result is the state after the checkpoint, whether or not it moved anything.
type Result struct {
	Run   *state.Run
	Graph *graph.Graph
	// Transition is nil when Duplicate is true: nothing moved.
	Transition *loop.Transition
	// Duplicate reports that this exact evidence was the last thing recorded,
	// so it was not recorded again.
	Duplicate bool
}

// transitionMatches reports whether a recorded transition carries a key.
//
// An event this package cannot read matches nothing, and why it is unreadable
// changes none of that, so this answers with a bool and never an error.
func transitionMatches(payload []byte, key string) bool {
	var previous transitionEvent
	if json.Unmarshal(payload, &previous) != nil {
		return false
	}
	return previous.Key != "" && previous.Key == key
}

// transitionEvent is the payload written for every advance. Key is what makes
// a replay recognisable.
type transitionEvent struct {
	From string `json:"from"`
	To   string `json:"to"`
	Via  string `json:"via"`
	Key  string `json:"key,omitempty"`
	// Skipped records that the destination was a human gate the graph declared
	// skippable and the run walked through it. In the journal because "no one
	// was asked here" is exactly the kind of thing an audit goes looking for.
	Skipped bool `json:"skipped,omitempty"`
}

// Apply records the outcome and advances the state.
func Apply(req Request) (*Result, error) {
	manifest := state.ManifestPath(req.WorkspaceRoot, req.Slug)
	run, err := state.Load(manifest)
	if err != nil {
		return nil, err
	}
	loaded, err := graph.LoadByID(req.GraphDir, run.GraphID)
	if err != nil {
		return nil, err
	}
	req, err = deriveAutoSkip(req, run, loaded)
	if err != nil {
		return nil, err
	}
	if err := req.authorize(loaded, run.CurrentNode); err != nil {
		return nil, err
	}

	logPath := state.EventLogPath(req.WorkspaceRoot, req.Slug)
	key := Key(req.Outcome)

	duplicate, err := replays(logPath, key)
	if err != nil {
		return nil, err
	}
	if duplicate {
		return &Result{Run: run, Graph: loaded, Duplicate: true}, nil
	}

	// Tokens are counted by the surface that has the session log; this package
	// only carries the figure to the runner. Monotonic, because a total that
	// could fall would let a budget un-exceed itself.
	if req.TokensUsed > run.TokensUsed {
		run.TokensUsed = req.TokensUsed
	}

	now := req.now()
	from := run.CurrentNode
	transition, err := loop.New(loaded).Advance(run, req.Outcome)
	if err != nil {
		return nil, err
	}

	// Append the event before saving state: an event without a matching state
	// change is recoverable, a state change with no record of why is not.
	payload, err := json.Marshal(transitionEvent{
		From: from, To: transition.To, Via: transition.Via, Key: key, Skipped: transition.Skipped,
	})
	if err != nil {
		return nil, fmt.Errorf("encode transition: %w", err)
	}
	if _, err := state.AppendRunEvent(logPath, state.Event{
		Type: state.EventTransition, Node: transition.To, Payload: payload, At: now,
	}); err != nil {
		return nil, err
	}
	if err := state.Save(manifest, run); err != nil {
		return nil, err
	}
	result := &Result{Run: run, Graph: loaded, Transition: transition}
	return followAutoGates(req, result)
}

// deriveAutoSkip turns a bare checkpoint at an auto run's undeclared verifier
// into the same runtime-produced skip as Verify. It exists for hosts that use
// one checkpoint command for every node while driving an auto run. Declared
// checks still require Verify, so this cannot bypass a real verifier.
func deriveAutoSkip(req Request, run *state.Run, loaded *graph.Graph) (Request, error) {
	if !run.Flags["auto"] || req.Outcome.Check != nil {
		return req, nil
	}

	node, ok := loaded.Node(run.CurrentNode)
	if !ok || node.Type != graph.NodeVerifier {
		return req, nil
	}

	plan, err := checkplan.Load(checkplan.DefaultPath(req.WorkspaceRoot))
	if err != nil {
		return req, err
	}
	if plan.Has(node.Check) {
		return req, nil
	}

	req.origin = originRuntime
	req.Outcome.Check = &loop.NamedCheck{
		Name:  node.Check,
		Check: verifier.Skipped(undeclaredCheckReason(plan, node.Check), req.now()).Check,
	}
	return req, nil
}

// advanceStuckVerifier moves past a verifier node when the check already passed
// but a replay left the run sitting on the node anyway.
func advanceStuckVerifier(req VerifyRequest, plan *Plan, run *state.Run, loaded *graph.Graph) (*Result, error) {
	if run.CurrentNode != plan.Node {
		return nil, nil
	}
	node, ok := loaded.Node(plan.Node)
	if !ok || node.Type != graph.NodeVerifier || node.Check != plan.Check {
		return nil, nil
	}
	existing, ok := run.Checks[plan.Check]
	if !ok || !existing.Passed || existing.Skipped {
		return nil, nil
	}
	manifest := state.ManifestPath(req.WorkspaceRoot, req.Slug)
	logPath := state.EventLogPath(req.WorkspaceRoot, req.Slug)

	now := req.now()
	from := run.CurrentNode
	outcome := loop.Outcome{Check: &loop.NamedCheck{Name: plan.Check, Check: existing}}
	transition, err := loop.New(loaded).Advance(run, outcome)
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(transitionEvent{
		From: from, To: transition.To, Via: transition.Via, Key: "stuck-" + plan.Check, Skipped: transition.Skipped,
	})
	if err != nil {
		return nil, fmt.Errorf("encode transition: %w", err)
	}
	if _, err := state.AppendRunEvent(logPath, state.Event{
		Type: state.EventTransition, Node: transition.To, Payload: payload, At: now,
	}); err != nil {
		return nil, err
	}
	if err := state.Save(manifest, run); err != nil {
		return nil, err
	}
	return &Result{Run: run, Graph: loaded, Transition: transition}, nil
}

func undeclaredCheckReason(plan *checkplan.Plan, check string) string {
	return fmt.Sprintf("%s declares no %s check; declared checks are %v",
		checkplan.FileName, check, plan.Names())
}

// authorize decides whether this request may write the check at all.
//
// Verifier nodes are the only nodes with a rule, because they are the only nodes
// whose evidence is supposed to come from something other than a person. A human
// gate's check *is* a human's word, so it keeps arriving from a caller.
//
// The rule reads from the workspace's check plan rather than from the request,
// so widening it means editing a file in git.
func (r Request) authorize(loaded *graph.Graph, currentNode string) error {
	if r.Outcome.Check == nil {
		return nil
	}
	// Verify already resolved the plan and the skip condition before producing
	// this, so re-deriving them here would only create a second place for the two
	// answers to disagree. Nothing outside this package can set the field.
	if r.origin == originRuntime {
		return nil
	}
	node, ok := loaded.Node(currentNode)
	if !ok || node.Type != graph.NodeVerifier {
		return nil
	}
	name := r.Outcome.Check.Name

	plan, err := checkplan.Load(checkplan.DefaultPath(r.WorkspaceRoot))
	if err != nil {
		return fmt.Errorf("node %q verifies %q, so the workspace has to declare how: %w", currentNode, name, err)
	}
	entry, err := plan.Entry(name)
	if err != nil {
		return err
	}

	if entry.Human() {
		// A person's word must be recorded as a person's word. Accepting
		// exit_code here would let the declaration launder provenance: the
		// manifest would claim a process ran when the plan says none exists.
		if r.Outcome.Check.Check.Source != state.SourceHumanEvent {
			return fmt.Errorf("check %q is declared %s in %s, so it needs source %s, not %s",
				name, checkplan.HumanVerifier, plan.Path(), state.SourceHumanEvent, r.Outcome.Check.Check.Source)
		}
		return nil
	}

	return fmt.Errorf("check %q at node %q comes from a verifier, not from arguments; "+
		"run `vibe-agent verify --slug %s` so %s produces the evidence, "+
		"or declare the check %s in %s if a person decides it",
		name, currentNode, r.Slug, verifierName(node, entry), checkplan.HumanVerifier, plan.Path())
}

// verifierName is the implementation that will produce a check, for an error
// message that names something the reader can look up.
func verifierName(node graph.Node, entry checkplan.Entry) string {
	if entry.Verifier != "" {
		return "the " + entry.Verifier + " verifier"
	}
	if node.Verifier != "" {
		return "the " + string(node.Verifier) + " verifier"
	}
	return "a verifier"
}

// replays reports whether the most recent transition recorded this same
// evidence.
//
// Only the most recent one is compared, on purpose. A graph with a loop can
// legitimately record the same check again after going around, and that is a
// second real result rather than a replay of the first. What it cannot be is
// two transitions in a row asserting exactly the same thing.
func replays(logPath, key string) (bool, error) {
	if key == "" {
		return false, nil
	}
	events, err := state.ReadEvents(logPath)
	if err != nil {
		return false, err
	}
	for i := len(events) - 1; i >= 0; i-- {
		if events[i].Type != state.EventTransition {
			continue
		}
		return transitionMatches(events[i].Payload, key), nil
	}
	return false, nil
}

// Key identifies a checkpoint by what it asserts.
//
// Two things are deliberately absent. The timestamp, because two calls a second
// apart carrying the same evidence are the same checkpoint and the clock would
// make every replay look new. And the node, because a retry arrives after the
// first attempt already moved the run, so keying on where the run is now would
// make the replay it is meant to catch look like a different checkpoint.
//
// An outcome that asserts nothing gets no key. A bare advance past an agent
// node carries nothing to recognise a replay by, and treating two of them as
// the same event would stall a run walking through consecutive nodes.
func Key(outcome loop.Outcome) string {
	if outcome.Check == nil && outcome.Blocker == "" && len(outcome.Result) == 0 {
		return ""
	}

	parts := []string{"blocker=" + outcome.Blocker}
	if outcome.Check != nil {
		parts = append(parts, fmt.Sprintf("check=%s passed=%t skipped=%t source=%s ref=%s",
			outcome.Check.Name, outcome.Check.Check.Passed, outcome.Check.Check.Skipped,
			outcome.Check.Check.Source, outcome.Check.Check.Ref))
	}

	results := make([]string, 0, len(outcome.Result))
	for name, value := range outcome.Result {
		results = append(results, fmt.Sprintf("%s=%t", name, value))
	}
	sort.Strings(results)
	parts = append(parts, "result="+strings.Join(results, ","))

	sum := sha256.Sum256([]byte(strings.Join(parts, "\n")))
	return hex.EncodeToString(sum[:])[:16]
}
