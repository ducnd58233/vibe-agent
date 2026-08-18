package app

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/graph"
	"github.com/ducnd58233/vibe-agent/runtime/internal/loop"
	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
)

var slugPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// StartDeliveryRun creates a delivery run at the graph initial node.
func StartDeliveryRun(workspaceRoot, toolkitRoot, slug, goal, graphID string) error {
	if !slugPattern.MatchString(slug) {
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
	manifest := state.ManifestPath(workspaceRoot, slug)
	if _, err := os.Stat(manifest); err == nil {
		return fmt.Errorf("run already exists")
	}
	current, err := state.NewRun(slug, goal, loaded.Metadata.ID, loaded.Spec.MaxTransitions, time.Now().UTC())
	if err != nil {
		return err
	}
	if err := loop.New(loaded).Enter(current); err != nil {
		return err
	}
	payload, err := json.Marshal(map[string]string{"goal": current.Goal, "graph": current.GraphID})
	if err != nil {
		return fmt.Errorf("encode start event: %w", err)
	}
	if _, err := state.AppendEvent(state.EventLogPath(workspaceRoot, slug),
		state.Event{Type: "run_started", Node: current.CurrentNode, At: current.CreatedAt, Payload: payload},
	); err != nil {
		return err
	}
	return state.Save(manifest, current)
}
