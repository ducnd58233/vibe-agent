// Package hostrunner runs an agent node by spawning a host CLI in print mode.
//
// This is the implementation that existed before the port did, in two copies:
// the web composer spawned a host to answer a chat message, and the routing
// eval spawned one to answer a fixture. Both built argv, chose between stdin
// and an argument, captured stderr, and read stdout. The differences between
// them were presentation, not behavior.
package hostrunner

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/agent/domain"
	"github.com/ducnd58233/vibe-agent/runtime/internal/safexec"
)

// Spec is a command that takes a prompt and prints an answer.
type Spec struct {
	// Binary is the executable name, resolved on PATH by safexec so a planted
	// binary in the working directory cannot shadow it.
	Binary string
	// Args is everything after the binary. The caller builds it, because the
	// flags a host accepts are the caller's business, not this package's.
	Args []string
	// PromptAsArg sends the prompt as the last argument instead of on stdin.
	// Two of the four hosts want it that way.
	PromptAsArg bool
	// Timeout bounds one call. Zero means no bound, which is what the composer
	// wants: host response time varies by model and load, and a fixed timeout
	// there turns a slow answer into a failed one.
	Timeout time.Duration
}

// Runner spawns one host CLI.
type Runner struct {
	Spec Spec
}

// New builds a runner for a spec.
func New(spec Spec) Runner { return Runner{Spec: spec} }

// FromCommand builds a spec from a whole command line, as the eval runner
// presets are written.
func FromCommand(command string, promptAsArg bool, timeout time.Duration) (Spec, error) {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return Spec{}, fmt.Errorf("empty runner command")
	}
	return Spec{
		Binary:      parts[0],
		Args:        append([]string{}, parts[1:]...),
		PromptAsArg: promptAsArg,
		Timeout:     timeout,
	}, nil
}

func (r Runner) Name() string { return r.Spec.Binary }

// ExecError carries what the host printed on stderr alongside the failure.
//
// It exists because exec.Cmd forces a choice: Output fills ExitError.Stderr
// only while Cmd.Stderr is nil, so capturing a successful call's warnings
// silently empties the field a failing call's diagnostics arrive in. One caller
// wanted each. Carrying the text explicitly gives both callers both, and means
// neither has to know which of the two mechanisms filled it.
type ExecError struct {
	Err    error
	Stderr string
}

func (e *ExecError) Error() string {
	if strings.TrimSpace(e.Stderr) == "" {
		return e.Err.Error()
	}
	return e.Err.Error() + ": " + strings.TrimSpace(e.Stderr)
}

func (e *ExecError) Unwrap() error { return e.Err }

// Run spawns the host and returns what it printed.
//
// Stdout and stderr are kept apart. A host that prints a settings warning on
// every call would otherwise have that warning parsed as part of the answer.
func (r Runner) Run(ctx context.Context, req domain.Request) (domain.Response, error) {
	if r.Spec.Binary == "" {
		return domain.Response{}, fmt.Errorf("runner has no binary")
	}
	if r.Spec.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, r.Spec.Timeout)
		defer cancel()
	}

	args := append([]string{}, r.Spec.Args...)
	if r.Spec.PromptAsArg {
		args = append(args, req.Prompt)
	}
	cmd, err := safexec.CommandContext(ctx, r.Spec.Binary, args...)
	if err != nil {
		return domain.Response{}, err
	}
	if !r.Spec.PromptAsArg {
		cmd.Stdin = strings.NewReader(req.Prompt)
	}
	var complaints strings.Builder
	cmd.Stderr = &complaints

	out, runErr := cmd.Output()
	response := domain.Response{Text: string(out), Stderr: complaints.String()}
	if runErr != nil {
		return response, &ExecError{Err: runErr, Stderr: complaints.String()}
	}
	return response, nil
}
