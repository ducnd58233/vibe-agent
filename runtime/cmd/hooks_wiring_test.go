package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ducnd58233/vibe-agent/runtime/internal/harness"
)

func writeConfig(t *testing.T, root, relative, body string) {
	t.Helper()
	path := filepath.Join(root, relative)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write %s: %v", relative, err)
	}
}

// The parse has to find every event across both host configs, because an event
// wired in only one of them is exactly the case that goes unnoticed: Claude works
// and Cursor does not, or the reverse.
func TestRegisteredEventsReadsBothHostConfigs(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, root, filepath.Join(".claude", "settings.json"), `{
  "hooks": {
    "SessionStart": [{"hooks": [{"command": "vibe-agent hook session-start --workspace ${CLAUDE_PROJECT_DIR}"}]}],
    "PostToolUse":  [{"hooks": [{"command": "vibe-agent hook post-tool-use --workspace ${CLAUDE_PROJECT_DIR}"}]}]
  }
}`)
	writeConfig(t, root, filepath.Join(".cursor", "hooks.json"), `{
  "hooks": {
    "stop": [{"command": "vibe-agent hook stop --client cursor"}],
    "postToolUse": [{"command": "vibe-agent hook post-tool-use --client cursor"}]
  }
}`)

	found, err := registeredEvents(root)
	if err != nil {
		t.Fatalf("registeredEvents: %v", err)
	}
	for _, want := range []string{"session-start", "post-tool-use", "stop"} {
		if _, ok := found.byEvent[want]; !ok {
			t.Errorf("%q was registered and not found; keys were %v", want, keysOf(found.byEvent))
		}
	}

	// An event wired in both files has to name both, so a failure message points
	// at every file that needs editing rather than one of them.
	sources := found.byEvent["post-tool-use"]
	if len(sources) != 2 {
		t.Errorf("post-tool-use is in both configs but reported %v", sources)
	}
}

// A consumer repo need not wire every host, and a missing file is not an empty
// config. Reporting no events when both files are absent would say the wiring is
// fine when it was never read.
func TestRegisteredEventsRefusesWhenNoConfigExists(t *testing.T) {
	if _, err := registeredEvents(t.TempDir()); err == nil {
		t.Error("a workspace with no host config reported readable wiring")
	}
}

func TestRegisteredEventsIgnoresOtherVibeAgentCommands(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, root, filepath.Join(".claude", "settings.json"), `{
  "hooks": {
    "SessionStart": [{"hooks": [{"command": "vibe-agent hook session-start"}]}]
  },
  "permissions": {"allow": ["Bash(vibe-agent verify *)", "Bash(vibe-agent run start *)"]}
}`)

	found, err := registeredEvents(root)
	if err != nil {
		t.Fatalf("registeredEvents: %v", err)
	}
	if len(found.byEvent) != 1 {
		t.Errorf("expected only the hook event, got %v", keysOf(found.byEvent))
	}
	for _, notAnEvent := range []string{"verify", "run", "start"} {
		if _, ok := found.byEvent[notAnEvent]; ok {
			t.Errorf("%q is a command, not a hook event, and was read as one", notAnEvent)
		}
	}
}

// The shipped configs are the ones that matter. This fails if a hook is wired to
// an event no build implements, which is the static half of the same check
// doctor runs.
func TestTheShippedConfigsOnlyWireImplementedEvents(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("resolve toolkit root: %v", err)
	}
	found, err := registeredEvents(root)
	if err != nil {
		t.Fatalf("registeredEvents on the toolkit itself: %v", err)
	}
	if len(found.byEvent) == 0 {
		t.Fatal("no hook events found in the shipped configs")
	}

	// The PATH comparison depends on what happens to be installed on the machine
	// running tests, so this asserts only the static half: nothing is wired to an
	// event this build does not implement.
	for event, sources := range found.byEvent {
		if !harness.Handles(harness.Event(event)) {
			t.Errorf("%s wires %q, which this build does not handle",
				strings.Join(sources, " and "), event)
		}
	}
}

func keysOf(m map[string][]string) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	return keys
}

