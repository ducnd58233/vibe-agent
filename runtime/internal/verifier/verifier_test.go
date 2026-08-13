package verifier

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
)

func shell(script string) (string, []string) {
	if runtime.GOOS == "windows" {
		return "cmd", []string{"/c", script}
	}
	return "sh", []string{"-c", script}
}

func TestCommandExitZeroPasses(t *testing.T) {
	name, args := shell("exit 0")
	result, err := Command{}.Verify(t.Context(), Request{
		Check: "unit", WorkspaceRoot: t.TempDir(), Command: name, Args: args,
	})
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if !result.Check.Passed {
		t.Errorf("exit 0 did not pass: %+v", result.Check)
	}
	if result.Check.Source != state.SourceExitCode {
		t.Errorf("source = %q, want exit_code", result.Check.Source)
	}
	if result.Check.ExitCode == nil || *result.Check.ExitCode != 0 {
		t.Errorf("exit code not recorded: %+v", result.Check.ExitCode)
	}
}

func TestCommandNonZeroFails(t *testing.T) {
	name, args := shell("exit 3")
	result, err := Command{}.Verify(t.Context(), Request{
		Check: "unit", WorkspaceRoot: t.TempDir(), Command: name, Args: args,
	})
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if result.Check.Passed {
		t.Error("exit 3 was recorded as a pass")
	}
	if result.Check.ExitCode == nil || *result.Check.ExitCode != 3 {
		t.Errorf("exit code = %v, want 3", result.Check.ExitCode)
	}
}

// A command that prints "PASS" and exits non-zero must still fail. The exit
// code is the evidence; output is only for the record.
func TestCommandIgnoresOutputWhenDecidingTheVerdict(t *testing.T) {
	name, args := shell("echo PASS && exit 1")
	result, err := Command{}.Verify(t.Context(), Request{
		Check: "unit", WorkspaceRoot: t.TempDir(), Command: name, Args: args,
	})
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if result.Check.Passed {
		t.Error("a command that printed PASS but exited 1 was recorded as a pass")
	}
}

func TestCommandCapturesOutputToTheEvidenceTree(t *testing.T) {
	root := t.TempDir()
	name, args := shell("echo hello from the verifier")
	result, err := Command{}.Verify(t.Context(), Request{
		Check: "unit", WorkspaceRoot: root, Slug: "demo", LogDir: "unit",
		Command: name, Args: args,
	})
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	expected := filepath.Join(root, "tmp", "demo", "unit", "unit.log")
	contents, err := os.ReadFile(filepath.Clean(expected))
	if err != nil {
		t.Fatalf("log not written where goal-verification-records.md says: %v", err)
	}
	if !strings.Contains(string(contents), "hello from the verifier") {
		t.Errorf("log does not contain the output: %q", contents)
	}
	if result.Check.Ref == "" {
		t.Error("check does not point at its log")
	}
}

// A killed process never produced a verdict. Reporting its exit code as an
// ordinary failure would misdescribe what happened.
func TestCommandTimeoutIsNotAnOrdinaryFailure(t *testing.T) {
	name, args := shell("sleep 5")
	if runtime.GOOS == "windows" {
		name, args = shell("ping -n 6 127.0.0.1 > nul")
	}
	result, err := Command{}.Verify(t.Context(), Request{
		Check: "slow", WorkspaceRoot: t.TempDir(),
		Command: name, Args: args, Timeout: 200 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if result.Check.Passed {
		t.Error("a timed-out command was recorded as a pass")
	}
	if !strings.Contains(result.Summary, "timed out") {
		t.Errorf("summary does not say it timed out: %q", result.Summary)
	}
}

func TestCommandNeedsACommand(t *testing.T) {
	if _, err := (Command{}).Verify(t.Context(), Request{}); err == nil {
		t.Error("Verify accepted an empty command")
	}
}

// A check plan that names a tool this machine does not have is a real and
// ordinary mistake, so it must fail closed rather than error out or pass. It
// also must not be described as an exit code: nothing exited.
func TestCommandThatCannotStartIsUnprovenNotPassed(t *testing.T) {
	result, err := Command{}.Verify(t.Context(), Request{
		Check: "absent", WorkspaceRoot: t.TempDir(),
		Command: "vibe-agent-no-such-tool-exists", Args: []string{"--version"},
	})
	if err != nil {
		t.Fatalf("a missing tool should be a failed check, not a Verify error: %v", err)
	}
	if result.Check.Passed {
		t.Error("a command that never ran was recorded as a pass")
	}
	if !strings.Contains(result.Summary, "could not start") {
		t.Errorf("summary describes it as having run: %q", result.Summary)
	}
}

func TestFilesPassesWhenEveryPathHasContent(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "docs", "SPEC.md"), "content")
	write(t, filepath.Join(root, "docs", "PLAN.md"), "content")

	result, err := Files{}.Verify(t.Context(), Request{
		WorkspaceRoot: root, Paths: []string{"docs/SPEC.md", "docs/PLAN.md"},
	})
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if !result.Check.Passed {
		t.Errorf("did not pass: %s", result.Summary)
	}
	if result.Check.Source != state.SourceFileAssert {
		t.Errorf("source = %q, want file_assert", result.Check.Source)
	}
}

