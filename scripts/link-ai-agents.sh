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
#
# The script also writes minimal hook configs when a host config is absent and
# adds generated discovery paths to <workspace>/.git/info/exclude when the
# workspace is a Git repository. This keeps local links and generated Codex
# agent files and generated command skills out of Git without root .gitignore
# rules in consumer repos. Codex CLI removed custom /prompts support in 0.117.0,
# so command prompts are generated as explicit skills: $<name> in a linked
# workspace, or $vibe-<name> after global install.

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
link_skill_tree "$WORKSPACE/.agents/commands" "$ASSETS/commands"

convert_command_body_for_codex_skill() {
  local assets_ref="$1"
  sed "s#../skills/#$assets_ref/skills/#g" \
    | sed "s#../references/#$assets_ref/references/#g" \
    | sed "s#../stack-profiles/#$assets_ref/stack-profiles/#g" \
    | sed "s#../commands/#$assets_ref/commands/#g" \
    | sed "s#../agents/#$assets_ref/agents/#g"
}

frontmatter_description() {
  awk '
    /^description:/ {
      line=$0
      sub(/^description:[[:space:]]*/, "", line)
      if (line ~ /^>/) { fold=1; next }
      gsub(/^["'\'']|["'\'']$/, "", line)
      print line
      exit
    }
    fold && /^[[:space:]]/ {
      gsub(/^[[:space:]]+/, "")
      print
      next
    }
    fold && /^[^[:space:]]/ { exit }
  ' "$1" | tr '\n' ' ' | sed 's/[[:space:]]*$//'
}

