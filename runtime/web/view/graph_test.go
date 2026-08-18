package view

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/graph"
	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
)

func TestWalkOrderStartsAtInitial(t *testing.T) {
	g := loadGoalDeliveryGraph(t)
	order := WalkOrder(g)
	if len(order) == 0 || order[0] != g.Spec.Initial {
		t.Fatalf("order = %v initial = %q", order, g.Spec.Initial)
	}
	if indexOf(order, "done") < 0 {
		t.Fatal("expected done terminal in walk order")
	}
}

func TestProjectGraphCurrentNodeAwaiting(t *testing.T) {
	g := loadGoalDeliveryGraph(t)
	run, err := state.NewRun("fixture", "goal", "goal-delivery", 50, time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	run.CurrentNode = "approve_spec"
	run.Status = state.StatusAwaitingHuman
	rows := ProjectGraph(g, run)
	var current GraphNodeRow
	for _, row := range rows {
		if row.ID == "approve_spec" {
			current = row
		}
		if row.ID == "intake" && row.Status != string(GraphStatusPassed) {
			t.Fatalf("intake status = %q", row.Status)
		}
	}
	if current.Status != string(GraphStatusAwaiting) || !current.Current {
		t.Fatalf("current row = %+v", current)
	}
}

func TestProjectGraphFailedRun(t *testing.T) {
	g := loadGoalDeliveryGraph(t)
	run, err := state.NewRun("fixture", "goal", "goal-delivery", 50, time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	run.CurrentNode = "build"
	run.Status = state.StatusFailed
	rows := ProjectGraph(g, run)
	for _, row := range rows {
		if row.ID == "build" && row.Status != string(GraphStatusFailed) {
			t.Fatalf("build status = %q", row.Status)
		}
	}
}

func TestProjectGraphSkippedResearchOptional(t *testing.T) {
	g := loadGoalDeliveryGraph(t)
	run, err := state.NewRun("fixture", "goal", "goal-delivery", 50, time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	run.Flags["research_required"] = false
	run.CurrentNode = "spec"
	rows := ProjectGraph(g, run)
	for _, row := range rows {
		if row.ID == "research" && row.Optional != true {
			t.Fatalf("research row = %+v", row)
		}
	}
}

func loadGoalDeliveryGraph(t *testing.T) *graph.Graph {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	g, err := graph.LoadByID(graph.DefaultDir(root), "goal-delivery")
	if err != nil {
		t.Fatal(err)
	}
	return g
}