// An absent binary is a supported state, said so in two places: the runtime is
// "optional: every asset works with this binary absent", and a missing one "makes
// every hook a quiet no-op". The first version of this check reported it as a
// defect, which broke every environment that runs the binary by path instead of
// installing it — including this repository's own CI, where nothing installs it.
//
// The distinction that matters is severity. Absent is announced; stale is
// invisible and half-works.
func TestAnAbsentBinaryIsNotADoctorFailure(t *testing.T) {
	root := t.TempDir()
	// Both halves of the outcome, because wiring only the success half is itself
	// a reported defect and would mask what this test is about.
	writeConfig(t, root, filepath.Join(".claude", "settings.json"), `{
  "hooks": {
    "PostToolUse": [{"hooks": [{"command": "vibe-agent hook post-tool-use"}]}],
    "PostToolUseFailure": [{"hooks": [{"command": "vibe-agent hook post-tool-use-failure"}]}]
  }
}`)

	// An empty PATH is the state a fresh clone is in.
	t.Setenv("PATH", t.TempDir())

	report := &diagnostics{}
	checkHookWiring(report, root)

	if report.problems != 0 {
		t.Errorf("doctor reported %d problems with no binary installed; "+
			"the design calls that a supported state, and CI runs in it",
			report.problems)
	}
}

// Registering the success half alone is the shape the journal ran in for months
// while recording nothing that mattered.
func TestWiringOnlySuccessfulToolCallsIsADoctorFailure(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, root, filepath.Join(".claude", "settings.json"), `{
  "hooks": {
    "PostToolUse": [{"hooks": [{"command": "vibe-agent hook post-tool-use"}]}]
  }
}`)
	t.Setenv("PATH", t.TempDir())

	report := &diagnostics{}
	checkHookWiring(report, root)

	if report.problems == 0 {
		t.Error("a config that journals successes and drops every failure was reported as fine")
	}
}

// Wiring neither half stays a choice. A consumer repo that wants no journal at
// all is not misconfigured, and reporting it would make doctor cry wolf.
func TestWiringNeitherHalfIsNotADoctorFailure(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, root, filepath.Join(".claude", "settings.json"), `{
  "hooks": {
    "SessionStart": [{"hooks": [{"command": "vibe-agent hook session-start"}]}]
  }
}`)
	t.Setenv("PATH", t.TempDir())

	report := &diagnostics{}
	checkHookWiring(report, root)

	if report.problems != 0 {
		t.Errorf("a config wiring neither outcome hook reported %d problems", report.problems)
	}
}

// One host wiring both halves says nothing about another that wired one, and
// reading the merged view let it say exactly that. This is the live shape: the
// shipped .claude/settings.json registered both, the shipped .cursor/hooks.json
// registered only the success half, and doctor was green while every failing
// tool call in every Cursor session went unrecorded.
func TestOneHostsCorrectWiringDoesNotCoverAnother(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, root, filepath.Join(".claude", "settings.json"), `{
  "hooks": {
    "PostToolUse": [{"hooks": [{"command": "vibe-agent hook post-tool-use"}]}],
    "PostToolUseFailure": [{"hooks": [{"command": "vibe-agent hook post-tool-use-failure"}]}]
  }
}`)
	writeConfig(t, root, filepath.Join(".cursor", "hooks.json"), `{
  "hooks": {
    "postToolUse": [{"command": "vibe-agent hook post-tool-use --client cursor"}]
  }
}`)
	t.Setenv("PATH", t.TempDir())

	report := &diagnostics{}
	checkHookWiring(report, root)

	if report.problems == 0 {
		t.Error("Cursor journals successes and drops every failure, and doctor was green " +
			"because Claude's config answered the question on its behalf")
	}
}

// Codex reports a failed tool call through PostToolUse itself, which "fires
// after tools complete, including when commands exit with a non-zero status".
// There is no second event to wire, so demanding one would send a consumer
// looking for a hook their host does not have.
//
// Ref: https://learn.chatgpt.com/docs/hooks
func TestAHostThatDoesNotSplitOutcomesIsNotAskedForTheFailureHalf(t *testing.T) {
	root := t.TempDir()
	// No --client: this build has no Codex envelope yet, and checkClients would
	// rightly refuse one. The fixture is about the outcome pair, so it wires the
	// only shape a Codex config could take today.
	writeConfig(t, root, filepath.Join(".codex", "hooks.json"), `{
  "hooks": {
    "PostToolUse": [{"command": "vibe-agent hook post-tool-use"}]
  }
}`)
	t.Setenv("PATH", t.TempDir())

	report := &diagnostics{}
	checkHookWiring(report, root)

	if report.problems != 0 {
		t.Errorf("a Codex config wiring the only outcome event it has reported %d problems",
			report.problems)
	}
}

// A file that exists and registers nothing is not wiring. opencode.json is
// always present in this repository and drives the control plane over MCP, so
// counting it as a readable hook config would report a workspace with no hooks
// at all as fully wired - the same "green because it looked at nothing" shape
// this checker exists to prevent.
func TestAConfigThatRegistersNoHooksIsNotWiring(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, root, "opencode.json", `{
  "mcp": {"vibe-agent": {"type": "local", "command": ["vibe-agent", "mcp", "serve"]}}
}`)

	if _, err := registeredEvents(root); err == nil {
		t.Error("a workspace whose only host config registers no hook was reported as wired")
	}
}

