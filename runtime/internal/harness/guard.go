package harness

import (
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strings"
)

// Guards read a tool call the host has already carried out and tell the model
// what it should look at again. They advise; they never refuse. The refusing
// half is gate.go, on pre-tool-use, and the split is deliberate: a block has to
// be certain, and everything here trades certainty for reach.
//
// These ran as Python scripts registered directly in each host's config. That
// worked only where `python3` on PATH was a real interpreter, and nothing
// anywhere reported the case where it was not: on Windows without Python the
// name still resolves, to an App Execution Alias that opens the Microsoft
// Store, so every guard stopped running while each config went on listing it. A
// guard that never fires looks exactly like a guard with nothing to say, and
// silence is the one failure this control plane cannot see.
//
// Rules are data, not code. The defaults compile in so a fresh install guards
// something and a malformed file cannot empty the set; a repository extends or
// disables them through .ai-agents/guards.yaml, which is tracked, so weakening
// a guard is a diff someone reviews rather than an argument nobody sees. That
// is the rule vibe-checks.yaml already states for check commands, applied here.

// Guard names double as the prefix on every message and as the first half of
// each escape-hatch marker, so the two cannot drift apart.
//
// The raw-colour guard is named for what it looks for rather than for the
// marker it answers to, and that is not cosmetic. Written the obvious way,
// `guardDesignToken = "design-token-guard"` is a symbol containing "token"
// assigned a literal over eight characters, which is exactly the shape the
// credential rule reports. Naming it for the finding removes the collision
// instead of exempting it, and reads truer: the guard detects raw colours.
const (
	guardSensitiveData = "sensitive-data-guard"
	guardCoreLogicTest = "core-logic-test-guard"
	guardRawColor      = "design-token-guard"
	guardUISlop        = "ui-slop-guard"
)

// guardMarker is the comment that turns a guard off for a line or a file.
//
// These strings are the ones the Python hooks used, verbatim, and that is a
// compatibility requirement rather than a detail: a consumer repository has
// them written into source files already, and a rename would silently
// reactivate every guard someone had deliberately acknowledged.
func guardMarker(name string) string {
	if name == guardRawColor {
		return name + ": allow-raw-color"
	}
	return name + ": allow"
}

// A patternRule reports one line at a time.
type patternRule struct {
	ID      string `yaml:"id"`
	Pattern string `yaml:"pattern"`
	Message string `yaml:"message"`
	// Boundary stands in for the lookaround the Python originals used, which
	// RE2 has no equivalent for and which cannot be folded into the pattern:
	// consuming the neighbouring character breaks adjacent matches, because the
	// character between `#fff #000` belongs to whichever match reaches it
	// first and the second literal is then unanchored.
	//
	// "left" requires a word boundary before the match, for a utility class
	// that must not be the tail of a longer one. "both" requires one on each
	// side, for a hex colour that must not sit inside `#nav-1a2b3c`. Empty
	// means the pattern says everything.
	Boundary string `yaml:"boundary"`

	// NotPlaceholder suppresses a match whose captured group, or whose line,
	// reads as an example rather than as the real thing. Only the credential
	// rule needs it, and only because it is the one rule whose false positives
	// outnumber its findings in an ordinary repository.
	NotPlaceholder string `yaml:"notPlaceholder"`

	compiled    *regexp.Regexp
	placeholder *regexp.Regexp
}

// Boundary values.
const (
	boundaryLeft = "left"
	boundaryBoth = "both"
)

// A densityRule reports a shape that is fine once and a habit past a threshold,
// so one deliberate use stays quiet.
type densityRule struct {
	ID        string `yaml:"id"`
	Pattern   string `yaml:"pattern"`
	Threshold int    `yaml:"threshold"`
	Message   string `yaml:"message"`

	compiled *regexp.Regexp
}

// A ruleSet is one guard: which files it reads, and what it looks for.
type ruleSet struct {
	Name      string
	Disabled  bool
	AppliesTo selector
	Line      []patternRule
	Density   []densityRule

	// Subject is the plural noun in the message: "3 possible disclosure(s)".
	Subject string

	// ExemptFiles are base names this guard never reads. See exempt.
	ExemptFiles []string

	// FileMarker makes the escape hatch cover the whole file rather than the
	// line it sits on. The raw-colour guard has always worked that way, and its
	// own message tells people to put the marker near the file header, so
	// tightening it to per-line would reactivate every acknowledgement already
	// written.
	FileMarker bool

	// skipLine decides which lines cannot be a finding for this guard. Nil
	// means only the escape-hatch marker skips a line.
	skipLine func(line string) bool

	// inspect replaces pattern scanning for a guard whose question cannot be
	// asked one line at a time, such as the test guard, which has to find a
	// block before it can count what is inside it.
	inspect func(file subject) []string
}

