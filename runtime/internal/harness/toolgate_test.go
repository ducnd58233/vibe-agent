package harness

import (
	"strings"
	"testing"
	"time"

	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
	"github.com/ducnd58233/vibe-agent/runtime/internal/testutil"
)

// workspaceWithActiveRun seeds the one condition the state-write guard needs:
// a run that has not finished.
func workspaceWithActiveRun(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	run, err := state.NewRun("demo", "gate parity", "goal-delivery", 50, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	run.CurrentNode = "build"
	run.Status = state.StatusRunning
	testutil.EnsureRunIndex(t, root, "demo")
	if err := state.Save(state.ManifestPath(root, "demo"), run); err != nil {
		t.Fatal(err)
	}
	return root
}

// The whole point of the exported seam: the inner loop's dispatcher and the
// host hook must reach the same decision about the same call. Two permission
// paths means one goes stale, and the stale one is always the one in use when
// it matters.
func TestVerdictAgreesWithTheHookPathOnTheSameCall(t *testing.T) {
	root := workspaceWithActiveRun(t)
	target := jsonPath(root, ".agent-state", "runs", "2026-08-21", "demo", "1", "manifest.json")

	// The hook half, as a host would send it.
	viaHook := runHook(t, Request{
		Event:         EventPreToolUse,
		Client:        ClientClaude,
		WorkspaceRoot: root,
		Stdin: strings.NewReader(
			`{"tool_name":"Write","tool_input":{"file_path":"` + target + `","content":"{}"}}`),
	})
	var hookBlock *BlockError
	if !asBlock(viaHook, &hookBlock) {
		t.Fatalf("the hook path allowed a write to run state: %v", viaHook)
	}

	// The dispatcher half, through the exported seam.
	viaSeam := Verdict(root, ToolCall{Name: "Write", FilePath: target, Content: "{}"})
	if viaSeam == nil {
		t.Fatal("the exported seam allowed what the hook refused")
	}

	if viaSeam.Reason != hookBlock.Reason {
		t.Errorf("the two paths gave different reasons:\n hook: %s\n seam: %s",
			hookBlock.Reason, viaSeam.Reason)
	}
}

// Agreement has to hold in both directions, or the seam could simply refuse
// everything and still pass the test above.
func TestVerdictAllowsWhatTheHookAllows(t *testing.T) {
	root := workspaceWithActiveRun(t)
	ordinary := jsonPath(root, "runtime", "main.go")

	if err := runHook(t, Request{
		Event:         EventPreToolUse,
		Client:        ClientClaude,
		WorkspaceRoot: root,
		Stdin: strings.NewReader(
			`{"tool_name":"Write","tool_input":{"file_path":"` + ordinary + `","content":"package main"}}`),
	}); err != nil {
		t.Fatalf("the hook path refused an ordinary write: %v", err)
	}

	if blocked := Verdict(root, ToolCall{
		Name: "Write", FilePath: ordinary, Content: "package main",
	}); blocked != nil {
		t.Errorf("the seam refused an ordinary write: %s", blocked.Reason)
	}
}

// A shell command that writes run state is refused the same way a direct write
// is, and the seam has to carry that too.
func TestVerdictRefusesAShellWriteToRunState(t *testing.T) {
	root := workspaceWithActiveRun(t)
	target := jsonPath(root, ".agent-state", "runs", "2026-08-21", "demo", "1", "manifest.json")

	blocked := Verdict(root, ToolCall{Name: "Bash", Command: "echo {} > " + target})
	if blocked == nil {
		t.Fatal("a shell write to run state was allowed")
	}
	if !strings.Contains(blocked.Reason, "run state") {
		t.Errorf("reason = %q, want it to name run state", blocked.Reason)
	}
}
