#!/usr/bin/env bash
# Downloads the vibe-agent runtime binary for this platform.
#
# The runtime is optional. Every asset under .ai-agents/ works without it; the
# binary adds enforced workflow transitions, persisted run state, and memory.
#
# Usage:
#   bash scripts/install-runtime.sh                 # latest release
#   bash scripts/install-runtime.sh v0.1.0          # a specific version
#   VIBE_INSTALL_DIR=~/bin bash scripts/install-runtime.sh

set -euo pipefail

REPO="${VIBE_REPO:-ducnd58233/vibe-agent}"
VERSION="${1:-latest}"
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

if [ "$VERSION" = "latest" ]; then
  echo "resolving the latest release"
  VERSION="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
    | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": *"\(.*\)".*/\1/')"
  [ -n "$VERSION" ] || die "could not resolve the latest release; pass a version explicitly"
  VERSION="${VERSION#runtime/}"
fi

asset="${BINARY}_${VERSION}_${os}_${arch}${ext}"
base="https://github.com/${REPO}/releases/download/runtime/${VERSION}"

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

echo "downloading ${asset}"
curl -fsSL -o "${tmp}/${asset}" "${base}/${asset}" \
  || die "download failed. Check that ${VERSION} has an asset for ${os}/${arch}."

# A binary that runs with the session's own privileges is worth verifying.
if curl -fsSL -o "${tmp}/SHA256SUMS" "${base}/SHA256SUMS" 2>/dev/null; then
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
  echo "warning: no SHA256SUMS published for ${VERSION}, skipping verification" >&2
fi

mkdir -p "$INSTALL_DIR"
install -m 0755 "${tmp}/${asset}" "${INSTALL_DIR}/${BINARY}${ext}" 2>/dev/null \
  || { cp "${tmp}/${asset}" "${INSTALL_DIR}/${BINARY}${ext}"; chmod +x "${INSTALL_DIR}/${BINARY}${ext}"; }

echo "installed ${INSTALL_DIR}/${BINARY}${ext}"

case ":${PATH}:" in
  *":${INSTALL_DIR}:"*) ;;
  *)
    echo
    echo "${INSTALL_DIR} is not on PATH. Hooks invoke the binary by name, so add it:"
    echo "  export PATH=\"${INSTALL_DIR}:\$PATH\""
    ;;
esac

echo
echo "check the install with:"
echo "  ${BINARY} version"
echo "  ${BINARY} doctor"
