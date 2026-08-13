package verifier

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
)

// DefaultCommandTimeout bounds a verifier command that did not set its own.
const DefaultCommandTimeout = 30 * time.Minute

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

	timeout := req.Timeout
	if timeout <= 0 {
		timeout = DefaultCommandTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, req.Command, req.Args...)
	cmd.Dir = req.WorkspaceRoot

	var captured bytes.Buffer
	cmd.Stdout = &captured
	cmd.Stderr = &captured

	started := time.Now()
	runErr := cmd.Run()
	elapsed := time.Since(started)

	exitCode := cmd.ProcessState.ExitCode()
	timedOut := errors.Is(ctx.Err(), context.DeadlineExceeded)
	// A command that could not start has no ProcessState. That is the case when
	// the plan names a tool this machine does not have, which is common enough
	// to deserve saying plainly rather than reporting as an exit code.
	neverRan := cmd.ProcessState == nil

	// A process the runtime killed never produced a verdict. Reporting its exit
	// code as a failure would be a lie about what happened.
	if timedOut {
		exitCode = -1
	}

	logPath, writeErr := writeLog(req, captured.Bytes())
	if writeErr != nil {
		return Result{}, writeErr
	}

	result := Result{
		Check: state.Check{
			Passed:   runErr == nil && !timedOut,
			Source:   state.SourceExitCode,
			ExitCode: &exitCode,
			At:       time.Now().UTC(),
		},
		Detail:  captured.String(),
		LogPath: logPath,
	}
	if logPath != "" {
		result.Check.Ref = filepath.ToSlash(logPath)
	}

	commandLine := strings.TrimSpace(req.Command + " " + strings.Join(req.Args, " "))
	switch {
	case timedOut:
		result.Summary = fmt.Sprintf("%s timed out after %s", commandLine, timeout)
	case neverRan:
		// Not a failing check in the usual sense, but still not a passing one: the
		// check is unproven, and unproven has to read as not passed.
		result.Summary = fmt.Sprintf("%s could not start: %v", commandLine, runErr)
	case runErr != nil:
		result.Summary = fmt.Sprintf("%s exited %d after %s", commandLine, exitCode, elapsed.Round(time.Millisecond))
	default:
		result.Summary = fmt.Sprintf("%s exited 0 after %s", commandLine, elapsed.Round(time.Millisecond))
	}
	return result, nil
}

// writeLog stores captured output under tmp/<slug>/<logDir>/, matching the
// layout in goal-verification-records.md.
func writeLog(req Request, output []byte) (string, error) {
	if req.Slug == "" || req.LogDir == "" {
		return "", nil
	}
	dir := filepath.Join(state.RunDir(req.WorkspaceRoot, req.Slug), req.LogDir)
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
