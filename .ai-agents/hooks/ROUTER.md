# Hooks router

Lookup table for hook scripts in this folder. **After you add, rename, or remove a hook script, update this table in the same change.**

| Event / concern / use case | Script | Permission notes |
|----------------------------|--------|------------------|
| *(none yet)* | — | When adding hooks, list events (`afterFileEdit`, etc.) and required `Bash`/`Read` rules |
| *(add rows for each hook)* | `your-hook.sh` | Wiring in `hooks.json` / Claude settings |

**Authoring:** [`TEMPLATE.md`](TEMPLATE.md) — document stdin/stdout JSON and wiring in [`.cursor/hooks.json`](../../.cursor/hooks.json) / [`.claude/settings.json`](../../.claude/settings.json).
