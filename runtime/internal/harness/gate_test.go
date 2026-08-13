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
	if err := os.MkdirAll(gitDir, 0o750); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("ref: refs/heads/"+branch+"\n"), 0o600); err != nil {
		t.Fatalf("WriteFile HEAD: %v", err)
	}
}

// defaultBranch records what the repository treats as its integration branch,
// the way git does: a symbolic ref for the remote's HEAD.
func withDefaultBranch(t *testing.T, root, branch string) {
	t.Helper()
	dir := filepath.Join(root, ".git", "refs", "remotes", "origin")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "HEAD"),
		[]byte("ref: refs/remotes/origin/"+branch+"\n"), 0o600); err != nil {
		t.Fatalf("WriteFile origin/HEAD: %v", err)
	}
}

// A project whose integration branch is not called main was silently
// unprotected: the guard held a fixed pair of names, and a push to develop or
// trunk reached everyone with nothing in the way. The repository knows its own
// default branch, so the guard asks instead of assuming.
func TestTheRepositorysOwnDefaultBranchIsProtected(t *testing.T) {
	for _, branch := range []string{"develop", "trunk", "production"} {
		t.Run(branch, func(t *testing.T) {
			root := workspaceWithRun(t)
			withDefaultBranch(t, root, branch)
			onBranch(t, root, branch)

			if blocked, _ := attempt(t, root, "git push origin "+branch); blocked == nil {
				t.Errorf("a push to %s, this repository's default branch, was allowed", branch)
			}
			if blocked, _ := attempt(t, root, "git push"); blocked == nil {
				t.Errorf("a bare push from %s was allowed", branch)
			}
		})
	}
}

// The conventional names stay protected whatever the default is. A repository
// that works on develop usually still has a main that means something, and a
// guard that stopped covering it would be a regression dressed as a fix.
func TestMainStaysProtectedWhenTheDefaultIsSomethingElse(t *testing.T) {
	root := workspaceWithRun(t)
	withDefaultBranch(t, root, "develop")
	onBranch(t, root, "main")

	if blocked, _ := attempt(t, root, "git push origin main"); blocked == nil {
		t.Error("a push to main was allowed because the default branch is develop")
	}
}

// A feature branch is still ordinary work.
func TestAFeatureBranchIsNotProtected(t *testing.T) {
	root := workspaceWithRun(t)
	withDefaultBranch(t, root, "develop")
	onBranch(t, root, "feature/webhooks")

	if blocked, _ := attempt(t, root, "git push origin feature/webhooks"); blocked != nil {
		t.Errorf("a push to a feature branch was blocked: %v", blocked)
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

// writeAttempt runs the gate against a file-writing tool call and reports the
// block, if any. Credential refusal is not conditional on an active run, so
// unlike attempt this needs no git plumbing and no recorded checks.
func writeAttempt(t *testing.T, root, target, content string) (*BlockError, string) {
	t.Helper()
	body, err := json.Marshal(map[string]any{
		"tool_name":  "Write",
		"tool_input": map[string]string{"file_path": target, "content": content},
	})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var out bytes.Buffer
	runErr := Run(Request{
		Event: EventPreToolUse, Client: ClientClaude, WorkspaceRoot: root,
		ToolkitRoot: toolkitRoot, Stdin: bytes.NewReader(body),
	}, &out)

	var blocked *BlockError
	if runErr != nil && !errors.As(runErr, &blocked) {
		t.Fatalf("gate failed writing %q: %v", target, runErr)
	}
	return blocked, out.String()
}

func TestALiveCredentialIsRefusedOnWrite(t *testing.T) {
	root := t.TempDir()

	// Each of these is assembled at run time rather than written as one literal,
	// so this test file does not itself become a credential-shaped file that
	// every other guard in the repository has to be told to ignore.
	cases := []struct {
		name    string
		content string
	}{
		{"private key block", "-----BEGIN RSA PRIVATE" + " KEY-----\nMIIEow==\n"},
		{"github token", "const t = \"gh" + "p_" + strings.Repeat("A1b2", 5) + "\""},
		{"openai style key", "OPENAI = 'sk-" + strings.Repeat("x9Y7", 5) + "'"},
		{"slack token", "SLACK = \"xox" + "b-" + strings.Repeat("12ab-", 4) + "\""},
		{"aws access key id", "id = \"AKIA" + strings.Repeat("ABCD", 4) + "\""},
		{"aws secret access key", "aws_secret_access" + "_key = " + strings.Repeat("z", 20)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			blocked, _ := writeAttempt(t, root, "config.ts", tc.content)
			if blocked == nil {
				t.Fatalf("expected a refusal for %s", tc.name)
			}
			if !strings.Contains(blocked.Reason, "live credential") {
				t.Fatalf("reason does not name the problem: %q", blocked.Reason)
			}
			if strings.Contains(blocked.Reason, tc.content) {
				t.Fatal("the refusal echoed the credential back into the transcript")
			}
		})
	}
}

func TestOrdinaryCodeIsNotRefused(t *testing.T) {
	root := t.TempDir()

	// Every one of these has tripped a naive credential regex somewhere. A gate
	// that refuses them gets switched off, and then it guards nothing.
	cases := []struct {
		name    string
		content string
	}{
		{"env reference", "const key = process.env.STRIPE_SECRET_KEY;"},
		{"placeholder", "API_KEY=your-api-key-here"},
		{"short sk word", "const risk = \"sk-1\";"},
		{"prose about tokens", "// The token is read from the vault at startup."},
		{"base64 blob that is not a key", "data:image/png;base64," + strings.Repeat("QUJD", 20)},
		{"sha hash", "sha256 = \"" + strings.Repeat("ab12cd34", 8) + "\""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if blocked, _ := writeAttempt(t, root, "app.ts", tc.content); blocked != nil {
				t.Fatalf("false refusal on %s: %s", tc.name, blocked.Reason)
			}
		})
	}
}

