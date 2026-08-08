package harness

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ducnd58233/vibe-agent/runtime/internal/state"
)

// BlockError stops the action a hook was called about instead of commenting on
// it. It is the one place this package is allowed to interfere with a session.
//
// Claude Code treats exit 2 as the only hard block: stdout is ignored and
// stderr is handed back to the model. The softer alternative, a JSON
// permissionDecision of "deny", fails open, because one stray line on stdout
// makes the JSON unparseable and the call proceeds. A gate in front of an
// irreversible action has to fail closed, so this maps to exit 2 in main.
type BlockError struct{ Reason string }

func (e *BlockError) Error() string { return e.Reason }

// The delivery graph names approve_merge "the only gate in front of an
// irreversible action". These are the checks that node reads, so the gate
// enforces what the graph already declares rather than inventing a second rule
// that could drift away from it.
const (
	shipCheck          = "ship"
	mergeApprovedCheck = "merge_approved"
)

// protectedRefs are the branches a push to which reaches everyone else.
var protectedRefs = map[string]bool{"main": true, "master": true}

// gate decides whether a tool call may run, and answers in whichever form the
// host understands.
//
// A payload this package cannot parse arrives here empty, and an empty payload
// matches no guard, so unreadable stdin lets the call through. That is the
// deliberate side to fail on: refusing every tool call whenever a host changes
// its JSON would wedge the session completely, while the thing actually being
// guarded is also written down in the run manifest, which this cannot corrupt.
func gate(req Request, body payload, out io.Writer) error {
	blocked := verdict(req, body)
	if blocked == nil {
		return nil
	}
	if req.Client == ClientCursor {
		// Cursor's beforeShellExecution decides through JSON rather than exit
		// codes, so the same verdict has to travel a different way.
		return write(out, map[string]any{
			"permission":   "deny",
			"agentMessage": blocked.Reason,
			"userMessage":  "vibe-agent blocked a command that would bypass the delivery graph.",
		})
	}
	return blocked
}

// verdict runs every guard this hook enforces. The first refusal wins.
func verdict(req Request, body payload) *BlockError {
	if blocked := shellVerdict(req, body.shellCommand()); blocked != nil {
		return blocked
	}
	return stateWriteVerdict(req, body)
}

// shellVerdict returns a BlockError when the command is the irreversible step
// and the run has not earned it, and nil in every other case.
func shellVerdict(req Request, command string) *BlockError {
	action, guarded := irreversibleAction(command, req.WorkspaceRoot)
	if !guarded {
		return nil
	}

	runs := activeRuns(req.WorkspaceRoot)
	if len(runs) == 0 {
		// Nothing is claiming to manage this workspace, so there is no state to
		// enforce. Blocking here would break every repo that uses the toolkit
		// without starting a run.
		return nil
	}
	for _, run := range runs {
		if passed(run, mergeApprovedCheck) {
			return nil
		}
	}
	return &BlockError{Reason: blockedReason(action, runs)}
}

// runStateFile matches a run's own bookkeeping: the manifest that says where a
// run is, and the append-only log that says how it got there.
var runStateFile = regexp.MustCompile(`(^|/)tmp/[^/]+/(manifest\.json|events\.ndjson)$`)

// shellWriters are the commands that change a file rather than read it. A
// redirection is handled separately, since it has no command word of its own.
var shellWriters = map[string]bool{
	"rm": true, "mv": true, "cp": true, "tee": true,
	"truncate": true, "dd": true, "sed": true, "install": true, "ln": true,
}

// stateWriteVerdict refuses to let anything but the runtime write a run's own
// state.
//
// The merge gate ends by asking the model not to edit the manifest to get past
// it. Asking is the wrong instrument in front of a file whose whole purpose is
// to be the thing that cannot be asserted: a hand-written check turns evidence
// into model output with extra steps, and every guard downstream reads it as
// real.
//
// With no active run there is nothing to protect, and refusing would break
// every workspace that uses this toolkit without starting a run.
func stateWriteVerdict(req Request, body payload) *BlockError {
	if len(activeRuns(req.WorkspaceRoot)) == 0 {
		return nil
	}

	if target := body.writeTarget(); protectedRunFile(target, req.WorkspaceRoot) {
		return &BlockError{Reason: stateWriteReason(target)}
	}
	if target, found := shellWritesRunState(body.shellCommand(), req.WorkspaceRoot); found {
		return &BlockError{Reason: stateWriteReason(target)}
	}
	return nil
}

