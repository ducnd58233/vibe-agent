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
	run.Date = "2026-08-18"
	run.Version = 1
	rows := ProjectGraph(g, run)
	prompts := AwaitingChatPrompts(rows, "fixture-session", "", run.Date, run.Version)
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
	if !strings.Contains(got.Prompt, "docs/2026-08-18/fixture-session/1/SPEC-2026-08-18.md") {
		t.Fatalf("prompt = %q", got.Prompt)
	}
	if strings.ContainsAny(got.Prompt, "<>") {
		t.Fatalf("prompt still has an unfilled placeholder, prompt = %q", got.Prompt)
	}
}

func TestAwaitingChatPromptsHumanGateShowsRunGoal(t *testing.T) {
	g := loadGoalDeliveryGraph(t)
	run, err := state.NewRun("fixture-session", "unique-goal-xyz", "goal-delivery", 50, time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	run.CurrentNode = "intake"
	run.Status = state.StatusAwaitingHuman
	rows := ProjectGraph(g, run)
	prompts := AwaitingChatPrompts(rows, "fixture-session", run.Goal, run.Date, run.Version)
	if len(prompts) != 1 {
		t.Fatalf("prompts = %+v", prompts)
	}
	got := prompts[0]
	if got.Title != confirmGoalTitle {
		t.Fatalf("title = %q want confirm line, not graph YAML", got.Title)
	}
	if !strings.Contains(got.Prompt, "unique-goal-xyz") {
		t.Fatalf("prompt = %q", got.Prompt)
	}
	if strings.Contains(got.Prompt, "measurable done line") {
		t.Fatalf("graph YAML must not replace the goal, prompt = %q", got.Prompt)
	}
}

func TestAwaitingChatPromptsApproveSpecKeepsGraphPrompt(t *testing.T) {
	g := loadGoalDeliveryGraph(t)
	run, err := state.NewRun("fixture-session", "unique-goal-xyz", "goal-delivery", 50, time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	run.CurrentNode = "approve_spec"
	run.Status = state.StatusAwaitingHuman
	run.Date = "2026-08-18"
	run.Version = 2
	rows := ProjectGraph(g, run)
	prompts := AwaitingChatPrompts(rows, "fixture-session", run.Goal, run.Date, run.Version)
	if len(prompts) != 1 {
		t.Fatalf("prompts = %+v", prompts)
	}
	got := prompts[0]
	if got.Title == confirmGoalTitle {
		t.Fatal("later human_gate cards must not reuse the intake confirm title")
	}
	if !strings.Contains(got.Prompt, "docs/2026-08-18/fixture-session/2/SPEC-2026-08-18.md") {
		t.Fatalf("approve_spec must keep graph YAML, prompt = %q", got.Prompt)
	}
	if strings.Contains(got.Prompt, "unique-goal-xyz") {
		t.Fatalf("run goal must not replace approve_spec copy, prompt = %q", got.Prompt)
	}
}

func TestAwaitingChatPromptsRedactsSecretInGoal(t *testing.T) {
	g := loadGoalDeliveryGraph(t)
	secret := "sk-0123456789abcdef0123456789ab"
	run, err := state.NewRun("fixture-session", "ship with "+secret, "goal-delivery", 50, time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	run.CurrentNode = "intake"
	run.Status = state.StatusAwaitingHuman
	rows := ProjectGraph(g, run)
	prompts := AwaitingChatPrompts(rows, "fixture-session", run.Goal, run.Date, run.Version)
	if len(prompts) != 1 {
		t.Fatalf("prompts = %+v", prompts)
	}
	if strings.Contains(prompts[0].Prompt, secret) {
		t.Fatalf("secret leaked in prompt %q", prompts[0].Prompt)
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
	prompts := AwaitingChatPrompts(rows, "fixture-session", "unique-goal-xyz", run.Date, run.Version)
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