func TestAMarkedLineIsExempt(t *testing.T) {
	root := t.TempDir()
	content := "const fixture = \"gh" + "p_" + strings.Repeat("A1b2", 5) + "\" // " + credentialAllowMarker

	if blocked, _ := writeAttempt(t, root, "fixture_test.ts", content); blocked != nil {
		t.Fatalf("the documented opt-out did not work: %s", blocked.Reason)
	}
}

func TestCredentialRefusalDoesNotDependOnAnActiveRun(t *testing.T) {
	// stateWriteVerdict deliberately stands down with no active run. This one
	// must not: a key entering source is the same event either way, and the
	// common case is a repository using the toolkit without starting a run.
	root := t.TempDir()
	if runs := activeRuns(root); len(runs) != 0 {
		t.Fatalf("expected a workspace with no run, got %d", len(runs))
	}

	content := "-----BEGIN EC PRIVATE" + " KEY-----"
	if blocked, _ := writeAttempt(t, root, "key.pem", content); blocked == nil {
		t.Fatal("a credential was allowed through because no run was active")
	}
}

func TestACredentialInAHeredocIsSeen(t *testing.T) {
	// A shell redirect writes a file with no tool_input.content at all, so the
	// command itself has to be part of what the gate reads.
	root := t.TempDir()
	command := "cat > id_rsa <<EOF\n-----BEGIN OPENSSH PRIVATE" + " KEY-----\nEOF"

	if blocked, _ := attempt(t, root, command); blocked == nil {
		t.Fatal("a credential written by shell redirect slipped past the gate")
	}
}

func TestCursorGetsADenyForACredentialToo(t *testing.T) {
	root := t.TempDir()
	body, err := json.Marshal(map[string]any{
		"tool_name":  "Write",
		"tool_input": map[string]string{"file_path": "k.ts", "content": "-----BEGIN DSA PRIVATE" + " KEY-----"},
	})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var out bytes.Buffer
	if err := Run(Request{
		Event: EventPreToolUse, Client: ClientCursor, WorkspaceRoot: root,
		ToolkitRoot: toolkitRoot, Stdin: bytes.NewReader(body),
	}, &out); err != nil {
		t.Fatalf("Cursor path returned an error instead of a decision: %v", err)
	}

	var decision map[string]any
	if err := json.Unmarshal(out.Bytes(), &decision); err != nil {
		t.Fatalf("Cursor decision is not JSON: %v (%q)", err, out.String())
	}
	if decision["permission"] != "deny" {
		t.Fatalf("expected deny, got %v", decision["permission"])
	}
}