func stateWriteReason(target string) string {
	return strings.Join([]string{
		fmt.Sprintf("Blocked: %s is run state, and only the runtime writes it.", target),
		"A hand-edited manifest is model output that every later guard reads as recorded evidence.",
		"Record the result properly instead:",
		"  vibe-agent checkpoint --slug <slug> --check <name> --source <exit_code|file_assert|ci_api|human_event> --passed",
		"Reading these files is fine. Writing them is not.",
	}, "\n")
}

// protectedRunFile reports whether a path points at a run's manifest or event
// log, whichever way the caller happened to spell it.
func protectedRunFile(path, workspaceRoot string) bool {
	trimmed := strings.Trim(strings.TrimSpace(path), `"'`)
	if trimmed == "" {
		return false
	}
	native := filepath.Clean(filepath.FromSlash(trimmed))
	if filepath.IsAbs(native) {
		if relative, err := filepath.Rel(workspaceRoot, native); err == nil {
			native = relative
		}
	}
	return runStateFile.MatchString(filepath.ToSlash(native))
}

// shellWritesRunState finds a run state file on the receiving end of a shell
// command that would change it.
//
// Like the rest of this file it over-approximates rather than parsing a shell,
// so it may inspect one extra word, never one fewer.
func shellWritesRunState(command, workspaceRoot string) (string, bool) {
	for _, segment := range shellSegments(command) {
		fields := strings.Fields(segment)
		if len(fields) == 0 {
			continue
		}
		writes := strings.Contains(segment, ">")
		for _, field := range fields {
			if shellWriters[strings.TrimSuffix(strings.ToLower(baseName(field)), ".exe")] {
				writes = true
				break
			}
		}
		if !writes {
			continue
		}
		for _, field := range fields {
			if candidate := strings.TrimLeft(field, "<>&"); protectedRunFile(candidate, workspaceRoot) {
				return candidate, true
			}
		}
	}
	return "", false
}

