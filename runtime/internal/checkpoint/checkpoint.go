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

	"github.com/ducnd58233/vibe-agent/runtime/internal/graph"
	"github.com/ducnd58233/vibe-agent/runtime/internal/loop"
	"github.com/ducnd58233/vibe-agent/runtime/internal/state"
)

// Request is one checkpoint.
type Request struct {
	WorkspaceRoot string
	// GraphDir holds the workflow graphs, normally graph.DefaultDir(toolkit).
	GraphDir string
	Slug     string
	Outcome  loop.Outcome
	// Now is injectable so tests do not depend on the clock.
	Now time.Time
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

// transitionEvent is the payload written for every advance. Key is what makes
// a replay recognisable.
type transitionEvent struct {
	From string `json:"from"`
	To   string `json:"to"`
	Via  string `json:"via"`
	Key  string `json:"key,omitempty"`
}

// Apply records the outcome and advances the run.
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

	logPath := state.EventLogPath(req.WorkspaceRoot, req.Slug)
	key := Key(req.Outcome)

	duplicate, err := replays(logPath, key)
	if err != nil {
		return nil, err
	}
	if duplicate {
		return &Result{Run: run, Graph: loaded, Duplicate: true}, nil
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
		From: from, To: transition.To, Via: transition.Via, Key: key,
	})
	if err != nil {
		return nil, fmt.Errorf("encode transition: %w", err)
	}
	if _, err := state.AppendEvent(logPath, state.Event{
		Type: "transition", Node: transition.To, Payload: payload, At: now,
	}); err != nil {
		return nil, err
	}
	if err := state.Save(manifest, run); err != nil {
		return nil, err
	}
	return &Result{Run: run, Graph: loaded, Transition: transition}, nil
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
		if events[i].Type != "transition" {
			continue
		}
		var previous transitionEvent
		if err := json.Unmarshal(events[i].Payload, &previous); err != nil {
			return false, nil
		}
		return previous.Key != "" && previous.Key == key, nil
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
