// Package hosts reports which agent CLIs are on PATH for Settings and eval.
package hosts

import (
	"fmt"

	"github.com/ducnd58233/vibe-agent/runtime/internal/safexec"
)

// Host is one print-mode runner the toolkit knows about.
type Host struct {
	ID          string
	Binary      string
	EvalCommand string
	PromptAsArg bool
}

// Entry is a host plus PATH lookup status.
type Entry struct {
	Host
	OnPath bool
	Reason string
}

// catalog is the fixed set of hosts eval routing may spawn.
var catalog = []Host{
	{ID: "codex", Binary: "codex", EvalCommand: "codex exec --ephemeral --sandbox read-only --json -"},
	{ID: "claude", Binary: "claude", EvalCommand: "claude -p"},
	{ID: "cursor-agent", Binary: "cursor-agent", EvalCommand: "cursor-agent --print --output-format stream-json --mode ask --trust", PromptAsArg: true},
	{ID: "opencode", Binary: "opencode", EvalCommand: "opencode run", PromptAsArg: true},

	// Three hosts nobody here has run. Listed so Inventory answers "not on
	// PATH" rather than saying nothing. Hook envelopes exist in
	// runtime/internal/harness/contracts.go and are marked UNVERIFIED until
	// someone watches them fire.
	{ID: "kimi", Binary: "kimi", EvalCommand: "kimi --print", PromptAsArg: true},
	{ID: "muse", Binary: "muse", EvalCommand: "muse exec --json", PromptAsArg: true},
	{ID: "antigravity", Binary: "antigravity", EvalCommand: "antigravity exec", PromptAsArg: true},
}

var lookPath = safexec.LookPath

// Catalog returns the hosts this build knows about.
func Catalog() []Host {
	out := make([]Host, len(catalog))
	copy(out, catalog)
	return out
}

// Inventory reports whether each host binary resolves on PATH.
func Inventory() []Entry {
	out := make([]Entry, len(catalog))
	for index, host := range catalog {
		entry := Entry{Host: host}
		if _, err := lookPath(host.Binary); err != nil {
			entry.Reason = fmt.Sprintf("%s not on PATH", host.Binary)
		} else {
			entry.OnPath = true
		}
		out[index] = entry
	}
	return out
}

// evalAlias maps the names `eval routing --runner` accepts to catalog ids.
//
// Only aliases live here: a name that already is a catalog id needs no row, and
// listing it would be a second place for the same fact to drift from.
var evalAlias = map[string]string{"cursor": "cursor-agent"}

// EvalRunnerNames are the preset keys `eval routing --runner` accepts.
//
// Written out rather than derived, because these are the names a person types
// and one of them deliberately differs from its catalog id: the runner is
// called cursor and the binary is cursor-agent. Deriving the list would rename
// a flag value to match an internal id, which is the tail wagging the dog.
//
// A test asserts every name here resolves through EvalHost, so the two lists
// cannot drift apart without something failing.
func EvalRunnerNames() []string {
	return []string{"codex", "claude", "cursor", "opencode", "kimi", "muse", "antigravity"}
}

// EvalHost returns the host entry for an eval runner name.
//
// By id, not by position. This used to switch on the name and return
// catalog[0] through catalog[3], so the catalog's order was a contract that
// nothing stated and nothing checked: inserting a host at the front, or sorting
// the list, silently remapped every lookup to the wrong runner. Nothing would
// have failed loudly - eval would just have spawned the wrong CLI.
func EvalHost(name string) (Host, bool) {
	if alias, ok := evalAlias[name]; ok {
		name = alias
	}
	for _, host := range catalog {
		if host.ID == name {
			return host, true
		}
	}
	return Host{}, false
}
