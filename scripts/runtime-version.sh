#!/usr/bin/env bash
# Computes runtime release tag, version, and metadata for GitHub Releases.
#
# Rolling builds reuse tag runtime/latest. The version triple is
# X.Y.(Z+N) from the latest stable runtime/v* tag (X.Y.Z) plus the commit
# count N since that tag, so every publish advances the visible semver
# (0.1.1, 0.1.2, …) instead of freezing after the first conventional bump.
# A -dev.<sha> suffix keeps the binary tied to the commit that built it.
#
# Usage (from repository root):
#   eval "$(bash scripts/runtime-version.sh rolling)"
#   eval "$(bash scripts/runtime-version.sh stable runtime/v0.1.0)"
#   eval "$(bash scripts/runtime-version.sh dispatch v0.2.0)"
#
# Prints shell assignments: tag, version, prerelease, title

set -euo pipefail

mode="${1:?mode required: rolling, stable, or dispatch}"

latest_stable_tag() {
  git fetch --tags --quiet 2>/dev/null || true
  git tag --list 'runtime/v*' --sort=-v:refname | head -n1
}

# Rolling: X.Y.(Z+N)-dev.<sha> from last stable tag + commits since it.
rolling_version() {
  local latest semver major minor patch build sha
  latest="$(latest_stable_tag)"
  if [ -z "$latest" ]; then
    major=0
    minor=1
    patch=0
    build=$(git rev-list --count HEAD)
  else
    if ! git merge-base --is-ancestor "$latest" HEAD 2>/dev/null; then
      echo "runtime-version: ${latest} is not an ancestor of HEAD (shallow clone?)" >&2
      exit 1
    fi
    semver="${latest#runtime/v}"
    major=$(echo "$semver" | cut -d. -f1)
    minor=$(echo "$semver" | cut -d. -f2)
    patch=$(echo "$semver" | cut -d. -f3)
    if ! printf '%s.%s.%s' "$major" "$minor" "$patch" | grep -qE '^[0-9]+\.[0-9]+\.[0-9]+$'; then
      echo "runtime-version: could not parse semver from ${latest}" >&2
      exit 1
    fi
    build=$(git rev-list --count "${latest}..HEAD")
  fi
  sha=$(git rev-parse --short HEAD)
  printf '%s.%s.%s-dev.%s' "$major" "$minor" "$((patch + build))" "$sha"
}

case "$mode" in
  rolling)
    tag='runtime/latest'
    version="$(rolling_version)"
    prerelease='true'
    title='runtime latest (main)'
    ;;
  stable)
    ref_tag="${2:?stable mode requires tag, for example runtime/v0.1.0}"
    tag="$ref_tag"
    version="${ref_tag#runtime/}"
    prerelease='false'
    title="runtime ${version}"
    ;;
  dispatch)
    version="${2:?dispatch mode requires version, for example v0.2.0}"
    tag="runtime/${version}"
    prerelease='false'
    title="runtime ${version}"
    ;;
  *)
    echo "runtime-version: unknown mode ${mode}" >&2
    exit 1
    ;;
esac

printf 'tag=%q\nversion=%q\nprerelease=%q\ntitle=%q\n' \
  "$tag" "$version" "$prerelease" "$title"
