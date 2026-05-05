#!/usr/bin/env bash
# Creates symlinks so Claude Code, Cursor, and opencode see the canonical .ai-agents trees.
# Usage: ./scripts/link-ai-agents.sh   (from repo root; script resolves paths)

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

link_skill_tree() {
  local link_path="$1"
  local target_path="$2"
  if [[ ! -d "$target_path" ]]; then
    echo "Missing target directory: $target_path" >&2
    exit 1
  fi
  mkdir -p "$(dirname "$link_path")"
  rm -rf "$link_path"
  ln -sfn "$ROOT/$target_path" "$ROOT/$link_path"
}

link_skill_tree ".claude/skills" ".ai-agents/skills"
link_skill_tree ".claude/agents" ".ai-agents/agents"
link_skill_tree ".claude/commands" ".ai-agents/commands"
link_skill_tree ".cursor/skills" ".ai-agents/skills"
link_skill_tree ".cursor/commands" ".ai-agents/commands"
link_skill_tree ".opencode/agents" ".ai-agents/agents"
link_skill_tree ".opencode/commands" ".ai-agents/commands"

echo "Symlinks created under .claude, .cursor, and .opencode pointing to .ai-agents."
