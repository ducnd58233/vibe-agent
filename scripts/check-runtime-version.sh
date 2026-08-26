#!/usr/bin/env bash
# Fixtures for scripts/runtime-version.sh. Runs from repository root.
#
# Usage: bash scripts/check-runtime-version.sh
# Fixture covers Z+N monotonic patch (find-how-ci-publish).

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
VERSION_SH="$SCRIPT_DIR/runtime-version.sh"

fail=0

assert_contains() {
  local haystack="$1"
  local needle="$2"
  local label="$3"
  if [[ "$haystack" != *"$needle"* ]]; then
    echo "check-runtime-version: ${label}: expected output to contain ${needle}" >&2
    echo "  got: ${haystack}" >&2
    fail=1
  fi
}

assert_eq() {
  local got="$1"
  local want="$2"
  local label="$3"
  if [[ "$got" != "$want" ]]; then
    echo "check-runtime-version: ${label}: want ${want}, got ${got}" >&2
    fail=1
  fi
}

cd "$ROOT"

if [[ ! -x "$VERSION_SH" ]]; then
  chmod +x "$VERSION_SH"
fi

# stable and dispatch are deterministic given explicit inputs.
eval "$(bash "$VERSION_SH" stable runtime/v0.1.0)"
assert_eq "$tag" "runtime/v0.1.0" "stable tag"
assert_eq "$version" "v0.1.0" "stable version"
assert_eq "$prerelease" "false" "stable prerelease"

eval "$(bash "$VERSION_SH" dispatch v0.2.0)"
assert_eq "$tag" "runtime/v0.2.0" "dispatch tag"
assert_eq "$version" "v0.2.0" "dispatch version"

# rolling: X.Y.(Z+N)-dev.sha — triple advances with commit count N.
rolling_out="$(bash "$VERSION_SH" rolling)"
eval "$rolling_out"
assert_eq "$tag" "runtime/latest" "rolling tag"
assert_eq "$prerelease" "true" "rolling prerelease"
if [[ ! "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+-dev\.[0-9a-f]+$ ]]; then
  echo "check-runtime-version: rolling version shape: want MAJOR.MINOR.PATCH-dev.sha, got ${version}" >&2
  fail=1
fi

latest="$(git tag --list 'runtime/v*' --sort=-v:refname | head -n1)"
if [[ -n "$latest" ]]; then
  semver="${latest#runtime/v}"
  z="$(echo "$semver" | cut -d. -f3)"
  n="$(git rev-list --count "${latest}..HEAD")"
  want_patch=$((z + n))
  triple="${version%%-dev*}"
  got_patch="$(echo "$triple" | cut -d. -f3)"
  assert_eq "$got_patch" "$want_patch" "rolling patch equals Z+N"
  # Must not freeze at the old conventional preview base.
  if [[ "$triple" == "0.2.0" ]]; then
    echo "check-runtime-version: rolling triple still stuck at 0.2.0" >&2
    fail=1
  fi
fi

# Isolated repo: N=5 then N=6 must raise the patch by exactly one.
fixture="$(mktemp -d)"
cleanup_fixture() { rm -rf "$fixture"; }
trap cleanup_fixture EXIT
(
  set -euo pipefail
  cd "$fixture"
  git init -q
  git config user.email "check@example.com"
  git config user.name "check"
  echo a >f && git add f && git commit -qm "init"
  git tag runtime/v0.1.0
  for i in 1 2 3 4 5; do
    echo "$i" >f && git commit -qam "c$i"
  done
  eval "$(bash "$VERSION_SH" rolling)"
  t5="${version%%-dev*}"
  p5="$(echo "$t5" | cut -d. -f3)"
  echo 6 >f && git commit -qam "c6"
  eval "$(bash "$VERSION_SH" rolling)"
  t6="${version%%-dev*}"
  p6="$(echo "$t6" | cut -d. -f3)"
  if [[ "$t5" != "0.1.5" ]]; then
    echo "check-runtime-version: fixture N=5: want 0.1.5, got ${t5}" >&2
    exit 1
  fi
  if [[ "$t6" != "0.1.6" ]]; then
    echo "check-runtime-version: fixture N=6: want 0.1.6, got ${t6}" >&2
    exit 1
  fi
  if [[ "$p6" -le "$p5" ]]; then
    echo "check-runtime-version: fixture patch did not rise (${p5} -> ${p6})" >&2
    exit 1
  fi
) || fail=1

if [[ "$fail" -ne 0 ]]; then
  echo "check-runtime-version: FAILED" >&2
  exit 1
fi

echo "check-runtime-version: OK"
