#!/usr/bin/env bash
# Computes runtime release tag, version, and metadata for GitHub Releases.
#
# Rolling builds reuse tag runtime/latest but carry a monotonic build counter
# (commits since the last stable runtime/v* tag) so every publish increments
# the version string even when the conventional semver base is unchanged.
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

conventional_base_since() {
  local latest="$1"
  local base major minor patch subjects bodies

  if [ -z "$latest" ]; then
    printf '%s' '0.1.0'
    return 0
  fi

  local semver="${latest#runtime/v}"
  major=$(echo "$semver" | cut -d. -f1)
  minor=$(echo "$semver" | cut -d. -f2)
  patch=$(echo "$semver" | cut -d. -f3)

  if ! git merge-base --is-ancestor "$latest" HEAD 2>/dev/null; then
    echo "runtime-version: ${latest} is not an ancestor of HEAD (shallow clone?)" >&2
    return 1
  fi

  subjects=$(git log --format=%s "${latest}..HEAD")
  bodies=$(git log --format=%B "${latest}..HEAD")
  echo "bump input: $(printf '%s' "$subjects" | grep -c .) commits since ${latest}" >&2

  if printf '%s' "$subjects" | grep -qE '^[a-z]+(\([^)]*\))?!:' \
    || printf '%s' "$bodies" | grep -qE '^BREAKING[ -]CHANGE:'; then
    major=$((major + 1)); minor=0; patch=0
  elif printf '%s' "$subjects" | grep -qE '^feat(\([^)]*\))?:'; then
    minor=$((minor + 1)); patch=0
  else
    patch=$((patch + 1))
  fi

  base="${major}.${minor}.${patch}"
  printf '%s' "$base"
}

rolling_version() {
  local latest base build sha
  latest="$(latest_stable_tag)"
  base="$(conventional_base_since "$latest")"
  if [ -n "$latest" ]; then
    build=$(git rev-list --count "${latest}..HEAD")
  else
    build=$(git rev-list --count HEAD)
  fi
  sha=$(git rev-parse --short HEAD)
  printf '%s-dev.%s.%s' "$base" "$build" "$sha"
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
