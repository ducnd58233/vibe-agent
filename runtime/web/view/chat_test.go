package view

import (
	"strings"
	"testing"
	"time"

	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
)

func TestAwaitingChatPromptsIncludesHumanGate(t *testing.T) {
	g := loadGoalDeliveryGraph(t)
	run, err := state.NewRun("fixture-session", "goal", "goal-delivery", 50, time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	run.CurrentNode = "approve_spec"
	run.Status = state.StatusAwaitingHuman
	rows := ProjectGraph(g, run)
	prompts := AwaitingChatPrompts(rows, "fixture-session")
	if len(prompts) != 1 {
		t.Fatalf("prompts = %+v", prompts)
	}
	got := prompts[0]
	if got.NodeID != "approve_spec" || got.Type != "human_gate" || !got.CanDecide {
		t.Fatalf("prompt = %+v", got)
	}
	if got.Check != "spec_approved" {
		t.Fatalf("check = %q", got.Check)
	}
	if !strings.Contains(got.Prompt, "docs/fixture-session/SPEC.md") {
		t.Fatalf("prompt = %q", got.Prompt)
	}
}

func TestAwaitingChatPromptsIncludesCurrentVerifier(t *testing.T) {
	g := loadGoalDeliveryGraph(t)
	run, err := state.NewRun("fixture-session", "goal", "goal-delivery", 50, time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	run.CurrentNode = "test"
	run.Status = state.StatusRunning
	rows := ProjectGraph(g, run)
	prompts := AwaitingChatPrompts(rows, "fixture-session")
	if len(prompts) != 1 || prompts[0].Type != "verifier" || prompts[0].CanDecide {
		t.Fatalf("prompts = %+v", prompts)
	}
	if strings.TrimSpace(prompts[0].Prompt) != "" {
		t.Fatalf("verifier with no graph prompt must not copy the description into the body, got %q", prompts[0].Prompt)
	}
	if strings.TrimSpace(prompts[0].Title) == "" {
		t.Fatal("verifier title should keep the node description")
	}
}
