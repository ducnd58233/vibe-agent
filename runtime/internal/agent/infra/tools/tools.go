// Package tools is the built-in tool set for the inner loop.
//
// Six tools, deliberately few: read, write, edit, grep, glob, and shell. The
// loop exists to run mechanical steps headlessly, not to be a second coding
// agent, and a tool surface grows faster than the judgement to use it well.
//
// Every dispatch passes the gate pre-tool-use enforces, through the same
// function, so a refusal is a refusal whichever entry point asked. Nothing here
// decides what may run; it asks.
package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	agent "github.com/ducnd58233/vibe-agent/runtime/internal/agent/domain"
	"github.com/ducnd58233/vibe-agent/runtime/internal/harness"
	"github.com/ducnd58233/vibe-agent/runtime/internal/safexec"
)

// MaxReadBytes bounds what one read may return. A tool that can pour a
// gigabyte into a context window is a tool that ends a run by accident.
const MaxReadBytes = 256 * 1024

// MaxMatches bounds grep and glob output for the same reason.
const MaxMatches = 200

// Names is the tool set, sorted. A test walks it, so adding a tool without a
// gate name fails rather than shipping unguarded.
func Names() []string {
	names := make([]string, 0, len(handlers))
	for name := range handlers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// gateName maps a tool to the name a host would report for the same action, so
// the gate matches the rules it already has rather than needing new ones.
var gateName = map[string]string{
	"read":  "Read",
	"write": "Write",
	"edit":  "Edit",
	"grep":  "Grep",
	"glob":  "Glob",
	"shell": "Bash",
}

// gateArgs is the union of fields the gate reads across the tool set.
type gateArgs struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	New     string `json:"new"`
	Command string `json:"command"`
}

type handler func(ctx context.Context, d *Dispatcher, input json.RawMessage) (string, error)

var handlers = map[string]handler{
	"read":  readFile,
	"write": writeFile,
	"edit":  editFile,
	"grep":  grepFiles,
	"glob":  globFiles,
	"shell": runShell,
}

// Dispatcher executes the built-in tools inside one workspace.
type Dispatcher struct {
	WorkspaceRoot string
}

// New builds a dispatcher rooted at a workspace.
func New(workspaceRoot string) *Dispatcher {
	return &Dispatcher{WorkspaceRoot: workspaceRoot}
}

// Dispatch runs one call and returns what the model should read.
//
// A failure comes back as a result marked IsError rather than as a Go error,
// because the model needs to see what went wrong and react. A refusal is a
// result too, so the model learns the boundary rather than retrying against it
// blindly.
func (d *Dispatcher) Dispatch(ctx context.Context, call agent.ToolCall) agent.ToolResult {
	run, known := handlers[call.Name]
	if !known {
		return failure(call, fmt.Sprintf("no tool %q; this loop has %s",
			call.Name, strings.Join(Names(), ", ")))
	}

	if blocked := d.gate(call); blocked != nil {
		return failure(call, "refused: "+blocked.Reason)
	}

	out, err := run(ctx, d, call.Input)
	if err != nil {
		return failure(call, err.Error())
	}
	return agent.ToolResult{CallID: call.ID, Content: out}
}

// gate asks the same question pre-tool-use asks, about the same call.
func (d *Dispatcher) gate(call agent.ToolCall) *harness.BlockError {
	var args gateArgs
	if json.Unmarshal(call.Input, &args) != nil {
		// An input the gate cannot parse is still gated. It becomes an empty
		// call, and an empty call matches no rule, which is how the hook half
		// treats stdin it cannot read.
		args = gateArgs{}
	}

	content := args.Content
	if content == "" {
		content = args.New
	}
	return harness.Verdict(d.WorkspaceRoot, harness.ToolCall{
		Name:     gateName[call.Name],
		Command:  args.Command,
		FilePath: args.Path,
		Content:  content,
	})
}

func failure(call agent.ToolCall, message string) agent.ToolResult {
	return agent.ToolResult{CallID: call.ID, Content: message, IsError: true}
}

