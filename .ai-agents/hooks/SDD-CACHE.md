# sdd-cache hook

## What

- **Scripts:** `.ai-agents/hooks/sdd-cache-pre.py`, `.ai-agents/hooks/sdd-cache-post.py`
- **Purpose:** Cache `WebFetch` responses with HTTP validator revalidation (`ETag` / `Last-Modified`).
- **Events:** `PreToolUse` (`WebFetch`) and `PostToolUse` (`WebFetch`).

## Why

- Reduce repeated documentation fetches across sessions.
- Preserve freshness guarantees by serving cache only when origin returns `304 Not Modified`.

## How

- `sdd-cache-post.py` stores `{url, prompt, etag, last_modified, content, fetched_at}` into `.claude/sdd-cache/*.json`.
- `sdd-cache-pre.py` sends conditional `HEAD` and:
  - returns `exit 2` + cached content on stderr when origin replies `304`
  - returns `exit 0` for miss/stale to let `WebFetch` run normally.
- Entries without validators are not cached.

## When

- Use with source-backed workflows where repeated `WebFetch` of the same URLs is common.
- Avoid if upstream docs are private endpoints that do not emit cache validators.

## Routing & discovery

- Hook assets live in `.ai-agents/hooks/`.
- Runtime wiring belongs in `.cursor/hooks.json` and `.claude/settings.json`.

## Permissions & authority

- Needs Python 3 runtime (stdlib only).
- Writes only under `.claude/sdd-cache/`.
- Do not log secrets in hook output.
