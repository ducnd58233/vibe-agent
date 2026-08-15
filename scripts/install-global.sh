#!/usr/bin/env sh
# Install this toolkit into the user-level asset directories, so every project on
# the machine can use it without a wrapper workspace holding it as a submodule.
#
# Companion to link-ai-agents.sh, not a replacement. That script wires one
# workspace and its permissions, hooks, and runtime gates; this one puts the
# markdown assets where all four harnesses look by default.
#
# POSIX sh. Runs on Linux, macOS, and Windows under Git Bash or WSL. The
# PowerShell port is scripts/install-global.ps1, and it exists because Git Bash
# on Windows cannot create real symlinks: it accepts `ln -s` and copies instead.
#
# Everything installs under a `vibe-` prefix, and the prefix is load-bearing
# rather than decorative. Claude Code resolves a skill-name collision in favour
# of the personal level ("personal overrides project"), so an unprefixed global
# install would make this toolkit override a repository's own skills instead of
# being the fallback AGENTS.md says it is. Prefixed, they cannot collide.
#
# Where things go, and why these paths:
#
#   skills     ~/.claude/skills  and  ~/.agents/skills
#              Two directories cover all four tools. Claude Code reads the
#              first; Codex reads only $HOME/.agents/skills; Cursor reads
#              ~/.agents/skills and ~/.cursor/skills; opencode reads all of
#              them plus its own. ~/.agents is the Agent Skills convention.
#   commands   Claude, Cursor, and opencode keep command directories; Codex
#              uses generated skills because custom /prompts were removed
#              in Codex CLI 0.117.0
#   subagents  ~/.claude/agents, ~/.cursor/agents, opencode agents
#   rules      a marked block in each tool's global instructions file, plus an
#              .mdc rule for Cursor, which has no global AGENTS.md
#
# Three install strategies, because the tools disagree about where an asset's
# identity comes from:
#
#   skills    the command name is the directory name, so a renamed link is
#             enough and edits in the repo stay live
#   commands  the command name is the file name, same deal
#   agents    the identity is the frontmatter `name:` and "the filename doesn't
#             have to match", so a renamed file namespaces nothing. These are
#             generated copies with the field rewritten, and they can go stale:
#             re-run after editing an agent, or use --check
#
# Permissions and hooks are deliberately NOT installed. This repo denies 21
# patterns and hooks six events; applying that to every unrelated repository on
# the machine is a decision for the user, not a side effect of installing
# markdown. Run link-ai-agents.sh in a project to get those.
#
# Usage: sh scripts/install-global.sh [--dry-run] [--check] [--uninstall] [--prefix P]
set -eu

SCRIPT_DIR="$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)"
TOOLKIT="$(CDPATH='' cd -- "$SCRIPT_DIR/.." && pwd)"
ASSETS="$TOOLKIT/.ai-agents"

PREFIX="vibe-"
MODE="install"
DRY=0

usage() {
  cat >&2 <<USAGE
Usage: $0 [options]

  --dry-run      Print what would change, write nothing
  --check        Report drift between installed copies and the toolkit, then exit
  --uninstall    Remove every asset this script installed, and nothing else
  --prefix P     Namespace prefix (default: ${PREFIX})
  -h, --help     This message

Installs from: $ASSETS
USAGE
  exit 2
}

while [ $# -gt 0 ]; do
  case "$1" in
    --dry-run) DRY=1 ;;
    --check) MODE="check" ;;
    --uninstall) MODE="uninstall" ;;
    --prefix) shift; [ $# -gt 0 ] || usage; PREFIX="$1" ;;
    --prefix=*) PREFIX="${1#--prefix=}" ;;
    -h|--help) usage ;;
    *) echo "Unknown argument: $1" >&2; usage ;;
  esac
  shift
done

[ -d "$ASSETS/skills" ] || { echo "Not a toolkit checkout: $ASSETS/skills missing" >&2; exit 1; }

HOME_DIR="${HOME:-${USERPROFILE:-}}"
[ -n "$HOME_DIR" ] && [ -d "$HOME_DIR" ] || { echo "Cannot resolve a home directory" >&2; exit 1; }

# Paths written into generated files are read by native tools, which on Windows
# do not understand the MSYS form Git Bash reports. Elsewhere this is identity.
native_path() {
  if command -v cygpath >/dev/null 2>&1; then cygpath -m "$1"; else printf '%s' "$1"; fi
}
NATIVE_TOOLKIT="$(native_path "$TOOLKIT")"
NATIVE_ASSETS="$(native_path "$ASSETS")"