func baseName(field string) string {
	base := strings.Trim(field, `"'`)
	if cut := strings.LastIndexAny(base, `/\`); cut >= 0 {
		base = base[cut+1:]
	}
	return base
}

func passed(run *state.Run, name string) bool {
	check, ok := run.Checks[name]
	return ok && check.Passed
}

// blockedReason is read by the model, so it names the missing evidence and the
// command that records it rather than only refusing.
func blockedReason(action string, runs []*state.Run) string {
	lines := []string{
		fmt.Sprintf("Blocked: %s is the irreversible step of the delivery graph, and no active run has passed %s.", action, mergeApprovedCheck),
	}
	for _, run := range runs {
		lines = append(lines, fmt.Sprintf("  run %s is at node %s: %s", run.Slug, orDash(run.CurrentNode), missingEvidence(run)))
	}
	lines = append(lines,
		"Finish the run through /ship, then record the human approval:",
		"  vibe-agent checkpoint --slug <slug> --check "+mergeApprovedCheck+" --source human_event --passed",
		"Do not edit tmp/<slug>/manifest.json to get past this.")
	return strings.Join(lines, "\n")
}

func missingEvidence(run *state.Run) string {
	if !passed(run, shipCheck) {
		return "check " + shipCheck + " has not passed, so approve_merge is not reachable yet"
	}
	return "check " + mergeApprovedCheck + " has not been recorded"
}

// irreversibleAction reports whether a shell command performs the outward,
// hard-to-walk-back step of the delivery graph.
//
// Exactly two commands qualify. Pushing a task branch is routine and happens at
// open_pr, well before the ship gate, so gating every push would wedge the loop
// this is meant to protect; only a push whose destination is a protected branch
// counts.
func irreversibleAction(command, workspaceRoot string) (string, bool) {
	for _, segment := range shellSegments(command) {
		fields := strings.Fields(segment)
		if branch, ok := gitPushTarget(fields, workspaceRoot); ok {
			return "git push to " + branch, true
		}
		if _, ok := after(fields, "gh", "pr", "merge"); ok {
			return "gh pr merge", true
		}
	}
	return "", false
}

// shellSegments splits a command line into the parts that run as separate
// commands, so a push hidden behind && is still seen.
//
// This is deliberately not a shell parser. It over-approximates, which for a
// guard means it may look at one extra fragment, never one fewer.
func shellSegments(command string) []string {
	replacer := strings.NewReplacer("&&", "\n", "||", "\n", ";", "\n", "|", "\n")
	return strings.Split(replacer.Replace(command), "\n")
}

// gitPushTarget returns the protected branch a push would land on, if any.
func gitPushTarget(fields []string, workspaceRoot string) (string, bool) {
	rest, ok := after(fields, "git", "push")
	if !ok {
		return "", false
	}
	positional := positionalArgs(rest)

	// With no refspec, git pushes the current branch, so the branch has to come
	// from the repository rather than the command line.
	if len(positional) <= 1 {
		branch := currentBranch(workspaceRoot)
		return branch, protectedRefs[branch]
	}
	for _, spec := range positional[1:] {
		if dst := refspecDestination(spec); protectedRefs[dst] {
			return dst, true
		}
	}
	return "", false
}

// after finds a subcommand anywhere in a segment, so a leading environment
// assignment or an absolute path to the binary does not hide it.
func after(fields []string, binary string, sub ...string) ([]string, bool) {
	for i, field := range fields {
		if !isBinary(field, binary) {
			continue
		}
		rest := fields[i+1:]
		if matchesWords(rest, sub) {
			return rest[len(sub):], true
		}
	}
	return nil, false
}

func matchesWords(rest, want []string) bool {
	if len(rest) < len(want) {
		return false
	}
	for i, word := range want {
		if rest[i] != word {
			return false
		}
	}
	return true
}

// isBinary matches a command word against a program name, allowing a path
// prefix and the Windows .exe suffix. Both separators are handled here rather
// than with filepath.Base, because the command may have been written on a
// different platform than the one evaluating it.
func isBinary(field, name string) bool {
	return strings.TrimSuffix(baseName(field), ".exe") == name
}

// positionalArgs drops flags. -o is the only push flag that takes a separate
// value; the rest spell theirs with an equals sign.
func positionalArgs(fields []string) []string {
	var positional []string
	for i := 0; i < len(fields); i++ {
		field := fields[i]
		if !strings.HasPrefix(field, "-") {
			positional = append(positional, field)
			continue
		}
		if field == "-o" || field == "--push-option" {
			i++
		}
	}
	return positional
}

// refspecDestination returns the remote side of a refspec: the part after the
// colon when there is one, the whole thing otherwise. A leading + forces the
// push and a refs/heads/ prefix spells out what git would infer; neither
// changes where the push lands.
//
// An empty source with a destination, as in :main, deletes the remote branch,
// which this treats the same as writing to it.
//
// Quotes are stripped because fields were split on whitespace alone: without
// this, `git push origin "main"` compares "main" against main and slips past.
func refspecDestination(spec string) string {
	dst := strings.Trim(spec, `"'`)
	if colon := strings.LastIndex(dst, ":"); colon >= 0 {
		dst = dst[colon+1:]
	}
	dst = strings.Trim(dst, `"'`)
	dst = strings.TrimPrefix(dst, "+")
	return strings.TrimPrefix(dst, "refs/heads/")
}

// currentBranch reads HEAD directly rather than shelling out to git, so the
// gate stays fast enough to sit in front of every shell call and cannot be
// affected by the command it is judging.
//
// A detached HEAD yields a commit sha, which is not a protected ref, so it
// falls through to allowed.
func currentBranch(workspaceRoot string) string {
	gitPath := filepath.Join(workspaceRoot, ".git")
	info, err := os.Stat(gitPath)
	if err != nil {
		return ""
	}

	dir := gitPath
	if !info.IsDir() {
		// A worktree or submodule keeps .git as a file pointing elsewhere.
		raw, err := os.ReadFile(gitPath)
		if err != nil {
			return ""
		}
		line := strings.TrimSpace(string(raw))
		if !strings.HasPrefix(line, "gitdir:") {
			return ""
		}
		dir = strings.TrimSpace(strings.TrimPrefix(line, "gitdir:"))
		if !filepath.IsAbs(dir) {
			dir = filepath.Join(workspaceRoot, dir)
		}
	}

	raw, err := os.ReadFile(filepath.Join(dir, "HEAD"))
	if err != nil {
		return ""
	}
	return strings.TrimPrefix(strings.TrimSpace(string(raw)), "ref: refs/heads/")
}
