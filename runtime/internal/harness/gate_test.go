package harness

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ducnd58233/vibe-agent/runtime/internal/state"
)

// approved records the evidence approve_merge reads, the way a checkpoint would.
func approved(run *state.Run) {
	if run.Checks == nil {
		run.Checks = map[string]state.Check{}
	}
	run.Checks[shipCheck] = state.Check{Passed: true, Source: state.SourceExitCode, At: at()}
	run.Checks[mergeApprovedCheck] = state.Check{Passed: true, Source: state.SourceHumanEvent, At: at()}
}

// onBranch writes the minimum git plumbing the gate reads, so a bare `git push`
// can be resolved without a real repository.
func onBranch(t *testing.T, root, branch string) {
	t.Helper()
	gitDir := filepath.Join(root, ".git")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("ref: refs/heads/"+branch+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile HEAD: %v", err)
	}
}

// attempt runs the gate against one shell command and reports the block, if any.
func attempt(t *testing.T, root, command string) (*BlockError, string) {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"tool_name":  "Bash",
		"tool_input": map[string]string{"command": command},
	})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var out bytes.Buffer
	runErr := Run(Request{
		Event: EventPreToolUse, Client: ClientClaude, WorkspaceRoot: root,
		ToolkitRoot: toolkitRoot, Stdin: bytes.NewReader(payload),
	}, &out)

	var blocked *BlockError
	if runErr != nil && !errors.As(runErr, &blocked) {
		t.Fatalf("gate failed on %q: %v", command, runErr)
	}
	return blocked, out.String()
}

func TestPushToMainIsBlockedWhileMergeApprovalIsMissing(t *testing.T) {
	root := workspaceWithRun(t)

	blocked, _ := attempt(t, root, "git push origin main")
	if blocked == nil {
		t.Fatal("a push to main was allowed with no merge approval on the run")
	}
	if !strings.Contains(blocked.Reason, mergeApprovedCheck) {
		t.Errorf("the refusal does not name the missing evidence: %s", blocked.Reason)
	}
	if !strings.Contains(blocked.Reason, "vibe-agent checkpoint") {
		t.Errorf("the refusal does not say how to satisfy it: %s", blocked.Reason)
	}
}

// The refusal should point at the first missing thing, not the last one, or it
// sends the reader to record approval for a ship that has not happened.
func TestTheRefusalNamesShipWhenShipHasNotPassed(t *testing.T) {
	root := workspaceWithRun(t)
	blocked, _ := attempt(t, root, "gh pr merge 12 --squash")
	if blocked == nil {
		t.Fatal("gh pr merge was allowed with no merge approval on the run")
	}
	if !strings.Contains(blocked.Reason, "check ship has not passed") {
		t.Errorf("the refusal skips past the ship check: %s", blocked.Reason)
	}
}

func TestApprovedMergeOpensTheGate(t *testing.T) {
	root := workspaceWithRun(t, approved)
	for _, command := range []string{"git push origin main", "gh pr merge 12 --squash"} {
		if blocked, _ := attempt(t, root, command); blocked != nil {
			t.Errorf("%q was blocked after approve_merge passed: %s", command, blocked.Reason)
		}
	}
}

// Pushing a task branch happens at open_pr, long before the ship gate. Blocking
// it would wedge the loop this gate exists to protect.
func TestTaskBranchPushesAreNotGated(t *testing.T) {
	root := workspaceWithRun(t)
	onBranch(t, root, "feature/webhook-idempotency-task-3")

	allowed := []string{
		"git push -u origin feature/webhook-idempotency-task-3",
		"git push",
		"git push --force-with-lease origin HEAD:feature/x",
		"git fetch origin main",
		"gh pr create --fill",
		"npm run build",
	}
	for _, command := range allowed {
		if blocked, _ := attempt(t, root, command); blocked != nil {
			t.Errorf("%q was blocked but does not reach anyone: %s", command, blocked.Reason)
		}
	}
}

