package harness

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/redact"
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

// conventionalRefs are the names that mean "everyone's branch" almost everywhere.
//
// A floor, not the definition. A repository that integrates on develop usually
// still has a main that means something, so these stay protected whatever the
// repository says its default is.
var conventionalRefs = map[string]bool{"main": true, "master": true}

// isProtected reports whether pushing to a branch reaches everyone else.
//
// The repository is asked rather than assumed. A fixed pair of names left every
// project that integrates on develop, trunk, or production silently unguarded:
// the push went through, nothing was said, and the guard looked like it was
// working because it never fired.
func isProtected(workspaceRoot, branch string) bool {
	if branch == "" {
		return false
	}
	if conventionalRefs[branch] {
		return true
	}
	return branch == repositoryDefaultBranch(workspaceRoot)
}

// defaultBranchCache keeps one answer per workspace.
//
// This runs in PreToolUse, so it is on the path of every shell command in a
// session. Reading two small files once per workspace is the difference between
// a guard nobody notices and a guard that taxes every call.
var defaultBranchCache sync.Map

// defaultBranch returns the branch this repository integrates on, or "".
//
// Read from the git directory rather than shelled out to, for the same reason
// currentBranch is: the gate must not spawn a process on every tool call, and
// the files it needs are two lines of text. refs/remotes/origin/HEAD is what
// `git remote set-head` writes and what clones get for free.
func repositoryDefaultBranch(workspaceRoot string) string {
	if cached, ok := defaultBranchCache.Load(workspaceRoot); ok {
		if branch, isString := cached.(string); isString {
			return branch
		}
	}
	branch := readDefaultBranch(workspaceRoot)
	defaultBranchCache.Store(workspaceRoot, branch)
	return branch
}

func readDefaultBranch(workspaceRoot string) string {
	dir := gitDir(workspaceRoot)
	if dir == "" {
		return ""
	}

	// The symbolic ref, as a file or as a line in packed-refs.
	if raw, err := os.ReadFile(filepath.Clean(filepath.Join(dir, "refs", "remotes", "origin", "HEAD"))); err == nil {
		if branch := afterPrefix(string(raw), "ref: refs/remotes/origin/"); branch != "" {
			return branch
		}
	}
	if raw, err := os.ReadFile(filepath.Clean(filepath.Join(dir, "packed-refs"))); err == nil {
		for _, line := range strings.Split(string(raw), "\n") {
			if branch := afterPrefix(line, "ref: refs/remotes/origin/"); branch != "" {
				return branch
			}
		}
	}
	return ""
}

// afterPrefix returns the trimmed remainder of a line after a prefix, or "".
func afterPrefix(line, prefix string) string {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, prefix) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(trimmed, prefix))
}

// gate decides whether a tool call may run, and answers in whichever form the
// host understands.
//
// A payload this package cannot parse arrives here empty, and an empty payload
// matches no guard, so unreadable stdin lets the call through. That is the
// deliberate side to fail on: refusing every tool call whenever a host changes
// its JSON would wedge the session completely, while the thing actually being
// guarded is also written down in the run manifest, which this cannot corrupt.
func gate(req Request, body payload, out io.Writer) error {
	// The safety gate first. The delegated cache can only make a call cheaper,
	// and a call this package is about to refuse should not be made cheaper.
	if blocked := verdict(req, body); blocked != nil {
		return deliverBlock(req, blocked, out)
	}
	// The WebFetch cache refuses too, by serving a stored page instead of
	// letting the fetch run. It is a refusal on the same event, so it leaves
	// through the same door.
	if blocked := sddCache(req, body, "sdd-cache-pre.py"); blocked != nil {
		return deliverBlock(req, blocked, out)
	}
	return nil
}

