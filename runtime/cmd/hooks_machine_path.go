package main

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/ducnd58233/vibe-agent/runtime/internal/harness"
)

// rootFlagValue captures the value given to --workspace or --toolkit, quoted
// or bare, in either the "--flag value" or "--flag=value" form.
// Quotes, and the backslashes that escape them inside JSON or TOML strings,
// are trimmed from the token afterwards rather than parsed here.
var rootFlagValue = regexp.MustCompile(`--(workspace|toolkit)[= ]+(\S+)`)

// machineRoot matches a value that names one machine's filesystem: a POSIX
// absolute path, a home-relative path, or a Windows drive path.
var machineRoot = regexp.MustCompile(`^(?:/|~|[A-Za-z]:[\\/])`)

func flagValue(token string) string {
	return strings.Trim(token, `"'\`)
}

// machinePathHookCommands returns `vibe-agent hook` commands that pin a root to
// one machine's path, and how many configs were read.
//
// --workspace never needs a path: the command walks up to its workspace. A
// --toolkit inside the workspace is found the same way, so pinning it only
// makes the config valid on the machine that wrote it. A toolkit outside the
// workspace is different: discovery cannot reach it, the install is
// machine-local by nature, and its path is the one thing the config must carry.
// A host variable such as ${CLAUDE_PROJECT_DIR} is fine, and so is an absolute
// script path for a non-vibe-agent command, which has no discovery at all.
func machinePathHookCommands(workspaceRoot string) (offenders []string, checked int) {
	for _, contract := range harness.HostContracts() {
		commands, ok := readHookCommands(filepath.Join(workspaceRoot, contract.ConfigPath))
		if !ok {
			continue
		}
		checked++
		for _, command := range commands {
			if !hookInvocation.MatchString(command) {
				continue
			}
			for _, match := range rootFlagValue.FindAllStringSubmatch(command, -1) {
				value := flagValue(match[2])
				if !machineRoot.MatchString(value) {
					continue
				}
				if match[1] == "toolkit" && !insideWorkspace(workspaceRoot, value) {
					continue
				}
				offenders = append(offenders, fmt.Sprintf("--%s %q in %s", match[1], value, contract.ConfigPath))
			}
		}
	}
	sort.Strings(offenders)
	return offenders, checked
}

func checkHookMachinePaths(report *diagnostics, workspaceRoot string) {
	offenders, checked := machinePathHookCommands(workspaceRoot)
	if checked == 0 {
		return
	}
	report.check(fmt.Sprintf("hook commands carry no machine path (%d configs)", checked),
		len(offenders) == 0,
		fmt.Sprintf("%s: valid only on the machine that wrote it. Drop the flag; "+
			"`vibe-agent hook` finds the workspace, and a toolkit inside it, by walking up. The link script leaves an existing config "+
			"alone, so edit this one by hand", strings.Join(offenders, "; ")))
}

// insideWorkspace reports whether path lies in or under workspaceRoot. A path it
// cannot relate (another volume, or a POSIX-style path on Windows) counts as
// outside, which errs toward accepting a machine-local toolkit install.
func insideWorkspace(workspaceRoot, path string) bool {
	root, err := filepath.Abs(workspaceRoot)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(root, filepath.Clean(path))
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}
