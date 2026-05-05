# simplify-ignore hook

## What

- **Scripts:** `.ai-agents/hooks/simplify-ignore.py`, `.ai-agents/hooks/simplify-ignore-test.py`
- **Purpose:** Hide annotated blocks from simplify flows and restore them safely.
- **Events:** `PreToolUse` (`Read`), `PostToolUse` (`Edit|Write`), and `Stop`.

## Why

- Protect hand-tuned logic and fragile sections from automated simplification.
- Allow simplification around protected code without exposing internals.

## How

- Annotate blocks with:
  - `simplify-ignore-start` (optional reason)
  - `simplify-ignore-end`
- Pre-read: replaces blocks with `BLOCK_<hash>` placeholders.
- Post-edit/write: restores block content, then re-filters placeholders.
- Stop: restores original content from `.claude/.simplify-ignore-cache/`.

## When

- Use for perf-critical code, compatibility shims, or sensitive code sections.
- Do not use as a substitute for tests or review.

## Routing & discovery

- Shared scripts are under `.ai-agents/hooks/`.
- Wire per runtime in `.cursor/hooks.json` / `.claude/settings.json`.

## Permissions & authority

- Requires Python 3 runtime (stdlib only).
- Reads and writes repository files and cache backups.
- Cache directory: `.claude/.simplify-ignore-cache/`.
