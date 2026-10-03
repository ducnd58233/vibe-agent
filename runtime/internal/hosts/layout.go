package hosts

import (
	"os"
	"path/filepath"
	"slices"
)

// SharedRulesFile is the instructions file every host reads, and the one the
// toolkit writes for them. Host-specific files are listed on each Host.
const SharedRulesFile = "AGENTS.md"

// Base is what a Root's directory is relative to.
type Base int

const (
	// Workspace is the checkout a session was opened on.
	Workspace Base = iota
	// Home is the user's home directory.
	Home
	// ConfigHome is the user's config directory: $XDG_CONFIG_HOME, else ~/.config.
	ConfigHome
)

// Root is one directory a host reads from, relative to a Base.
type Root struct {
	Base Base
	// Dir is slash-separated.
	Dir string
}

// sharedSkillRoots are the vendor-neutral skill directories. Codex, Kimi Code,
// Muse, and Antigravity all read them, in the workspace and in the user's home,
// so a skill placed there reaches every one of them.
var sharedSkillRoots = []Root{{Workspace, ".agents/skills"}, {Home, ".agents/skills"}}

// RulesFiles lists every standing-instructions file a host may leave at a
// workspace root, shared one first.
func RulesFiles() []string {
	files := []string{SharedRulesFile}
	for _, host := range catalog {
		for _, name := range host.RulesFiles {
			if !slices.Contains(files, name) {
				files = append(files, name)
			}
		}
	}
	return files
}

// RulesDirs lists every directory of rules a host loads, relative to a workspace
// root, with a trailing slash.
func RulesDirs() []string {
	var dirs []string
	for _, host := range catalog {
		dirs = append(dirs, host.RulesDirs...)
	}
	return dirs
}

// SkillDirs returns every directory a known host reads skills from, workspace
// roots first.
func SkillDirs(workspaceRoot string) []string {
	var roots []Root
	for _, host := range catalog {
		roots = append(roots, host.SkillRoots...)
	}
	return resolve(append(roots, sharedSkillRoots...), workspaceRoot)
}

// CommandDirs returns every directory a known host reads slash commands from,
// workspace roots first.
func CommandDirs(workspaceRoot string) []string {
	var roots []Root
	for _, host := range catalog {
		roots = append(roots, host.CommandRoots...)
	}
	return resolve(roots, workspaceRoot)
}

// resolve turns roots into paths. Workspace roots come first so a checkout's own
// files win over the user's, and roots whose base cannot be found are skipped
// rather than guessed.
func resolve(roots []Root, workspaceRoot string) []string {
	home, _ := os.UserHomeDir()
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" && home != "" {
		configHome = filepath.Join(home, ".config")
	}
	bases := map[Base]string{
		Workspace:  filepath.Clean(workspaceRoot),
		Home:       home,
		ConfigHome: configHome,
	}

	var dirs []string
	for _, base := range []Base{Workspace, Home, ConfigHome} {
		if bases[base] == "" {
			continue
		}
		for _, root := range roots {
			if root.Base == base {
				dirs = append(dirs, filepath.Join(bases[base], filepath.FromSlash(root.Dir)))
			}
		}
	}
	return dirs
}
