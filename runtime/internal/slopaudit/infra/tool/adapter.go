package tool

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ducnd58233/vibe-agent/runtime/internal/slopaudit/domain"
)

const (
	ToolAstGrep      = "ast-grep-tree-sitter"
	BinaryAstGrep    = "ast-grep"
	ToolSemgrep      = "semgrep"
	ToolSlopDetector = "slop-detector"

	ReasonNotInstalled = "tool is not installed"
	ReasonNoConfig     = "project config not found"
)

var semgrepConfigs = []string{".semgrep.yml", ".semgrep.yaml", "semgrep.yml", "semgrep.yaml"}

type Executor interface {
	LookPath(file string) (string, error)
	Run(ctx context.Context, name string, args ...string) (int, string, error)
}

type OSExecutor struct{}

func (OSExecutor) LookPath(file string) (string, error) {
	return exec.LookPath(file)
}

func (OSExecutor) Run(ctx context.Context, name string, args ...string) (int, string, error) {
	cmd, err := command(ctx, name, args...)
	if err != nil {
		return 1, "", err
	}
	output, err := cmd.CombinedOutput()
	if err == nil {
		return 0, string(output), nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode(), string(output), err
	}
	return 1, string(output), err
}

func command(ctx context.Context, name string, args ...string) (*exec.Cmd, error) {
	switch name {
	case BinaryAstGrep:
		return exec.CommandContext(ctx, BinaryAstGrep, args...), nil
	case ToolSemgrep:
		return exec.CommandContext(ctx, ToolSemgrep, args...), nil
	case ToolSlopDetector:
		return exec.CommandContext(ctx, ToolSlopDetector, args...), nil
	default:
		return nil, fmt.Errorf("unsupported adapter tool %q", name)
	}
}

type Adapter struct {
	name     string
	binary   string
	args     func(string) ([]string, string)
	executor Executor
}

func NewAdapters(executor Executor) []Adapter {
	if executor == nil {
		executor = OSExecutor{}
	}
	return []Adapter{
		{name: ToolAstGrep, binary: BinaryAstGrep, args: astGrepArgs, executor: executor},
		{name: ToolSemgrep, binary: ToolSemgrep, args: semgrepArgs, executor: executor},
		{name: ToolSlopDetector, binary: ToolSlopDetector, args: slopDetectorArgs, executor: executor},
	}
}

func (a Adapter) Run(ctx context.Context, target string) domain.AdapterResult {
	if _, err := a.executor.LookPath(a.binary); err != nil {
		return domain.AdapterResult{Name: a.name, Status: domain.AdapterSkipped, Reason: ReasonNotInstalled}
	}
	args, reason := a.args(target)
	if reason != "" {
		return domain.AdapterResult{Name: a.name, Status: domain.AdapterSkipped, Reason: reason}
	}
	command := append([]string{a.binary}, args...)
	exitCode, output, err := a.executor.Run(ctx, a.binary, args...)
	if err != nil {
		return domain.AdapterResult{Name: a.name, Status: domain.AdapterFailed, Command: command, Reason: summarize(output, err), ExitCode: exitCode}
	}
	return domain.AdapterResult{Name: a.name, Status: domain.AdapterPassed, Command: command}
}

func astGrepArgs(target string) ([]string, string) {
	return []string{"outline", "--json=compact", target}, ""
}

func semgrepArgs(target string) ([]string, string) {
	config := firstConfig(target, semgrepConfigs)
	if config == "" {
		return nil, ReasonNoConfig
	}
	return []string{"scan", "--json", "--config", config, target}, ""
}

func slopDetectorArgs(target string) ([]string, string) {
	return []string{"scan", target, "--format", "json"}, ""
}

func firstConfig(target string, names []string) string {
	root := target
	if info, err := os.Stat(root); err == nil && !info.IsDir() {
		root = filepath.Dir(root)
	}
	if absolute, err := filepath.Abs(root); err == nil {
		root = absolute
	}
	for {
		for _, name := range names {
			candidate := filepath.Join(root, name)
			if _, err := os.Stat(candidate); err == nil {
				return candidate
			}
		}
		parent := filepath.Dir(root)
		if parent == root {
			return ""
		}
		root = parent
	}
}

func summarize(output string, err error) string {
	message := strings.TrimSpace(output)
	if message == "" {
		message = err.Error()
	}
	fields := strings.Fields(message)
	const maxFields = 24
	if len(fields) > maxFields {
		fields = fields[:maxFields]
	}
	return strings.Join(fields, " ")
}
