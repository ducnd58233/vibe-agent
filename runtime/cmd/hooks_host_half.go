package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/ducnd58233/vibe-agent/runtime/internal/harness"
)

// The checks in hooks_wiring.go read one half of a hook command:
//
//	var hookInvocation = regexp.MustCompile(`vibe-agent\s+hook\s+([a-z][a-z-]*)`)
//
// That is the vibe-agent half. It proves this build understands the event, and
// it has never proved the other two things a working hook needs: that the host
// will ever fire the key the config filed it under, and that the command will
// resolve the right directory when it does.
//
// Both gaps fail silently, which is why they survived five fixes. A config key
// the host does not publish is simply never called, and a path resolved against
// the wrong directory reads state that is not there. In both cases every check
// stays green, every hook exits 0, and the control plane reports nothing.
//
// These two checks read the host half, against the contract table in
// internal/harness. One source, so correcting the table corrects the checks and
// the reference document together.

// unpublishedKeys returns the config keys their host never fires, and how many
// configs were examined.
//
// Split from the reporting so a test can assert on which keys were caught
// rather than on how many checks failed. The check reports once with every
// offender joined, so a count of failures is always one and proves nothing
// about what was found.
func unpublishedKeys(workspaceRoot string) (offenders []string, checked int) {
	for _, contract := range harness.HostContracts() {
		keys, ok := readHookKeys(filepath.Join(workspaceRoot, contract.ConfigPath))
		if !ok {
			continue // a consumer repo need not wire every host
		}
		checked++
		published := contract.HostKeys()
		for _, key := range keys {
			if !contains(published, key) {
				offenders = append(offenders, fmt.Sprintf("%q in %s", key, contract.ConfigPath))
			}
		}
	}
	sort.Strings(offenders)
	return offenders, checked
}

// checkHostEventKeys reports config keys the host does not publish.
func checkHostEventKeys(report *diagnostics, workspaceRoot string) {
	offenders, checked := unpublishedKeys(workspaceRoot)
	if checked == 0 {
		return
	}
	report.check(fmt.Sprintf("every hook key is one its host publishes (%d configs)", checked),
		len(offenders) == 0,
		fmt.Sprintf("%s: the host never fires this key, so the hook is registered and dead. "+
			"Compare against %s, generated from internal/harness/contracts.go",
			strings.Join(offenders, "; "), harness.HostContractsDoc))
}

// checkHookPathsResolve reports a hook command that depends on the working
// directory its host happens to run it in.
//
// Only Claude Code documents a project-directory variable, so a relative path
// in any other host's config is resolved against a directory nobody has
// specified. The failure is quiet: the interpreter reports a missing file, the
// host logs it somewhere the person is not looking, and the hook simply never
// does its work.
//
// `vibe-agent hook` itself is exempt, and deliberately: it discovers its own
// workspace by walking up from wherever it starts, so it is the one command
// here that does not need the config to solve this. A script path has no such
// recourse.
func cwdDependentPaths(workspaceRoot string) (offenders []string, checked int) {
	for _, contract := range harness.HostContracts() {
		commands, ok := readHookCommands(filepath.Join(workspaceRoot, contract.ConfigPath))
		if !ok {
			continue
		}
		checked++
		for _, command := range commands {
			if hookInvocation.MatchString(command) {
				continue
			}
			for _, path := range relativePaths(command) {
				offenders = append(offenders, fmt.Sprintf("%q in %s", path, contract.ConfigPath))
			}
		}
	}
	sort.Strings(offenders)
	return offenders, checked
}

func checkHookPathsResolve(report *diagnostics, workspaceRoot string) {
	offenders, checked := cwdDependentPaths(workspaceRoot)
	if checked == 0 {
		return
	}
	report.check(fmt.Sprintf("every hook command resolves its own paths (%d configs)", checked),
		len(offenders) == 0,
		fmt.Sprintf("%s: resolved against whatever directory the host runs the hook in, which only "+
			"Claude Code documents. Use the host's project-directory variable, or move the work into "+
			"`vibe-agent hook`, which discovers its own workspace", strings.Join(offenders, "; ")))
}

// scriptPath matches an argument that names a file inside the repository.
//
// A path is anything with a separator in it. Absolute paths, host variables and
// shell substitutions are all excluded by the prefixes below, because each of
// them resolves to something the config chose rather than something it inherited.
var scriptPath = regexp.MustCompile(`(?:^|\s)([^\s"']*[/\\][^\s"']*)`)

// resolvedPrefixes begin a path that does not depend on the working directory.
var resolvedPrefixes = []string{"/", "~", "$", "%", "\\\\"}

// substitution matches a shell command substitution, `$(...)` or backticks.
//
// These are removed before scanning, because they contain whitespace and the
// path scan tokenises on it. `$(git rev-parse --show-toplevel)/hooks/x.py`
// would otherwise be read as the token `--show-toplevel)/hooks/x.py`, which
// carries no `$` and looks exactly like the relative path it is the fix for.
var substitution = regexp.MustCompile(`\$\([^)]*\)|` + "`[^`]*`")

