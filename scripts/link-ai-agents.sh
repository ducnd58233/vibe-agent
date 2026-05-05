#!/usr/bin/env bash
# Creates symlinks so Claude Code, Cursor, and opencode see the canonical .ai-agents trees.
#
# Default: workspace = parent of scripts/, assets = <that>/.ai-agents (this toolkit repo).
#
# Consumer (from consumer repo root; prefer forward slashes on Git Bash / MSYS):
#   bash .vibe-agent/scripts/link-ai-agents.sh --workspace "$PWD" --assets "$PWD/.vibe-agent/.ai-agents"
#
# Or avoid argv quoting issues under Git Bash / PowerShell -lc:
#   export LINK_WORKSPACE="D:/projects/my-repo"
#   export LINK_ASSETS="D:/projects/my-repo/.vibe-agent/.ai-agents"
#   bash .vibe-agent/scripts/link-ai-agents.sh
#
# Short flags: -w / -a

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TOOLKIT_HOME="$(cd "$SCRIPT_DIR/.." && pwd)"

WORKSPACE=""
ASSETS=""
workspace_from_args=0
assets_from_args=0

usage() {
  echo "Usage: $0 [--workspace DIR] [--assets DIR]" >&2
  echo "  Or set LINK_WORKSPACE and LINK_ASSETS (used when the matching flag is omitted)." >&2
  echo "  Defaults: workspace=$TOOLKIT_HOME, assets=$TOOLKIT_HOME/.ai-agents" >&2
  exit 1
}

# Git Bash / MSYS: convert Windows paths (D:/... or D:\\...) to /d/... so cd and test -d work.
to_unix_path_if_needed() {
  local p="$1"
  if command -v cygpath >/dev/null 2>&1; then
    if [[ "$p" =~ ^[A-Za-z]: ]]; then
      cygpath -u -- "$p"
      return
    fi
  fi
  printf '%s\n' "$p"
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --workspace=*)
      WORKSPACE="${1#*=}"
      workspace_from_args=1
      shift
      ;;
    --assets=*)
      ASSETS="${1#*=}"
      assets_from_args=1
      shift
      ;;
    --workspace|-w)
      [[ $# -ge 2 ]] || usage
      WORKSPACE="$2"
      workspace_from_args=1
      shift 2
      ;;
    --assets|-a)
      [[ $# -ge 2 ]] || usage
      ASSETS="$2"
      assets_from_args=1
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

if [[ $workspace_from_args -eq 0 ]]; then
  WORKSPACE="${LINK_WORKSPACE:-}"
fi
if [[ $assets_from_args -eq 0 ]]; then
  ASSETS="${LINK_ASSETS:-}"
fi

if [[ -z "$WORKSPACE" ]]; then
  WORKSPACE="$TOOLKIT_HOME"
fi
if [[ -z "$ASSETS" ]]; then
  ASSETS="$TOOLKIT_HOME/.ai-agents"
fi

WORKSPACE="$(to_unix_path_if_needed "$WORKSPACE")"
ASSETS="$(to_unix_path_if_needed "$ASSETS")"

if [[ ! -d "$WORKSPACE" ]]; then
  echo "Workspace directory not found: $WORKSPACE" >&2
  if [[ "$WORKSPACE" =~ ^[A-Za-z]:[^/\\\\] ]]; then
    echo "Hint: under Git Bash, backslashes in double-quoted paths are eaten (D:\\\\projects becomes D:projects)." >&2
    echo "      Use forward slashes (D:/projects/...) or set LINK_WORKSPACE / LINK_ASSETS and run this script without --workspace/--assets." >&2
  fi
  exit 1
fi
if [[ ! -d "$ASSETS" ]]; then
  echo "Assets directory not found: $ASSETS" >&2
  if [[ "$ASSETS" =~ ^[A-Za-z]:[^/\\\\] ]]; then
    echo "Hint: use D:/path/... or MSYS paths /d/path/...; see workspace hint above." >&2
  fi
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
