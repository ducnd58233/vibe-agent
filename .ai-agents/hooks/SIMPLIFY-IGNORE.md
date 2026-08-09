# simplify-ignore hook

## What

<context>

- **Scripts:** `.ai-agents/hooks/simplify-ignore.py`, `.ai-agents/hooks/simplify-ignore-test.py`
- **Purpose:** Hide annotated blocks from simplify flows and restore them safely.
- **Events:** `PreToolUse` (`Read`), `PostToolUse` (`Edit|Write`), and `Stop`.
</context>

## Routing & discovery

<routing>

- Shared scripts are under `.ai-agents/hooks/`.
- Wire per runtime in `.cursor/hooks.json` / `.claude/settings.json`.
- Use for perf-critical code, compatibility shims, or sensitive code sections.
- Do not use as a substitute for tests or review.
</routing>

## Permissions & authority

<required>

- Requires Python 3 runtime (stdlib only).
- Reads and writes repository files and cache backups.
- Cache directory: `.claude/.simplify-ignore-cache/`.
</required>
