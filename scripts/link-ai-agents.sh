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
link_skill_tree "$WORKSPACE/.agents/skills" "$ASSETS/skills"
link_skill_tree "$WORKSPACE/.agents/commands" "$ASSETS/commands"

sync_codex_agents_from_md() {
  local assets="$1"
  local workspace="$2"
  local agents_src="$assets/agents"
  local agents_dest="$workspace/.codex/agents"
  mkdir -p "$agents_dest"
  local -a generated=()
  local md base name
  for md in "$agents_src"/*.md; do
    [[ -f "$md" ]] || continue
    base="$(basename "$md")"
    case "$base" in
      TEMPLATE.md|README.md|ROUTER.md) continue ;;
    esac
    if ! awk 'BEGIN{n=0} /^---$/ { n++; next } n>=1 { if (n>=2) body=body $0 ORS } END{ exit (n<2) }' "$md" >/dev/null; then
      echo "Warning: skipping $base (missing YAML frontmatter)." >&2
      continue
    fi
    name="$(awk -F': ' '/^name:/ {print $2; exit}' "$md" | tr -d '"')"
    if [[ -z "$name" ]]; then
      echo "Warning: skipping $base (missing name in frontmatter)." >&2
      continue
    fi
    description="$(awk '
      /^description:/ {
        line=$0
        sub(/^description:[[:space:]]*/, "", line)
        if (line ~ /^>/) { fold=1; next }
        print line
        exit
      }
      fold && /^[[:space:]]/ {
        gsub(/^[[:space:]]+/, "")
        print
        next
      }
      fold && /^[^[:space:]]/ { exit }
    ' "$md" | tr '\n' ' ' | sed 's/[[:space:]]*$//')"
    if [[ -z "$description" ]]; then
      description="$name"
    fi
    body="$(awk 'BEGIN{n=0} /^---$/ { n++; next } n>=2 { print }' "$md")"
    body="$(printf '%s\n' "$body" \
      | sed 's#../skills/#.ai-agents/skills/#g' \
      | sed 's#../references/#.ai-agents/references/#g' \
      | sed 's#../stack-profiles/#.ai-agents/stack-profiles/#g' \
      | sed 's#../commands/#.ai-agents/commands/#g' \
      | sed 's#../agents/#.ai-agents/agents/#g')"
    sandbox_line=""
    if awk '/^tools:/{f=1} f && /^[[:space:]]+(Bash|Edit|Write|NotebookEdit|Task):[[:space:]]*true/{bad=1} END{exit bad}' "$md"; then
      sandbox_line=$'sandbox_mode = "read-only"\n'
    fi
    toml_path="$agents_dest/${name}.toml"
    {
      printf '# Generated by scripts/link-ai-agents.sh from .ai-agents/agents/%s - do not edit; re-run link script.\n\n' "$base"
      printf 'name = "%s"\n' "$name"
      printf 'description = "%s"\n' "$description"
      printf '%s' "$sandbox_line"
      printf 'developer_instructions = """\n'
      printf 'Codex note: this file is generated from `.ai-agents/agents`. Resolve shared asset links from the workspace root (for example `.ai-agents/skills/...`), not from `.codex/agents`.\n\n'
      printf '%s\n' "$body"
      printf '"""\n'
    } > "$toml_path"
    generated+=("$name")
  done
  for existing in "$agents_dest"/*.toml; do
    [[ -f "$existing" ]] || continue
    name="$(basename "$existing" .toml)"
    keep=0
    for g in "${generated[@]}"; do
      if [[ "$g" == "$name" ]]; then
        keep=1
        break
      fi
    done
    if [[ $keep -eq 0 ]]; then
      rm -f "$existing"
    fi
  done
}

sync_codex_agents_from_md "$ASSETS" "$WORKSPACE"

echo "Symlinks created under $WORKSPACE (.claude, .cursor, .opencode, .agents) -> $ASSETS"
echo "Codex custom agents synced to $WORKSPACE/.codex/agents"
