package app

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/graph"
	"github.com/ducnd58233/vibe-agent/runtime/internal/loop"
	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/validate"
)

// StartDeliveryRun creates a delivery run at the graph initial node.
func StartDeliveryRun(workspaceRoot, toolkitRoot, slug, goal, graphID string) error {
	if !validate.Slug(slug) {
		return fmt.Errorf("invalid slug")
	}
	if goal == "" {
		return fmt.Errorf("goal required")
	}
	if graphID == "" {
		graphID = "goal-delivery"
	}
	loaded, err := graph.LoadByID(graph.DefaultDir(toolkitRoot), graphID)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	entry, err := state.PrepareStart(workspaceRoot, slug, now)
	if err != nil {
		return err
	}
	current, err := state.NewRun(slug, goal, loaded.Metadata.ID, loaded.Spec.MaxTransitions, now)
	if err != nil {
		return err
	}
	current.Date = entry.Date
	current.Version = entry.Version
	if err := loop.New(loaded).Enter(current); err != nil {
		return err
	}
	payload, err := json.Marshal(map[string]string{"goal": current.Goal, "graph": current.GraphID})
	if err != nil {
		return fmt.Errorf("encode start event: %w", err)
	}
	if _, err := state.AppendRunEvent(state.EventLogPath(workspaceRoot, slug),
		state.Event{Type: state.EventRunStarted, Node: current.CurrentNode, At: current.CreatedAt, Payload: payload},
	); err != nil {
		return err
	}
	return state.Save(state.ManifestPath(workspaceRoot, slug), current)
}
