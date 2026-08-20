package hostrunner

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/agent/domain"
)

func TestFromCommandSplitsACommandLine(t *testing.T) {
	spec, err := FromCommand("claude -p --output-format stream-json", false, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if spec.Binary != "claude" {
		t.Errorf("binary = %q, want claude", spec.Binary)
	}
	if strings.Join(spec.Args, " ") != "-p --output-format stream-json" {
		t.Errorf("args = %v", spec.Args)
	}
}

func TestFromCommandRefusesAnEmptyCommand(t *testing.T) {
	if _, err := FromCommand("   ", false, 0); err == nil {
		t.Fatal("an empty command produced a runnable spec")
	}
}

func TestARunnerWithNoBinaryIsRefused(t *testing.T) {
	_, err := New(Spec{}).Run(context.Background(), domain.Request{Prompt: "hello"})
	if err == nil {
		t.Fatal("a runner with no binary ran")
	}
}

// exec.Cmd fills ExitError.Stderr only while Cmd.Stderr is nil, and this runner
// sets it. Carrying the text explicitly is what stops a failing host's
// diagnostics from disappearing when a successful host's warnings were captured.
func TestAFailureCarriesTheStderrItPrinted(t *testing.T) {
	// go is on PATH wherever this suite runs, and an unknown subcommand is a
	// deterministic non-zero exit with something on stderr.
	spec, err := FromCommand("go definitely-not-a-subcommand", false, 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	response, runErr := New(spec).Run(context.Background(), domain.Request{Prompt: ""})
	if runErr == nil {
		t.Skip("go accepted an unknown subcommand; nothing to assert")
	}

	var execErr *ExecError
	if !errors.As(runErr, &execErr) {
		t.Fatalf("error = %T, want an ExecError carrying stderr", runErr)
	}
	if strings.TrimSpace(execErr.Stderr) == "" {
		t.Error("the failure carried no stderr")
	}
	if strings.TrimSpace(response.Stderr) == "" {
		t.Error("the response carried no stderr")
	}
	if !strings.Contains(execErr.Error(), "definitely-not-a-subcommand") {
		t.Errorf("error text does not say what went wrong: %s", execErr.Error())
	}
}

// A runner is a port, so the compiler should be the one enforcing that.
func TestAHostRunnerSatisfiesThePort(t *testing.T) {
	var runner domain.Runner = New(Spec{Binary: "go"})
	if runner.Name() != "go" {
		t.Errorf("name = %q, want go", runner.Name())
	}
}
