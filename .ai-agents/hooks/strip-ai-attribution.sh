#!/bin/sh
# Strip AI/agent attribution from a commit message, in place.
#
# Cross-harness enforcement of the repo "No Agent Attribution" policy (see
# AGENTS.md and skills/git-workflow-and-versioning/SKILL.md). Claude Code
# suppresses attribution via the empty `attribution` block in
# .claude/settings.json, but Cursor, Codex, opencode, and manual commits have no
# equivalent setting, so this runs at the git layer they all share.
#
# Usage (git prepare-commit-msg hook): strip-ai-attribution.sh <commit-msg-file>
#
# Removes:
#   - Co-authored-by trailers whose name/email match a known AI-assistant
#     signature (human co-authors are kept).
#   - "Generated with ..." / "Co-authored with ..." attribution lines.
#   - Robot-emoji attribution lines.
#
# Conservative and non-blocking: only denylisted lines are removed, and any
# failure exits 0 so it never breaks `git commit`.

set -u

f="${1:-}"
[ -n "$f" ] || exit 0
[ -f "$f" ] || exit 0

tmp="${f}.strip.$$"

# Pass the robot emoji in as a literal; matching its bytes inside an awk regex is
# not portable across awk implementations, so we detect it with index().
robot=$(printf '\360\237\244\226')

awk -v robot="$robot" '
{
  line = $0
  l = tolower(line)
  drop = 0

  has_agent = (l ~ /claude|anthropic|cursor|codex|openai|chatgpt|gpt-?[0-9]|copilot|opencode|noreply@anthropic\.com|ai assistant/)
  has_robot = (robot != "" && index(line, robot) > 0)

  # Co-authored-by trailer attributed to an AI assistant (human co-authors stay).
  if (l ~ /^[[:space:]]*co-authored-by[[:space:]]*:/ && (has_agent || l ~ /\bbot\b/)) drop = 1

  # "generated/created/written/authored with ..." attribution. Require an agent
  # signature or the robot emoji so plain prose ("generated with care") is safe.
  if (l ~ /(generated|created|written|authored)[[:space:]]+with/ && (has_agent || has_robot)) drop = 1

  # Bare robot-emoji attribution line that also names an agent.
  if (has_robot && has_agent) drop = 1

  if (!drop) lines[n++] = line
}
END {
  # Drop blank lines left dangling at the end of the message.
  while (n > 0 && lines[n-1] ~ /^[[:space:]]*$/) n--
  for (i = 0; i < n; i++) print lines[i]
}
' "$f" > "$tmp" 2>/dev/null || { rm -f "$tmp"; exit 0; }

mv "$tmp" "$f" 2>/dev/null || rm -f "$tmp"
exit 0
