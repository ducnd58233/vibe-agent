#!/usr/bin/env bash
# Layout and front-matter checks for docs/<date>/<slug>/<version>/.
#
# This toolkit gitignores /docs/, so CI cannot scan real deliverables here.
# The load-bearing proof is the Go fixture suite; consumers with tracked docs
# get the same rules from `vibe-agent doctor`.
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root/runtime"
go test ./internal/shared/docmeta/ -count=1 -run 'CheckWorkspace|ParseFrontMatter|Validate'
