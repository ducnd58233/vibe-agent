package main

import (
	"testing"

	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
)

func TestGoalCommandStartsDeliveryRun(t *testing.T) {
	root := t.TempDir()
	flags := []string{"--workspace", root, "--toolkit", toolkitRoot,
		"Ship webhook idempotency keys"}
	if err := goalCommand(flags); err != nil {
		t.Fatal(err)
	}
	run, err := state.Load(state.ManifestPath(root, "ship-webhook-idempotency-keys"))
	if err != nil {
		t.Fatal(err)
	}
	if run.GraphID != "goal-delivery" {
		t.Fatalf("graph = %q", run.GraphID)
	}
	if run.Flags["auto"] {
		t.Fatal("goal command must not set auto flag")
	}
}

func TestResearchCommandStartsResearcherGraph(t *testing.T) {
	root := t.TempDir()
	flags := []string{"--workspace", root, "--toolkit", toolkitRoot,
		"Compare RAG chunking strategies"}
	if err := researchCommand(flags); err != nil {
		t.Fatal(err)
	}
	run, err := state.Load(state.ManifestPath(root, "compare-rag-chunking-strategies"))
	if err != nil {
		t.Fatal(err)
	}
	if run.GraphID != "researcher-delivery" {
		t.Fatalf("graph = %q", run.GraphID)
	}
}

func TestAutoResearchSubcommand(t *testing.T) {
	root := t.TempDir()
	optedIn(t, root, false)
	startAuto(t, root, "research", "Evaluate dense retrieval for legal QA")
	run, err := state.Load(state.ManifestPath(root, "evaluate-dense-retrieval-legal"))
	if err != nil {
		t.Fatal(err)
	}
	if run.GraphID != "researcher-delivery" {
		t.Fatalf("graph = %q", run.GraphID)
	}
	if !run.Flags["auto"] {
		t.Fatal("expected auto flag")
	}
}
