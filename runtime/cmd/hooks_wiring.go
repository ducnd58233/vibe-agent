package main

import (
	"context"
	"errors"
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
	"github.com/ducnd58233/vibe-agent/runtime/internal/state"
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
// workspace root.
//
// The workspace, because that is the directory a host is opened on and the only
// one whose config it reads. A repository that mounts the toolkit at
// .vibe-agent/ gets that copy's settings.json too, and it is wiring for opening
// the toolkit standalone: nothing in it fires for the repository around it.
// Checking the toolkit's copy reported a workspace as wired while every hook in
// it was dead.
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
		raw, err := os.ReadFile(filepath.Clean(path))
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

// errNotInstalled marks the one state this file must not report as a defect.
//
// An absent binary is a supported configuration, said so in two places: the
// runtime is "optional: every asset works with this binary absent", and a missing
// one "makes every hook a quiet no-op". Reporting it as a failure would contradict
// the design and fail every environment that runs the binary by path instead of
// installing it, which is what a test harness does.
//
// Staleness is the opposite. Nothing supports it, nothing announces it, and the
// hooks half-work. That stays a failure.
var errNotInstalled = errors.New("vibe-agent is not on PATH")

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
		return "", nil, errNotInstalled
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
func checkHookWiring(report *diagnostics, workspaceRoot string) {
	registered, err := registeredEvents(workspaceRoot)
	if err != nil {
		// No wiring is a preference until the workspace has runs. Then it is a
		// half-connected control plane: the loop is being driven through the CLI
		// while nothing journals a tool call, and the symptom is a memory
		// database that stays empty however long the work goes on.
		if hasRunState(workspaceRoot) {
			report.check("the workspace wires the control plane it is running", false,
				fmt.Sprintf("%v, yet tmp/ holds run state. Runs advance and nothing records "+
					"what the session did, so memory stays empty. Add the hook blocks from the "+
					"toolkit's %s to the one in this workspace", err, hookConfigs[0]))
			return
		}
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

	checkOutcomePair(report, registered)

	// The staleness comparison. Its own failure is reported rather than skipped:
	// "could not ask" is not "the binary is fine". The one exception is an absent
	// binary, which the design supports.
	binary, supported, err := pathBinaryEvents(context.Background())
	switch {
	case errors.Is(err, errNotInstalled):
		fmt.Println("  note  no vibe-agent on PATH, so the hooks these configs register do not run. " +
			"That is a supported state: every asset works without it. " +
			"Install with `cd runtime && make install` to turn them on.")
		return
	case err != nil:
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

// hasRunState reports whether anything has ever started a run here.
//
// Presence of a manifest, not its status. A finished run is still evidence that
// this workspace drives the control plane and therefore wants the hooks that
// record it.
func hasRunState(workspaceRoot string) bool {
	entries, err := os.ReadDir(filepath.Join(workspaceRoot, "tmp"))
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if _, err := os.Stat(state.ManifestPath(workspaceRoot, entry.Name())); err == nil {
			return true
		}
	}
	return false
}

// checkOutcomePair reports a config that registers one half of a tool call's
// outcome and not the other.
//
// Wiring a subset of events is a legitimate choice, so doctor does not ask for
// all of them. These two are not a subset of each other: a host fires exactly
// one per tool call, post-tool-use on success and post-tool-use-failure on
// failure. A config naming only the first therefore does not record less, it
// records the wrong half. The journal exists to remember what broke, and what
// broke is precisely what it never saw.
//
// This is the check that would have caught the defect it was written for. The
// journal ran for months against a config with no failure hook, every test
// green, because nothing compared the pair.
func checkOutcomePair(report *diagnostics, registered map[string][]string) {
	success, hasSuccess := registered[string(harness.EventPostToolUse)]
	_, hasFailure := registered[string(harness.EventPostToolUseFailure)]
	if !hasSuccess || hasFailure {
		return
	}
	report.check("tool-call outcomes are wired in both halves", false, fmt.Sprintf(
		"%s registers %s but not %s, so every successful tool call is journalled "+
			"and every failed one is dropped; add %s alongside it",
		strings.Join(success, ", "), harness.EventPostToolUse,
		harness.EventPostToolUseFailure, harness.EventPostToolUseFailure))
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
