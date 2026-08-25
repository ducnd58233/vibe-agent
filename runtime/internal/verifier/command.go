package verifier

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
	"github.com/ducnd58233/vibe-agent/runtime/internal/safexec"
	"github.com/ducnd58233/vibe-agent/runtime/internal/sandbox"
)

// DefaultCommandTimeout bounds a verifier command that did not set its own.
const DefaultCommandTimeout = safexec.DefaultTimeout

// Command runs a subprocess and reports its exit code.
//
// The exit code is the evidence. Output is captured for the record, never
// parsed for a verdict: grepping stdout for "PASS" would put the decision back
// in the hands of whatever wrote that text.
type Command struct{}

func (Command) Kind() string { return "command" }

func (c Command) Verify(ctx context.Context, req Request) (Result, error) {
	if req.Command == "" {
		return Result{}, errors.New("command verifier needs a command")
	}
	if req.Runner != "" {
		return c.verifyViaRunner(ctx, req)
	}
	return c.verifyHost(ctx, req)
}

func (c Command) verifyViaRunner(ctx context.Context, req Request) (Result, error) {
	timeout := req.Timeout
	if timeout <= 0 {
		timeout = DefaultCommandTimeout
	}
	started := time.Now()
	execResult := sandbox.Exec(ctx, sandbox.ExecRequest{
		WorkspaceRoot: req.WorkspaceRoot,
		Slug:          req.Slug,
		UseCase:       req.Check,
		Runner:        req.Runner,
		Command:       req.Command,
		Args:          req.Args,
		Timeout:       timeout,
	})
	elapsed := time.Since(started)

	logPath, writeErr := writeLog(req, execResult.Output)
	if writeErr != nil {
		return Result{}, writeErr
	}

	exitCode := execResult.ExitCode
	timedOut := execResult.TimedOut
	neverRan := execResult.NeverRan
	runErr := execResult.Err
	if timedOut {
		exitCode = -1
	}

	result := Result{
		Check: state.Check{
			Passed:   runErr == nil && !timedOut && !neverRan && exitCode == 0,
			Source:   state.SourceExitCode,
			ExitCode: &exitCode,
			At:       time.Now().UTC(),
		},
		Detail:  string(execResult.Output),
		LogPath: logPath,
	}
	if logPath != "" {
		result.Check.Ref = filepath.ToSlash(logPath)
	}

	commandLine := strings.TrimSpace(req.Command + " " + strings.Join(req.Args, " "))
	via := fmt.Sprintf("via runner %q", req.Runner)
	switch {
	case neverRan && runErr != nil:
		result.Summary = fmt.Sprintf("%s %s could not start: %v", commandLine, via, runErr)
		result.Check.Passed = false
	case timedOut:
		result.Summary = fmt.Sprintf("%s %s timed out after %s", commandLine, via, timeout)
	case neverRan:
		result.Summary = fmt.Sprintf("%s %s could not start", commandLine, via)
	case runErr != nil || exitCode != 0:
		result.Summary = fmt.Sprintf("%s %s exited %d after %s", commandLine, via, exitCode, elapsed.Round(time.Millisecond))
		result.Check.Passed = false
	default:
		result.Summary = fmt.Sprintf("%s %s exited 0 after %s", commandLine, via, elapsed.Round(time.Millisecond))
	}
	return result, nil
}

func (c Command) verifyHost(ctx context.Context, req Request) (Result, error) {
	timeout := req.Timeout
	if timeout <= 0 {
		timeout = DefaultCommandTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	started := time.Now()
	captured := safexec.RunCaptured(ctx, req.WorkspaceRoot, req.Command, req.Args...)
	elapsed := time.Since(started)

	logPath, writeErr := writeLog(req, captured.Output)
	if writeErr != nil {
		return Result{}, writeErr
	}

	exitCode := captured.ExitCode
	result := Result{
		Check: state.Check{
			Passed:   captured.Err == nil && !captured.TimedOut,
			Source:   state.SourceExitCode,
			ExitCode: &exitCode,
			At:       time.Now().UTC(),
		},
		Detail:  string(captured.Output),
		LogPath: logPath,
	}
	if logPath != "" {
		result.Check.Ref = filepath.ToSlash(logPath)
	}

	commandLine := strings.TrimSpace(req.Command + " " + strings.Join(req.Args, " "))
	switch {
	case captured.TimedOut:
		result.Summary = fmt.Sprintf("%s timed out after %s", commandLine, timeout)
	case captured.NeverRan:
		// Not a failing check in the usual sense, but still not a passing one: the
		// check is unproven, and unproven has to read as not passed.
		result.Summary = fmt.Sprintf("%s could not start: %v", commandLine, captured.Err)
	case captured.Err != nil:
		result.Summary = fmt.Sprintf("%s exited %d after %s", commandLine, exitCode, elapsed.Round(time.Millisecond))
	default:
		result.Summary = fmt.Sprintf("%s exited 0 after %s", commandLine, elapsed.Round(time.Millisecond))
	}
	return result, nil
}

// writeLog stores captured output under .agent-state/runs/.../<logDir>/.
func writeLog(req Request, output []byte) (string, error) {
	if req.Slug == "" || req.LogDir == "" {
		return "", nil
	}
	runDir := state.RunDir(req.WorkspaceRoot, req.Slug)
	if runDir == "" {
		return "", fmt.Errorf("no run directory for slug %q", req.Slug)
	}
	dir := filepath.Join(runDir, req.LogDir)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", fmt.Errorf("create log directory: %w", err)
	}
	name := req.Check
	if name == "" {
		name = "command"
	}
	path := filepath.Join(dir, name+".log")
	if err := os.WriteFile(path, output, 0o600); err != nil {
		return "", fmt.Errorf("write log: %w", err)
	}
	return relativeTo(req.WorkspaceRoot, path), nil
}