# Kept in step with GlobalToolkitDir in runtime/cmd/common.go, which probes it.
GLOBAL_TOOLKIT_DIR=".vibe-agent"

CLAUDE_HOME="$HOME_DIR/.claude"
CURSOR_HOME="$HOME_DIR/.cursor"
CODEX_HOME="${CODEX_HOME:-$HOME_DIR/.codex}"
AGENTS_HOME="$HOME_DIR/.agents"
OPENCODE_HOME="${XDG_CONFIG_HOME:-$HOME_DIR/.config}/opencode"

# A manifest records what this script owns, so --uninstall removes exactly that
# and never a hand-written asset that happens to share the prefix.
MANIFEST="$CLAUDE_HOME/.vibe-agent-global.manifest"
OLD_MANIFEST_COPY=""
if [ -f "$MANIFEST" ]; then
  OLD_MANIFEST_COPY="$(mktemp)"
  cp "$MANIFEST" "$OLD_MANIFEST_COPY"
fi

installed=0
linked=0
copied=0
drifted=0

# The manifest is shared with the PowerShell port, which cannot resolve the MSYS
# form (/c/Users/...) that Git Bash reports. Record the native form instead; Git
# Bash resolves C:/Users/... perfectly well, so this costs the shell nothing.
record() {
  printf '%s\n' "$(native_path "$1")" >> "$MANIFEST.tmp"
}

# Try a real symlink, then verify it. Git Bash on Windows accepts `ln -s` and
# silently copies unless MSYS=winsymlinks:nativestrict and Developer Mode are
# both set, so the verification is the point: without it this reports links it
# did not create, and the user never learns the assets can go stale.
link_or_copy() {
  src="$1"; dest="$2"
  if [ "$DRY" -eq 1 ]; then printf 'would install %s\n' "$dest"; return 0; fi
  rm -rf "$dest"
  mkdir -p "$(dirname "$dest")"
  if ln -s "$src" "$dest" 2>/dev/null && [ -L "$dest" ]; then
    linked=$((linked + 1))
  else
    rm -rf "$dest"
    cp -R "$src" "$dest"
    copied=$((copied + 1))
  fi
  record "$dest"
  installed=$((installed + 1))
}

# The rewrite lives in one function so --check can re-run it and compare. A
# check that re-derives the transform separately is a second implementation,
# and the two drift.
write_agent_body() {
  awk -v newname="$3" -v assets="$NATIVE_ASSETS" '
    NR == 1 && $0 == "---" { infm = 1; print; next }
    infm && /^name:[ \t]/ { print "name: " newname; next }
    infm && $0 == "---" { infm = 0; print; next }
    {
      # ../foo/bar.md is relative to .ai-agents/agents/, so one level up is the
      # assets root. Resolved from ~/.claude/agents/ it points at nothing.
      out = ""
      rest = $0
      while (match(rest, /\]\(\.\.\/[^)]+\)/)) {
        head = substr(rest, 1, RSTART)                   # up to and including "]"
        target = substr(rest, RSTART + 5, RLENGTH - 6)   # strip "](../" and ")"
        out = out head "(" assets "/" target ")"
        rest = substr(rest, RSTART + RLENGTH)
      }
      print out rest
    }
  ' "$1" > "$2"
}

