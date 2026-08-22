#!/usr/bin/env bash
# Fixtures for scripts/runtime-version.sh. Runs from repository root.
#
# Usage: bash scripts/check-runtime-version.sh

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

# rolling must include a monotonic build counter between -dev. and the commit sha.
rolling_out="$(bash "$VERSION_SH" rolling)"
eval "$rolling_out"
assert_eq "$tag" "runtime/latest" "rolling tag"
assert_eq "$prerelease" "true" "rolling prerelease"
if [[ ! "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+-dev\.[0-9]+\.[0-9a-f]+$ ]]; then
  echo "check-runtime-version: rolling version shape: want MAJOR.MINOR.PATCH-dev.N.sha, got ${version}" >&2
  fail=1
fi

latest="$(git tag --list 'runtime/v*' --sort=-v:refname | head -n1)"
if [[ -n "$latest" ]]; then
  want_build="$(git rev-list --count "${latest}..HEAD")"
  assert_contains "$version" "-dev.${want_build}." "rolling build counter"
fi

if [[ "$fail" -ne 0 ]]; then
  echo "check-runtime-version: FAILED" >&2
  exit 1
fi

echo "check-runtime-version: OK"
