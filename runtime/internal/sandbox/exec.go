package sandbox

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/safexec"
)

// ExecRequest is one command through a resolved runner.
type ExecRequest struct {
	WorkspaceRoot string
	Slug          string
	UseCase       string
	// Runner overrides useCases / defaultRunner when non-empty (check-plan runner:).
	Runner  string
	Command string
	Args    []string
	Timeout time.Duration
}

// ExecResult is the subprocess outcome without deciding pass/fail for a check.
type ExecResult struct {
	ExitCode   int
	Output     []byte
	RunnerName string
	Driver     string
	NeverRan   bool
	TimedOut   bool
	Err        error
}

// Up brings a use-case runner online and writes STATUS.md.
func Up(ctx context.Context, workspaceRoot, slug, useCase, explicitRunner string) error {
	cfg, found, err := Load(workspaceRoot)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("no %s; declare runners before sandbox up", Path(workspaceRoot))
	}
	name, spec, err := cfg.ResolveRunner(useCase, explicitRunner)
	if err != nil {
		return err
	}
	container := ""
	switch spec.Driver {
	case "local":
		// Host process; nothing to start.
	case "docker":
		if err := dockerAvailable(ctx); err != nil {
			_ = WriteStatus(workspaceRoot, slug, useCase, Status{
				State: StatusFailed, Runner: name,
			})
			return err
		}
	default:
		return fmt.Errorf("unsupported driver %q", spec.Driver)
	}
	return WriteStatus(workspaceRoot, slug, useCase, Status{
		State: StatusUp, Runner: name, Container: container,
	})
}

// Down marks a use-case runner offline. Local and ephemeral docker need no teardown.
func Down(workspaceRoot, slug, useCase string) error {
	st, found, err := ReadStatus(workspaceRoot, slug, useCase)
	if err != nil {
		return err
	}
	runner := ""
	if found {
		runner = st.Runner
	}
	return WriteStatus(workspaceRoot, slug, useCase, Status{
		State: StatusDown, Runner: runner, Container: "",
	})
}

// Exec runs a command on the runner for a use case. Ensures STATUS is up first.
func Exec(ctx context.Context, req ExecRequest) ExecResult {
	if req.Command == "" {
		return ExecResult{ExitCode: -1, NeverRan: true, Err: errors.New("sandbox exec needs a command")}
	}
	cfg, found, err := Load(req.WorkspaceRoot)
	if err != nil {
		return ExecResult{ExitCode: -1, NeverRan: true, Err: err}
	}
	if !found {
		return ExecResult{
			ExitCode: -1, NeverRan: true,
			Err: fmt.Errorf("%s is missing; declare runners before sandbox exec (fail closed when a check sets runner:)",
				Path(req.WorkspaceRoot)),
		}
	}
	name, spec, err := cfg.ResolveRunner(req.UseCase, req.Runner)
	if err != nil {
		return ExecResult{ExitCode: -1, NeverRan: true, Err: err}
	}

	if err := Up(ctx, req.WorkspaceRoot, req.Slug, req.UseCase, req.Runner); err != nil {
		return ExecResult{ExitCode: -1, NeverRan: true, RunnerName: name, Driver: spec.Driver, Err: err}
	}

	timeout := req.Timeout
	if timeout <= 0 {
		timeout = safexec.DefaultTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var result ExecResult
	result.RunnerName = name
	result.Driver = spec.Driver

	switch spec.Driver {
	case "local":
		result = runLocal(ctx, req)
	case "docker":
		result = runDocker(ctx, req, spec)
	default:
		return ExecResult{ExitCode: -1, NeverRan: true, RunnerName: name, Err: fmt.Errorf("unsupported driver %q", spec.Driver)}
	}
	result.RunnerName = name
	result.Driver = spec.Driver
	if result.TimedOut || (result.Err != nil && result.NeverRan) {
		_ = WriteStatus(req.WorkspaceRoot, req.Slug, req.UseCase, Status{
			State: StatusFailed, Runner: name,
		})
	}
	return result
}

func runLocal(ctx context.Context, req ExecRequest) ExecResult {
	captured := safexec.RunCaptured(ctx, req.WorkspaceRoot, req.Command, req.Args...)
	return ExecResult{
		ExitCode: captured.ExitCode,
		Output:   captured.Output,
		NeverRan: captured.NeverRan,
		TimedOut: captured.TimedOut,
		Err:      captured.Err,
	}
}

func runDocker(ctx context.Context, req ExecRequest, spec RunnerSpec) ExecResult {
	workdir := spec.Workdir
	if workdir == "" {
		workdir = "/workspace"
	}
	args := []string{
		"run", "--rm",
		"-v", req.WorkspaceRoot + ":" + workdir,
		"-w", workdir,
		spec.Image,
		req.Command,
	}
	args = append(args, req.Args...)
	captured := safexec.RunCaptured(ctx, "", "docker", args...)
	return ExecResult{
		ExitCode: captured.ExitCode,
		Output:   captured.Output,
		NeverRan: captured.NeverRan,
		TimedOut: captured.TimedOut,
		Err:      captured.Err,
	}
}

func dockerAvailable(ctx context.Context) error {
	cmd, err := safexec.CommandContext(ctx, "docker", "version", "--format", "{{.Server.Version}}")
	if err != nil {
		return fmt.Errorf("docker driver needs docker on PATH: %w", err)
	}
	out, runErr := cmd.CombinedOutput()
	if runErr != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = runErr.Error()
		}
		return fmt.Errorf("docker is not usable: %s", msg)
	}
	return nil
}

// ExampleConfig is a starter sandbox.yaml body for docs and tests.
func ExampleConfig() string {
	return `apiVersion: vibe-agent/v1
kind: SandboxConfig
spec:
  defaultRunner: local
  runners:
    local:
      driver: local
    docker:
      driver: docker
      image: vibe-agent/sandbox:local
      workdir: /workspace
  useCases:
    unit: local
    e2e: docker
`
}

// WriteExample writes ExampleConfig when the file is absent.
func WriteExample(workspaceRoot string) (string, error) {
	path := Path(workspaceRoot)
	if _, err := os.Stat(path); err == nil {
		return path, fmt.Errorf("%s already exists", path)
	} else if !os.IsNotExist(err) {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(ExampleConfig()), 0o600); err != nil {
		return "", err
	}
	return path, nil
}