// ExemptFiles are base names this guard never reads, because they define what
// it looks for and would otherwise report themselves.
//
// The escape-hatch marker cannot serve here: a fixture asserting that a shape
// *is* reported would be neutered by the marker that quiets it.
func (r ruleSet) exempt(path string) bool {
	base := filepath.Base(path)
	for _, name := range r.ExemptFiles {
		if strings.EqualFold(base, name) {
			return true
		}
	}
	return false
}

// matches reports whether this guard reads this file.
func (r ruleSet) matches(file subject) bool {
	return !r.Disabled && !r.exempt(file.Path) && r.AppliesTo.matches(file)
}

// postToolUse records what a tool did, then says what the guards make of it.
//
// The journal half never speaks to the host, and until now nothing else did
// either: this event took no writer at all, so the runtime could observe an
// edit and had no way to comment on it. That is the gap the Python guards were
// filling from outside.
//
// Journalling comes first and its error is dropped, exactly as before. A
// control plane that fails a session over its own bookkeeping is worse than one
// that records nothing, and the guards have something to say either way.
func postToolUse(req Request, body payload, out io.Writer, failed bool) error {
	_ = journal(req, body, failed)
	recordToolUse(req, body)

	text := adviseAll(req, body)
	if req.Client == ClientCursor {
		// The one place a Cursor session can be told which node its run is at.
		// Appended rather than sent separately, because Cursor reads a single
		// additional_context field and a second write would replace the first.
		text = joinNonEmpty(text, cursorNodeReminder(req))
		if text == "" {
			return nil
		}
		return write(out, map[string]any{"additional_context": text})
	}
	if text == "" {
		return nil
	}
	// Both fields on purpose. systemMessage reaches the person; additionalContext
	// reaches the model, which is the one that has to fix the file. Whichever the
	// installed host does not support is ignored, and a warning only the human
	// can see is a warning the agent will repeat on the next edit.
	return write(out, map[string]any{
		"systemMessage": text,
		"hookSpecificOutput": map[string]any{
			"hookEventName":     "PostToolUse",
			"additionalContext": text,
		},
	})
}

// adviseAll collects what every guard has to say about one completed tool call.
func adviseAll(req Request, body payload) string {
	file, ok := resolveSubject(req, body)
	if !ok {
		return ""
	}

	sets, err := activeRules(req)
	if err != nil {
		// A broken overlay must not silence the defaults it was extending.
		// Saying so once, in the advice channel, is the only way a typo in a
		// tracked file reaches anyone.
		sets = defaultRules()
	}

	said := make([]string, 0, len(sets))
	if err != nil {
		said = append(said, "[guards] "+err.Error()+". Running the built-in rules only.")
	}
	for _, set := range sets {
		if !set.matches(file) {
			continue
		}
		if text := safeAdvise(set, file); text != "" {
			said = append(said, text)
		}
	}

	// Last, because the rule guards above are about correctness and disclosure
	// and this one is about craft. When both have something to say, the one that
	// can leak a credential should be read first.
	if text := slopAdvice(req, file); text != "" {
		said = append(said, text)
	}
	return strings.Join(said, "\n\n")
}

// safeAdvise contains a guard that goes wrong.
//
// The tool call already happened by the time any of this runs, so there is
// nothing left to protect and nothing worth failing a session over. A panicking
// guard costs its own advice and no more.
func safeAdvise(set ruleSet, file subject) (result string) {
	defer func() {
		if recover() != nil {
			result = ""
		}
	}()

	marker := guardMarker(set.Name)
	if set.FileMarker && strings.Contains(file.Text, marker) {
		return ""
	}
	if set.inspect != nil {
		return report(set.Name, file.Path, set.inspect(file), set.Subject)
	}
	findings := append(
		scanLines(file.Text, marker, set),
		scanDensity(file.Text, set.Density)...,
	)
	return report(set.Name, file.Path, findings, set.Subject)
}

