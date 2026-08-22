// Package e2e drives the built binary against a fixture consumer repository.
//
// Everything else in this module tests a package. This is the only test that
// compiles and runs the program, which is what proves the pieces work together.
package e2e_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
	"github.com/ducnd58233/vibe-agent/runtime/internal/safexec"
	"github.com/ducnd58233/vibe-agent/runtime/internal/session"
	"github.com/ducnd58233/vibe-agent/runtime/internal/testutil"
	"github.com/ducnd58233/vibe-agent/runtime/internal/web/domain"
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

// binaryDir holds the one binary every test in this package shares. Set by
// TestMain so it can be removed when the suite ends; a t.TempDir() would be
// gone before the next test asked for it.
var binaryDir string

// sharedBinary builds the program once for the whole package.
//
// Every test needs the same bytes, and there are twenty-five call sites. Each
// used to link its own copy into its own t.TempDir() at roughly eight seconds,
// which is most of what the suite spent. sync.OnceValues is the idiom this
// module already uses for the danger and suppression plans; here it wraps a
// process that genuinely should run once.
//
// The error is returned rather than raised, so the test that asked is the one
// that fails, with the build output attached.
var sharedBinary = sync.OnceValues(func() (string, error) {
	if _, err := safexec.LookPath("go"); err != nil {
		return "", errNoToolchain
	}
	name := "vibe-agent"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	binary := filepath.Join(binaryDir, name)

	root, err := filepath.Abs("..")
	if err != nil {
		return "", fmt.Errorf("resolve module root: %w", err)
	}
	build, err := safexec.CommandContext(context.Background(), "go", "build", "-o", binary, "./cmd")
	if err != nil {
		return "", fmt.Errorf("build command: %w", err)
	}
	build.Dir = root
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, buildErr := build.CombinedOutput(); buildErr != nil {
		return "", fmt.Errorf("build: %w: %s", buildErr, out)
	}
	return binary, nil
})

// errNoToolchain is a skip rather than a failure: this suite is the only one
// that compiles the program, and a machine without Go can still run the rest.
var errNoToolchain = errors.New("go toolchain not on PATH")

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "vibe-e2e-")
	if err != nil {
		fmt.Fprintf(os.Stderr, "e2e: temp dir: %v\n", err)
		os.Exit(1)
	}
	binaryDir = dir
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