// An artifact node that produced a zero-byte SPEC.md has not produced a spec.
func TestFilesRejectsAnEmptyFile(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "docs", "SPEC.md"), "")

	result, err := Files{}.Verify(t.Context(), Request{
		WorkspaceRoot: root, Paths: []string{"docs/SPEC.md"},
	})
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if result.Check.Passed {
		t.Error("an empty file was accepted as a produced artifact")
	}
	if !strings.Contains(result.Summary, "empty") {
		t.Errorf("summary does not mention empty: %q", result.Summary)
	}
}

func TestFilesRejectsAMissingPath(t *testing.T) {
	result, err := Files{}.Verify(t.Context(), Request{
		WorkspaceRoot: t.TempDir(), Paths: []string{"docs/SPEC.md"},
	})
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if result.Check.Passed {
		t.Error("a missing file passed")
	}
	if !strings.Contains(result.Summary, "missing") {
		t.Errorf("summary does not mention missing: %q", result.Summary)
	}
}

func TestFilesNeedsAtLeastOnePath(t *testing.T) {
	if _, err := (Files{}).Verify(t.Context(), Request{}); err == nil {
		t.Error("Verify accepted an empty path list")
	}
}

func TestGitReportsBranchAndCleanliness(t *testing.T) {
	root := initRepo(t)
	result, err := Git{}.Verify(t.Context(), Request{
		WorkspaceRoot: root, Expect: GitExpectation{CleanTree: true},
	})
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if !result.Check.Passed {
		t.Errorf("a clean repo failed the clean-tree check: %s", result.Summary)
	}
	if result.Check.Ref == "" {
		t.Error("head sha not recorded")
	}
}

func TestGitFailsADirtyTreeWhenCleanIsRequired(t *testing.T) {
	root := initRepo(t)
	write(t, filepath.Join(root, "untracked.txt"), "x")

	result, err := Git{}.Verify(t.Context(), Request{
		WorkspaceRoot: root, Expect: GitExpectation{CleanTree: true},
	})
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if result.Check.Passed {
		t.Error("a dirty tree passed the clean-tree check")
	}
}

// The delivery rule is that work never happens on the default branch.
func TestGitCanRequireNotBeingOnAGivenBranch(t *testing.T) {
	root := initRepo(t)
	observation, err := Observe(t.Context(), root)
	if err != nil {
		t.Fatalf("Observe: %v", err)
	}

	result, err := Git{}.Verify(t.Context(), Request{
		WorkspaceRoot: root, Expect: GitExpectation{NotBranch: observation.Branch},
	})
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if result.Check.Passed {
		t.Errorf("being on %q passed a NotBranch check for the same branch", observation.Branch)
	}
}

func TestSkippedIsNotPassed(t *testing.T) {
	result := Skipped("e2e not in scope", time.Now())
	if result.Check.Passed {
		t.Error("a skipped check reports as passed")
	}
	if !result.Check.Skipped {
		t.Error("a skipped check is not marked skipped")
	}
}

func TestRegistryKnowsTheThreeVerifiers(t *testing.T) {
	registry := Default()
	for _, kind := range []string{"command", "files", "git"} {
		verifier, err := registry.Get(kind)
		if err != nil {
			t.Errorf("Get(%q): %v", kind, err)
			continue
		}
		if verifier.Kind() != kind {
			t.Errorf("Get(%q) returned a %q verifier", kind, verifier.Kind())
		}
	}
	if _, err := registry.Get("wishful"); err == nil {
		t.Error("Get accepted an unknown verifier kind")
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

func initRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	root := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.CommandContext(t.Context(), "git", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, out)
		}
	}
	run("init")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "Test")
	write(t, filepath.Join(root, "README.md"), "hello")
	run("add", ".")
	run("commit", "-m", "initial")
	return root
}