// A wrong event name is refused loudly. A wrong host name was not refused at
// all: it fell through to Claude's envelope, so the hook fired on every tool
// call and the host discarded every reply. Nothing anywhere said so.
func TestAnUnknownClientIsADoctorFailure(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, root, filepath.Join(".claude", "settings.json"), `{
  "hooks": {
    "SessionStart": [{"hooks": [{"command": "vibe-agent hook session-start --client windsurf"}]}]
  }
}`)
	t.Setenv("PATH", t.TempDir())

	report := &diagnostics{}
	checkHookWiring(report, root)

	if report.problems == 0 {
		t.Error("a hook naming a host this build has no envelope for was reported as fine")
	}
}

// Every host this build claims to answer has to survive the same check, or the
// list and the checker could disagree and nobody would find out from a test.
func TestEveryKnownClientPassesTheWiringCheck(t *testing.T) {
	for _, client := range harness.Clients() {
		root := t.TempDir()
		writeConfig(t, root, filepath.Join(".claude", "settings.json"), `{
  "hooks": {
    "SessionStart": [{"hooks": [{"command": "vibe-agent hook session-start --client `+string(client)+`"}]}]
  }
}`)
		t.Setenv("PATH", t.TempDir())

		report := &diagnostics{}
		checkHookWiring(report, root)

		if report.problems != 0 {
			t.Errorf("client %q is in Clients() and doctor rejects it", client)
		}
	}
}

// A host reads hooks from the project directory it was opened on, so the
// workspace is the only place wiring can take effect. A vendored toolkit ships
// its own .claude/settings.json for when it is opened standalone, and reading
// that one instead reports a workspace as wired when no hook there can fire.
//
// This was live: a consumer repo with five runs, no workspace hook config, and a
// green doctor, while every tool call went unjournalled and memory.db stayed
// empty for months.
func TestAVendoredToolkitsOwnConfigIsNotTheWorkspaceWiring(t *testing.T) {
	workspace := t.TempDir()
	writeConfig(t, workspace, filepath.Join(".vibe-agent", ".claude", "settings.json"), `{
  "hooks": {
    "PostToolUse": [{"hooks": [{"command": "vibe-agent hook post-tool-use"}]}],
    "PostToolUseFailure": [{"hooks": [{"command": "vibe-agent hook post-tool-use-failure"}]}]
  }
}`)
	// A run in flight is what makes the gap actionable rather than a preference:
	// the loop is being driven and nothing is recording it.
	writeConfig(t, workspace, filepath.Join("tmp", "demo", "manifest.json"), `{
  "schemaVersion": 1,
  "runId": "run_fixture",
  "graphId": "goal-delivery",
  "slug": "demo",
  "goal": "fixture",
  "currentNode": "build",
  "status": "running",
  "iteration": 1,
  "maxTransitions": 50,
  "createdAt": "2026-08-18T10:00:00Z",
  "updatedAt": "2026-08-18T10:00:00Z"
}`)
	t.Setenv("PATH", t.TempDir())

	report := &diagnostics{}
	checkHookWiring(report, workspace)

	if report.problems == 0 {
		t.Error("a workspace with runs and no hook config of its own was reported as fine; " +
			"the toolkit's vendored config was read as though the host would see it")
	}
}

// Without runs it stays a preference. A repository that mounts the toolkit for
// its skills and commands and wants no control plane is not misconfigured.
func TestAWorkspaceWithNoRunsAndNoHooksIsNotADoctorFailure(t *testing.T) {
	workspace := t.TempDir()
	writeConfig(t, workspace, filepath.Join(".vibe-agent", ".claude", "settings.json"), `{
  "hooks": {
    "PostToolUse": [{"hooks": [{"command": "vibe-agent hook post-tool-use"}]}]
  }
}`)
	t.Setenv("PATH", t.TempDir())

	report := &diagnostics{}
	checkHookWiring(report, workspace)

	if report.problems != 0 {
		t.Errorf("a workspace that simply does not use the control plane reported %d problems",
			report.problems)
	}
}

// The other half. A config wiring an event no build implements is a real defect
// whether or not anything is installed, because no install will fix it.
func TestAnUnimplementedEventIsADoctorFailure(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, root, filepath.Join(".claude", "settings.json"), `{
  "hooks": {
    "Whenever": [{"hooks": [{"command": "vibe-agent hook before-everything"}]}]
  }
}`)
	t.Setenv("PATH", t.TempDir())

	report := &diagnostics{}
	checkHookWiring(report, root)

	if report.problems == 0 {
		t.Error("a hook wired to an event no build implements was reported as fine")
	}
}
