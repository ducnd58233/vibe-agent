package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/harness"
)

// This file answers one question doctor could not: is the binary that will
// actually answer the hooks the same one that was just built?
//
// The failure it exists to catch happened. A host config registered
// `post-tool-use`, the source implemented it, and the binary on PATH was ten days
// older than both. Five hooks kept working and the sixth was refused, which reads
// as a broken hook rather than as an out-of-date install. Nothing in the repo
// compared the two: `make install` writes without checking, and the version
// string is "dev" on both sides so comparing versions proves nothing.
//
// So the comparison is behavioural. Ask the binary on PATH what it handles, and
// diff that against what the configs call for.

// hookConfigs are the host files that register hook commands, relative to the
// toolkit root.
var hookConfigs = []string{
	filepath.Join(".claude", "settings.json"),
	filepath.Join(".cursor", "hooks.json"),
}

// hookInvocation matches `vibe-agent hook <event>` however a config quotes it.
var hookInvocation = regexp.MustCompile(`vibe-agent\s+hook\s+([a-z][a-z-]*)`)

// registeredEvents returns the events every host config asks for, and which file
// asked, so a failure names something to open.
func registeredEvents(toolkitRoot string) (map[string][]string, error) {
	found := map[string][]string{}
	var read int
	for _, relative := range hookConfigs {
		path := filepath.Join(toolkitRoot, relative)
		raw, err := os.ReadFile(path)
		if err != nil {
			continue // a consumer repo need not wire every host
		}
		read++
		for _, match := range hookInvocation.FindAllStringSubmatch(string(raw), -1) {
			event := match[1]
			if !contains(found[event], relative) {
				found[event] = append(found[event], relative)
			}
		}
	}
	if read == 0 {
		return nil, fmt.Errorf("no host hook config under %s", toolkitRoot)
	}
	return found, nil
}

// exeSuffix is the executable extension this platform requires.
func exeSuffix() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}

// extensionlessOnPath finds a vibe-agent that exists on PATH but lacks the
// extension this platform needs to resolve it by name.
//
// Only Windows has that state. Returning "" elsewhere keeps the caller's message
// honest rather than inventing a cause.
func extensionlessOnPath() string {
	if exeSuffix() == "" {
		return ""
	}
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		if dir == "" {
			continue
		}
		candidate := filepath.Join(dir, "vibe-agent")
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return ""
}

func contains(list []string, want string) bool {
	for _, item := range list {
		if item == want {
			return true
		}
	}
	return false
}

// pathBinaryEvents asks the vibe-agent on PATH which events it handles.
//
// Deliberately not this process's own list. doctor may be running from
// ./dist/vibe-agent or `go run` while the hooks are answered by whatever is on
// PATH, and it is precisely that gap this reports.
//
// An older binary predates --events and exits non-zero, which is itself the
// answer: it is old enough to lack a flag added alongside this check.
func pathBinaryEvents(ctx context.Context) (path string, events []string, err error) {
	path, err = exec.LookPath("vibe-agent")
	if err != nil {
		// Distinguish "absent" from "present but unresolvable", because the fixes
		// differ and one of them is invisible. An extensionless file on Windows is
		// executable from Git Bash and not by name from anything else, so hooks
		// appear to work while every native lookup fails.
		if stranded := extensionlessOnPath(); stranded != "" {
			return stranded, nil, fmt.Errorf(
				"%s exists but has no %s extension, so only a POSIX shell can run it by name; "+
					"rebuild with `cd runtime && make install`", stranded, exeSuffix())
		}
		return "", nil, fmt.Errorf("no vibe-agent on PATH; hooks call it by name, so none of them run: %w", err)
	}

	// A resolvable binary is not the end of it. On Windows an extensionless
	// sibling shadows the .exe for any POSIX shell, so `vibe-agent` means one
	// file to Git Bash and another to everything else. Hooks then run whichever
	// the host's shell picked, and the two can be different builds: this was
	// found with a ten-day-old extensionless file answering while a fresh .exe
	// sat beside it unused.
	if shadow := extensionlessOnPath(); shadow != "" {
		return path, nil, fmt.Errorf(
			"%s and %s both exist, and a POSIX shell resolves the one without %s first, "+
				"so hooks may run a different build than this one; delete %s",
			shadow, path, exeSuffix(), shadow)
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, path, "hook", "--events").Output()
	if err != nil {
		return path, nil, fmt.Errorf("%s does not understand `hook --events`, which means it predates this check: %w", path, err)
	}
	for _, line := range strings.Split(string(out), "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			events = append(events, trimmed)
		}
	}
	return path, events, nil
}

// checkHookWiring reports whether every registered event will actually be
// handled.
//
// Two comparisons, because they fail for different reasons and want different
// fixes. Against this build: a config naming an event no version implements is a
// typo. Against the binary on PATH: a config ahead of the install is a missing
// `make install`.
func checkHookWiring(report *diagnostics, toolkitRoot string) {
	registered, err := registeredEvents(toolkitRoot)
	if err != nil {
		fmt.Printf("  note  %v\n", err)
		return
	}

	events := make([]string, 0, len(registered))
	for event := range registered {
		events = append(events, event)
	}
	sort.Strings(events)

	var unimplemented []string
	for _, event := range events {
		if !harness.Handles(harness.Event(event)) {
			unimplemented = append(unimplemented, fmt.Sprintf("%s (in %s)",
				event, strings.Join(registered[event], ", ")))
		}
	}
	report.check(fmt.Sprintf("every registered hook event is implemented (%d)", len(events)),
		len(unimplemented) == 0,
		"no handler for "+strings.Join(unimplemented, "; ")+
			"; either the event name is wrong or this build is older than the config")

	// The staleness comparison. Its own failure is reported rather than skipped:
	// "could not ask" is not "the binary is fine".
	binary, supported, err := pathBinaryEvents(context.Background())
	if err != nil {
		report.check("the vibe-agent on PATH handles every registered hook", false, err.Error())
		return
	}

	var missing []string
	for _, event := range events {
		if !contains(supported, event) {
			missing = append(missing, event)
		}
	}
	report.check(fmt.Sprintf("the vibe-agent on PATH handles every registered hook (%s)", binary),
		len(missing) == 0,
		fmt.Sprintf("%s does not handle %s. It is older than the config that calls it; "+
			"run `cd runtime && make install`", binary, strings.Join(missing, ", ")))

	// Same binary, same build? A matching event list is necessary and not
	// sufficient: a stale install can still be missing commands and fixes that
	// added no event. Version is the only thing that can say, so it is only
	// reported when it can say something.
	if built := buildVersion(); built != "" && built != "dev" {
		if installed := binaryVersion(binary); installed != "" && installed != built {
			report.check("the vibe-agent on PATH is this build", false,
				fmt.Sprintf("%s reports %s, this build is %s; run `cd runtime && make install`",
					binary, installed, built))
		}
	}
}

func buildVersion() string { return version }

func binaryVersion(path string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, path, "version").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