// deliverBlock hands one refusal to the host in the form that host understands.
//
// Separate from gate because gate is no longer the only source of a refusal:
// the delegated WebFetch cache also refuses, by serving cached content instead
// of letting the fetch run. Leaving the translation inside gate meant that
// refusal returned an error nobody rendered, so Cursor and Codex received exit
// 0 and an empty reply while Claude got the cached page. That is the same class
// of silent per-host divergence the contract table was added to catch, produced
// while fixing another one.
func deliverBlock(req Request, blocked *BlockError, out io.Writer) error {
	switch req.Client {
	case ClientCursor:
		// Cursor decides through JSON rather than exit codes, so the same
		// verdict has to travel a different way.
		//
		// snake_case, and that was the defect. This wrote agentMessage and
		// userMessage, which Cursor does not read: it honoured the deny and
		// discarded both messages, so the agent was refused with no stated
		// reason and retried. The same file spelled additional_context and
		// followup_message correctly, so this was one field pair out of step
		// rather than a convention anyone had chosen.
		//
		// "deny" and not "ask": beforeShellExecution accepts all three of allow,
		// deny and ask, while preToolUse accepts only allow and deny. Emitting
		// the value both events share is what lets one branch answer both, and
		// TestCursorNeverAsks keeps it that way.
		return write(out, map[string]any{
			"permission":    "deny",
			"agent_message": blocked.Reason,
			"user_message":  "vibe-agent blocked a command that would bypass the delivery graph.",
		})
	case ClientOpencode:
		// The plugin turns this into permission.ask returning status "deny",
		// which is opencode's only refusal path: tool.execute.before can throw,
		// but a thrown error reads to the model as a broken tool rather than a
		// decision about one.
		return write(out, map[string]any{
			"permission": "deny",
			"reason":     blocked.Reason,
		})
	case ClientCodex:
		// Codex ignores exit 2 outright. It was measured running the command
		// anyway while the hook exited 2 and printed the refusal, and blocking
		// it with this shape instead - the model then reported the reason back
		// verbatim. A gate that fails open is not a gate, so this is the branch
		// that had to be established by experiment rather than inference.
		return write(out, map[string]any{
			"hookSpecificOutput": map[string]any{
				"hookEventName":            "PreToolUse",
				"permissionDecision":       "deny",
				"permissionDecisionReason": blocked.Reason,
			},
		})
	case ClientClaude:
	}
	return blocked
}

// verdict runs every guard this hook enforces. The first refusal wins.
func verdict(req Request, body payload) *BlockError {
	// The danger list runs first. Every other refusal here is about the state
	// of one delivery run; this one is about the action itself, and it holds
	// whether or not a run exists.
	if blocked := dangerVerdict(req, body); blocked != nil {
		return blocked
	}
	if blocked := shellVerdict(req, body.shellCommand()); blocked != nil {
		return blocked
	}
	if blocked := credentialVerdict(body); blocked != nil {
		return blocked
	}
	if blocked := suppressionVerdict(req, body); blocked != nil {
		return blocked
	}
	return stateWriteVerdict(req, body)
}

// credentialLiteral matches high-confidence shapes from shared/redact. Gate
// stays on the narrow literal set; redact.Text adds format and contextual
// patterns for logs and UI.

// allowMarker lets a line hold one of these shapes on purpose, which a
// test fixture and this project's own documentation both need. It is a single
// greppable string rather than a config setting, so every exemption in the
// repository can be listed with one search and reviewed as a diff.
const allowMarker = "vibe-agent: allow-credential-literal" // sensitive-data-guard: allow this is the marker's definition, not a credential

// credentialVerdict refuses to let a live credential be written into the
// workspace.
//
// Unlike stateWriteVerdict this does not require an active state. Run state is
// meaningful only inside a run; a private key landing in source is the same
// event whether or not anyone started one, and gating it on run state would
// leave the common case, a repository using the toolkit without a run, wide
// open.
func credentialVerdict(body payload) *BlockError {
	text := body.writtenText()
	if text == "" {
		return nil
	}

	for line := range strings.SplitSeq(text, "\n") {
		if strings.Contains(line, allowMarker) {
			continue
		}
		for _, pattern := range redact.LiteralPatterns() {
			if match := pattern.FindString(line); match != "" {
				return &BlockError{Reason: credentialReason(match, body.writeTarget())}
			}
		}
	}
	return nil
}

// credentialReason names the shape without echoing the value, since a hook
// reason is written to logs and transcripts that are exactly the channels this
// gate exists to keep credentials out of.
func credentialReason(match, target string) string {
	shape := match
	if len(shape) > 12 {
		shape = shape[:12] + "..."
	}
	lines := []string{
		fmt.Sprintf("Blocked: this writes a live credential (%s) into the workspace.", shape), // sensitive-data-guard: allow refusal text; shape is truncated above
	}
	if target != "" {
		lines = append(lines, "  target: "+target)
	}
	return strings.Join(append(lines,
		"Committed history is permanent, so the only real fix afterwards is rotation.",
		"Read it from the configured secret source instead, and keep a placeholder in the file.",
		"If this shape is deliberate, such as a test fixture, mark that line with:",
		"  "+allowMarker,
		"That marker is greppable on purpose: every exemption should be reviewable as a diff.",
	), "\n")
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
		// without starting a state.
		return nil
	}
	for _, run := range runs {
		if passed(run, mergeApprovedCheck) {
			return nil
		}
	}
	return &BlockError{Reason: blockedReason(action, runs)}
}

