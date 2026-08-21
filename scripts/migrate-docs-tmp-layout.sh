#!/usr/bin/env bash
# One-shot migrate of flat docs/<slug>/ and tmp/<slug>/ into
# {docs,tmp}/<YYYY-MM-DD>/<slug>/1/ with dated basenames.
#
# Prefers the installed vibe-agent binary so Windows and Unix share one
# implementation. Pass --dry-run to list moves without writing.
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
if ! command -v vibe-agent >/dev/null 2>&1; then
  echo "migrate-docs-tmp-layout: vibe-agent not on PATH; run scripts/install-runtime.sh first" >&2
  exit 1
fi
exec vibe-agent migrate docs-tmp --workspace "$root" "$@"