write_agent() {
  src="$1"; dest="$2"; newname="$3"
  if [ "$DRY" -eq 1 ]; then printf 'would generate %s (name: %s)\n' "$dest" "$newname"; return 0; fi
  mkdir -p "$(dirname "$dest")"
  write_agent_body "$src" "$dest" "$newname"
  record "$dest"
  installed=$((installed + 1))
  copied=$((copied + 1))
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

write_command_skill() {
  src="$1"; dest="$2"; newname="$3"
  if [ "$DRY" -eq 1 ]; then printf 'would generate %s (name: %s)\n' "$dest" "$newname"; return 0; fi
  mkdir -p "$(dirname "$dest")"
  command_base="$(basename "$src")"
  description="$(frontmatter_description "$src")"
  [ -n "$description" ] || description="Run the vibe-agent $newname command"
  {
    printf -- '---\n'
    printf 'name: %s\n' "$newname"
    printf 'description: >-\n'
    printf '  Codex-compatible command adapter. Use only when the user explicitly mentions\n'
    printf '  $%s or asks to run this vibe-agent command: %s\n' "$newname" "$description"
    printf 'disable-model-invocation: true\n'
    printf -- '---\n\n'
    printf '# %s\n\n' "$newname"
    printf 'This is the Codex-compatible form of `%s/commands/%s`.\n' "$NATIVE_ASSETS" "$command_base"
    printf 'Codex CLI removed custom `/prompts` support in 0.117.0, so command prompts are exposed as explicit skills.\n\n'
    printf 'Treat any text after `$%s` as the command arguments, then follow the command prompt below.\n\n' "$newname"
    printf '<command_prompt>\n'
    awk 'BEGIN{n=0} /^---$/ { n++; next } n>=2 { print }' "$src" \
      | sed "s#../skills/#$NATIVE_ASSETS/skills/#g" \
      | sed "s#../references/#$NATIVE_ASSETS/references/#g" \
      | sed "s#../stack-profiles/#$NATIVE_ASSETS/stack-profiles/#g" \
      | sed "s#../commands/#$NATIVE_ASSETS/commands/#g" \
      | sed "s#../agents/#$NATIVE_ASSETS/agents/#g"
    printf '\n</command_prompt>\n'
  } > "$dest"
  record "$dest"
  installed=$((installed + 1))
  copied=$((copied + 1))
}

BEGIN_MARK="<!-- vibe-agent:begin -->"
END_MARK="<!-- vibe-agent:end -->"

pointer_text() {
  printf 'This machine has the vibe-agent toolkit installed at `%s`.\n\n' "$NATIVE_TOOLKIT"
  printf 'Router: `%s/ROUTER.md`. Charter: `%s/AGENTS.md`. Read those before applying a\n' \
    "$NATIVE_ASSETS" "$NATIVE_TOOLKIT"
  printf 'toolkit default. A repository'"'"'s own rules win over both.\n\n'
  printf 'Shared assets are installed under the `%s` prefix.\n' "$PREFIX"
}

# A marked block keeps the user's own file intact: anything outside the markers
# is theirs and is never touched.
write_managed_block() {
  file="$1"
  if [ "$DRY" -eq 1 ]; then printf 'would write managed block into %s\n' "$file"; return 0; fi
  mkdir -p "$(dirname "$file")"
  body="$BEGIN_MARK
$(pointer_text)$END_MARK"
  if [ -f "$file" ] && grep -qF "$BEGIN_MARK" "$file" 2>/dev/null; then
    awk -v b="$BEGIN_MARK" -v e="$END_MARK" -v body="$body" '
      $0 == b { print body; skip = 1; next }
      $0 == e { skip = 0; next }
      !skip { print }
    ' "$file" > "$file.tmp" && mv "$file.tmp" "$file"
  else
    [ -f "$file" ] && printf '\n' >> "$file"
    printf '%s\n' "$body" >> "$file"
  fi
  record "$file"
}

# Cursor has no global AGENTS.md. Its user-level rules are .mdc files with
# frontmatter, and alwaysApply puts this one in front of every session.
write_cursor_rule() {
  file="$CURSOR_HOME/rules/${PREFIX}toolkit.mdc"
  if [ "$DRY" -eq 1 ]; then printf 'would write %s\n' "$file"; return 0; fi
  mkdir -p "$(dirname "$file")"
  {
    printf -- '---\n'
    printf 'description: Points at the vibe-agent toolkit installed on this machine\n'
    printf 'alwaysApply: true\n'
    printf -- '---\n\n'
    pointer_text
  } > "$file"
  record "$file"
}

# The manifest is shared with the PowerShell port. Windows tools may rewrite it
# with CRLF, and a trailing CR makes every path miss by one character, which
# looks exactly like "nothing was installed". Read from a stripped copy rather
# than trusting the line endings.
manifest_lines() {
  tr -d '\015' < "$MANIFEST"
}

if [ "$MODE" = "uninstall" ]; then
  [ -f "$MANIFEST" ] || { echo "Nothing to uninstall: no manifest at $MANIFEST"; exit 0; }
  removed=0
  while IFS= read -r entry; do
    [ -n "$entry" ] || continue
    case "$entry" in
      *AGENTS.md|*CLAUDE.md)
        # A rules file is the user's. Strip only the marked block.
        [ -f "$entry" ] || continue
        awk -v b="$BEGIN_MARK" -v e="$END_MARK" '
          $0 == b { s = 1 } !s { print } $0 == e { s = 0 }
        ' "$entry" > "$entry.tmp" && mv "$entry.tmp" "$entry"
        removed=$((removed + 1))
        ;;
      *)
        { [ -e "$entry" ] || [ -L "$entry" ]; } || continue
        rm -rf "$entry"
        removed=$((removed + 1))
        ;;
    esac
  done <<MANIFEST_EOF
