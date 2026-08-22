package hosts

import (
	"strings"
	"testing"
)

func TestInventoryMissingBinary(t *testing.T) {
	lookPath = func(name string) (string, error) {
		return "", errNotFound(name)
	}
	t.Cleanup(func() { lookPath = defaultLookPath })

	entry := Inventory()[0]
	if entry.OnPath {
		t.Fatal("expected missing host to be off PATH")
	}
	if entry.Reason == "" || !strings.Contains(entry.Reason, "not on PATH") {
		t.Fatalf("reason = %q", entry.Reason)
	}
}

func TestInventoryPresentBinary(t *testing.T) {
	lookPath = func(name string) (string, error) {
		return "/usr/bin/" + name, nil
	}
	t.Cleanup(func() { lookPath = defaultLookPath })

	for _, entry := range Inventory() {
		if !entry.OnPath {
			t.Fatalf("%s should be on PATH", entry.ID)
		}
		if entry.Reason != "" {
			t.Fatalf("reason should be empty when present: %q", entry.Reason)
		}
	}
}

// A count told you a host had been added and nothing about whether it should
// have been. Each entry now carries why it is here, and one with no reason
// fails - the same rule the MCP tool surface uses, for the same reason: a list
// that grows without an argument is a list nobody can prune.
func TestEveryCatalogedHostHasAReason(t *testing.T) {
	reasons := map[string]string{
		"codex":        "hooks through config.toml; the fallback MCP surface exists for it",
		"claude":       "the reference host; every hook event is wired and verified here",
		"cursor-agent": "hooks through .cursor/hooks.json; refuses through JSON rather than exit codes",
		"opencode":     "plugin at .opencode/plugin; permission.ask is its only refusal path",
		"kimi":         "skills at ~/.config/agents/skills; hooks snippet at .kimi/hooks.toml, merge into user config.toml",
		"muse":         "skills via .codex/.claude paths; hooks at .muse/hooks.json, UNVERIFIED until trusted and observed",
		"antigravity":  "hooks at .agents/hooks.json; PreToolUse uses decision/reason, UNVERIFIED until observed",
	}

	catalog := Catalog()
	if len(catalog) != len(reasons) {
		t.Errorf("catalog has %d hosts, the reason list has %d", len(catalog), len(reasons))
	}
	for _, host := range catalog {
		if reasons[host.ID] == "" {
			t.Errorf("host %q is catalogued with no reason recorded; add one here or take it out", host.ID)
		}
		if host.Binary == "" || host.EvalCommand == "" {
			t.Errorf("host %q is missing a binary or an eval command: %+v", host.ID, host)
		}
	}
}

func TestEvalHostAcceptsCursorAlias(t *testing.T) {
	byCursor, ok := EvalHost("cursor")
	if !ok {
		t.Fatal("cursor")
	}
	byBinary, ok := EvalHost("cursor-agent")
	if !ok {
		t.Fatal("cursor-agent should resolve; the composer posts the catalog id")
	}
	if byCursor.Binary != "cursor-agent" || byBinary.Binary != "cursor-agent" {
		t.Fatalf("cursor=%q cursor-agent=%q", byCursor.Binary, byBinary.Binary)
	}
}

func errNotFound(name string) error {
	return &pathError{name: name}
}

type pathError struct{ name string }

func (e *pathError) Error() string { return e.name + ": not found" }

var defaultLookPath = lookPath

// The preset list and the catalog are two lists that have to agree, and nothing
// checked that they did. EvalHost used to index the catalog by position, so the
// catalog's order was a contract nothing stated: inserting a host at the front
// silently remapped every lookup, and eval would have spawned the wrong CLI
// without anything failing.
func TestEveryPresetNameResolvesToAHost(t *testing.T) {
	for _, name := range EvalRunnerNames() {
		host, ok := EvalHost(name)
		if !ok {
			t.Errorf("preset %q resolves to no host", name)
			continue
		}
		if host.Binary == "" || host.EvalCommand == "" {
			t.Errorf("preset %q resolved to an empty host: %+v", name, host)
		}
	}
}

// Lookup is by id, not by position. Reordering the catalog must change nothing.
func TestEvalHostIsIndependentOfCatalogOrder(t *testing.T) {
	before := map[string]Host{}
	for _, name := range EvalRunnerNames() {
		host, ok := EvalHost(name)
		if !ok {
			t.Fatalf("preset %q resolves to no host", name)
		}
		before[name] = host
	}

	original := catalog
	t.Cleanup(func() { catalog = original })
	reversed := make([]Host, len(original))
	for i, host := range original {
		reversed[len(original)-1-i] = host
	}
	catalog = reversed

	for name, want := range before {
		got, ok := EvalHost(name)
		if !ok {
			t.Errorf("preset %q stopped resolving when the catalog was reordered", name)
			continue
		}
		if got.ID != want.ID {
			t.Errorf("preset %q resolved to %q after reordering, want %q", name, got.ID, want.ID)
		}
	}
}

// The alias exists because the runner is called cursor and the binary is
// cursor-agent. Both have to work.
func TestBothCursorNamesResolveToTheSameHost(t *testing.T) {
	short, okShort := EvalHost("cursor")
	full, okFull := EvalHost("cursor-agent")
	if !okShort || !okFull {
		t.Fatalf("cursor=%t cursor-agent=%t", okShort, okFull)
	}
	if short.ID != full.ID {
		t.Errorf("cursor -> %q, cursor-agent -> %q", short.ID, full.ID)
	}
}