write_codex_command_skill() {
  local command_file="$1"
  local skill_root="$2"
  local skill_name="$3"
  local assets_ref="$4"
  local command_base description body skill_dir
  command_base="$(basename "$command_file")"
  description="$(frontmatter_description "$command_file")"
  if [[ -z "$description" ]]; then
    description="Run the vibe-agent $skill_name command"
  fi
  body="$(awk 'BEGIN{n=0} /^---$/ { n++; next } n>=2 { print }' "$command_file" | convert_command_body_for_codex_skill "$assets_ref")"
  skill_dir="$skill_root/$skill_name"
  mkdir -p "$skill_dir"
  cat > "$skill_dir/SKILL.md" <<EOF
---
name: $skill_name
description: >-
  Codex-compatible command adapter. Use only when the user explicitly mentions
  \$$skill_name or asks to run this vibe-agent command: $description
disable-model-invocation: true
---

# $skill_name

This is the Codex-compatible form of \`$assets_ref/commands/$command_base\`.
Codex CLI removed custom \`/prompts\` support in 0.117.0, so command prompts are exposed as explicit skills.

Treat any text after \`\$$skill_name\` as the command arguments, then follow the command prompt below.

<command_prompt>
$body
</command_prompt>
EOF
}

sync_codex_command_skills() {
  local assets="$1"
  local skill_root="$2"
  local name_prefix="$3"
  local assets_ref="$4"
  local include_canonical="$5"
  rm -rf "$skill_root"
  mkdir -p "$skill_root"
  if [[ "$include_canonical" == "1" ]]; then
    local dir name
    for dir in "$assets"/skills/*/; do
      [[ -d "$dir" ]] || continue
      name="$(basename "$dir")"
      link_skill_tree "$skill_root/$name" "$dir"
    done
  fi
  local md base command_name
  for md in "$assets"/commands/*.md; do
    [[ -f "$md" ]] || continue
    base="$(basename "$md")"
    case "$base" in
      TEMPLATE.md|README.md|ROUTER.md) continue ;;
    esac
    command_name="$(basename "$md" .md)"
    write_codex_command_skill "$md" "$skill_root" "$name_prefix$command_name" "$assets_ref"
  done
}

codex_home_dir() {
  local home="${CODEX_HOME:-}"
  if [[ -z "$home" ]]; then
    home="${HOME:-${USERPROFILE:-}}"
    [[ -n "$home" ]] || return 1
    home="$home/.codex"
  fi
  to_unix_path_if_needed "$home"
}

remove_codex_prompt_copies() {
  local workspace="$1"
  rm -rf "$workspace/.codex/prompts"
  local codex_home manifest old
  if ! codex_home="$(codex_home_dir)"; then
    return 0
  fi
  manifest="$codex_home/.vibe-agent-prompts.manifest"
  if [[ -f "$manifest" ]]; then
    while IFS= read -r old; do
      [[ -n "$old" ]] || continue
      old="$(to_unix_path_if_needed "$old")"
      rm -f "$old"
    done < "$manifest"
    rm -f "$manifest"
  fi
}

sync_codex_agents_from_md() {
  local assets="$1"
  local workspace="$2"
  local assets_ref="$3"
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
      | sed "s#../skills/#$assets_ref/skills/#g" \
      | sed "s#../references/#$assets_ref/references/#g" \
      | sed "s#../stack-profiles/#$assets_ref/stack-profiles/#g" \
      | sed "s#../commands/#$assets_ref/commands/#g" \
      | sed "s#../agents/#$assets_ref/agents/#g")"
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
      printf 'Codex note: this file is generated from `%s/agents`. Resolve shared asset links from that assets root, not from `.codex/agents`.\n\n' "$assets_ref"
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

sync_codex_command_skills "$ASSETS" "$WORKSPACE/.agents/skills" "" "$ASSETS" "1"
remove_codex_prompt_copies "$WORKSPACE"
sync_codex_agents_from_md "$ASSETS" "$WORKSPACE" "$ASSETS"

json_escape() {
  printf '%s' "$1" | sed 's/\\/\\\\/g; s/"/\\"/g'
}

toolkit_root_for_hooks() {
  dirname "$ASSETS"
}

hook_command() {
  event="$1"
  client="$2"
  printf 'vibe-agent hook %s --workspace \\"%s\\" --toolkit \\"%s\\" --client %s' \
    "$event" "$(json_escape "$WORKSPACE")" "$(json_escape "$(toolkit_root_for_hooks)")" "$client"
}

python_hook_command() {
  script="$1"
  printf 'python3 \\"%s/hooks/%s\\"' "$(json_escape "$ASSETS")" "$script"
}

install_workspace_hook_configs() {
  # Directory links make commands visible, but hooks still need host config.
  # Write minimal configs only when absent; an existing repository policy is
  # local authority and is never overwritten by the link script.
  if [ ! -f "$WORKSPACE/.claude/settings.json" ]; then
    mkdir -p "$WORKSPACE/.claude"
    cat > "$WORKSPACE/.claude/settings.json" <<EOF
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "WebFetch",
        "hooks": [
          { "type": "command", "command": "$(python_hook_command sdd-cache-pre.py)" }
        ]
      },
      {
        "matcher": "Bash|Edit|Write|NotebookEdit",
        "hooks": [
          { "type": "command", "command": "$(hook_command pre-tool-use claude)" }
        ]
      }
    ],
    "PostToolUseFailure": [
      {
        "matcher": "Bash|Edit|Write|NotebookEdit",
        "hooks": [
          { "type": "command", "command": "$(hook_command post-tool-use-failure claude)" }
        ]
      }
    ],
    "PostToolUse": [
      {
        "matcher": "Bash|Edit|Write|NotebookEdit",
        "hooks": [
          { "type": "command", "command": "$(hook_command post-tool-use claude)" }
        ]
      },
      {
        "matcher": "WebFetch",
        "hooks": [
          { "type": "command", "command": "$(python_hook_command sdd-cache-post.py)" }
        ]
      }
    ],
    "Stop": [
      { "hooks": [ { "type": "command", "command": "$(hook_command stop claude)" } ] }
    ],
    "SessionStart": [
      { "hooks": [ { "type": "command", "command": "$(hook_command session-start claude)" } ] }
    ],
    "UserPromptSubmit": [
      { "hooks": [ { "type": "command", "command": "$(hook_command user-prompt-submit claude)" } ] }
    ],
    "SubagentStop": [
      { "hooks": [ { "type": "command", "command": "$(hook_command subagent-stop claude)" } ] }
    ]
  },
  "disableAllHooks": false
}
EOF
    echo "Installed minimal Claude hook config at $WORKSPACE/.claude/settings.json"
  fi

  if [ ! -f "$WORKSPACE/.cursor/hooks.json" ]; then
    mkdir -p "$WORKSPACE/.cursor"
    cat > "$WORKSPACE/.cursor/hooks.json" <<EOF
{
  "version": 1,
  "hooks": {
    "sessionStart": [
      { "command": "$(hook_command session-start cursor)" }
    ],
    "preToolUse": [
      { "matcher": "WebFetch", "command": "$(python_hook_command sdd-cache-pre.py)" },
      { "matcher": "Write|Delete", "command": "$(hook_command pre-tool-use cursor)" }
    ],
    "beforeShellExecution": [
      { "command": "$(hook_command pre-tool-use cursor)" }
    ],
    "postToolUse": [
      { "matcher": "WebFetch", "command": "$(python_hook_command sdd-cache-post.py)" },
      { "matcher": "Shell|Write|Delete", "command": "$(hook_command post-tool-use cursor)" }
    ],
    "postToolUseFailure": [
      { "matcher": "Shell|Write|Delete", "command": "$(hook_command post-tool-use-failure cursor)" }
    ],
    "subagentStop": [
      { "command": "$(hook_command subagent-stop cursor)" }
    ],
    "stop": [
      { "command": "$(hook_command stop cursor)" }
    ]
  }
}
EOF
    echo "Installed minimal Cursor hook config at $WORKSPACE/.cursor/hooks.json"
  fi

  if [ ! -f "$WORKSPACE/.codex/hooks.json" ]; then
    mkdir -p "$WORKSPACE/.codex"
    cat > "$WORKSPACE/.codex/hooks.json" <<EOF
{
  "hooks": {
    "SessionStart": [
      { "hooks": [ { "type": "command", "command": "$(hook_command session-start codex)" } ] }
    ],
    "UserPromptSubmit": [
      { "hooks": [ { "type": "command", "command": "$(hook_command user-prompt-submit codex)" } ] }
    ],
    "PreToolUse": [
      { "hooks": [ { "type": "command", "command": "$(hook_command pre-tool-use codex)" } ] }
    ],
    "PostToolUse": [
      { "hooks": [ { "type": "command", "command": "$(hook_command post-tool-use codex)" } ] }
    ],
    "Stop": [
      { "hooks": [ { "type": "command", "command": "$(hook_command stop codex)" } ] }
    ],
    "SubagentStop": [
      { "hooks": [ { "type": "command", "command": "$(hook_command subagent-stop codex)" } ] }
    ]
  }
}
EOF
    echo "Installed minimal Codex hook config at $WORKSPACE/.codex/hooks.json"
  fi
}

install_local_git_exclude() {
  local workspace="$1"
  local exclude_path
  exclude_path="$(git_path "$workspace" "info/exclude")"
  if [[ -z "$exclude_path" ]]; then
    echo "No .git directory at $workspace; skipped local git exclude rules." >&2
    return 0
  fi
  local info_dir
  info_dir="$(dirname "$exclude_path")"
  mkdir -p "$info_dir"
  touch "$exclude_path"

  local -a rules=(
    "/.claude/skills/"
    "/.claude/agents/"
    "/.claude/commands/"
    "/.cursor/skills/"
    "/.cursor/commands/"
    "/.opencode/agents/"
    "/.opencode/commands/"
    "/.agents/skills/"
    "/.agents/commands/"
    "/.codex/agents/"
  )
  local -a missing=()
  local rule
  for rule in "${rules[@]}"; do
    if ! grep -Fxq "$rule" "$exclude_path"; then
      missing+=("$rule")
    fi
  done
  if [[ ${#missing[@]} -eq 0 ]]; then
    echo "Local git exclude rules already present at $exclude_path"
    return 0
  fi
  {
    printf '\n'
    printf '# Generated vibe-agent discovery paths\n'
    printf '%s\n' "${missing[@]}"
  } >> "$exclude_path"
  echo "Installed local git exclude rules at $exclude_path"
}

# Install the git prepare-commit-msg hook that strips AI/agent attribution.
# This is the only cross-harness enforcement point: Cursor/Codex/opencode and
# manual commits have no `attribution` setting like Claude Code does.
install_commit_attribution_hook() {
  local assets="$1"
  local workspace="$2"
  local hooks_dir
  hooks_dir="$(git_path "$workspace" "hooks")"
  if [[ -z "$hooks_dir" ]]; then
    echo "No .git directory at $workspace; skipped prepare-commit-msg attribution hook." >&2
    return 0
  fi
  mkdir -p "$hooks_dir"
  local hook_path="$hooks_dir/prepare-commit-msg"
  {
    printf '#!/bin/sh\n'
    printf '# Installed by scripts/link-ai-agents.sh - strips AI/agent attribution from commit messages.\n'
    printf '# Source: .ai-agents/hooks/strip-ai-attribution.sh\n'
    printf 'exec sh "%s/hooks/strip-ai-attribution.sh" "$1"\n' "$assets"
  } > "$hook_path"
  chmod +x "$hook_path"
  echo "Installed git prepare-commit-msg attribution hook at $hook_path"
}

git_path() {
  local workspace="$1"
  local rel="$2"
  local path
  path="$(git -C "$workspace" rev-parse --git-path "$rel" 2>/dev/null || true)"
  [[ -n "$path" ]] || return 0
  path="$(to_unix_path_if_needed "$path")"
  case "$path" in
    /*|[A-Za-z]:*) printf '%s\n' "$path" ;;
    *) printf '%s\n' "$workspace/$path" ;;
  esac
}

# Fetches the optional runtime binary that the wired hooks invoke by name.
#
# Always fetches, even when a binary is already present. It used to skip in that
# case, which meant a consumer who installed once never got another update: the
# hooks kept calling a binary that fell further behind the configs registering
# them. That failure is invisible from the outside, because a stale binary answers
# the events it knows and refuses the rest.
#
# Skipped only when LINK_SKIP_RUNTIME is set, and in CI, where a network download
# would make an unrelated outage look like a broken link script.
#
# Never fails the link run. The runtime is optional by design: without it every
# hook is a quiet no-op and the markdown assets work exactly as before.
install_runtime() {
  local installer="$SCRIPT_DIR/install-runtime.sh"

  if [[ -n "${LINK_SKIP_RUNTIME:-}" ]]; then
    echo "Runtime install skipped (LINK_SKIP_RUNTIME set)."
    return 0
  fi
  if [[ -n "${CI:-}" ]]; then
    echo "Runtime install skipped (CI). Run bash scripts/install-runtime.sh to install it."
    return 0
  fi
  if [[ ! -f "$installer" ]]; then
    echo "No install-runtime.sh next to this script; skipped runtime install." >&2
    return 0
  fi

  local before=""
  if command -v vibe-agent >/dev/null 2>&1; then
    before="$(vibe-agent version 2>/dev/null || echo unknown)"
    echo "Refreshing the runtime binary (installed: ${before})..."
  else
    echo "Installing the optional runtime binary..."
  fi

  if bash "$installer"; then
    # Report the change rather than only the fact of a download, so an unchanged
    # version is visible as unchanged instead of looking like work was done.
    if command -v vibe-agent >/dev/null 2>&1; then
      local after
      after="$(vibe-agent version 2>/dev/null || echo unknown)"
      if [[ -n "$before" && "$before" == "$after" ]]; then
        echo "Runtime already current: ${after}"
      elif [[ -n "$before" ]]; then
        echo "Runtime updated: ${before} -> ${after}"
      fi
    fi
    return 0
  fi
  echo "Runtime install did not complete. This is not fatal: the toolkit works without it." >&2
  echo "Retry later with: bash $installer" >&2
  return 0
}

emit_plugin_manifests() {
  local workspace="$1"
  local plugin_name="vibe-agent"
  local plugin_desc="Domain-agnostic agent workflows: skills, commands, hooks, and delivery graphs."

  # Claude Code plugin
  mkdir -p "$workspace/.claude-plugin"
  cat > "$workspace/.claude-plugin/plugin.json" <<EOF
{
  "name": "$plugin_name",
  "description": "$plugin_desc"
}
EOF

  cat > "$workspace/.claude-plugin/marketplace.json" <<EOF
{
  "name": "$plugin_name",
  "owner": {
    "name": "$plugin_name"
  },
  "plugins": [
    {
      "name": "$plugin_name",
      "source": "./",
      "description": "Skills, slash commands, hooks, and goal-delivery graphs. Canonical assets live under .ai-agents/."
    }
  ]
}
EOF

  # Codex plugin
  mkdir -p "$workspace/.codex-plugin"
  cat > "$workspace/.codex-plugin/plugin.json" <<EOF
{
  "name": "$plugin_name",
  "description": "Domain-agnostic agent workflows: skills and hooks for Codex."
}
EOF

  # Cursor plugin (host-specific)
  mkdir -p "$workspace/.cursor-plugin"
  cat > "$workspace/.cursor-plugin/plugin.json" <<EOF
{
  "\$schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json",
  "name": "$plugin_name",
  "description": "Domain-agnostic agent workflows for Cursor Agent Plugins."
}
EOF

  # Agent Plugins 1.0.0 (root, portable)
  cat > "$workspace/plugin.json" <<EOF
{
  "\$schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json",
  "name": "$plugin_name",
  "description": "$plugin_desc"
}
EOF
  echo "Plugin manifests emitted under $workspace"
}

install_local_git_exclude "$WORKSPACE"
install_commit_attribution_hook "$ASSETS" "$WORKSPACE"
install_workspace_hook_configs
emit_plugin_manifests "$WORKSPACE"
install_runtime

echo "Symlinks created under $WORKSPACE (.claude, .cursor, .opencode, .agents) -> $ASSETS"
echo "Codex custom agents synced to $WORKSPACE/.codex/agents"
echo "Codex command skills synced to $WORKSPACE/.agents/skills as <name>"
echo "Codex command form in a linked workspace: \$<name> (custom /prompts and top-level /vibe-* are not available in Codex CLI 0.147.0)"
