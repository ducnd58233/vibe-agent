package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ducnd58233/vibe-agent/runtime/internal/sandbox"
)

func TestSandboxIsolationNoteNamesRunnerPort(t *testing.T) {
	if !strings.Contains(sandboxIsolationNote, "runner port") {
		t.Fatalf("note must say runner port: %q", sandboxIsolationNote)
	}
	if !strings.Contains(sandboxIsolationNote, "local = no isolation") {
		t.Fatalf("note must describe local: %q", sandboxIsolationNote)
	}
	if !strings.Contains(sandboxIsolationNote, "Seatbelt") && !strings.Contains(sandboxIsolationNote, "bubblewrap") {
		t.Fatalf("note must contrast Claude OS sandbox: %q", sandboxIsolationNote)
	}
}

func TestCheckSandboxConfigPrintsIsolationNoteWhenPresent(t *testing.T) {
	root := t.TempDir()
	agentState := filepath.Join(root, ".agent-state")
	if err := os.MkdirAll(agentState, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(agentState, sandbox.FileName), []byte(sandbox.ExampleConfig()), 0o600); err != nil {
		t.Fatal(err)
	}

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	report := &diagnostics{}
	checkSandboxConfig(report, root)
	_ = w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	_ = r.Close()
	out := buf.String()
	if !strings.Contains(out, sandboxIsolationNote) {
		t.Fatalf("doctor output missing isolation note:\n%s", out)
	}
}