$(manifest_lines)
MANIFEST_EOF
  rm -f "$MANIFEST"
  echo "Removed $removed installed entries. Nothing outside the manifest was touched."
  exit 0
fi

if [ "$MODE" = "check" ]; then
  [ -f "$MANIFEST" ] || { echo "Not installed: no manifest at $MANIFEST"; exit 0; }
  while IFS= read -r entry; do
    [ -n "$entry" ] || continue
    [ -L "$entry" ] && continue          # a link cannot drift
    case "$entry" in *AGENTS.md|*CLAUDE.md|*.mdc) continue ;; esac
    base="$(basename "$entry")"
    stem="${base%.md}"
    plain="${stem#"$PREFIX"}"

    if [ ! -e "$entry" ]; then
      echo "MISSING  $entry"; drifted=$((drifted + 1)); continue
    fi

    # Skills and commands are verbatim copies, so any difference is drift. Agent
    # files are rewritten on install, so a byte comparison would report the
    # intended rewrite as drift on every run.
    if [ -d "$ASSETS/skills/$plain" ]; then
      diff -r "$ASSETS/skills/$plain" "$entry" >/dev/null 2>&1 \
        || { echo "DRIFTED  $entry"; drifted=$((drifted + 1)); }
    elif [ -f "$ASSETS/commands/$plain.md" ]; then
      cmp -s "$ASSETS/commands/$plain.md" "$entry" \
        || { echo "DRIFTED  $entry"; drifted=$((drifted + 1)); }
    elif [ -f "$ASSETS/agents/$plain.md" ]; then
      expected="$(mktemp)"
      write_agent_body "$ASSETS/agents/$plain.md" "$expected" "$PREFIX$plain"
      cmp -s "$expected" "$entry" || { echo "DRIFTED  $entry"; drifted=$((drifted + 1)); }
      rm -f "$expected"
    fi
  done <<MANIFEST_EOF
$(manifest_lines)
MANIFEST_EOF
  if [ "$drifted" -eq 0 ]; then
    echo "install-global --check: OK (no drift)"
    exit 0
  fi
  echo ""
  echo "$drifted entry(ies) differ from the toolkit. Re-run: sh scripts/install-global.sh"
  exit 1
fi

mkdir -p "$CLAUDE_HOME"
: > "$MANIFEST.tmp"

