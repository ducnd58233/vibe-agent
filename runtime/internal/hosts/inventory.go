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

// EvalRunnerNames are preset keys used by `eval routing --runner`.
func EvalRunnerNames() []string {
	return []string{"codex", "claude", "cursor", "opencode"}
}

// EvalHost returns the host entry for an eval runner name.
func EvalHost(name string) (Host, bool) {
	switch name {
	case "codex":
		return catalog[0], true
	case "claude":
		return catalog[1], true
	case "cursor", "cursor-agent":
		return catalog[2], true
	case "opencode":
		return catalog[3], true
	default:
		return Host{}, false
	}
}