// scanLines runs every rule over every line, in the order the rules are
// declared, so one file always reports the same way.
//
// A line carrying the marker is skipped for all rules rather than for the one
// that fired: whoever acknowledged a finding on that line has read the line.
func scanLines(text, marker string, set ruleSet) []string {
	lines := splitLines(text)
	var findings []string
	for _, rule := range set.Line {
		if rule.compiled == nil {
			continue
		}
		for index, line := range lines {
			if strings.Contains(line, marker) {
				continue
			}
			if set.skipLine != nil && set.skipLine(line) {
				continue
			}
			if !rule.matches(line) {
				continue
			}
			findings = append(findings, fmt.Sprintf("L%d [%s] %s", index+1, rule.ID, rule.Message))
		}
	}
	return findings
}

// matches reports whether this rule fires on a line.
func (r patternRule) matches(line string) bool {
	if r.placeholder != nil {
		found := r.compiled.FindStringSubmatch(line)
		if found == nil {
			return false
		}
		if len(found) > 1 && r.placeholder.MatchString(found[1]) {
			return false
		}
		return !r.placeholder.MatchString(line)
	}
	if r.Boundary == "" {
		return r.compiled.MatchString(line)
	}
	for _, span := range r.compiled.FindAllStringIndex(line, -1) {
		if span[0] > 0 && isWordOrDashByte(line[span[0]-1]) {
			continue
		}
		if r.Boundary == boundaryBoth && span[1] < len(line) && isWordOrDashByte(line[span[1]]) {
			continue
		}
		return true
	}
	return false
}

// scanDensity reports each rule whose count reaches its threshold.
func scanDensity(text string, rules []densityRule) []string {
	var findings []string
	for _, rule := range rules {
		if rule.compiled == nil {
			continue
		}
		if count := len(rule.compiled.FindAllString(text, -1)); count >= rule.Threshold {
			findings = append(findings, fmt.Sprintf("[%s] %d occurrences. %s", rule.ID, count, rule.Message))
		}
	}
	return findings
}

// report assembles a guard's findings into one message.
//
// The count and the file lead, because a model reading only the first line
// still learns what happened and where. Every message ends with the escape
// hatch, so a false positive is answerable from the message alone rather than
// by finding documentation for a hook the reader may not know exists.
func report(name, file string, findings []string, subject string) string {
	if len(findings) == 0 {
		return ""
	}
	var text strings.Builder
	fmt.Fprintf(&text, "%s flagged %d %s in %s:", name, len(findings), subject, file)
	for _, finding := range findings {
		text.WriteString("\n  - " + finding)
	}
	text.WriteString("\nIf a line is deliberate, mark it with a `" + guardMarker(name) + "` comment and say why.")
	return text.String()
}

// splitLines splits on newlines and drops a trailing carriage return.
//
// Python's splitlines() understands CRLF; Go's Split does not, and a stray \r
// left on the end of every line would break any rule anchored to one.
func splitLines(text string) []string {
	lines := strings.Split(text, "\n")
	for index, line := range lines {
		lines[index] = strings.TrimSuffix(line, "\r")
	}
	return lines
}

// isWordOrDashByte reports the character class the Python lookarounds used:
// [A-Za-z0-9_-]. A match touching one of these sits inside a longer name.
func isWordOrDashByte(char byte) bool {
	switch {
	case char >= 'a' && char <= 'z', char >= 'A' && char <= 'Z', char >= '0' && char <= '9':
		return true
	case char == '_' || char == '-':
		return true
	default:
		return false
	}
}

// hasSuffixFold reports whether path ends in any of these extensions, ignoring
// case, because a host reports whatever the filesystem gave it and Windows
// gives back ".TSX" as readily as ".tsx".
func hasSuffixFold(path string, suffixes []string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	for _, suffix := range suffixes {
		if ext == strings.ToLower(suffix) {
			return true
		}
	}
	return false
}

// isFileWrite reports whether a tool call put text into a file.
//
// Each host config used to do this with a matcher - "Edit|Write|NotebookEdit"
// beside the script - and that filtering does not survive the move: one binary
// answers every PostToolUse, so a Read, which also carries a file_path, would
// otherwise be scanned as though it had written the file it opened.
//
// An empty name passes. Cursor's afterFileEdit is already edit-only and sends
// no tool name, and refusing it would silence every guard on that host.
func isFileWrite(tool string) bool {
	switch tool {
	case "", "Edit", "Write", "NotebookEdit", "MultiEdit":
		return true
	default:
		return false
	}
}
