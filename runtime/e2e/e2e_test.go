// Package e2e drives the built binary against a fixture consumer repository.
//
// Everything else in this module tests a package. This is the only test that
// compiles and runs the program, which is what proves the pieces work together.
package e2e_test

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// This exercises the consumer-repo shape from AGENTS.md: a workspace that keeps
// the toolkit at a separate path and holds only its own code, docs, and state.
//
// It is the only test that runs the built binary. Everything else tests
// packages; this proves the pieces work together as a program.

// moduleRoot is the runtime module, one level up from this package.
func moduleRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatalf("resolve module root: %v", err)
	}
	return root
}

func toolkitRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("resolve toolkit root: %v", err)
	}
	return root
}

func buildBinary(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not on PATH")
	}
	name := "vibe-agent"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	binary := filepath.Join(t.TempDir(), name)

	build := exec.Command("go", "build", "-o", binary, "./cmd")
	build.Dir = moduleRoot(t)
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v: %s", err, out)
	}
	return binary
}

// consumerRepo builds a workspace that has its own rules and no .ai-agents of
// its own, matching a repo that mounts this toolkit as a submodule.
func consumerRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write(t, filepath.Join(root, "AGENTS.md"), "# Consumer rules\n\nDomain constraints live here.\n")
	write(t, filepath.Join(root, ".gitignore"), "/tmp/\n/.agent-state/\n")
	write(t, filepath.Join(root, "src", "main.go"), "package main\n\nfunc main() {}\n")
	return root
}

type cli struct {
	t       *testing.T
	binary  string
	root    string
	toolkit string
}

