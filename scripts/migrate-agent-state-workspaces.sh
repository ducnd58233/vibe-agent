#!/usr/bin/env bash
# Discover workspaces that already have .agent-state under the given roots and
# run `vibe-agent migrate state` on each. Never scans or writes under G:.
# Skips the vibe-agent toolkit checkout itself.
#
# Usage:
#   bash scripts/migrate-agent-state-workspaces.sh --dry-run D:/projects D:/research
#   bash scripts/migrate-agent-state-workspaces.sh D:/projects D:/competitions D:/research
#   TOOLKIT=/path/to/vibe-agent bash scripts/migrate-agent-state-workspaces.sh ...
#
# Paths: this script resolves the toolkit from its own directory so it works
# no matter what the caller's cwd is.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TOOLKIT="${TOOLKIT:-$(cd "${SCRIPT_DIR}/.." && pwd)}"
DRY_RUN=0
ROOTS=()

die() { echo "migrate-agent-state-workspaces: $*" >&2; exit 1; }

while [ $# -gt 0 ]; do
  case "$1" in
    --dry-run) DRY_RUN=1; shift ;;
    --toolkit)
      [ $# -ge 2 ] || die "--toolkit needs a path"
      TOOLKIT="$2"
      shift 2
      ;;
    --toolkit=*)
      TOOLKIT="${1#*=}"
      shift
      ;;
    -h|--help)
      sed -n '2,12p' "$0" | sed 's/^# \{0,1\}//'
      exit 0
      ;;
    --) shift; break ;;
    -*) die "unknown option: $1" ;;
    *) ROOTS+=("$1"); shift ;;
  esac
done
while [ $# -gt 0 ]; do
  ROOTS+=("$1")
  shift
done

[ ${#ROOTS[@]} -gt 0 ] || die "pass one or more roots (for example D:/projects D:/research)"

command -v vibe-agent >/dev/null 2>&1 || die "vibe-agent not on PATH; run scripts/install-runtime.sh first"

# Reject G: /g/ roots and any workspace whose resolved path is the toolkit.
is_forbidden() {
  local path="$1"
  local norm
  norm="$(cd "$path" 2>/dev/null && pwd -P)" || return 0
  case "$norm" in
    /[gG]/*|[gG]:*|/*/[gG]/*) return 0 ;;
  esac
  # Windows drive letter via cygpath when available
  if command -v cygpath >/dev/null 2>&1; then
    local win
    win="$(cygpath -w "$norm" 2>/dev/null || true)"
    case "$win" in
      [gG]:*) return 0 ;;
    esac
  fi
  local toolkit_norm
  toolkit_norm="$(cd "$TOOLKIT" && pwd -P)"
  if [ "$norm" = "$toolkit_norm" ]; then
    return 0
  fi
  return 1
}

# Collect unique workspace roots that contain .agent-state (maxdepth 6).
declare -a TARGETS=()
declare -A SEEN=()

add_target() {
  local ws="$1"
  local key
  key="$(cd "$ws" && pwd -P)"
  if [ -n "${SEEN[$key]+x}" ]; then
    return
  fi
  if is_forbidden "$ws"; then
    echo "skip (forbidden or toolkit): $ws"
    return
  fi
  SEEN[$key]=1
  TARGETS+=("$ws")
}

for root in "${ROOTS[@]}"; do
  if is_forbidden "$root"; then
    echo "skip root (forbidden): $root"
    continue
  fi
  [ -d "$root" ] || { echo "skip missing root: $root"; continue; }
  while IFS= read -r -d '' state_dir; do
    add_target "$(dirname "$state_dir")"
  done < <(find "$root" -maxdepth 6 -type d -name '.agent-state' -print0 2>/dev/null)
done

	if [ ${#TARGETS[@]} -eq 0 ]; then
  echo "no .agent-state workspaces found under: ${ROOTS[*]}"
  exit 0
fi

echo "toolkit: $TOOLKIT"
echo "targets: ${#TARGETS[@]}"
failed=0
for ws in "${TARGETS[@]}"; do
  if [ "$DRY_RUN" -eq 1 ]; then
    echo "dry-run: would migrate $ws"
    continue
  fi
  echo "=== migrate state: $ws ==="
  if ! vibe-agent migrate state --workspace "$ws" --toolkit "$TOOLKIT"; then
    echo "FAILED: $ws" >&2
    failed=$((failed + 1))
  fi
done
if [ "$failed" -ne 0 ]; then
  die "$failed workspace(s) failed migrate state"
fi
