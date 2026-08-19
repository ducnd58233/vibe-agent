# Tasks: harness plugin packaging (v1)

## Task 1: manifest emission in link scripts

**Branch:** `harness-plugin/manifest-emission`

**What:** Add functions to both `scripts/link-ai-agents.sh` and `scripts/link-ai-agents.ps1` that generate:
- `.claude-plugin/plugin.json` (name, description from `.ai-agents/`)
- `.claude-plugin/marketplace.json` (owner, plugin list pointing at `./`)
- `.codex-plugin/plugin.json` (name, description)
- `.cursor-plugin/plugin.json` (Agent Plugins schema, name, description)
- Root `plugin.json` (Agent Plugins schema, name, description)

Read name/description from a single source: `.ai-agents/metadata.json` (new file, or fallback to hardcoded `vibe-agent` if absent).

**Acceptance:**
- Running the link script writes all five manifest files.
- Content matches what is currently hand-written (backward compatible).
- Generated files carry a comment or `_generated` field so check script can identify them.

## Task 2: drift check for manifests

**What:** Extend `scripts/check-generated-views.sh` to verify plugin manifests match what the link script would produce, or at minimum check they exist and contain expected fields.

**Acceptance:**
- `bash scripts/check-generated-views.sh` exits 1 if a manifest is missing or has wrong name/description.

## Task 3: verify

**What:** Run `go -C runtime test ./...`, e2e, slop, and confirm `vibe-agent doctor` OK.

**Acceptance:** All checks green. Checkpoint advances run past review.
