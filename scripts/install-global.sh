#!/usr/bin/env sh
# Install this toolkit into the user-level asset directories, so every project on
# the machine can use it without a wrapper workspace holding it as a submodule.
#
# Companion to link-ai-agents.sh, not a replacement. That script wires one
# workspace; this one wires the machine. They can both be in effect: a project
# keeps its own .claude/ and still sees the global assets.
#
# Everything installed is prefixed `vibe-`, and the prefix is load-bearing rather
# than decorative. Claude Code resolves a skill-name collision in favour of the
# personal level ("personal overrides project"), which would make this toolkit
# override a repository's own skills instead of being the fallback that
# AGENTS.md says it is. A prefix means the two can never collide, so the
# precedence question never arises.
#
# Three install strategies, because the harnesses disagree about where an
# asset's identity comes from:
#
#   skills    the command name is the directory name, so a renamed symlink is
#             enough, and edits in the repo are live
#   commands  the command name is the file name, same deal
#   agents    the identity is the frontmatter `name:` field and "the filename
#             doesn't have to match", so a renamed file namespaces nothing.
#             These are generated copies with the field rewritten, which means
#             they go stale: re-run after editing an agent, or use --check
#
# Permissions and hooks are deliberately NOT installed. This repo's settings
# carry 21 deny rules and hooks on six events; applying those to every unrelated
# repository on the machine is a decision for the user to make by hand, not a
# side effect of installing some markdown.
#
# Usage:
#   sh scripts/install-global.sh [--dry-run] [--check] [--uninstall] [--prefix P]
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

# Paths written into generated files are read by Windows-native tools, which do
# not understand the MSYS form Git Bash reports. `cygpath -m` gives D:/path/...;
# everywhere else the path is already native.
native_path() {
  if command -v cygpath >/dev/null 2>&1; then cygpath -m "$1"; else printf '%s' "$1"; fi
}
NATIVE_TOOLKIT="$(native_path "$TOOLKIT")"
NATIVE_ASSETS="$(native_path "$ASSETS")"

HOME_DIR="${HOME:-$USERPROFILE}"
[ -n "$HOME_DIR" ] && [ -d "$HOME_DIR" ] || { echo "Cannot resolve a home directory" >&2; exit 1; }

CLAUDE_HOME="$HOME_DIR/.claude"
CODEX_HOME="${CODEX_HOME:-$HOME_DIR/.codex}"
OPENCODE_HOME="${XDG_CONFIG_HOME:-$HOME_DIR/.config}/opencode"

# A marker file records what this script owns, so --uninstall removes exactly
# that and never a hand-written asset that happens to share the prefix.
MANIFEST="$CLAUDE_HOME/.vibe-agent-global.manifest"

installed=0
linked=0
copied=0
drifted=0

say() { [ "$DRY" -eq 1 ] && printf 'would %s\n' "$*" || printf '%s\n' "$*"; }

# Try a real symlink, verify it, and fall back to a copy. Git Bash on Windows
# accepts `ln -s` and silently copies unless MSYS=winsymlinks:nativestrict and
# Developer Mode are both set, so the verification is the point: without it the
# script reports links it did not create.
link_or_copy() {
  src="$1"; dest="$2"
  [ "$DRY" -eq 1 ] && { printf 'would install %s\n' "$dest"; return 0; }
  rm -rf "$dest"
  mkdir -p "$(dirname "$dest")"
  if ln -s "$src" "$dest" 2>/dev/null && [ -L "$dest" ]; then
    linked=$((linked + 1))
  else
    rm -rf "$dest"
    cp -R "$src" "$dest"
    copied=$((copied + 1))
  fi
  printf '%s\n' "$dest" >> "$MANIFEST.tmp"
  installed=$((installed + 1))
}

