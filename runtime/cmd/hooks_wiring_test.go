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
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
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
		if _, ok := found[want]; !ok {
			t.Errorf("%q was registered and not found; keys were %v", want, keysOf(found))
		}
	}

	// An event wired in both files has to name both, so a failure message points
	// at every file that needs editing rather than one of them.
	sources := found["post-tool-use"]
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
	if len(found) != 1 {
		t.Errorf("expected only the hook event, got %v", keysOf(found))
	}
	for _, notAnEvent := range []string{"verify", "run", "start"} {
		if _, ok := found[notAnEvent]; ok {
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
	if len(found) == 0 {
		t.Fatal("no hook events found in the shipped configs")
	}

	// The PATH comparison depends on what happens to be installed on the machine
	// running tests, so this asserts only the static half: nothing is wired to an
	// event this build does not implement.
	for event, sources := range found {
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
