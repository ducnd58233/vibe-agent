#!/usr/bin/env bash
# Downloads the vibe-agent runtime binary for this platform.
#
# The runtime is optional. Every asset under .ai-agents/ works without it; the
# binary adds enforced workflow transitions, persisted run state, and memory.
#
# Prefers a published release, which needs no Go. When no release is reachable
# it falls back to building from runtime/, which does. That fallback is what
# makes the toolkit usable before the first release exists.
#
# Downloads every time it runs. It does not skip because a binary is already
# present: the link script used to do that, and a consumer who installed once
# never got another update, so the hooks kept calling a binary that fell further
# behind the configs registering them.
#
# Usage:
#   bash scripts/install-runtime.sh                 # latest release, else build
#   bash scripts/install-runtime.sh v0.1.0          # a specific version
#   VIBE_FROM_SOURCE=1 bash scripts/install-runtime.sh   # always build
#   VIBE_INSTALL_DIR=~/bin bash scripts/install-runtime.sh

set -euo pipefail

REPO="${VIBE_REPO:-ducnd58233/vibe-agent}"
VERSION="latest"
CHANNEL="auto"
while [ $# -gt 0 ]; do
  case "$1" in
    --channel) CHANNEL="${2:-}"; shift 2 ;;
    --channel=*) CHANNEL="${1#*=}"; shift ;;
    -*) die "unknown option: $1" ;;
    *) VERSION="$1"; shift ;;
  esac
done
case "$CHANNEL" in
  auto|stable|rolling|source) ;;
  *) die "unknown channel: $CHANNEL (auto, stable, rolling, or source)" ;;
esac
INSTALL_DIR="${VIBE_INSTALL_DIR:-$HOME/.local/bin}"
BINARY="vibe-agent"

die() { echo "install-runtime: $*" >&2; exit 1; }

detect_os() {
  case "$(uname -s)" in
    Linux*)   echo linux ;;
    Darwin*)  echo darwin ;;
    MINGW*|MSYS*|CYGWIN*) echo windows ;;
    *) die "unsupported operating system: $(uname -s)" ;;
  esac
}

detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64) echo amd64 ;;
    arm64|aarch64) echo arm64 ;;
    *) die "unsupported architecture: $(uname -m). Build from source: cd runtime && make install" ;;
  esac
}

os="$(detect_os)"
arch="$(detect_arch)"
ext=""
[ "$os" = "windows" ] && ext=".exe"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RUNTIME_DIR="${SCRIPT_DIR}/../runtime"

# Falls back to compiling when no published binary is reachable. A release is
# the normal path and needs no Go; this keeps the toolkit usable before the
# first release exists, and on architectures no release covers.
build_from_source() {
  local reason="$1"
  [ -f "${RUNTIME_DIR}/go.mod" ] || die "${reason}, and there is no runtime/ directory to build from."
  command -v go >/dev/null 2>&1 || die "$(cat <<EOF
${reason}

Two ways forward:
  1. Install Go, then rerun this script. It will build the binary for you.
  2. Wait for a published release: https://github.com/${REPO}/releases

Neither is required to use the toolkit. Without the binary every hook is a
quiet no-op and the markdown assets work exactly as they do today.
EOF
)"

  echo "${reason}"
  echo "building from source with $(go version)"
  mkdir -p "$INSTALL_DIR"
  ( cd "$RUNTIME_DIR" && CGO_ENABLED=0 go build -ldflags="-s -w -X main.version=source" \
      -o "${INSTALL_DIR}/${BINARY}${ext}" ./cmd ) \
    || die "build failed. Run 'cd runtime && make check' to see why."
  report_success
  exit 0
}

report_success() {
  local installed=""
  installed="$("${INSTALL_DIR}/${BINARY}${ext}" version 2>/dev/null | head -1 || true)"
  if [ -n "$installed" ]; then
    echo "installed ${INSTALL_DIR}/${BINARY}${ext} (${installed})"
  else
    echo "installed ${INSTALL_DIR}/${BINARY}${ext}"
  fi
  case ":${PATH}:" in
    *":${INSTALL_DIR}:"*) ;;
    *)
      echo
      echo "${INSTALL_DIR} is not on PATH. Hooks invoke the binary by name, so add it:"
      echo "  export PATH=\"${INSTALL_DIR}:\$PATH\""
      ;;
  esac

  # What matters is which copy PATH finds, not which one was just written. A
  # shadowed install is invisible, and its symptom is a hook behaving like a
  # version you thought you replaced. Reported here, in the PowerShell installer,
  # and by `make install`, because all three write the binary and any of them can
  # be the one being shadowed.
  local resolved
  resolved="$(command -v "$BINARY" 2>/dev/null || true)"
  if [ -n "$resolved" ] && [ -n "$installed" ]; then
    local winner
    winner="$("$resolved" version 2>/dev/null | head -1 || true)"
    if [ -n "$winner" ] && [ "$winner" != "$installed" ]; then
      echo
      echo "warning: PATH resolves ${BINARY} to a different build:"
      echo "           ${resolved} (${winner})"
      echo "         so this install does not change what the hooks run."
      echo "         remove that copy, or put ${INSTALL_DIR} earlier on PATH."
    fi
  fi

  echo
  echo "check the install with:"
  echo "  ${BINARY} version"
  echo "  ${BINARY} doctor"
}