// withRoot runs fn against a root-scoped handle on the workspace.
//
// os.Root is the reason this package does not hand-roll a path check. Comparing
// a cleaned string against the workspace prefix checks the name; a root is a
// check the operating system enforces on the open, including through symlinks
// and against the file being swapped between the check and the use. One helper
// rather than one open per tool, so there is a single place the handle is
// closed.
func (d *Dispatcher) withRoot(fn func(root *os.Root) (string, error)) (string, error) {
	if d.WorkspaceRoot == "" {
		return "", fmt.Errorf("dispatcher has no workspace root")
	}
	root, err := os.OpenRoot(d.WorkspaceRoot)
	if err != nil {
		return "", err
	}
	defer func() { _ = root.Close() }()
	return fn(root)
}

// relative rejects the shapes a root cannot take, with a message naming which
// rule was broken rather than leaving the caller to read an errno.
func relative(path string) (string, error) {
	switch {
	case path == "":
		return "", fmt.Errorf("path is required")
	case filepath.IsAbs(path):
		return "", fmt.Errorf("%s is absolute; tool paths are relative to the workspace", path)
	}
	clean := filepath.Clean(filepath.FromSlash(path))
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%s leaves the workspace", path)
	}
	return clean, nil
}

// shellOperator names the syntax a caller most often expects a shell to
// interpret. Refusing is louder than passing it through as an argument, where
// the command succeeds having done something other than what was asked.
var shellOperator = map[string]bool{
	"|": true, "||": true, "&&": true, ">": true, ">>": true, "<": true, ";": true, "&": true,
}

// skippable reports whether a walk error is one to step over rather than fail
// on. A grep that dies on the first unreadable file is a grep nobody can rely
// on; a grep that hides a real fault is worse.
func skippable(err error) bool {
	return errors.Is(err, fs.ErrPermission) || errors.Is(err, fs.ErrNotExist)
}

func decode(input json.RawMessage, into any) error {
	if len(input) == 0 {
		return fmt.Errorf("the call carried no input")
	}
	if err := json.Unmarshal(input, into); err != nil {
		return fmt.Errorf("input is not the shape this tool takes: %w", err)
	}
	return nil
}

func readFile(_ context.Context, d *Dispatcher, input json.RawMessage) (string, error) {
	var args struct {
		Path string `json:"path"`
	}
	if err := decode(input, &args); err != nil {
		return "", err
	}
	name, err := relative(args.Path)
	if err != nil {
		return "", err
	}
	return d.withRoot(func(root *os.Root) (string, error) {
		data, readErr := root.ReadFile(name)
		if readErr != nil {
			return "", readErr
		}
		if len(data) > MaxReadBytes {
			return string(data[:MaxReadBytes]) +
				fmt.Sprintf("\n\n[clipped at %d bytes of %d]", MaxReadBytes, len(data)), nil
		}
		return string(data), nil
	})
}

func writeFile(_ context.Context, d *Dispatcher, input json.RawMessage) (string, error) {
	var args struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := decode(input, &args); err != nil {
		return "", err
	}
	name, err := relative(args.Path)
	if err != nil {
		return "", err
	}
	return d.withRoot(func(root *os.Root) (string, error) {
		if parent := filepath.Dir(name); parent != "." {
			if mkErr := root.MkdirAll(parent, 0o750); mkErr != nil {
				return "", mkErr
			}
		}
		if writeErr := root.WriteFile(name, []byte(args.Content), 0o600); writeErr != nil {
			return "", writeErr
		}
		return fmt.Sprintf("wrote %d bytes to %s", len(args.Content), args.Path), nil
	})
}

func editFile(_ context.Context, d *Dispatcher, input json.RawMessage) (string, error) {
	var args struct {
		Path string `json:"path"`
		Old  string `json:"old"`
		New  string `json:"new"`
	}
	if err := decode(input, &args); err != nil {
		return "", err
	}
	if args.Old == "" {
		return "", fmt.Errorf("old is required; an empty match would rewrite the file")
	}
	name, err := relative(args.Path)
	if err != nil {
		return "", err
	}
	return d.withRoot(func(root *os.Root) (string, error) {
		data, readErr := root.ReadFile(name)
		if readErr != nil {
			return "", readErr
		}
		text := string(data)
		// One occurrence only. A replace-all on an ambiguous match changes
		// lines nobody looked at, and the model cannot see what else it hit.
		if count := strings.Count(text, args.Old); count != 1 {
			return "", fmt.Errorf("old matches %d times in %s; it has to match exactly once", count, args.Path)
		}
		replaced := strings.Replace(text, args.Old, args.New, 1)
		if writeErr := root.WriteFile(name, []byte(replaced), 0o600); writeErr != nil {
			return "", writeErr
		}
		return "edited " + args.Path, nil
	})
}

