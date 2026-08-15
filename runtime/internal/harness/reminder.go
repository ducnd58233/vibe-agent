package harness

import (
	"os"
	"path/filepath"
	"regexp"
)

// Two reminders that used to be Python scripts, moved here for the reason
// guard.go gives: a hook that needs an interpreter the machine may not have is
// a hook that stops running without saying so.
//
// The session-start one also arrived broken. It printed {"priority", "message"},
// which is not a shape any host reads, so its text reached nobody while the
// config went on listing it. Every other hook in that folder already used
// hookSpecificOutput. Rather than port the shape, this folds the intent into
// sessionContext, which does reach the model.

// MetaSkill is the routing entry point a session should read before choosing an
// asset. Named rather than inlined: the script used to paste the whole file
// into every session, and a pointer is what "one source of truth, referenced"
// asks for.
const MetaSkill = ".ai-agents/skills/using-agent-skills/SKILL.md"

// metaSkillLine points a new session at the router flow, when the toolkit
// shipped one.
func metaSkillLine(toolkitRoot string) string {
	if toolkitRoot == "" {
		return ""
	}
	path := filepath.Join(toolkitRoot, filepath.FromSlash(MetaSkill))
	if _, err := os.Stat(path); err != nil {
		return ""
	}
	return "Choosing a skill, agent, or command: read " + MetaSkill +
		", then the folder router it names. Do not work from a memorised asset list."
}

// RE2 has no Unicode-aware \b: it treats only ASCII letters, digits, and
// underscore as word characters. A Vietnamese verb beginning with one of the
// letters it does not - `đổi` - would then never match after a space, because
// space and `đ` are both non-word and there is no boundary between them.
//
// These consume the boundary character instead. That is harmless here and only
// here: every pattern below answers yes or no about a whole prompt, so nothing
// depends on where the match started or on two matches sitting side by side.
const (
	wordStart = `(?:^|[^\p{L}\p{N}_])`
	wordEnd   = `(?:$|[^\p{L}\p{N}_])`
)

// The intents that benefit from the authoring reminder, kept narrow on purpose
// so an ordinary prompt stays free of extra context. Verbs cover English and
// Vietnamese, because prompts in this repository arrive in both.
var (
	assetNoun = regexp.MustCompile(`(?i)` + wordStart +
		`(?:skill|subagent|command|hook|reference|stack[- ]profile|router|template)s?` + wordEnd)

	actionVerb = regexp.MustCompile(`(?i)` + wordStart +
		`(?:new|add|create|author|write|update|rename|delete|remove|edit|refactor` +
		`|t[aạ]o|th[eê]m|s[uử]a|vi[eế]t|x[oó]a|xo[aá]|c[aậ]p\s*nh[aậ]t|đ[oổ]i\s*t[eê]n|m[oớ]i)` + wordEnd)

	routingAsk = regexp.MustCompile(`(?i)(?:` +
		wordStart + `(?:which|what|n[aà]o)` + `[^.\n]{0,40}` + wordStart + `(?:skill|agent|command|profile|asset)` + wordEnd +
		`|` + wordStart + `rout(?:e|ing)` + wordEnd + `)`)
)

// authoringReminder is what an asset-authoring prompt gets alongside it.
//
// Four rules that are easy to skip and expensive to skip: the precedence order,
// where routing starts, which template applies, and the router table that has
// to move in the same change.
const authoringReminder = `Repository reminders for this request:
1. Local-first precedence - check the workspace root for its own rules and templates (AGENTS.md, CLAUDE.md, CLAUDE.local.md, .cursor/rules/, its own TEMPLATE.md, existing file patterns) before applying any toolkit default. On conflict follow the local rule and state the divergence.
2. Routing - read .ai-agents/ROUTER.md, then the matching folder ROUTER.md, before selecting an asset.
3. Authoring - follow that folder's TEMPLATE.md and complete every required section.
4. Router tables - after adding, renaming, or removing an asset, update that folder's ROUTER.md in the same change.`

// authoringContext returns the reminder when a prompt reads as asset work.
func authoringContext(prompt string) string {
	if prompt == "" || !remindAboutAuthoring(prompt) {
		return ""
	}
	return authoringReminder
}

// remindAboutAuthoring reports whether this prompt is about assets.
//
// Either a direct routing question, or an asset noun with something being done
// to it. The noun alone is too common to act on: half the prompts in this
// repository mention a skill without proposing to change one.
func remindAboutAuthoring(prompt string) bool {
	if routingAsk.MatchString(prompt) {
		return true
	}
	return assetNoun.MatchString(prompt) && actionVerb.MatchString(prompt)
}
