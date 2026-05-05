# Hooks

Shared hook scripts live here. See **[`TEMPLATE.md`](TEMPLATE.md)** to author new hooks and **[`ROUTER.md`](ROUTER.md)** for routing; **after adding a hook**, update **`ROUTER.md`** in this folder.

Wire paths from the repo root into [`.cursor/hooks.json`](../../.cursor/hooks.json) and Claude **`hooks`** in [`.claude/settings.json`](../../.claude/settings.json). Protocol details: [Cursor hooks](https://docs.cursor.com), [Claude Code hooks](https://code.claude.com/docs/en/hooks).

## Cross-tool compatibility

- **Cursor:** native runtime via [`.cursor/hooks.json`](../../.cursor/hooks.json).
- **Claude Code:** native runtime via `hooks` in [`.claude/settings.json`](../../.claude/settings.json).
- **opencode / codex:** no repository-native hook runtime is currently configured in this repo. Keep scripts in `.ai-agents/hooks/` as shared assets, and document/manual-run workflows when needed.

## Manual invocation examples

- `python3 .ai-agents/hooks/session-start.py`
- `echo '{"tool_input":{"url":"https://example.com"}}' | python3 .ai-agents/hooks/sdd-cache-pre.py`
- `echo '{}' | python3 .ai-agents/hooks/simplify-ignore.py`
