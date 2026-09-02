package checkpoint

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/graph"
	"github.com/ducnd58233/vibe-agent/runtime/internal/loop"
	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/runpath"
	"github.com/ducnd58233/vibe-agent/runtime/internal/testutil"
)

func TestAutoResearchCheckpointSkipsApplicabilityGate(t *testing.T) {
	root := t.TempDir()
	graphDir := filepath.Join("..", "..", "..", ".ai-agents", "graphs")
	loaded, err := graph.LoadByID(graphDir, "researcher-delivery")
	if err != nil {
		t.Fatalf("load graph: %v", err)
	}

	run, err := state.NewRun("auto-research-gate", "topic", loaded.Metadata.ID, loaded.Spec.MaxTransitions, time.Date(2026, 8, 22, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	run.Flags = map[string]bool{"auto": true}
	run.CurrentNode = "literature"
	run.Status = state.StatusRunning
	testutil.EnsureRunIndex(t, root, run.Slug)
	entry, err := runpath.Resolve(root, run.Slug)
	if err != nil {
		t.Fatal(err)
	}
	run.Date = entry.Date
	run.Version = entry.Version
	if err := state.Save(state.ManifestPath(root, run.Slug), run); err != nil {
		t.Fatal(err)
	}

	writeResearchDoc(t, root, entry.Date, entry.Slug, entry.Version, settledResearchBody())

	result, err := Apply(Request{
		WorkspaceRoot: root,
		GraphDir:      graphDir,
		Slug:          run.Slug,
		Outcome:       loop.Outcome{},
		Now:           time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if result.Run.CurrentNode != "hypothesis" {
		t.Fatalf("current node = %q, want hypothesis after auto gate follow-through", result.Run.CurrentNode)
	}
	if result.Run.Status != state.StatusRunning {
		t.Fatalf("status = %q, want running", result.Run.Status)
	}
	check, recorded := result.Run.Checks["applicability_approved"]
	if !recorded || !check.Skipped || check.Passed {
		t.Fatalf("applicability gate = %+v, want skipped and not passed", check)
	}
}

func TestGoalResearchCheckpointStopsAtApplicabilityGate(t *testing.T) {
	root := t.TempDir()
	graphDir := filepath.Join("..", "..", "..", ".ai-agents", "graphs")
	loaded, err := graph.LoadByID(graphDir, "researcher-delivery")
	if err != nil {
		t.Fatalf("load graph: %v", err)
	}

	run, err := state.NewRun("manual-research-gate", "topic", loaded.Metadata.ID, loaded.Spec.MaxTransitions, time.Date(2026, 8, 22, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	run.CurrentNode = "literature"
	run.Status = state.StatusRunning
	testutil.EnsureRunIndex(t, root, run.Slug)
	entry, err := runpath.Resolve(root, run.Slug)
	if err != nil {
		t.Fatal(err)
	}
	run.Date = entry.Date
	run.Version = entry.Version
	if err := state.Save(state.ManifestPath(root, run.Slug), run); err != nil {
		t.Fatal(err)
	}

	writeResearchDoc(t, root, entry.Date, entry.Slug, entry.Version, settledResearchBody())

	result, err := Apply(Request{
		WorkspaceRoot: root,
		GraphDir:      graphDir,
		Slug:          run.Slug,
		Outcome:       loop.Outcome{},
		Now:           time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if result.Run.CurrentNode != "approve_applicability" {
		t.Fatalf("current node = %q, want approve_applicability for goal mode", result.Run.CurrentNode)
	}
	if result.Run.Status != state.StatusAwaitingHuman {
		t.Fatalf("status = %q, want awaiting_human", result.Run.Status)
	}
}

func TestAutoCheckpointAnswersMergeApprovalFromOptIn(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".agent-state"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(root, ".agent-state", "auto.yaml"),
		[]byte("apiVersion: vibe-agent/v1\nkind: AutoConfig\nspec:\n  merge: true\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}

	loaded, err := graph.LoadByID(graphDir, "goal-delivery")
	if err != nil {
		t.Fatal(err)
	}
	run, err := state.NewRun(
		"auto-merge", "ship the change", loaded.Metadata.ID,
		loaded.Spec.MaxTransitions, at(),
	)
	if err != nil {
		t.Fatal(err)
	}
	run.Flags["auto"] = true
	run.CurrentNode = "approve_merge"
	run.Status = state.StatusAwaitingHuman
	testutil.EnsureRunIndex(t, root, run.Slug)
	entry, err := runpath.Resolve(root, run.Slug)
	if err != nil {
		t.Fatal(err)
	}
	run.Date = entry.Date
	run.Version = entry.Version
	if err := state.Save(state.ManifestPath(root, run.Slug), run); err != nil {
		t.Fatal(err)
	}

	result, err := followAutoGates(Request{
		WorkspaceRoot: root,
		GraphDir:      graphDir,
		Slug:          run.Slug,
		Now:           at(),
	}, &Result{
		Run:   run,
		Graph: loaded,
		Transition: &loop.Transition{
			From: "release_review",
			To:   "approve_merge",
		},
	})
	if err != nil {
		t.Fatalf("followAutoGates: %v", err)
	}
	if result.Run.CurrentNode != "merge_ci" {
		t.Fatalf("current node = %q, want merge_ci", result.Run.CurrentNode)
	}
	approval, ok := result.Run.Checks["merge_approved"]
	if !ok {
		t.Fatal("auto merge approval was not recorded")
	}
	if !approval.Passed || approval.Skipped {
		t.Fatalf("merge approval = %+v, want passed and not skipped", approval)
	}
	if approval.Source != state.SourceFileAssert {
		t.Fatalf("merge approval source = %q, want file_assert", approval.Source)
	}
	if !strings.Contains(approval.Ref, "merge=true") {
		t.Fatalf("merge approval ref = %q, want the opt-in answer", approval.Ref)
	}
}

func TestAutoCheckpointLeavesMergeApprovalForUnoptedWorkspace(t *testing.T) {
	root := t.TempDir()
	loaded, err := graph.LoadByID(graphDir, "goal-delivery")
	if err != nil {
		t.Fatal(err)
	}
	run, err := state.NewRun(
		"auto-no-merge", "ship the change", loaded.Metadata.ID,
		loaded.Spec.MaxTransitions, at(),
	)
	if err != nil {
		t.Fatal(err)
	}
	run.Flags["auto"] = true
	run.CurrentNode = "approve_merge"
	run.Status = state.StatusAwaitingHuman
	testutil.EnsureRunIndex(t, root, run.Slug)
	entry, err := runpath.Resolve(root, run.Slug)
	if err != nil {
		t.Fatal(err)
	}
	run.Date = entry.Date
	run.Version = entry.Version
	if err := state.Save(state.ManifestPath(root, run.Slug), run); err != nil {
		t.Fatal(err)
	}

	result, err := followAutoGates(Request{
		WorkspaceRoot: root,
		GraphDir:      graphDir,
		Slug:          run.Slug,
		Now:           at(),
	}, &Result{
		Run:   run,
		Graph: loaded,
		Transition: &loop.Transition{
			From: "release_review",
			To:   "approve_merge",
		},
	})
	if err != nil {
		t.Fatalf("followAutoGates: %v", err)
	}
	if result.Run.CurrentNode != "approve_merge" {
		t.Fatalf("current node = %q, want approve_merge", result.Run.CurrentNode)
	}
	if _, ok := result.Run.Checks["merge_approved"]; ok {
		t.Fatal("merge approval was recorded without opt-in")
	}
}

func writeResearchDoc(t *testing.T, root, date, slug string, version int, body string) {
	t.Helper()
	dir := filepath.Join(root, "docs", date, slug, fmt.Sprintf("%d", version))
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	name := "RESEARCH-" + date + ".md"
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func settledResearchBody() string {
	return strings.Join([]string{
		"# Research",
		"",
		"## Open questions",
		"",
		"- None.",
		"",
		"## Applicability",
		"",
		"| Source | Reuse | Reject | Gap |",
		"| --- | --- | --- | --- |",
		"| Paper A | method X | claim Y | no finance data |",
		"",
		"## Refine",
		"",
		"- Drop claim Y; add our ticker universe.",
		"",
		"```mermaid",
		"flowchart LR",
		"  lit --> apply",
		"```",
	}, "\n")
}
