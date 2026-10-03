// Package hosts reports which agent CLIs are on PATH for Settings and eval.
package hosts

import (
	"fmt"

	"github.com/ducnd58233/vibe-agent/runtime/internal/safexec"
)

// Host is one agent the toolkit knows about: how to run it headless, and where
// it keeps the files the toolkit reads. Everything that differs per host lives
// in this table so no other package has to name one.
type Host struct {
	ID          string
	Binary      string
	EvalCommand string
	PromptAsArg bool

	// RulesFiles are the files at a workspace root this host loads as standing
	// instructions, beyond the shared AGENTS.md.
	RulesFiles []string
	// RulesDirs are directories of rules this host loads, with a trailing slash.
	RulesDirs []string
	// SkillRoots and CommandRoots are where this host reads skills and slash
	// commands from.
	SkillRoots   []Root
	CommandRoots []Root
	// SkillsAgent is this host's identifier in the `skills` installer CLI. Empty
	// means the installer does not target it.
	SkillsAgent string
}

// Entry is a host plus PATH lookup status.
type Entry struct {
	Host
	OnPath bool
	Reason string
}

// catalog is the fixed set of hosts eval routing may spawn.
var catalog = []Host{
	{
		ID: "codex", Binary: "codex", EvalCommand: "codex exec --ephemeral --sandbox read-only --json -",
		SkillRoots:  []Root{{Workspace, ".codex/skills"}},
		SkillsAgent: "codex",
	},
	{
		ID: "claude", Binary: "claude", EvalCommand: "claude -p",
		RulesFiles:   []string{"CLAUDE.md", "CLAUDE.local.md"},
		SkillRoots:   []Root{{Workspace, ".claude/skills"}, {Home, ".claude/skills"}},
		CommandRoots: []Root{{Workspace, ".claude/commands"}, {Home, ".claude/commands"}},
		SkillsAgent:  "claude-code",
	},
	{
		ID: "cursor-agent", Binary: "cursor-agent", EvalCommand: "cursor-agent --print --output-format stream-json --mode ask --trust", PromptAsArg: true,
		RulesFiles:   []string{"CURSOR.md"},
		RulesDirs:    []string{".cursor/rules/"},
		SkillRoots:   []Root{{Workspace, ".cursor/skills"}, {Home, ".cursor/skills"}},
		CommandRoots: []Root{{Workspace, ".cursor/commands"}, {Home, ".cursor/commands"}},
		SkillsAgent:  "cursor",
	},
	{
		ID: "opencode", Binary: "opencode", EvalCommand: "opencode run", PromptAsArg: true,
		SkillRoots:   []Root{{ConfigHome, "opencode/skills"}},
		CommandRoots: []Root{{ConfigHome, "opencode/commands"}},
		SkillsAgent:  "opencode",
	},

	// Three hosts nobody here has run. Their layout and hook contracts are read
	// from vendor documentation and independent measurement; the contracts stay
	// UNVERIFIED in runtime/internal/harness/contracts.go until someone watches
	// a hook fire.
	{
		ID: "kimi", Binary: "kimi", EvalCommand: "kimi --print", PromptAsArg: true,
		SkillRoots: []Root{{Workspace, ".kimi-code/skills"}, {Home, ".kimi-code/skills"}},
	},
	{
		ID: "muse", Binary: "muse", EvalCommand: "muse exec --json", PromptAsArg: true,
		// Muse reads an existing CLAUDE.md alongside AGENTS.md, and skills from the
		// shared .agents/skills root.
		RulesFiles: []string{"CLAUDE.md"},
	},
	{
		ID: "antigravity", Binary: "antigravity", EvalCommand: "antigravity exec", PromptAsArg: true,
		// Antigravity keeps workspace customisation under .agents/, whose skills
		// root is shared. Its user directory is documented for hooks only
		// (~/.gemini/config/hooks.json), so no user skill root is claimed.
	},
}

var lookPath = safexec.LookPath

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

// DefaultEvalRunner is the runner `eval routing` uses when none is named.
const DefaultEvalRunner = "codex"

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

// SkillsAgents are the identifiers the `skills` installer CLI knows the hosts by.
func SkillsAgents() []string {
	var agents []string
	for _, host := range catalog {
		if host.SkillsAgent != "" {
			agents = append(agents, host.SkillsAgent)
		}
	}
	return agents
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