# A subagent is identified by its frontmatter `name:`, so namespacing it means
# rewriting the file. Relative links are rewritten to absolute paths at the same
# time: `../skills/x/SKILL.md` resolved from ~/.claude/agents/ would point at a
# directory that does not exist, and a broken pointer in an always-available
# asset is worse than no asset.
# The transform lives here alone so that --check can re-run it and compare. A
# check that re-derives the rewrite separately is a second implementation, and
# the two drift.
write_agent_body() {
  src="$1"; dest="$2"; newname="$3"
  awk -v newname="$newname" -v assets="$NATIVE_ASSETS" '
    NR == 1 && $0 == "---" { infm = 1; print; next }
    infm && /^name:[ \t]/ { print "name: " newname; next }
    infm && $0 == "---" { infm = 0; print; next }
    {
      # ../foo/bar.md is relative to .ai-agents/agents/, so one level up is the
      # assets root. Resolved from ~/.claude/agents/ it would point at nothing.
      out = ""
      rest = $0
      while (match(rest, /\]\(\.\.\/[^)]+\)/)) {
        head = substr(rest, 1, RSTART)            # up to and including "]"
        # The match is "](../" + target + ")", so 5 leading and 1 trailing char.
        target = substr(rest, RSTART + 5, RLENGTH - 6)
        out = out head "(" assets "/" target ")"
        rest = substr(rest, RSTART + RLENGTH)
      }
      print out rest
    }
  ' "$src" > "$dest"
}

write_agent() {
  src="$1"; dest="$2"; newname="$3"
  [ "$DRY" -eq 1 ] && { printf 'would generate %s (name: %s)\n' "$dest" "$newname"; return 0; }
  mkdir -p "$(dirname "$dest")"
  write_agent_body "$src" "$dest" "$newname"
  printf '%s\n' "$dest" >> "$MANIFEST.tmp"
  installed=$((installed + 1))
  copied=$((copied + 1))
}