# Skills. These two directories reach all four tools; the rest is redundancy
# that would multiply copies on a platform without symlinks.
for dir in "$ASSETS"/skills/*/; do
  [ -f "$dir/SKILL.md" ] || continue
  name="$(basename "$dir")"
  link_or_copy "$dir" "$CLAUDE_HOME/skills/$PREFIX$name"
  link_or_copy "$dir" "$AGENTS_HOME/skills/$PREFIX$name"
done

# Commands. No shared convention exists, so each tool gets its own.
for file in "$ASSETS"/commands/*.md; do
  [ -f "$file" ] || continue
  name="$(basename "$file" .md)"
  case "$name" in ROUTER|TEMPLATE) continue ;; esac
  link_or_copy "$file" "$CLAUDE_HOME/commands/$PREFIX$name.md"
  link_or_copy "$file" "$CURSOR_HOME/commands/$PREFIX$name.md"
  link_or_copy "$file" "$OPENCODE_HOME/commands/$PREFIX$name.md"
  write_command_skill "$file" "$AGENTS_HOME/skills/$PREFIX$name/SKILL.md" "$PREFIX$name"
done

# Subagents, rewritten because the frontmatter carries the identity.
for file in "$ASSETS"/agents/*.md; do
  [ -f "$file" ] || continue
  name="$(basename "$file" .md)"
  case "$name" in ROUTER|TEMPLATE|README) continue ;; esac
  write_agent "$file" "$CLAUDE_HOME/agents/$PREFIX$name.md" "$PREFIX$name"
  write_agent "$file" "$CURSOR_HOME/agents/$PREFIX$name.md" "$PREFIX$name"
  write_agent "$file" "$OPENCODE_HOME/agents/$PREFIX$name.md" "$PREFIX$name"
done

# The control plane. `vibe-agent doctor` and the delivery commands need the
# workflow graphs and hook wiring, and both live under .ai-agents. Without this
# a repository that has not vendored the toolkit fails doctor on a missing
# .ai-agents/graphs even though nothing about it is repository-specific. The
# runtime probes ~/.vibe-agent last, after the workspace and any submodule, so
# a repository that does ship its own assets is never shadowed.
link_or_copy "$ASSETS" "$HOME_DIR/$GLOBAL_TOOLKIT_DIR/.ai-agents"

# Instructions. Codex concatenates the global file with the project's rather
# than choosing between them, so a pointer here is additive by construction.
write_managed_block "$CODEX_HOME/AGENTS.md"
write_managed_block "$OPENCODE_HOME/AGENTS.md"
write_managed_block "$CLAUDE_HOME/CLAUDE.md"
write_cursor_rule

if [ "$DRY" -eq 1 ]; then
  rm -f "$MANIFEST.tmp"
  [ -n "$OLD_MANIFEST_COPY" ] && rm -f "$OLD_MANIFEST_COPY"
  echo ""
  echo "Dry run. Nothing was written."
  exit 0
fi

if [ -n "$OLD_MANIFEST_COPY" ]; then
  while IFS= read -r entry; do
    [ -n "$entry" ] || continue
    if grep -Fxq "$entry" "$MANIFEST.tmp"; then
      continue
    fi
    case "$entry" in
      *".codex/prompts/$PREFIX"*|*".codex\\prompts\\$PREFIX"*) rm -f "$entry" ;;
    esac
  done < "$OLD_MANIFEST_COPY"
  rm -f "$OLD_MANIFEST_COPY"
fi
mv "$MANIFEST.tmp" "$MANIFEST"

# Install the binary too, mirroring link-ai-agents.sh. Linking .ai-agents into
# ~/.vibe-agent is what lets the runtime find its graphs, and that is worth
# nothing if the runtime is not on PATH: `vibe-agent doctor` would simply not be
# a command. Downloads a published release, falling back to a build from source.
#
# Skipped by VIBE_SKIP_RUNTIME, and in CI where a network download would make an
# unrelated outage look like a broken installer.
#
# Never fatal. The runtime is optional by design: without it every hook is a
# quiet no-op and the markdown assets work exactly as before, so a failed
# download must not undo an otherwise complete install.
install_runtime() {
  installer="$SCRIPT_DIR/install-runtime.sh"
  if [ -n "${VIBE_SKIP_RUNTIME:-}" ]; then
    echo ""
    echo "Runtime install skipped (VIBE_SKIP_RUNTIME set)."
    return 0
  fi
  if [ -n "${CI:-}" ]; then
    echo ""
    echo "Runtime install skipped (CI). Run sh scripts/install-runtime.sh to install it."
    return 0
  fi
  if [ ! -f "$installer" ]; then
    echo ""
    echo "No install-runtime.sh next to this script; skipped the runtime." >&2
    return 0
  fi

  echo ""
  if command -v vibe-agent >/dev/null 2>&1; then
    echo "Refreshing the runtime binary (installed: $(vibe-agent version 2>/dev/null || echo unknown))..."
  else
    echo "Installing the runtime binary..."
  fi
  if bash "$installer"; then
    return 0
  fi
  echo "Runtime install did not complete. The assets above are installed and usable;" >&2
  echo "the delivery commands that need the control plane are not. Retry with:" >&2
  echo "  bash $installer" >&2
  return 0
}

echo ""
echo "Installed $installed entries: $linked symlinked, $copied copied."
echo "  skills      $CLAUDE_HOME/skills, $AGENTS_HOME/skills   (all four tools; Codex command adapters in $AGENTS_HOME/skills)"
echo "  commands    claude, cursor, opencode                  (as /${PREFIX}<name> where supported)"
echo "  codex       $AGENTS_HOME/skills/${PREFIX}<name>       (Codex form: \$${PREFIX}<name>)"
echo "  subagents   generated with name: ${PREFIX}<name>"
echo "  rules       marked block in each global instructions file, plus a Cursor .mdc"
echo "  manifest    $MANIFEST"

if [ "$copied" -gt 0 ]; then
  echo ""
  echo "$copied entries are copies, not links, so they go stale when the toolkit changes."
  if command -v cygpath >/dev/null 2>&1; then
    echo "Git Bash cannot create Windows symlinks. For live links, run the PowerShell"
    echo "port instead, with Developer Mode on:"
    echo "  powershell -ExecutionPolicy Bypass -File scripts/install-global.ps1"
  fi
  echo "Otherwise re-run this script after editing an asset, and use --check for drift."
fi

echo ""
echo "Permissions and hooks were not installed. To apply this repo's policy to a"
echo "project, run link-ai-agents.sh in that project instead."
echo ""
echo "Codex does not install these as top-level /${PREFIX}<name> commands in CLI 0.147.0, because custom prompts were removed."

install_runtime