// relativePaths returns the arguments in a command that are resolved against
// the working directory.
func relativePaths(command string) []string {
	var found []string
	// Replaced with a marker rather than deleted, so what followed the
	// substitution stays attached to something resolved instead of becoming a
	// bare relative path.
	command = substitution.ReplaceAllString(command, "$$SUBST")
	for _, match := range scriptPath.FindAllStringSubmatch(command, -1) {
		candidate := strings.Trim(match[1], `"'`)
		if candidate == "" || isResolved(candidate) {
			continue
		}
		found = append(found, candidate)
	}
	return found
}

// isResolved reports whether a path already says where it starts from.
func isResolved(path string) bool {
	for _, prefix := range resolvedPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	// A Windows drive letter, C:\ or C:/.
	if len(path) >= 3 && path[1] == ':' && (path[2] == '\\' || path[2] == '/') {
		return true
	}
	// A substitution anywhere in the path, such as ${CLAUDE_PROJECT_DIR}/x.
	return strings.ContainsAny(path, "$%")
}

// readHookKeys returns the event keys a JSON config files its hooks under.
//
// Not TOML: .codex/config.toml registers an MCP server rather than hooks, and
// the regex pass in hooks_wiring.go already reads what it does register. A
// missing or unparseable file is not an error here, because a workspace need
// not wire every host and reporting a parse failure is the JSON schema's job.
func readHookKeys(path string) ([]string, bool) {
	if strings.HasSuffix(filepath.ToSlash(path), ".kimi/hooks.toml") {
		return readKimiHookEvents(path)
	}
	hooks, ok := readHooksObject(path)
	if !ok {
		return nil, false
	}
	keys := make([]string, 0, len(hooks))
	for key := range hooks {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys, true
}

// readHookCommands returns every command string under the hooks object.
//
// Under the hooks object specifically, not anywhere in the file. Claude's
// settings.json also has a statusLine command and a permissions block full of
// command patterns, and neither is a hook.
func readHookCommands(path string) ([]string, bool) {
	hooks, ok := readHooksObject(path)
	if !ok {
		return nil, false
	}
	var commands []string
	for _, entry := range hooks {
		collectCommands(entry, &commands)
	}
	sort.Strings(commands)
	return commands, true
}

// kimiHookEvent matches `event = "PreToolUse"` lines in a TOML hooks snippet.
var kimiHookEvent = regexp.MustCompile(`(?m)^\s*event\s*=\s*"([^"]+)"`)

func readKimiHookEvents(path string) ([]string, bool) {
	raw, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, false
	}
	matches := kimiHookEvent.FindAllStringSubmatch(string(raw), -1)
	if len(matches) == 0 {
		return nil, false
	}
	keys := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) > 1 {
			keys = append(keys, match[1])
		}
	}
	sort.Strings(keys)
	return keys, true
}

// readHooksObject parses a JSON config and returns its hooks object.
func readHooksObject(path string) (map[string]json.RawMessage, bool) {
	if filepath.Ext(path) != ".json" {
		return nil, false
	}
	raw, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, false
	}
	if hooks, ok := readAntigravityHooksObject(path, raw); ok {
		return hooks, true
	}
	var file struct {
		Hooks map[string]json.RawMessage `json:"hooks"`
	}
	if err := json.Unmarshal(raw, &file); err != nil {
		return nil, false
	}
	if len(file.Hooks) == 0 {
		return nil, false
	}
	return file.Hooks, true
}

// readAntigravityHooksObject parses .agents/hooks.json, where hook-set names
// map to objects carrying PreToolUse and siblings rather than a top-level
// hooks key.
func readAntigravityHooksObject(path string, raw []byte) (map[string]json.RawMessage, bool) {
	if filepath.Base(path) != "hooks.json" || filepath.Base(filepath.Dir(path)) != ".agents" {
		return nil, false
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		return nil, false
	}
	merged := map[string]json.RawMessage{}
	for _, groupRaw := range top {
		var group map[string]json.RawMessage
		if err := json.Unmarshal(groupRaw, &group); err != nil {
			continue
		}
		for key, value := range group {
			if key == "enabled" {
				continue
			}
			merged[key] = value
		}
	}
	if len(merged) == 0 {
		return nil, false
	}
	return merged, true
}

// collectCommands walks a hook entry for "command" strings.
//
// A walk rather than a struct, because the nesting differs per host: Cursor
// puts the command directly on the entry, Claude wraps it in a second "hooks"
// array beside a matcher. Decoding one shape would silently read nothing from
// the other.
func collectCommands(raw json.RawMessage, into *[]string) {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err == nil {
		for key, value := range object {
			if key == "command" {
				var command string
				if err := json.Unmarshal(value, &command); err == nil {
					*into = append(*into, command)
					continue
				}
			}
			collectCommands(value, into)
		}
		return
	}

	var list []json.RawMessage
	if err := json.Unmarshal(raw, &list); err == nil {
		for _, item := range list {
			collectCommands(item, into)
		}
	}
}