# A managed block keeps the user's own file intact. Anything outside the markers
# is theirs and is never touched.
write_managed_block() {
  file="$1"
  begin="<!-- vibe-agent:begin -->"
  end="<!-- vibe-agent:end -->"
  body="$(printf '%s\n%s\n\n%s\n%s\n' \
    "$begin" \
    "This machine has the vibe-agent toolkit installed at \`$NATIVE_TOOLKIT\`." \
    "Router: \`$NATIVE_ASSETS/ROUTER.md\`. Charter: \`$NATIVE_TOOLKIT/AGENTS.md\`. Read those before applying a toolkit default. A repository's own rules win over both." \
    "$end")"
  [ "$DRY" -eq 1 ] && { printf 'would write managed block into %s\n' "$file"; return 0; }
  mkdir -p "$(dirname "$file")"
  if [ -f "$file" ] && grep -qF "$begin" "$file" 2>/dev/null; then
    awk -v b="$begin" -v e="$end" -v body="$body" '
      $0 == b { print body; skip = 1; next }
      $0 == e { skip = 0; next }
      !skip { print }
    ' "$file" > "$file.tmp" && mv "$file.tmp" "$file"
  else
    [ -f "$file" ] && printf '\n' >> "$file"
    printf '%s\n' "$body" >> "$file"
  fi
  printf '%s\n' "$file" >> "$MANIFEST.tmp"
}

if [ "$MODE" = "uninstall" ]; then
  [ -f "$MANIFEST" ] || { echo "Nothing to uninstall: no manifest at $MANIFEST"; exit 0; }
  removed=0
  while IFS= read -r entry; do
    [ -n "$entry" ] || continue
    case "$entry" in
      *AGENTS.md|*CLAUDE.md)
        # A rules file is the user's; strip only the managed block.
        [ -f "$entry" ] || continue
        awk '/<!-- vibe-agent:begin -->/{s=1} !s{print} /<!-- vibe-agent:end -->/{s=0}' \
          "$entry" > "$entry.tmp" && mv "$entry.tmp" "$entry"
        removed=$((removed + 1))
        ;;
      *)
        [ -e "$entry" ] || [ -L "$entry" ] || continue
        rm -rf "$entry"
        removed=$((removed + 1))
        ;;
    esac
  done < "$MANIFEST"
  rm -f "$MANIFEST"
  echo "Removed $removed installed entries. Nothing outside the manifest was touched."
  exit 0
fi

if [ "$MODE" = "check" ]; then
  [ -f "$MANIFEST" ] || { echo "Not installed: no manifest at $MANIFEST"; exit 0; }
  while IFS= read -r entry; do
    [ -n "$entry" ] || continue
    [ -L "$entry" ] && continue          # a link cannot drift
    case "$entry" in *AGENTS.md|*CLAUDE.md) continue ;; esac
    [ -e "$entry" ] || { echo "MISSING  $entry"; drifted=$((drifted + 1)); continue; }
    base="$(basename "$entry")"
    stem="${base%.md}"
    plain="${stem#"$PREFIX"}"

    # Skills and commands are verbatim copies, so any difference is drift. Agent
    # files are rewritten on install (`name:` and the links), so comparing them
    # byte for byte would report the intended rewrite as drift on every run.
    if [ -d "$ASSETS/skills/$plain" ]; then
      diff -r "$ASSETS/skills/$plain" "$entry" >/dev/null 2>&1 \
        || { echo "DRIFTED  $entry"; drifted=$((drifted + 1)); }
    elif [ -f "$ASSETS/commands/$plain.md" ]; then
      cmp -s "$ASSETS/commands/$plain.md" "$entry" \
        || { echo "DRIFTED  $entry"; drifted=$((drifted + 1)); }
    elif [ -f "$ASSETS/agents/$plain.md" ]; then
      # Re-run the same rewrite and compare against that, so the check tests what
      # install actually produces instead of a second guess at it.
      expected="$(mktemp)"
      NATIVE_ASSETS="$NATIVE_ASSETS" write_agent_body "$ASSETS/agents/$plain.md" "$expected" "$PREFIX$plain"
      cmp -s "$expected" "$entry" || { echo "DRIFTED  $entry"; drifted=$((drifted + 1)); }
      rm -f "$expected"
    fi
  done < "$MANIFEST"
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

for dir in "$ASSETS"/skills/*/; do
  [ -f "$dir/SKILL.md" ] || continue
  name="$(basename "$dir")"
  link_or_copy "$dir" "$CLAUDE_HOME/skills/$PREFIX$name"
  [ -d "$CODEX_HOME" ] && link_or_copy "$dir" "$CODEX_HOME/skills/$PREFIX$name"
done

for file in "$ASSETS"/commands/*.md; do
  [ -f "$file" ] || continue
  name="$(basename "$file" .md)"
  case "$name" in ROUTER|TEMPLATE) continue ;; esac
  link_or_copy "$file" "$CLAUDE_HOME/commands/$PREFIX$name.md"
  [ -d "$OPENCODE_HOME" ] && link_or_copy "$file" "$OPENCODE_HOME/commands/$PREFIX$name.md"
done

for file in "$ASSETS"/agents/*.md; do
  [ -f "$file" ] || continue
  name="$(basename "$file" .md)"
  case "$name" in ROUTER|TEMPLATE|README) continue ;; esac
  write_agent "$file" "$CLAUDE_HOME/agents/$PREFIX$name.md" "$PREFIX$name"
  [ -d "$OPENCODE_HOME" ] && write_agent "$file" "$OPENCODE_HOME/agents/$PREFIX$name.md" "$PREFIX$name"
done

# Codex concatenates a global AGENTS.md with the project's rather than choosing
# between them, so a pointer here is additive and cannot shadow a repository.
[ -d "$CODEX_HOME" ] && write_managed_block "$CODEX_HOME/AGENTS.md"
[ -d "$OPENCODE_HOME" ] && write_managed_block "$OPENCODE_HOME/AGENTS.md"

if [ "$DRY" -eq 1 ]; then
  rm -f "$MANIFEST.tmp"
  echo ""
  echo "Dry run. Nothing was written."
  exit 0
fi

mv "$MANIFEST.tmp" "$MANIFEST"

echo ""
echo "Installed $installed entries: $linked symlinked, $copied copied."
echo "  skills and commands  $CLAUDE_HOME  (invoke as /${PREFIX}<name>)"
echo "  subagents            generated with name: ${PREFIX}<name>"
[ -d "$CODEX_HOME" ] && echo "  codex                $CODEX_HOME"
[ -d "$OPENCODE_HOME" ] && echo "  opencode             $OPENCODE_HOME"
echo "  manifest             $MANIFEST"

if [ "$copied" -gt 0 ]; then
  echo ""
  echo "$copied entries are copies, not links, so they go stale when the toolkit changes."
  echo "Re-run this script after editing an asset, and use --check to detect drift."
fi

cat <<CURSOR

Cursor cannot be scripted: its global rules are a settings field, not a file.
Open Cursor, go to Customize -> Rules, and add:

  The vibe-agent toolkit is installed at $NATIVE_TOOLKIT.
  Read $NATIVE_ASSETS/ROUTER.md before applying a default. A repository's own rules win.

Permissions and hooks were not installed. To apply this repo's policy to a
project, run link-ai-agents.sh in that project instead.
CURSOR
