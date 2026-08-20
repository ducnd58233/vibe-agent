package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	agent "github.com/ducnd58233/vibe-agent/runtime/internal/agent/domain"
	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
)

func call(name string, args map[string]any) agent.ToolCall {
	raw, _ := json.Marshal(args)
	return agent.ToolCall{ID: "c1", Name: name, Input: raw}
}

func run(t *testing.T, d *Dispatcher, name string, args map[string]any) agent.ToolResult {
	t.Helper()
	return d.Dispatch(context.Background(), call(name, args))
}

// Adding a handler without a gate name would hand the gate an empty tool name,
// which matches no rule, which is a tool shipped unguarded. This is the check
// that makes that impossible to do quietly.
func TestEveryToolMapsToAGateName(t *testing.T) {
	for _, name := range Names() {
		if gateName[name] == "" {
			t.Errorf("tool %q has no gate name, so the gate would see an empty call", name)
		}
	}
	if len(Names()) != len(gateName) {
		t.Errorf("%d tools and %d gate names; one side has an entry the other does not",
			len(Names()), len(gateName))
	}
}

// A refusal reaches the model as a result rather than ending the loop, so it
// learns the boundary instead of retrying against it blindly.
func TestARefusedCallComesBackAsAnErrorResult(t *testing.T) {
	root := t.TempDir()
	run, err := state.NewRun("demo", "tool gate", "goal-delivery", 50, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	run.CurrentNode = "build"
	run.Status = state.StatusRunning
	if err := state.Save(state.ManifestPath(root, "demo"), run); err != nil {
		t.Fatal(err)
	}

	manifest := filepath.ToSlash(filepath.Join("tmp", "demo", "manifest.json"))
	result := New(root).Dispatch(context.Background(),
		call("write", map[string]any{"path": manifest, "content": "{}"}))

	if !result.IsError {
		t.Fatalf("a write to run state was allowed: %+v", result)
	}
	if !strings.Contains(result.Content, "refused") {
		t.Errorf("content = %q, want it to say it was refused", result.Content)
	}
	// The refusal has to have stopped the write, not merely described it.
	if _, err := os.Stat(filepath.Join(root, "tmp", "demo", "manifest.json")); err == nil {
		if data, readErr := os.ReadFile(filepath.Clean(filepath.Join(root, "tmp", "demo", "manifest.json"))); readErr == nil && string(data) == "{}" {
			t.Error("the file was written anyway")
		}
	}
}

func TestWriteReadAndEditRoundTrip(t *testing.T) {
	d := New(t.TempDir())

	if result := run(t, d, "write", map[string]any{"path": "a/b.txt", "content": "hello world"}); result.IsError {
		t.Fatalf("write: %s", result.Content)
	}
	read := run(t, d, "read", map[string]any{"path": "a/b.txt"})
	if read.IsError || read.Content != "hello world" {
		t.Fatalf("read = %+v", read)
	}
	if result := run(t, d, "edit", map[string]any{"path": "a/b.txt", "old": "world", "new": "there"}); result.IsError {
		t.Fatalf("edit: %s", result.Content)
	}
	if after := run(t, d, "read", map[string]any{"path": "a/b.txt"}); after.Content != "hello there" {
		t.Errorf("after edit = %q", after.Content)
	}
}

// A replace-all on an ambiguous match changes lines nobody looked at, and the
// model cannot see what else it hit.
func TestEditRefusesAMatchThatIsNotUnique(t *testing.T) {
	d := New(t.TempDir())
	if result := run(t, d, "write", map[string]any{"path": "x.txt", "content": "a\na\n"}); result.IsError {
		t.Fatal(result.Content)
	}

	result := run(t, d, "edit", map[string]any{"path": "x.txt", "old": "a", "new": "b"})
	if !result.IsError {
		t.Fatal("an ambiguous edit was applied")
	}
	if !strings.Contains(result.Content, "exactly once") {
		t.Errorf("content = %q", result.Content)
	}
}

// Clamping a path would silently act on a different file than the model asked
// for, which is worse than failing.
func TestAPathOutsideTheWorkspaceIsRefused(t *testing.T) {
	d := New(t.TempDir())
	for _, path := range []string{"../escape.txt", "a/../../escape.txt"} {
		result := run(t, d, "write", map[string]any{"path": path, "content": "x"})
		if !result.IsError {
			t.Errorf("%s was written outside the workspace", path)
		}
	}
}

func TestGrepAndGlobFindWhatWasWritten(t *testing.T) {
	d := New(t.TempDir())
	if result := run(t, d, "write", map[string]any{"path": "src/main.go", "content": "package main\nfunc Work() {}\n"}); result.IsError {
		t.Fatal(result.Content)
	}

	found := run(t, d, "grep", map[string]any{"pattern": "func Work"})
	if found.IsError || !strings.Contains(found.Content, "src/main.go:2") {
		t.Errorf("grep = %+v", found)
	}
	listed := run(t, d, "glob", map[string]any{"pattern": "src/*.go"})
	if listed.IsError || !strings.Contains(listed.Content, "src/main.go") {
		t.Errorf("glob = %+v", listed)
	}
	if empty := run(t, d, "grep", map[string]any{"pattern": "nothing here at all"}); empty.Content != "no matches" {
		t.Errorf("a miss should say so plainly: %q", empty.Content)
	}
}

func TestShellRunsAndReportsFailure(t *testing.T) {
	d := New(t.TempDir())

	ok := run(t, d, "shell", map[string]any{"command": "go version"})
	if ok.IsError || !strings.Contains(ok.Content, "go version") {
		t.Errorf("shell = %+v", ok)
	}
	bad := run(t, d, "shell", map[string]any{"command": "go definitely-not-a-subcommand"})
	if !bad.IsError {
		t.Error("a failing command reported success")
	}
}

func TestAnUnknownToolNamesTheOnesThatExist(t *testing.T) {
	result := run(t, New(t.TempDir()), "teleport", map[string]any{})
	if !result.IsError {
		t.Fatal("an unknown tool ran")
	}
	for _, name := range Names() {
		if !strings.Contains(result.Content, name) {
			t.Errorf("the error does not mention %q, so the model cannot recover", name)
		}
	}
}

func TestACallWithNoInputFailsRatherThanGuessing(t *testing.T) {
	d := New(t.TempDir())
	result := d.Dispatch(context.Background(), agent.ToolCall{ID: "c1", Name: "read"})
	if !result.IsError {
		t.Fatal("a read with no input was accepted")
	}
}