func buildBinary(t *testing.T) string {
	t.Helper()
	binary, err := sharedBinary()
	if errors.Is(err, errNoToolchain) {
		t.Skip(err.Error())
	}
	if err != nil {
		t.Fatalf("%v", err)
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
	write(t, filepath.Join(root, "vibe-checks.yaml"), passingPlan)
	return root
}

// passingPlan declares every verifier node's check as a command that exits 0.
//
// `go version` is the portable stand-in: the suite already skips without a Go
// toolchain, so it is present wherever these tests run, and it needs no network
// or fixture. tasks_remaining is the one check with no runtime verifier, so the
// plan says so out loud rather than letting a caller decide per invocation.
const passingPlan = `apiVersion: vibe-agent/v1
kind: CheckPlan
metadata:
  description: Fixture plan for the delivery walk.
spec:
  checks:
    unit:
      command: go
      args: [version]
    e2e:
      command: go
      args: [version]
    slop:
      command: go
      args: [version]
    pr_open:
      command: go
      args: [version]
    ci:
      command: go
      args: [version]
    reviews:
      command: go
      args: [version]
    ship:
      command: go
      args: [version]
    tasks_remaining:
      verifier: human
      description: a person decides whether another in-scope task remains
`

type cli struct {
	t       *testing.T
	binary  string
	root    string
	toolkit string
}

func (c cli) run(args ...string) (string, error) {
	c.t.Helper()
	full := append(args, "--workspace", c.root, "--toolkit", c.toolkit)
	cmd, err := safexec.CommandContext(c.t.Context(), c.binary, full...)
	if err != nil {
		c.t.Fatalf("run command: %v", err)
	}
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// hook feeds a host payload on stdin and returns the exit status, because for
// a hook the status is the behavior: only 2 blocks.
func (c cli) hook(stdin string, args ...string) (string, int) {
	c.t.Helper()
	full := append(args, "--workspace", c.root, "--toolkit", c.toolkit)
	cmd, err := safexec.CommandContext(c.t.Context(), c.binary, full...)
	if err != nil {
		c.t.Fatalf("hook command: %v", err)
	}
	cmd.Stdin = strings.NewReader(stdin)
	out, err := cmd.CombinedOutput()

	var exit *safexec.ExitError
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

	current, err := state.Load(state.ManifestPath(root, "webhook-idempotency"))
	if err != nil {
		t.Fatalf("state was not written into the consumer workspace: %v", err)
	}
	if current.Date == "" || current.Version < 1 {
		t.Fatalf("new run missing date/version: date=%q version=%d", current.Date, current.Version)
	}
	manifest := filepath.Join(root, ".agent-state", "runs", current.Date, "webhook-idempotency",
		fmt.Sprintf("%d", current.Version), "manifest.json")
	if _, err := os.Stat(manifest); err != nil {
		t.Fatalf("versioned manifest missing at %s: %v", manifest, err)
	}

	// Walk the delivery loop on evidence alone, exactly as a host would.
	//
	// The `verify` steps are the ones a caller cannot fake. Every verifier node
	// now gets its check from a command the workspace declared in
	// vibe-checks.yaml, so this walk exercises the real path rather than typing
	// the conclusion at each gate.
	steps := []struct {
		verify bool   // produce the check by running the plan's command
		check  string // otherwise checkpoint this check by hand
		source string
		flag   string
		want   string
	}{
		{false, "intake_confirmed", "human_event", "--passed", "spec"},
		{false, "", "", "", "approve_spec"},
		{false, "spec_approved", "human_event", "--passed", "plan"},
		{false, "", "", "", "approve_plan"},
		{false, "plan_approved", "human_event", "--passed", "build"},
		{false, "", "", "", "test"},
		{true, "unit", "", "", "e2e"},
		{true, "e2e", "", "", "slop"},
		{true, "slop", "", "", "review"},
		{false, "", "", "", "open_pr"},
		{true, "pr_open", "", "", "pr_checks"},
		{true, "ci", "", "", "external_reviews"},
		{true, "reviews", "", "", "ship"},
		{true, "ship", "", "", "approve_merge"},
		{false, "merge_approved", "human_event", "--passed", "task_complete"},
		{false, "tasks_remaining", "human_event", "--failed", "done"},
	}

	for i, step := range steps {
		var args []string
		switch {
		case step.verify:
			args = []string{"verify", "--slug", "webhook-idempotency"}
		case step.check != "":
			args = []string{"checkpoint", "--slug", "webhook-idempotency",
				"--check", step.check, "--source", step.source, step.flag}
		default:
			args = []string{"checkpoint", "--slug", "webhook-idempotency"}
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

	// The e2e check must carry an exit code. That field is what distinguishes a
	// result a process produced from a conclusion someone typed: only the command
	// verifier writes it, and it is the reason this walk cannot be faked.
	checks := object(t, final["checks"], "checks")
	e2e := object(t, checks["e2e"], "checks.e2e")
	if _, recorded := e2e["exitCode"]; !recorded {
		t.Errorf("the e2e check has no exit code, so nothing ran to produce it: %v", e2e)
	}
	if e2e["source"] != "exit_code" {
		t.Errorf("e2e source = %v, want exit_code from the verifier", e2e["source"])
	}

	events := filepath.Join(root, ".agent-state", "runs", current.Date, "webhook-idempotency",
		fmt.Sprintf("%d", current.Version), "events.ndjson")
	raw, err := os.ReadFile(filepath.Clean(events))
	if err != nil {
		t.Fatalf("event log missing: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != len(steps)+1 {
		t.Errorf("event log has %d lines, want %d (one start plus one per transition)", len(lines), len(steps)+1)
	}
}

// walkTo drives a fresh run to a node by way of the gates in front of it. It
// stops before the node it names, so the caller decides what happens there.
func walkTo(t *testing.T, run cli, slug, node string) {
	t.Helper()
	steps := [][]string{
		{"--check", "intake_confirmed", "--source", "human_event", "--passed"},
		{},
		{"--check", "spec_approved", "--source", "human_event", "--passed"},
		{},
		{"--check", "plan_approved", "--source", "human_event", "--passed"},
		{},
	}
	for _, extra := range steps {
		out := run.mustRun(append([]string{"checkpoint", "--slug", slug}, extra...)...)
		if strings.Contains(out, "-> "+node) {
			return
		}
	}
	t.Fatalf("never reached %q", node)
}

// The reported bug, at the process boundary.
//
// An agent that ran a mobile app on an emulator, saw a white screen, and typed
// the passing checkpoint used to walk straight past the e2e gate. --source
// exit_code says what kind of evidence a check claims to be; it does not say a
// process ran. Now the only thing that can write a verifier node's check is a
// verifier, and the refusal has to name the command that would work.
func TestAVerifierNodeCannotBeWalkedByAssertion(t *testing.T) {
	root := consumerRepo(t)
	run := cli{t, buildBinary(t), root, toolkitRoot(t)}
	run.mustRun("run", "start", "--slug", "asserted", "--goal", "prove a typed pass is refused")
	walkTo(t, run, "asserted", "test")

	out, err := run.run("checkpoint", "--slug", "asserted",
		"--check", "unit", "--source", "exit_code", "--passed")
	if err == nil {
		t.Fatalf("a typed exit_code advanced a verifier node:\n%s", out)
	}
	if !strings.Contains(out, "verify") {
		t.Errorf("the refusal does not name the command that would work:\n%s", out)
	}

	// Skipping is the same hole wearing a different flag: the e2e_ok guard
	// accepts a skipped check, so a caller that can skip can bypass the gate.
	if out, err := run.run("checkpoint", "--slug", "asserted",
		"--check", "unit", "--source", "file_assert", "--skipped"); err == nil {
		t.Errorf("a hand-written skip advanced a verifier node:\n%s", out)
	}

	status := run.mustRun("run", "status", "--slug", "asserted")
	if !strings.Contains(status, "node       test") {
		t.Errorf("the run left the verifier node anyway:\n%s", status)
	}
}

// The other half: when the planned command really fails, the run must route back
// to build rather than stall or pass. This is the mobile case working correctly.
func TestAFailingPlannedCommandRoutesBackToBuild(t *testing.T) {
	root := consumerRepo(t)
	write(t, filepath.Join(root, "vibe-checks.yaml"), `apiVersion: vibe-agent/v1
kind: CheckPlan
spec:
  checks:
    unit:
      command: go
      args: [run, ./does-not-exist]
`)
	run := cli{t, buildBinary(t), root, toolkitRoot(t)}
	run.mustRun("run", "start", "--slug", "failing", "--goal", "prove a real failure routes back")
	walkTo(t, run, "failing", "test")

	// verify exits 0 on a failing check on purpose: the loop handled the failure,
	// which is not a broken tool call.
	out := run.mustRun("verify", "--slug", "failing")
	if !strings.Contains(out, "-> build") {
		t.Fatalf("a failing check did not route back to build:\n%s", out)
	}
	if !strings.Contains(out, "unit fail") {
		t.Errorf("the failure was not reported as one:\n%s", out)
	}

	status := run.mustRun("run", "status", "--slug", "failing", "--json")
	var final map[string]any
	if err := json.Unmarshal([]byte(status), &final); err != nil {
		t.Fatalf("status is not JSON: %v\n%s", err, status)
	}
	unit := object(t, object(t, final["checks"], "checks")["unit"], "checks.unit")
	if unit["passed"] == true {
		t.Error("a command that exited non-zero was recorded as passed")
	}
	if unit["exitCode"] == nil {
		t.Error("the failing check carries no exit code")
	}
}

// A workspace with no e2e suite must not stall at the e2e gate, and must not
// have it recorded as a pass either. The plan omitting the check is the tracked
// statement that there is none, and the manifest has to say skipped.
func TestAPlanThatOmitsACheckSkipsItVisibly(t *testing.T) {
	root := consumerRepo(t)
	write(t, filepath.Join(root, "vibe-checks.yaml"), `apiVersion: vibe-agent/v1
kind: CheckPlan
spec:
  checks:
    unit:
      command: go
      args: [version]
`)
	run := cli{t, buildBinary(t), root, toolkitRoot(t)}
	run.mustRun("run", "start", "--slug", "no-e2e", "--goal", "prove an omitted check skips visibly")
	walkTo(t, run, "no-e2e", "test")
	run.mustRun("verify", "--slug", "no-e2e") // test -> e2e

	out := run.mustRun("verify", "--slug", "no-e2e")
	if !strings.Contains(out, "e2e skipped") {
		t.Fatalf("an omitted check was not reported as skipped:\n%s", out)
	}
	if !strings.Contains(out, "vibe-checks.yaml") {
		t.Errorf("the skip does not say what caused it:\n%s", out)
	}
	if !strings.Contains(out, "-> slop") {
		t.Errorf("the run did not proceed past the gate:\n%s", out)
	}

	// slop is the same property one node later, and worth asserting rather than
	// walking past. It was added to a graph every workspace shares, so the
	// question it raises is whether a workspace that has not declared it stalls.
	// It must skip for the same reason and by the same route.
	out = run.mustRun("verify", "--slug", "no-e2e")
	if !strings.Contains(out, "slop skipped") {
		t.Fatalf("an omitted slop check did not skip:\n%s", out)
	}
	if !strings.Contains(out, "-> review") {
		t.Errorf("the run did not proceed past the slop gate:\n%s", out)
	}

	status := run.mustRun("run", "status", "--slug", "no-e2e", "--json")
	var final map[string]any
	if err := json.Unmarshal([]byte(status), &final); err != nil {
		t.Fatalf("status is not JSON: %v", err)
	}
	e2e := object(t, object(t, final["checks"], "checks")["e2e"], "checks.e2e")
	if e2e["passed"] == true {
		t.Error("a skipped e2e check was recorded as passed")
	}
	if e2e["skipped"] != true {
		t.Errorf("the skip marker is missing, so a reader cannot tell nothing ran: %v", e2e)
	}
}

// Flags were readable from the day the runner existed and nothing wrote one, so
// every flag-sourced guard was permanently false. In the delivery graph that made
// the research node unreachable: no run could ever take that edge. This is the
// writer, and the constraint that comes with it.
func TestAFlagOpensABranchThatWasUnreachable(t *testing.T) {
	root := consumerRepo(t)
	run := cli{t, buildBinary(t), root, toolkitRoot(t)}
	run.mustRun("run", "start", "--slug", "flagged", "--goal", "prove a flag routes the run")

	// A flag no guard reads would change nothing, so it is refused rather than
	// stored where nobody would notice it.
	if out, err := run.run("run", "flag", "--slug", "flagged", "--set", "not_a_guard"); err == nil {
		t.Errorf("a flag no guard reads was accepted:\n%s", out)
	}
	// A check-sourced guard is not settable either: that would be a way to write
	// evidence through the flag door.
	if out, err := run.run("run", "flag", "--slug", "flagged", "--set", "spec_approved"); err == nil {
		t.Errorf("a check-sourced guard was settable as a flag:\n%s", out)
	}

	run.mustRun("run", "flag", "--slug", "flagged", "--set", "research_required",
		"--note", "the spec needs outside sources")

	out := run.mustRun("checkpoint", "--slug", "flagged",
		"--check", "intake_confirmed", "--source", "human_event", "--passed")
	if !strings.Contains(out, "-> research") {
		t.Fatalf("the flag did not route the run to research:\n%s", out)
	}

	// Outside a human gate there is nobody being asked, so there is nobody whose
	// answer this would be recording.
	if out, err := run.run("run", "flag", "--slug", "flagged", "--clear", "research_required"); err == nil {
		t.Errorf("a flag was set while the run was at %q:\n%s", "research", out)
	}
}

// A skipped check must not open a gate that did not ask for it.
//
// The runner used to answer every check-sourced guard with
// `passed || skipped`, while the delivery graph documented e2e_ok as the only
// place the two were treated alike. They were treated alike everywhere, so a
// workspace whose plan omitted `unit` walked past the unit gate on the strength
// of the suite not having run. Only e2e_ok opts in now.
func TestASkippedCheckDoesNotOpenAGateThatDidNotOptIn(t *testing.T) {
	root := consumerRepo(t)
	write(t, filepath.Join(root, "vibe-checks.yaml"), `apiVersion: vibe-agent/v1
kind: CheckPlan
spec:
  checks:
    lint:
      command: go
      args: [version]
`)
	run := cli{t, buildBinary(t), root, toolkitRoot(t)}
	run.mustRun("run", "start", "--slug", "undeclared", "--goal", "prove a skip does not pass the unit gate")
	walkTo(t, run, "undeclared", "test")

	out := run.mustRun("verify", "--slug", "undeclared")
	if !strings.Contains(out, "unit skipped") {
		t.Fatalf("an undeclared check was not reported as skipped:\n%s", out)
	}
	if !strings.Contains(out, "-> build") {
		t.Fatalf("a skipped unit check passed its own gate:\n%s", out)
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
	for _, want := range []string{"workflow graphs load and validate", "doctor: OK"} {
		if !strings.Contains(out, want) {
			t.Errorf("doctor output missing %q:\n%s", want, out)
		}
	}
	// Either wording is a pass; a fresh consumer repo has stored nothing yet.
	// What must not happen is silence, because then nobody can tell whether the
	// store was checked.
	if !strings.Contains(out, "memory database") {
		t.Errorf("doctor says nothing about the memory store:\n%s", out)
	}

	// And it must not have made one. A diagnostic that seeds a database in the
	// directory it was run from breaks the rule the memory package states for
	// itself: reads never create state.
	if entries, err := os.ReadDir(filepath.Join(root, ".agent-state")); err == nil && len(entries) > 0 {
		t.Errorf("doctor created workspace state: %v", entries)
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

	// Drive the real write path rather than a diagnostic. doctor used to create
	// the database as a side effect of checking it, which made this test pass
	// without anything ever having been stored, and made running doctor in an
	// unrelated directory leave state behind.
	for _, root := range []string{first, second} {
		run := cli{t, binary, root, toolkit}
		run.mustRun("run", "start", "--slug", "isolation", "--goal", "prove workspaces do not share memory")
		run.hook(`{"tool_name":"Bash","tool_input":{"command":"go build ./..."},`+
			`"error":"Exit code 2\nundefined: Foo","is_interrupt":false}`,
			"hook", "post-tool-use-failure", "--client", "claude")
	}

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
	if err := os.MkdirAll(mount, 0o750); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	copyTree(t, filepath.Join(toolkitRoot(t), ".ai-agents"), filepath.Join(mount, ".ai-agents"))

	// No --toolkit anywhere below.
	noToolkit := func(args ...string) (string, error) {
		full := append(args, "--workspace", root)
		cmd, cmdErr := safexec.CommandContext(t.Context(), binary, full...)
		if cmdErr != nil {
			return "", cmdErr
		}
		out, err := cmd.CombinedOutput()
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
	if err := os.MkdirAll(dst, 0o750); err != nil {
		t.Fatalf("mkdir %s: %v", dst, err)
	}
	// os.Root confines every write to dst. Walking a tree and joining names onto
	// a destination is the shape a symlink escapes through: the check and the
	// write are separate steps, and a link swapped in between lands the copy
	// outside the directory it was supposed to fill.
	root, err := os.OpenRoot(dst)
	if err != nil {
		t.Fatalf("open %s: %v", dst, err)
	}
	defer func() { _ = root.Close() }()

	source, err := os.OpenRoot(src)
	if err != nil {
		t.Fatalf("open %s: %v", src, err)
	}
	defer func() { _ = source.Close() }()

	err = filepath.Walk(src, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, relErr := filepath.Rel(src, path)
		if relErr != nil {
			return relErr
		}
		if rel == "." {
			return nil
		}
		if info.IsDir() {
			return root.Mkdir(rel, 0o750)
		}
		original, openErr := source.Open(rel)
		if openErr != nil {
			return openErr
		}
		data, readErr := io.ReadAll(original)
		_ = original.Close()
		if readErr != nil {
			return readErr
		}
		file, createErr := root.Create(rel)
		if createErr != nil {
			return createErr
		}
		defer func() { _ = file.Close() }()
		_, writeErr := file.Write(data)
		return writeErr
	})
	if err != nil {
		t.Fatalf("copy %s: %v", src, err)
	}
}

func TestGraphValidateRunsAgainstTheShippedGraph(t *testing.T) {
	run := cli{t, buildBinary(t), t.TempDir(), toolkitRoot(t)}
	cmd, cmdErr := safexec.CommandContext(run.t.Context(), run.binary, "graph", "validate", "--toolkit", run.toolkit)
	if cmdErr != nil {
		t.Fatalf("graph validate command: %v", cmdErr)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("graph validate failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "ok goal-delivery") {
		t.Errorf("goal-delivery did not validate:\n%s", out)
	}
}

const e2eWebSecret = "sk-0123456789abcdef0123456789ab"

func TestWebServesFixtureTrajectory(t *testing.T) {
	root := consumerRepo(t)
	slug := "fixture-web"
	writeWebFixtureSession(t, root, slug)
	run := cli{t, buildBinary(t), root, toolkitRoot(t)}
	port := "13081"
	cmd, err := safexec.CommandContext(run.t.Context(), run.binary, "web", "--port", port, "--workspace", root, "--toolkit", run.toolkit)
	if err != nil {
		t.Fatalf("web command: %v", err)
	}
	cmd.Dir = moduleRoot(t)
	if err := cmd.Start(); err != nil {
		t.Fatalf("start web: %v", err)
	}
	defer func() {
		_ = cmd.Process.Kill()
	}()
	url := "http://127.0.0.1:" + port + "/session/" + slug
	var body string
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		req, reqErr := http.NewRequestWithContext(run.t.Context(), http.MethodGet, url, nil)
		if reqErr != nil {
			t.Fatalf("request: %v", reqErr)
		}
		resp, getErr := http.DefaultClient.Do(req)
		if getErr == nil && resp.StatusCode == http.StatusOK {
			raw, readErr := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if readErr == nil {
				body = string(raw)
				break
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	if body == "" {
		t.Fatal("web did not serve session page in time")
	}
	if !strings.Contains(body, `data-seq="2"`) || !strings.Contains(body, "user") {
		t.Fatalf("fixture seq/role missing:\n%s", body)
	}
	if strings.Contains(body, e2eWebSecret) {
		t.Fatal("fixture secret leaked into HTML")
	}
}

func TestWebComposerWorkspaceAndCatalog(t *testing.T) {
	root := consumerRepo(t)
	other := t.TempDir()
	write(t, filepath.Join(other, "AGENTS.md"), "# Other workspace\n\nMARKER_OTHER_WS\n")
	otherID := domain.NewRegistry(root, []string{other}).ID(other)
	run := cli{t, buildBinary(t), root, toolkitRoot(t)}
	port := "13082"
	cmd, err := safexec.CommandContext(run.t.Context(), run.binary, "web",
		"--port", port, "--workspace", root, "--workspaces", other, "--toolkit", run.toolkit)
	if err != nil {
		t.Fatalf("web command: %v", err)
	}
	cmd.Dir = moduleRoot(t)
	if err := cmd.Start(); err != nil {
		t.Fatalf("start web: %v", err)
	}
	defer func() { _ = cmd.Process.Kill() }()

	base := "http://127.0.0.1:" + port
	waitForHTTPContains(t, run.t.Context(), base+"/", `data-testid="workspace-list"`)
	waitForHTTPContains(t, run.t.Context(), base+"/catalog/commands?q=build", `data-testid="catalog-item"`)
	waitForHTTPAbsent(t, run.t.Context(), base+"/catalog/commands?q=zzzz-not-a-command-xyzzy", `data-testid="catalog-item"`)

	client := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	switchReq, err := http.NewRequestWithContext(run.t.Context(), http.MethodPost, base+"/workspace/switch",
		strings.NewReader("workspace_id="+otherID))
	if err != nil {
		t.Fatal(err)
	}
	switchReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	switchResp, err := client.Do(switchReq)
	if err != nil {
		t.Fatal(err)
	}
	_ = switchResp.Body.Close()
	if switchResp.StatusCode != http.StatusSeeOther {
		t.Fatalf("switch status = %d", switchResp.StatusCode)
	}
	shellReq, err := http.NewRequestWithContext(run.t.Context(), http.MethodGet, base+switchResp.Header.Get("Location"), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, cookie := range switchResp.Cookies() {
		shellReq.AddCookie(cookie)
	}
	shellResp, err := client.Do(shellReq)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(shellResp.Body)
	_ = shellResp.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	otherBase := filepath.Base(other)
	if !strings.Contains(string(raw), otherBase) {
		t.Fatalf("active workspace label missing after switch:\n%s", raw)
	}
	if !strings.Contains(string(raw), "is-active") {
		t.Fatalf("expected active workspace marker in shell:\n%s", raw)
	}
}

func waitForHTTPContains(t *testing.T, ctx context.Context, url, want string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err == nil && resp.StatusCode == http.StatusOK {
			body, readErr := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if readErr == nil && strings.Contains(string(body), want) {
				return
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for %q at %s", want, url)
}

func waitForHTTPAbsent(t *testing.T, ctx context.Context, url, absent string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err == nil && resp.StatusCode == http.StatusOK {
			body, readErr := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if readErr == nil && !strings.Contains(string(body), absent) {
				return
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for absence of %q at %s", absent, url)
}

func writeWebFixtureSession(t *testing.T, root, slug string) {
	t.Helper()
	stamp := time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC)
	entry, err := state.PrepareStart(root, slug, stamp)
	if err != nil {
		t.Fatal(err)
	}
	path := session.LogPath(root, slug)
	records := []session.Record{
		{Type: session.TypeSessionStart, Source: session.SourceHook, Client: "cursor", Event: "SessionStart"},
		{Type: session.TypePromptSubmit, Source: session.SourceHook, Client: "cursor", Body: "plan with key " + e2eWebSecret},
		{Type: session.TypeMessage, Source: session.SourceTranscript, Role: "assistant", Body: "ready"},
	}
	for _, rec := range records {
		rec.At = stamp
		if _, err := session.Append(path, rec); err != nil {
			t.Fatal(err)
		}
	}
	run, err := state.NewRun(slug, "fixture", "goal-delivery", 50, stamp)
	if err != nil {
		t.Fatal(err)
	}
	run.Date = entry.Date
	run.Version = entry.Version
	testutil.EnsureRunIndex(t, root, slug)
	if err := state.Save(state.ManifestPath(root, slug), run); err != nil {
		t.Fatal(err)
	}
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

// object narrows a decoded manifest, failing the test rather than
// panicking, so a shape change names itself instead of arriving as an
// interface conversion three frames down.
func object(t *testing.T, value any, what string) map[string]any {
	t.Helper()
	narrowed, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("%s is %T, want an object", what, value)
	}
	return narrowed
}
