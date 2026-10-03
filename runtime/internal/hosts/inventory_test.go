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
		"kimi":         "Kimi Code: skills at .kimi-code/skills and .agents/skills; hooks snippet at .kimi-code/hooks.toml, merged into ~/.kimi-code/config.toml",
		"muse":         "reads AGENTS.md and CLAUDE.md, skills at .agents/skills; hooks at .muse/hooks.json once the folder is trusted",
		"antigravity":  "hooks at .agents/hooks.json (camelCase stdin); PreToolUse uses decision/reason, UNVERIFIED until observed",
	}

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

// An alias exists where the name a person types differs from the catalog id.
// Both have to resolve, and to the same host.
func TestEveryAliasAndItsIDResolveToTheSameHost(t *testing.T) {
	for alias, id := range evalAlias {
		byAlias, okAlias := EvalHost(alias)
		byID, okID := EvalHost(id)
		if !okAlias || !okID {
			t.Fatalf("%s=%t %s=%t", alias, okAlias, id, okID)
		}
		if byAlias.ID != byID.ID {
			t.Errorf("%s -> %q, %s -> %q", alias, byAlias.ID, id, byID.ID)
		}
	}
}
