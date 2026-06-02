#!/usr/bin/env bash
# Verifies each `.ai-agents/<folder>/ROUTER.md` lookup table matches on-disk routable assets.
# Does not validate `.ai-agents/ROUTER.md` (hub index, not a folder manifest).
#
# Usage: from toolkit repository root — bash scripts/check-ai-agents-routers.sh
# Optional: set AI_AGENTS_ROOT to override the .ai-agents path.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
AI_CANDIDATE="${AI_AGENTS_ROOT:-}"
if [[ -n "$AI_CANDIDATE" ]]; then
  AI="$AI_CANDIDATE"
elif [[ -d "$ROOT/.ai-agents" ]]; then
  AI="$ROOT/.ai-agents"
elif [[ -d "$ROOT/.vibe-agent/.ai-agents" ]]; then
  AI="$ROOT/.vibe-agent/.ai-agents"
else
  AI="$ROOT/.ai-agents"
fi

fail=0

die() {
  echo "check-ai-agents-routers: FAILED" >&2
  exit 1
}

# First markdown table body: keep rows after the |---| separator; stop at first non-| line once body rows exist.
get_table_body_rows() {
  local file="$1"
  awk '
  {
    t = $0
    sub(/[ \t]+$/, "", t)
    if (t !~ /^\|/) {
      if (seenSep && count > 0) exit
      next
    }
    if (t ~ /^\|[[:space:]\-:|]+\|[[:space:]]*$/) {
      seenSep = 1
      next
    }
    if (!seenSep) next
    print
    count++
  }
  ' "$file"
}

extract_link_targets() {
  perl -ne 'while (/\[[^\]]*\]\(([^)]+)\)/g) { print "$1\n" }' <<< "$1"
}

# Trim-split markdown table row; print link targets only from 0-based column index.
links_in_column() {
  local line="$1"
  local idx="$2"
  export LINE="$line"
  export COL_IDX="$idx"
  perl -e '
    use strict; use warnings;
    my $line = $ENV{LINE} // "";
    my $col_idx = int($ENV{COL_IDX});
    chomp $line;
    $line =~ s/^\|//;
    $line =~ s/\|\s*$//;
    my @c = ();
    for my $chunk (split(/\|/, $line, -1)) {
      $chunk =~ s/^\s+|\s+$//g;
      push @c, $chunk;
    }
    my $cell = $c[$col_idx] // "";
    while ($cell =~ /\[[^\]]*\]\(([^)]+)\)/g) {
      print "$1\n";
    }
  '
  unset LINE COL_IDX
}

compare_sets() {
  local label="$1"
  local disk_raw="$2"
  local table_raw="$3"

  local d t
  d="$(mktemp)"
  t="$(mktemp)"
  printf '%s\n' "$disk_raw" | grep -v '^[[:space:]]*$' | sort -u > "$d"
  printf '%s\n' "$table_raw" | grep -v '^[[:space:]]*$' | sort -u > "$t"

  if cmp -s "$d" "$t"; then
    rm -f "$d" "$t"
    return 0
  fi

  echo "" >&2
  echo "[$label]" >&2
  local missing stale
  missing=$(comm -23 "$d" "$t")
  stale=$(comm -13 "$d" "$t")
  if [[ -n "$missing" ]]; then
    echo "  Missing from ROUTER.md: $(echo "$missing" | paste -sd, -)" >&2
  fi
  if [[ -n "$stale" ]]; then
    echo "  Listed in ROUTER.md but not on disk: $(echo "$stale" | paste -sd, -)" >&2
  fi
  rm -f "$d" "$t"
  return 1
}

nonempty_lines() {
  printf '%s' "$1" | grep -c . 2>/dev/null || true
}