// A bare push takes its destination from HEAD, so the branch has to be read
// from the repository rather than the command line.
func TestABarePushIsGatedByTheCheckedOutBranch(t *testing.T) {
	root := workspaceWithRun(t)
	onBranch(t, root, "main")

	if blocked, _ := attempt(t, root, "git push"); blocked == nil {
		t.Error("a bare push from main was allowed")
	}
	if blocked, _ := attempt(t, root, "git push origin"); blocked == nil {
		t.Error("a push naming only the remote was allowed while on main")
	}
}

func TestTheGateSeesPastCommandDressing(t *testing.T) {
	root := workspaceWithRun(t)
	onBranch(t, root, "feature/x")

	gated := []string{
		"cd /repo && git push origin main",
		"git push origin HEAD:main",
		"git push origin +main",
		"git push origin :main",
		"git push origin refs/heads/main",
		"git push --force origin master",
		"git push -o ci.skip origin main",
		"/usr/bin/git push origin main",
		"git.exe push origin main",
		"echo pushing; gh pr merge 12",
		// Fields are split on whitespace, so quotes have to be stripped or the
		// comparison is against a string that only looks like a branch name.
		`git push origin "main"`,
		"git push origin 'main'",
		`git push "origin" "HEAD:main"`,
	}
	for _, command := range gated {
		if blocked, _ := attempt(t, root, command); blocked == nil {
			t.Errorf("%q reached main but was allowed", command)
		}
	}
}

// Without a run there is no state to enforce, and every repo that uses the
// toolkit without starting a run has to keep working.
func TestAWorkspaceWithNoRunIsNotGated(t *testing.T) {
	root := t.TempDir()
	onBranch(t, root, "main")
	if blocked, _ := attempt(t, root, "git push origin main"); blocked != nil {
		t.Errorf("a push was blocked with no run to enforce: %s", blocked.Reason)
	}
}

func TestAFinishedRunDoesNotGate(t *testing.T) {
	root := workspaceWithRun(t, func(run *state.Run) { run.Status = state.StatusDone })
	if blocked, _ := attempt(t, root, "git push origin main"); blocked != nil {
		t.Errorf("a finished run still gated a push: %s", blocked.Reason)
	}
}

// Cursor's beforeShellExecution decides through JSON, so the same verdict has
// to arrive in a different shape or it is silently ignored.
func TestCursorReceivesADenyDecisionRatherThanAnError(t *testing.T) {
	var out bytes.Buffer
	err := Run(Request{
		Event: EventPreToolUse, Client: ClientCursor, WorkspaceRoot: workspaceWithRun(t),
		ToolkitRoot: toolkitRoot,
		Stdin:       strings.NewReader(`{"command":"git push origin main","cwd":"/repo"}`),
	}, &out)
	if err != nil {
		t.Fatalf("Cursor gate returned an error it cannot deliver: %v", err)
	}

	var body map[string]any
	if err := json.Unmarshal(out.Bytes(), &body); err != nil {
		t.Fatalf("Cursor output is not JSON: %v: %s", err, out.String())
	}
	if body["permission"] != "deny" {
		t.Errorf("Cursor was not told to deny: %s", out.String())
	}
	if body["agentMessage"] == nil {
		t.Error("the agent was given no reason for the denial")
	}
}

func TestTheGateSurvivesGarbageAndEmptyInput(t *testing.T) {
	root := workspaceWithRun(t)
	for _, stdin := range []string{"", "{}", "not json at all", `{"tool_input":{}}`} {
		var out bytes.Buffer
		if err := Run(Request{
			Event: EventPreToolUse, Client: ClientClaude, WorkspaceRoot: root,
			ToolkitRoot: toolkitRoot, Stdin: strings.NewReader(stdin),
		}, &out); err != nil {
			t.Errorf("stdin %q produced an error: %v", stdin, err)
		}
	}
}