func (c cli) run(args ...string) (string, error) {
	c.t.Helper()
	full := append(args, "--workspace", c.root, "--toolkit", c.toolkit)
	cmd := exec.Command(c.binary, full...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// hook feeds a host payload on stdin and returns the exit status, because for
// a hook the status is the behavior: only 2 blocks.
func (c cli) hook(stdin string, args ...string) (string, int) {
	c.t.Helper()
	full := append(args, "--workspace", c.root, "--toolkit", c.toolkit)
	cmd := exec.Command(c.binary, full...)
	cmd.Stdin = strings.NewReader(stdin)
	out, err := cmd.CombinedOutput()

	var exit *exec.ExitError
	switch {
	case err == nil:
		return string(out), 0
	case errors.As(err, &exit):
		return string(out), exit.ExitCode()
	default:
		c.t.Fatalf("hook %v: %v", args, err)
		return "", -1
	}
}

func (c cli) mustRun(args ...string) string {
	c.t.Helper()
	out, err := c.run(args...)
	if err != nil {
		c.t.Fatalf("%v: %v\n%s", args, err, out)
	}
	return out
}

func TestConsumerRepoRunsAGoalToCompletion(t *testing.T) {
	root := consumerRepo(t)
	run := cli{t, buildBinary(t), root, toolkitRoot(t)}

	if out := run.mustRun("run", "start", "--slug", "webhook-idempotency",
		"--goal", "make webhook delivery idempotent"); !strings.Contains(out, "node     intake") {
		t.Fatalf("run did not start at intake:\n%s", out)
	}

	manifest := filepath.Join(root, "tmp", "webhook-idempotency", "manifest.json")
	if _, err := os.Stat(manifest); err != nil {
		t.Fatalf("state was not written into the consumer workspace: %v", err)
	}

	// Walk the delivery loop on evidence alone, exactly as a host would.
	steps := []struct {
		check  string
		source string
		flag   string
		want   string
	}{
		{"intake_confirmed", "human_event", "--passed", "spec"},
		{"", "", "", "approve_spec"},
		{"spec_approved", "human_event", "--passed", "plan"},
		{"", "", "", "approve_plan"},
		{"plan_approved", "human_event", "--passed", "build"},
		{"", "", "", "test"},
		{"unit", "exit_code", "--passed", "e2e"},
		{"e2e", "file_assert", "--skipped", "review"},
		{"", "", "", "open_pr"},
		{"pr_open", "ci_api", "--passed", "pr_checks"},
		{"ci", "ci_api", "--passed", "external_reviews"},
		{"reviews", "ci_api", "--passed", "ship"},
		{"ship", "exit_code", "--passed", "approve_merge"},
		{"merge_approved", "human_event", "--passed", "task_complete"},
		{"tasks_remaining", "file_assert", "--failed", "done"},
	}

	for i, step := range steps {
		args := []string{"checkpoint", "--slug", "webhook-idempotency"}
		if step.check != "" {
			args = append(args, "--check", step.check, "--source", step.source, step.flag)
		}
		out := run.mustRun(args...)
		if !strings.Contains(out, "-> "+step.want) {
			t.Fatalf("step %d did not reach %q:\n%s", i, step.want, out)
		}

		// The merge gate must open exactly here: after approve_merge recorded
		// the human decision, and while the run is still active. Asserting it
		// after the walk would prove nothing, since a finished run is not gated
		// for an entirely different reason.
		if step.check == "merge_approved" {
			push := `{"tool_name":"Bash","tool_input":{"command":"git push origin main"}}`
			if out, code := run.hook(push, "hook", "pre-tool-use"); code != 0 {
				t.Errorf("the merge gate stayed shut at approve_merge, exit %d:\n%s", code, out)
			}
		}
	}

	status := run.mustRun("run", "status", "--slug", "webhook-idempotency", "--json")
	var final map[string]any
	if err := json.Unmarshal([]byte(status), &final); err != nil {
		t.Fatalf("status is not JSON: %v\n%s", err, status)
	}
	if final["status"] != "done" {
		t.Errorf("final status = %v, want done", final["status"])
	}

	// A skipped check must remain distinguishable from a passed one.
	checks := final["checks"].(map[string]any)
	e2e := checks["e2e"].(map[string]any)
	if e2e["passed"] == true {
		t.Error("a skipped e2e check was recorded as passed")
	}

	events := filepath.Join(root, "tmp", "webhook-idempotency", "events.ndjson")
	raw, err := os.ReadFile(events)
	if err != nil {
		t.Fatalf("event log missing: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != len(steps)+1 {
		t.Errorf("event log has %d lines, want %d (one start plus one per transition)", len(lines), len(steps)+1)
	}
}

// Exit 2 is what actually blocks a host. A unit test can prove the verdict but
// not the status the host reads, so this is the only place the gate is really
// tested.
func TestPushToMainExitsTwoUntilTheMergeIsApproved(t *testing.T) {
	root := consumerRepo(t)
	run := cli{t, buildBinary(t), root, toolkitRoot(t)}
	run.mustRun("run", "start", "--slug", "gated", "--goal", "prove the gate holds across the process boundary")

	push := `{"tool_name":"Bash","tool_input":{"command":"git push origin main"}}`
	out, code := run.hook(push, "hook", "pre-tool-use")
	if code != 2 {
		t.Fatalf("a push to main exited %d, want 2, the only status a host treats as a block:\n%s", code, out)
	}
	if !strings.Contains(out, "merge_approved") {
		t.Errorf("the refusal does not name the missing evidence:\n%s", out)
	}

	branch := `{"tool_name":"Bash","tool_input":{"command":"git push -u origin feature/x"}}`
	if out, code := run.hook(branch, "hook", "pre-tool-use"); code != 0 {
		t.Errorf("a task branch push exited %d, which would wedge the loop this protects:\n%s", code, out)
	}

	// There is no shortcut to the evidence: a checkpoint may only write the
	// check its current node declares, so the gate cannot be opened out of turn.
	// Where it does open is asserted mid-walk in the full delivery test.
	if _, err := run.run("checkpoint", "--slug", "gated",
		"--check", "merge_approved", "--source", "human_event", "--passed"); err == nil {
		t.Error("merge_approved was recorded from intake, which would open the gate out of turn")
	}
}

// Evidence provenance has to hold at the process boundary, not only in Go.
func TestCheckpointRefusesAModelAssertedCheckFromTheCLI(t *testing.T) {
	root := consumerRepo(t)
	run := cli{t, buildBinary(t), root, toolkitRoot(t)}
	run.mustRun("run", "start", "--slug", "demo", "--goal", "g")

	out, err := run.run("checkpoint", "--slug", "demo",
		"--check", "intake_confirmed", "--source", "model", "--passed")
	if err == nil {
		t.Fatalf("the CLI accepted a model-asserted check:\n%s", out)
	}
	if !strings.Contains(out, "exit_code") {
		t.Errorf("the error does not name the allowed sources:\n%s", out)
	}
}

func TestCheckpointRefusesEvidenceWithoutProvenance(t *testing.T) {
	root := consumerRepo(t)
	run := cli{t, buildBinary(t), root, toolkitRoot(t)}
	run.mustRun("run", "start", "--slug", "demo", "--goal", "g")

	if out, err := run.run("checkpoint", "--slug", "demo",
		"--check", "intake_confirmed", "--passed"); err == nil {
		t.Fatalf("a check without --source was accepted:\n%s", out)
	}
}

// A fresh process must resume from the manifest alone.
func TestRunResumesAcrossProcesses(t *testing.T) {
	root := consumerRepo(t)
	run := cli{t, buildBinary(t), root, toolkitRoot(t)}

	run.mustRun("run", "start", "--slug", "resume-me", "--goal", "prove resume works")
	run.mustRun("checkpoint", "--slug", "resume-me",
		"--check", "intake_confirmed", "--source", "human_event", "--passed")

	out := run.mustRun("run", "status", "--slug", "resume-me")
	if !strings.Contains(out, "node       spec") {
		t.Errorf("a separate process did not resume at spec:\n%s", out)
	}
}

func TestDoctorPassesOnAWiredConsumerRepo(t *testing.T) {
	root := consumerRepo(t)
	run := cli{t, buildBinary(t), root, toolkitRoot(t)}
	out := run.mustRun("doctor")
	for _, want := range []string{"workflow graphs load and validate", "memory database opens", "doctor: OK"} {
		if !strings.Contains(out, want) {
			t.Errorf("doctor output missing %q:\n%s", want, out)
		}
	}
}

// The memory database follows --workspace, not --toolkit. Deriving it from the
// toolkit would pool every consumer repo's memories into one shared database.
//
// The check is that two different workspaces sharing one toolkit get two
// separate databases. Asserting the toolkit has none would be wrong: used
// standalone, the toolkit is its own workspace and legitimately has one.
func TestEachWorkspaceGetsItsOwnMemoryDatabase(t *testing.T) {
	binary := buildBinary(t)
	toolkit := toolkitRoot(t)

	first := consumerRepo(t)
	second := consumerRepo(t)
	cli{t, binary, first, toolkit}.mustRun("doctor")
	cli{t, binary, second, toolkit}.mustRun("doctor")

	for _, root := range []string{first, second} {
		if _, err := os.Stat(filepath.Join(root, ".agent-state", "memory.db")); err != nil {
			t.Errorf("workspace %s has no memory database of its own: %v", root, err)
		}
	}
	if first == second {
		t.Fatal("the two workspaces are the same directory, so this proves nothing")
	}
}

// Host hook configs pass only --workspace, because a config that has to know
// where the toolkit sits would differ per consumer repo. So the binary must
// find .ai-agents itself when the toolkit is a submodule.
func TestToolkitIsFoundWhenMountedAsASubmodule(t *testing.T) {
	root := consumerRepo(t)
	binary := buildBinary(t)

	// Mirror the documented layout: toolkit under .vibe-agent, no .ai-agents at
	// the workspace root.
	mount := filepath.Join(root, ".vibe-agent")
	if err := os.MkdirAll(mount, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	copyTree(t, filepath.Join(toolkitRoot(t), ".ai-agents"), filepath.Join(mount, ".ai-agents"))

	// No --toolkit anywhere below.
	noToolkit := func(args ...string) (string, error) {
		full := append(args, "--workspace", root)
		out, err := exec.Command(binary, full...).CombinedOutput()
		return string(out), err
	}

	if out, err := noToolkit("run", "start", "--slug", "demo", "--goal", "prove discovery"); err != nil {
		t.Fatalf("run start could not find the toolkit: %v\n%s", err, out)
	}

	out, err := noToolkit("run", "status", "--slug", "demo")
	if err != nil {
		t.Fatalf("run status: %v\n%s", err, out)
	}
	// The node description only appears when the graph actually loaded.
	if !strings.Contains(out, "next       [human_gate]") {
		t.Errorf("the graph was not loaded, so the node description is missing:\n%s", out)
	}
}

func copyTree(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		t.Fatalf("copy %s: %v", src, err)
	}
}

func TestGraphValidateRunsAgainstTheShippedGraph(t *testing.T) {
	run := cli{t, buildBinary(t), t.TempDir(), toolkitRoot(t)}
	cmd := exec.Command(run.binary, "graph", "validate", "--toolkit", run.toolkit)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("graph validate failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "ok goal-delivery") {
		t.Errorf("goal-delivery did not validate:\n%s", out)
	}
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}