check_skills() {
  local dir="$AI/skills"
  local disk=""
  local ent

  shopt -s nullglob
  for ent in "$dir"/*; do
    [[ -d "$ent" ]] || continue
    local base
    base="$(basename "$ent")"
    [[ "$base" != .* ]] || continue
    [[ -f "$ent/SKILL.md" ]] || continue
    disk+="$base"$'\n'
  done
  shopt -u nullglob

  local table=""
  local row target
  while IFS= read -r row; do
    while IFS= read -r target; do
      [[ -z "$target" ]] && continue
      [[ "$target" == *'/'* ]] && continue
      [[ "$target" == *..* ]] && continue
      [[ "$target" == *.md ]] && continue
      [[ "$target" =~ ^[a-zA-Z0-9_.-]+$ ]] || continue
      table+="$target"$'\n'
    done < <(extract_link_targets "$row")
  done < <(get_table_body_rows "$dir/ROUTER.md")

  compare_sets ".ai-agents/skills/ROUTER.md" "$disk" "$table" || fail=1
}

check_md_router() {
  local label="$1"
  local subdir="$2"
  local extra_exclude="$3"
  local dir="$AI/$subdir"
  local disk=""
  local f

  shopt -s nullglob
  for f in "$dir"/*.md; do
    local b
    b="$(basename "$f")"
    [[ "$b" == "ROUTER.md" ]] && continue
    [[ "$b" == "TEMPLATE.md" ]] && continue
    case " $extra_exclude " in
      *" $b "*) continue ;;
    esac
    disk+="$b"$'\n'
  done
  shopt -u nullglob

  local table=""
  local row target
  while IFS= read -r row; do
    while IFS= read -r target; do
      [[ -z "$target" ]] && continue
      [[ "$target" == *'/'* ]] && continue
      [[ "$target" == *..* ]] && continue
      [[ "$target" != *.md ]] && continue
      [[ "$target" == "ROUTER.md" ]] && continue
      [[ "$target" == "TEMPLATE.md" ]] && continue
      case " $extra_exclude " in
        *" $target "*) continue ;;
      esac
      table+="$target"$'\n'
    done < <(extract_link_targets "$row")
  done < <(get_table_body_rows "$dir/ROUTER.md")

  compare_sets "$label" "$disk" "$table" || fail=1
}

check_stack_profiles() {
  local dir="$AI/stack-profiles"
  local disk=""
  local f

  shopt -s nullglob
  for f in "$dir"/*.md; do
    local b
    b="$(basename "$f")"
    [[ "$b" == "README.md" || "$b" == "ROUTER.md" || "$b" == "TEMPLATE.md" ]] && continue
    disk+="$b"$'\n'
  done
  shopt -u nullglob

  local table=""
  local row target
  while IFS= read -r row; do
    while IFS= read -r target; do
      [[ -z "$target" ]] && continue
      [[ "$target" == *'/'* ]] && continue
      [[ "$target" == *..* ]] && continue
      [[ "$target" != *.md ]] && continue
      [[ "$target" == "README.md" || "$target" == "ROUTER.md" || "$target" == "TEMPLATE.md" ]] && continue
      table+="$target"$'\n'
    done < <(links_in_column "$row" "0")
  done < <(get_table_body_rows "$dir/ROUTER.md")

  local disk_n table_n
  disk_n="$(printf '%s' "$disk" | grep -c . 2>/dev/null || true)"
  disk_n="${disk_n:-0}"
  table_n="$(printf '%s' "$table" | grep -c . 2>/dev/null || true)"
  table_n="${table_n:-0}"

  if [[ "$disk_n" -eq 0 ]]; then
    if [[ "$table_n" -eq 0 ]]; then
      return 0
    fi
    echo "" >&2
    echo "[.ai-agents/stack-profiles/ROUTER.md]" >&2
    echo "  No profile *.md on disk but ROUTER lists: $(printf '%s\n' "$table" | grep -v '^$' | sort -u | paste -sd, -)" >&2
    fail=1
    return 0
  fi

  compare_sets ".ai-agents/stack-profiles/ROUTER.md" "$disk" "$table" || fail=1
}

check_hooks() {
  local dir="$AI/hooks"
  local hook_files=""
  local f

  shopt -s nullglob
  for f in "$dir"/*.sh "$dir"/*.py "$dir"/*.ps1; do
    [[ -f "$f" ]] || continue
    hook_files+="$(basename "$f")"$'\n'
  done
  shopt -u nullglob

  local hook_n
  hook_n="$(printf '%s' "$hook_files" | grep -c . 2>/dev/null || true)"
  hook_n="${hook_n:-0}"
  if [[ "$hook_n" -eq 0 ]]; then
    return 0
  fi

  local table=""
  local row target
  while IFS= read -r row; do
    while IFS= read -r target; do
      [[ -z "$target" ]] && continue
      [[ "$target" == *'/'* ]] && continue
      [[ "$target" == *..* ]] && continue
      [[ "$target" == *.sh || "$target" == *.py || "$target" == *.ps1 ]] || continue
      table+="$target"$'\n'
    done < <(links_in_column "$row" "1")
  done < <(get_table_body_rows "$dir/ROUTER.md")

  compare_sets ".ai-agents/hooks/ROUTER.md" "$hook_files" "$table" || fail=1
}

# Routing evals: every relative link target in references/routing-evals.md must exist.
check_routing_evals() {
  local f="$AI/references/routing-evals.md"
  [[ -f "$f" ]] || return 0  # optional fixtures file
  local missing=0 target
  while IFS= read -r target; do
    [[ "$target" == ../* ]] || continue
    if [[ ! -e "$AI/references/$target" ]]; then
      if [[ "$missing" -eq 0 ]]; then
        echo "" >&2
        echo "[.ai-agents/references/routing-evals.md]" >&2
      fi
      echo "  Routing eval target missing: $target" >&2
      missing=1
    fi
  done < <(extract_link_targets "$(cat "$f")")
  [[ "$missing" -eq 0 ]] || fail=1
}

main() {
  if [[ ! -d "$AI" ]]; then
    echo "Missing .ai-agents directory at $AI" >&2
    exit 1
  fi

  check_skills
  check_md_router ".ai-agents/agents/ROUTER.md" "agents" "README.md"
  check_md_router ".ai-agents/commands/ROUTER.md" "commands" "README.md"
  check_md_router ".ai-agents/references/ROUTER.md" "references" ""
  check_stack_profiles
  check_hooks
  check_routing_evals

  if [[ "$fail" -ne 0 ]]; then
    echo "" >&2
    die
  fi
  echo "check-ai-agents-routers: OK"
}

main "$@"
