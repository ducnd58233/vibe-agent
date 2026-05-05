#!/usr/bin/env bash
# Creates symlinks so Claude Code, Cursor, and opencode see the canonical .ai-agents trees.
#
# Default: workspace = parent of scripts/, assets = <that>/.ai-agents (this toolkit repo).
#
# Consumer example (from consumer repo root):
#   bash .vibe-agent/scripts/link-ai-agents.sh --workspace "$PWD" --assets "$PWD/.vibe-agent/.ai-agents"
#
# Short flags: -w / -a

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TOOLKIT_HOME="$(cd "$SCRIPT_DIR/.." && pwd)"

WORKSPACE=""
ASSETS=""

usage() {
  echo "Usage: $0 [--workspace DIR] [--assets DIR]" >&2
  echo "  Defaults: workspace=$TOOLKIT_HOME, assets=$TOOLKIT_HOME/.ai-agents" >&2
  exit 1
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --workspace|-w)
      [[ $# -ge 2 ]] || usage
      WORKSPACE="$2"
      shift 2
      ;;
    --assets|-a)
      [[ $# -ge 2 ]] || usage
      ASSETS="$2"
      shift 2
      ;;
    -h|--help)
      usage
      ;;
    *)
      echo "Unknown option: $1" >&2
      usage
      ;;
  esac
done

if [[ -z "$WORKSPACE" ]]; then
  WORKSPACE="$TOOLKIT_HOME"
fi
if [[ -z "$ASSETS" ]]; then
  ASSETS="$TOOLKIT_HOME/.ai-agents"
fi

if [[ ! -d "$WORKSPACE" ]]; then
  echo "Workspace directory not found: $WORKSPACE" >&2
  exit 1
fi
if [[ ! -d "$ASSETS" ]]; then
  echo "Assets directory not found: $ASSETS" >&2
  exit 1
fi

WORKSPACE="$(cd "$WORKSPACE" && pwd)"
ASSETS="$(cd "$ASSETS" && pwd)"

for name in skills agents commands; do
  if [[ ! -d "$ASSETS/$name" ]]; then
    echo "Assets root must contain '$name' directory: $ASSETS/$name" >&2
    exit 1
  fi
done

link_skill_tree() {
  local link_path="$1"
  local target_path="$2"
  if [[ ! -d "$target_path" ]]; then
    echo "Missing target directory: $target_path" >&2
    exit 1
  fi
  mkdir -p "$(dirname "$link_path")"
  rm -rf "$link_path"
  ln -sfn "$target_path" "$link_path"
}

link_skill_tree "$WORKSPACE/.claude/skills" "$ASSETS/skills"
link_skill_tree "$WORKSPACE/.claude/agents" "$ASSETS/agents"
link_skill_tree "$WORKSPACE/.claude/commands" "$ASSETS/commands"
link_skill_tree "$WORKSPACE/.cursor/skills" "$ASSETS/skills"
link_skill_tree "$WORKSPACE/.cursor/commands" "$ASSETS/commands"
link_skill_tree "$WORKSPACE/.opencode/agents" "$ASSETS/agents"
link_skill_tree "$WORKSPACE/.opencode/commands" "$ASSETS/commands"

echo "Symlinks created under $WORKSPACE (.claude, .cursor, .opencode) -> $ASSETS"