// runStateFile matches a run's own bookkeeping under .agent-state/runs/.
var runStateFile = regexp.MustCompile(`(^|/)\.agent-state/runs/[^/]+/[^/]+/[0-9]+/(manifest\.json|events\.ndjson)$`)

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
// every workspace that uses this toolkit without starting a state.
func stateWriteVerdict(req Request, body payload) *BlockError {
	// The payload question first, then the disk.
	//
	// activeRuns reads the runs directory and parses every non-terminal
	// manifest, and this used to happen before asking whether the call writes
	// anything at all. For a Read, a Grep, or a Glob the answer is no and the
	// whole load was thrown away - on the path of every tool call, at a cost
	// that grows with every run a workspace has ever started.
	//
	// shellVerdict above already works this way round: it decides the command
	// is a push or a merge before it loads anything.
	// Both subjects, because there are two ways to write a run file: a Write or
	// Edit names one, and a shell command can redirect into one. Testing only
	// the first would have let a redirect through, which is the reordering
	// changing what the gate refuses rather than when it decides.
	target := body.writeTarget()
	command := body.shellCommand()
	if target == "" && command == "" {
		return nil
	}
	if len(activeRuns(req.WorkspaceRoot)) == 0 {
		return nil
	}

	if protectedRunFile(target, req.WorkspaceRoot) {
		return &BlockError{Reason: stateWriteReason(target)}
	}
	if target, found := shellWritesRunState(command, req.WorkspaceRoot); found {
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
		lines = append(lines, fmt.Sprintf("  run %s is at node %s: %s", run.Slug, orNotEntered(run.CurrentNode), missingEvidence(run)))
	}
	lines = append(lines,
		"Finish the run through /ship, then record the human approval:",
		"  vibe-agent checkpoint --slug <slug> --check "+mergeApprovedCheck+" --source human_event --passed",
		"Do not edit .agent-state/runs/.../manifest.json to get past this.")
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
	return strings.Split(replacer.Replace(stripComments(command)), "\n")
}

// stripComments removes shell comments before anything matches against the text.
//
// A comment cannot execute, so matching one refuses a description of a command
// rather than a command. Not theoretical: this gate refused a `cat` whose
// trailing comment named a destructive statement, and refused the goal string
// of the run that fixed it, because a pattern matched the words "truncate in
// silence". A gate that fires on English is one people route around, and a gate
// nobody trusts protects nothing.
//
// Quoted text stays in scope. Destructive commands are very often written
// inside quotes, so the stripper has to know where a quote begins and ends
// rather than cutting at the first hash it sees. Cutting blindly would lose
// real commands: in `echo "issue #1" && rm -rf /data` the hash sits inside
// quotes, and dropping the rest of the line would drop the command with it.
//
// A hash also only opens a comment at the start of a word, which is why
// `foo#bar` survives whole.
func stripComments(command string) string {
	var out strings.Builder
	out.Grow(len(command))

	var quote rune // zero outside quotes, otherwise the opening quote
	atWordStart := true
	inComment := false

	for _, r := range command {
		switch {
		case inComment:
			// A comment ends at the newline, and the next line is a new command.
			if r == '\n' {
				inComment = false
				out.WriteRune(r)
				atWordStart = true
			}
			continue
		case quote != 0:
			if r == quote {
				quote = 0
			}
		case r == '\'' || r == '"':
			quote = r
		case r == '#' && atWordStart:
			inComment = true
			continue
		}
		out.WriteRune(r)
		atWordStart = r == ' ' || r == '\t' || r == '\n'
	}
	return out.String()
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
		return branch, isProtected(workspaceRoot, branch)
	}
	for _, spec := range positional[1:] {
		if dst := refspecDestination(spec); isProtected(workspaceRoot, dst) {
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
	dir := gitDir(workspaceRoot)
	if dir == "" {
		return ""
	}
	raw, err := os.ReadFile(filepath.Clean(filepath.Join(dir, "HEAD")))
	if err != nil {
		return ""
	}
	return strings.TrimPrefix(strings.TrimSpace(string(raw)), "ref: refs/heads/")
}

// gitDir resolves a workspace's git directory, or "" if it has none.
func gitDir(workspaceRoot string) string {
	gitPath := filepath.Join(workspaceRoot, ".git")
	info, err := os.Stat(gitPath)
	if err != nil {
		return ""
	}
	if info.IsDir() {
		return gitPath
	}
	// A worktree or submodule keeps .git as a file pointing elsewhere.
	raw, err := os.ReadFile(filepath.Clean(gitPath))
	if err != nil {
		return ""
	}
	line := strings.TrimSpace(string(raw))
	if !strings.HasPrefix(line, "gitdir:") {
		return ""
	}
	dir := strings.TrimSpace(strings.TrimPrefix(line, "gitdir:"))
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(workspaceRoot, dir)
	}
	return dir
}