if [ "${VIBE_FROM_SOURCE:-}" = "1" ]; then
  build_from_source "VIBE_FROM_SOURCE=1"
fi

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

# Asset names embed the version, and the rolling build's version carries a
# commit sha nobody can predict. So the release JSON is the source of truth for
# what to download, rather than a URL assembled from guesses.
fetch_release() {
  local endpoint="$1"
  curl -fsSL "https://api.github.com/repos/${REPO}/releases/${endpoint}" 2>/dev/null
}

# Resolution order:
#   1. an explicit version, when one was passed
#   2. the newest stable release
#   3. the rolling build from main, which is a prerelease and so is skipped by
#      the /releases/latest endpoint
#
# This only finds JSON or returns empty. Falling back to a source build happens
# in the caller, because build_from_source exits the script and would exit only
# the subshell if it ran inside a command substitution.
resolve_release() {
  if [ "$VERSION" != "latest" ]; then
    fetch_release "tags/runtime/${VERSION}"
    return 0
  fi

  if [ "$CHANNEL" = "rolling" ]; then
    echo "channel rolling: the build from main" >&2
    fetch_release tags/runtime/latest
    return 0
  fi

  echo "looking for a published release" >&2
  local json
  if json="$(fetch_release latest)" && [ -n "$json" ]; then
    printf '%s' "$json"
    return 0
  fi

  echo "no stable release yet, trying the rolling build from main" >&2
  fetch_release tags/runtime/latest
  return 0
}

# own_checkout reports whether this script is running inside a git clone of the
# toolkit itself rather than a consumer workspace that vendored it.
#
# Both have scripts/ and one may have runtime/. Only the clone has both a
# runtime module and its own .git, and that pair is the question being asked:
# is the source of this binary sitting right here.
own_checkout() {
  [ -f "${RUNTIME_DIR}/go.mod" ] && [ -e "${SCRIPT_DIR}/../.git" ]
}

# Somebody running this from the toolkit's own checkout is asking to install
# this toolkit. Downloading a release there answers a different question, and
# answers it badly: /releases/latest skips prereleases, so a clone of main got a
# binary older than the source it was standing in. That is not hypothetical - it
# replaced a working build with one three weeks stale.
if [ "$VERSION" = "latest" ] && [ "$CHANNEL" = "auto" ] && own_checkout; then
  build_from_source "Running inside the toolkit's own checkout, so building what is here."
fi
if [ "$CHANNEL" = "source" ]; then
  build_from_source "Channel source requested."
fi

release_json="$(resolve_release)" || true
if [ -z "$release_json" ]; then
  if [ "$VERSION" != "latest" ]; then
    build_from_source "No release tagged runtime/${VERSION}."
  fi
  build_from_source "No published release found for ${REPO}."
fi

# Pull the asset URL out of the JSON rather than reconstructing it. Matching on
# the platform suffix keeps this working whatever the version string turned out
# to be.
asset_url="$(printf '%s' "$release_json" \
  | grep -o '"browser_download_url": *"[^"]*"' \
  | sed 's/.*"browser_download_url": *"\([^"]*\)".*/\1/' \
  | grep "_${os}_${arch}${ext}\$" | head -1)"

if [ -z "$asset_url" ]; then
  build_from_source "That release has no asset for ${os}/${arch}."
fi

sums_url="$(printf '%s' "$release_json" \
  | grep -o '"browser_download_url": *"[^"]*SHA256SUMS"' \
  | sed 's/.*"browser_download_url": *"\([^"]*\)".*/\1/' | head -1)"

asset="$(basename "$asset_url")"
echo "downloading ${asset}"
curl -fsSL -o "${tmp}/${asset}" "$asset_url" || build_from_source "Download of ${asset} failed."

# A binary that runs with the session's own privileges is worth verifying.
if [ -n "$sums_url" ] && curl -fsSL -o "${tmp}/SHA256SUMS" "$sums_url" 2>/dev/null; then
  echo "verifying checksum"
  if command -v sha256sum >/dev/null 2>&1; then
    (cd "$tmp" && grep " \*\?${asset}\$" SHA256SUMS | sha256sum -c -) \
      || die "checksum mismatch; do not run this file"
  elif command -v shasum >/dev/null 2>&1; then
    expected="$(grep " \*\?${asset}\$" "${tmp}/SHA256SUMS" | awk '{print $1}')"
    actual="$(shasum -a 256 "${tmp}/${asset}" | awk '{print $1}')"
    [ "$expected" = "$actual" ] || die "checksum mismatch; do not run this file"
  else
    echo "warning: no sha256sum or shasum available, skipping verification" >&2
  fi
else
  echo "warning: no SHA256SUMS published alongside this asset, skipping verification" >&2
fi

mkdir -p "$INSTALL_DIR"
install -m 0755 "${tmp}/${asset}" "${INSTALL_DIR}/${BINARY}${ext}" 2>/dev/null \
  || { cp "${tmp}/${asset}" "${INSTALL_DIR}/${BINARY}${ext}"; chmod +x "${INSTALL_DIR}/${BINARY}${ext}"; }

report_success
