package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
	"github.com/ducnd58233/vibe-agent/runtime/internal/session"
)

const cliTestSecret = "sk-0123456789abcdef0123456789ab"

func workspaceWithSessionLog(t *testing.T) (root, slug string) {
	t.Helper()
	root = t.TempDir()
	slug = "demo"
	run, err := state.NewRun(slug, "session cli test", "goal-delivery", 50, time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if err := state.Save(state.ManifestPath(root, slug), run); err != nil {
		t.Fatal(err)
	}
	path := session.LogPath(root, slug)
	if _, err := session.Append(path, session.Record{
		Type:   session.TypePromptSubmit,
		Source: session.SourceHook,
		Client: "claude",
		Body:   "deploy with " + cliTestSecret,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := session.Append(path, session.Record{
		Type:    session.TypeToolUse,
		Source:  session.SourceHook,
		Client:  "claude",
		Tool:    "bash",
		Command: "echo ok",
	}); err != nil {
		t.Fatal(err)
	}
	return root, slug
}

func TestSessionShowListsMonotonicSequence(t *testing.T) {
	root, slug := workspaceWithSessionLog(t)
	var out bytes.Buffer
	if err := writeSessionShow(&out, session.LogPath(root, slug)); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	if !strings.Contains(text, "#1 prompt_submit") || !strings.Contains(text, "#2 tool_use") {
		t.Fatalf("unexpected show output: %s", text)
	}
	if strings.Contains(text, cliTestSecret) {
		t.Fatalf("secret leaked in show output: %s", text)
	}
}

func TestSessionListIncludesRunSlug(t *testing.T) {
	root, slug := workspaceWithSessionLog(t)
	given := root
	flags := rootFlags{workspace: &given, toolkit: new(string)}
	workspaceRoot, _, err := flags.resolve()
	if err != nil {
		t.Fatal(err)
	}
	slugs, err := state.List(workspaceRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(slugs) != 1 || slugs[0] != slug {
		t.Fatalf("List = %v, want [%s]", slugs, slug)
	}
}

func TestListMissingRunsDirIsEmpty(t *testing.T) {
	root := t.TempDir()
	slugs, err := state.List(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(slugs) != 0 {
		t.Fatalf("List = %v, want empty", slugs)
	}
}

func TestSessionListCommandPrintsSlug(t *testing.T) {
	root, slug := workspaceWithSessionLog(t)
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	err = sessionList([]string{"--workspace", root})
	if closeErr := w.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	os.Stdout = oldStdout
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if _, err := out.ReadFrom(r); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), slug) {
		t.Fatalf("list output = %q", out.String())
	}
}

func TestUsageMentionsSessionCommands(t *testing.T) {
	if !strings.Contains(usage, "session list") || !strings.Contains(usage, "session show") {
		t.Fatal("usage text missing session commands")
	}
}

func TestSessionShowMissingLogIsNotAnError(t *testing.T) {
	root := t.TempDir()
	var out bytes.Buffer
	path := filepath.Join(root, "tmp", "missing", session.LogName)
	if err := writeSessionShow(&out, path); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "No session log yet") {
		t.Fatalf("output = %q", out.String())
	}
}
