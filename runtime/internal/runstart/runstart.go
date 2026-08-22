// Package runstart creates a run and places it at the graph's initial node.
//
// cmd and mcp share this path so slug derivation and graph selection stay in
// one place rather than drifting between surfaces.
package runstart

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/auto"
	"github.com/ducnd58233/vibe-agent/runtime/internal/graph"
	"github.com/ducnd58233/vibe-agent/runtime/internal/graphroute"
	"github.com/ducnd58233/vibe-agent/runtime/internal/loop"
	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
)

// Options is everything needed to start a run once graphroute has resolved ids.
type Options struct {
	WorkspaceRoot string
	ToolkitRoot   string
	Resolved      graphroute.Resolved
	Auto          bool
	TaskSource    string
	TokenBudget   int
	WallclockSec  int
	Now           time.Time
}

// Result is the started run and where it was written.
type Result struct {
	Run      *state.Run
	Manifest string
	Events   string
}

// Start loads the graph, creates run state, and enters the initial node.
func Start(opts Options) (Result, error) {
	if opts.WorkspaceRoot == "" || opts.ToolkitRoot == "" {
		return Result{}, fmt.Errorf("workspace and toolkit roots are required")
	}
	now := opts.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}

	loaded, err := graph.LoadByID(graph.DefaultDir(opts.ToolkitRoot), opts.Resolved.GraphID)
	if err != nil {
		return Result{}, err
	}

	recorded := opts.Resolved.Goal
	if opts.TaskSource != "" {
		recorded = auto.Task(opts.TaskSource, opts.Resolved.Goal)
	}

	entry, err := state.PrepareStart(opts.WorkspaceRoot, opts.Resolved.Slug, now)
	if err != nil {
		return Result{}, err
	}

	current, err := state.NewRun(opts.Resolved.Slug, recorded, loaded.Metadata.ID, loaded.Spec.MaxTransitions, now)
	if err != nil {
		return Result{}, err
	}
	current.Date = entry.Date
	current.Version = entry.Version
	current.TokenBudget = opts.TokenBudget
	current.WallclockSeconds = opts.WallclockSec
	if opts.Auto {
		if err := current.SetFlagAt("auto", true, now); err != nil {
			return Result{}, err
		}
	}
	if err := loop.New(loaded).Enter(current); err != nil {
		return Result{}, err
	}

	payload, err := json.Marshal(map[string]any{
		"goal": current.Goal, "graph": current.GraphID, "auto": opts.Auto, "taskSource": opts.TaskSource,
	})
	if err != nil {
		return Result{}, fmt.Errorf("encode start event: %w", err)
	}
	manifest := state.ManifestPath(opts.WorkspaceRoot, opts.Resolved.Slug)
	events := state.EventLogPath(opts.WorkspaceRoot, opts.Resolved.Slug)
	if _, err := state.AppendRunEvent(events,
		state.Event{Type: state.EventRunStarted, Node: current.CurrentNode, At: current.CreatedAt, Payload: payload},
	); err != nil {
		return Result{}, err
	}
	if err := state.Save(manifest, current); err != nil {
		return Result{}, err
	}

	return Result{Run: current, Manifest: manifest, Events: events}, nil
}