func grepFiles(_ context.Context, d *Dispatcher, input json.RawMessage) (string, error) {
	var args struct {
		Pattern string `json:"pattern"`
		Path    string `json:"path"`
	}
	if err := decode(input, &args); err != nil {
		return "", err
	}
	if args.Pattern == "" {
		return "", fmt.Errorf("pattern is required")
	}
	from := "."
	if args.Path != "" {
		name, err := relative(args.Path)
		if err != nil {
			return "", err
		}
		from = filepath.ToSlash(name)
	}

	return d.withRoot(func(root *os.Root) (string, error) {
		var matches []string
		walkErr := fs.WalkDir(root.FS(), from, func(path string, entry fs.DirEntry, err error) error {
			switch {
			case err != nil && skippable(err):
				return nil
			case err != nil:
				return err
			case entry.IsDir(), len(matches) >= MaxMatches:
				return nil
			}
			// Size first. Reading a 500MB binary to search it for a string
			// would end the run on memory rather than on a budget, and the
			// answer would be no matches either way.
			info, statErr := entry.Info()
			if statErr != nil {
				if skippable(statErr) {
					return nil
				}
				return statErr
			}
			if info.Size() > MaxReadBytes {
				return nil
			}
			data, readErr := root.ReadFile(filepath.FromSlash(path))
			if readErr != nil {
				if skippable(readErr) {
					return nil
				}
				return readErr
			}
			for number, line := range strings.Split(string(data), "\n") {
				if !strings.Contains(line, args.Pattern) {
					continue
				}
				matches = append(matches, fmt.Sprintf("%s:%d:%s", path, number+1, strings.TrimSpace(line)))
				if len(matches) >= MaxMatches {
					return nil
				}
			}
			return nil
		})
		if walkErr != nil {
			return "", walkErr
		}
		if len(matches) == 0 {
			return "no matches", nil
		}
		return strings.Join(matches, "\n"), nil
	})
}

func globFiles(_ context.Context, d *Dispatcher, input json.RawMessage) (string, error) {
	var args struct {
		Pattern string `json:"pattern"`
	}
	if err := decode(input, &args); err != nil {
		return "", err
	}
	if args.Pattern == "" {
		return "", fmt.Errorf("pattern is required")
	}
	return d.withRoot(func(root *os.Root) (string, error) {
		found, globErr := fs.Glob(root.FS(), filepath.ToSlash(args.Pattern))
		if globErr != nil {
			return "", globErr
		}
		if len(found) == 0 {
			return "no matches", nil
		}
		if len(found) > MaxMatches {
			found = found[:MaxMatches]
		}
		return strings.Join(found, "\n"), nil
	})
}

// runShell runs one command. It is not a shell.
//
// The command is split on whitespace and executed directly, so pipes,
// redirects, globs, and variable expansion are passed through as literal
// arguments rather than interpreted. That is the safer default and it is a
// semantic the model has to be told, or it will write `a | b` and read the
// absence of a pipe as the command having failed.
func runShell(ctx context.Context, d *Dispatcher, input json.RawMessage) (string, error) {
	var args struct {
		Command string `json:"command"`
	}
	if err := decode(input, &args); err != nil {
		return "", err
	}
	if d.WorkspaceRoot == "" {
		return "", fmt.Errorf("dispatcher has no workspace root")
	}
	fields := strings.Fields(args.Command)
	if len(fields) == 0 {
		return "", fmt.Errorf("command is required")
	}
	for _, field := range fields {
		if shellOperator[field] {
			return "", fmt.Errorf("%q is shell syntax, and this tool runs one command without a shell; "+
				"run the parts separately or use a program that does the work", field)
		}
	}
	// safexec, never exec.Command: it resolves the binary on PATH so a planted
	// executable in the working directory cannot shadow the one meant to run.
	cmd, err := safexec.CommandContext(ctx, fields[0], fields[1:]...)
	if err != nil {
		return "", err
	}
	cmd.Dir = d.WorkspaceRoot
	out, runErr := cmd.CombinedOutput()
	text := string(out)
	if len(text) > MaxReadBytes {
		text = text[:MaxReadBytes] + "\n[clipped]"
	}
	if runErr != nil {
		return "", fmt.Errorf("%s: %w\n%s", fields[0], runErr, text)
	}
	return text, nil
}
