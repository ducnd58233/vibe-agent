# sdd-cache hook

## What

<context>

- **Scripts:** `.ai-agents/hooks/sdd-cache-pre.py`, `.ai-agents/hooks/sdd-cache-post.py`
- **Purpose:** Cache `WebFetch` responses with HTTP validator revalidation (`ETag` / `Last-Modified`).
- **Events:** `PreToolUse` (`WebFetch`) and `PostToolUse` (`WebFetch`).
</context>

## Routing & discovery

<routing>

- Hook assets live in `.ai-agents/hooks/`.
- Runtime wiring belongs in `.cursor/hooks.json` and `.claude/settings.json`.
- Use with source-backed workflows where repeated `WebFetch` of the same URLs is common.
- Avoid if upstream docs are private endpoints that do not emit cache validators.
</routing>

## Permissions & authority

<required>

- Needs Python 3 runtime (stdlib only, including `sqlite3`).
- Reads/writes the `sdd_cache` table in the workspace's shared `.agent-state/memory.db`, opened
  directly via `VIBE_MEMORY_DB_PATH` (falls back to `.agent-state/memory.db` for a standalone run).
- Do not log secrets in hook output.
</required>
