#!/usr/bin/env sh
# Fail when a generated harness view has drifted from .ai-agents.
#
# This exists because it happened: an edit to .ai-agents/commands/goal.md was
# verified by every other check in the repo (routers OK, links resolve, tests
# green) while .claude/commands/goal.md still served the previous text to the
# session reading it. On Windows the link script produces copies rather than
# symlinks, so a canonical edit reaches the harness only after re-running it,
# and nothing said so.
set -eu

root="$(cd "$(dirname "$0")/.." && pwd)"
assets="$root/.ai-agents"
drift=0

for view in "$root/.claude" "$root/.cursor" "$root/.opencode" "$root/.agents"; do
  [ -d "$view" ] || continue
  for kind in commands agents skills; do
    viewdir="$view/$kind"
    srcdir="$assets/$kind"
    [ -d "$viewdir" ] || continue
    [ -d "$srcdir" ] || continue
    # Codex uses .agents/skills for both canonical skills and generated command
    # adapters. scripts/check-codex-assets.ps1 validates that mixed view.
    if [ "$view" = "$root/.agents" ] && [ "$kind" = "skills" ]; then
      continue
    fi
    # A symlink or junction always matches by construction; only compare copies.
    if [ -L "$viewdir" ]; then continue; fi
    if ! diff -r -q "$srcdir" "$viewdir" >/dev/null 2>&1; then
      echo "DRIFT: $viewdir differs from $srcdir"
      drift=1
    fi
  done
done

# Plugin manifests: verify they exist and contain the expected name field.
for manifest in \
  "$root/.claude-plugin/plugin.json" \
  "$root/.claude-plugin/marketplace.json" \
  "$root/.codex-plugin/plugin.json" \
  "$root/.cursor-plugin/plugin.json" \
  "$root/plugin.json"; do
  if [ ! -f "$manifest" ]; then
    echo "MISSING: $manifest"
    drift=1
  elif ! grep -q '"vibe-agent"' "$manifest" 2>/dev/null; then
    echo "DRIFT: $manifest does not contain expected name"
    drift=1
  fi
done

if [ "$drift" -ne 0 ]; then
  echo ""
  echo "A generated view is stale, so the harness is reading different text than .ai-agents."
  echo "Re-run the link script:"
  echo "  bash scripts/link-ai-agents.sh"
  echo "  powershell -ExecutionPolicy Bypass -File scripts/link-ai-agents.ps1"
  exit 1
fi

echo "check-generated-views: OK"
