#!/usr/bin/env bash
# Proves workspace linking is enough for host commands and hook entrypoints.
#
# Global install puts prefixed assets in user-level folders. Workspace install
# must still stand on its own: generated command views live in the repository,
# and hook configs call the runtime with explicit workspace and toolkit roots.
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

git -C "$tmp" init >/dev/null

LINK_SKIP_RUNTIME=1 bash "$root/scripts/link-ai-agents.sh" \
  --workspace "$tmp" \
  --assets "$root/.ai-agents" >/dev/null

fail() {
  echo "check-workspace-install: $*" >&2
  exit 1
}

for path in \
  "$tmp/.claude/settings.json" \
  "$tmp/.cursor/hooks.json" \
  "$tmp/.codex/hooks.json" \
  "$tmp/.agents/skills/build/SKILL.md" \
  "$tmp/.agents/skills/research/SKILL.md" \
  "$tmp/.agents/skills/investigate/SKILL.md" \
  "$tmp/.codex/agents/code-reviewer.toml" \
  "$tmp/.git/hooks/prepare-commit-msg"; do
  [ -e "$path" ] || fail "missing $path"
done

for command in "$root"/.ai-agents/commands/*.md; do
  base="$(basename "$command" .md)"
  case "$base" in
    README|ROUTER|TEMPLATE) continue ;;
  esac
  [ -f "$tmp/.agents/skills/$base/SKILL.md" ] || fail "missing Codex command skill for $base"
done

for file in \
  "$tmp/.claude/settings.json" \
  "$tmp/.cursor/hooks.json" \
  "$tmp/.codex/hooks.json"; do
  grep -q 'vibe-agent hook' "$file" || fail "no runtime hook command in $file"
  grep -q -- '--workspace' "$file" || fail "hook command in $file omits --workspace"
  grep -q -- '--toolkit' "$file" || fail "hook command in $file omits --toolkit"
done

assets_ref="$root/.ai-agents"
for command in "$root"/.ai-agents/commands/*.md; do
  base="$(basename "$command" .md)"
  case "$base" in
    README|ROUTER|TEMPLATE) continue ;;
  esac
  file="$tmp/.agents/skills/$base/SKILL.md"
  grep -q "$assets_ref" "$file" || fail "generated Codex command skill does not point at $assets_ref: $file"
done
for file in "$tmp"/.codex/agents/*.toml; do
  grep -q "$assets_ref" "$file" || fail "generated Codex agent does not point at $assets_ref: $file"
done

legacy='design-token-guard.py|ui-slop-guard.py|sensitive-data-guard.py|core-logic-test-guard.py|session-start.py|precedence-reminder.py|subagent-grounding-guard.py'
if grep -R -E "$legacy" "$tmp/.claude" "$tmp/.cursor" "$tmp/.codex" >/dev/null; then
  fail "workspace hook config still references a migrated Python hook"
fi
if find "$root/.ai-agents/hooks" -maxdepth 1 -type f | grep -E "$legacy" >/dev/null; then
  fail "migrated Python hook file still exists under .ai-agents/hooks"
fi

echo "check-workspace-install: OK"
