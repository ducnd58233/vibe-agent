package safexec

import (
	"bytes"
	"context"
	"errors"
	"os/exec"

	"golang.org/x/sys/execabs"
)

var errOutputAlreadySet = errors.New("exec output already set")

// Cmd is an exec.Cmd with context cancellation wired at run time.
type Cmd struct {
	*exec.Cmd
	ctx context.Context
}

type ExitError = exec.ExitError

// CommandContext resolves name to an absolute executable before building the
// command. It keeps the path-security behavior of execabs without routing every
// dynamic verifier through exec.CommandContext.
func CommandContext(ctx context.Context, name string, args ...string) (*Cmd, error) {
	path, err := execabs.LookPath(name)
	if err != nil {
		return nil, err
	}
	cmd := exec.Cmd{
		Path: path,
		Args: append([]string{name}, args...),
	}
	return &Cmd{Cmd: &cmd, ctx: ctx}, nil
}

// Run starts the command and kills it if the context ends first.
func (c *Cmd) Run() error {
	stop := context.AfterFunc(c.ctx, func() { _ = c.kill() })
	defer stop()
	return c.Cmd.Run()
}

// CombinedOutput matches exec.Cmd.CombinedOutput with the same context kill.
func (c *Cmd) CombinedOutput() ([]byte, error) {
	if c.Stdout != nil {
		return nil, errOutputAlreadySet
	}
	if c.Stderr != nil {
		return nil, errOutputAlreadySet
	}
	var buffer bytes.Buffer
	c.Stdout = &buffer
	c.Stderr = &buffer
	err := c.Run()
	return buffer.Bytes(), err
}

// Output matches exec.Cmd.Output with the same context kill.
func (c *Cmd) Output() ([]byte, error) {
	if c.Stdout != nil {
		return nil, errOutputAlreadySet
	}
	var buffer bytes.Buffer
	c.Stdout = &buffer
	err := c.Run()
	return buffer.Bytes(), err
}

func (c *Cmd) kill() error {
	if c.Process == nil {
		return nil
	}
	return c.Process.Kill()
}

// LookPath exposes the same absolute-path lookup used by CommandContext.
func LookPath(name string) (string, error) {
	return execabs.LookPath(name)
}
