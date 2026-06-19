# Hooks

Shared hook scripts live here. See **[`TEMPLATE.md`](TEMPLATE.md)** to author new hooks and **[`ROUTER.md`](ROUTER.md)** for routing; **after adding a hook**, update **`ROUTER.md`** in this folder.

Wire paths from the repo root into [`.cursor/hooks.json`](../../.cursor/hooks.json) and Claude **`hooks`** in [`.claude/settings.json`](../../.claude/settings.json). Protocol details: [Cursor hooks](https://docs.cursor.com), [Claude Code hooks](https://code.claude.com/docs/en/hooks).

## Cross-tool compatibility

- **Cursor:** native runtime via [`.cursor/hooks.json`](../../.cursor/hooks.json).
- **Claude Code:** native runtime via `hooks` in [`.claude/settings.json`](../../.claude/settings.json).
- **opencode / codex:** no repository-native hook runtime is currently configured in this repo. Keep scripts in `.ai-agents/hooks/` as shared assets, and document/manual-run workflows when needed.

## Git-level hooks (all tools + manual commits)

Some policy is enforced below every harness, at the git layer. [`strip-ai-attribution.sh`](strip-ai-attribution.sh) (with the PowerShell equivalent [`strip-ai-attribution.ps1`](strip-ai-attribution.ps1)) implements the **No Agent Attribution** rule (see [`AGENTS.md`](../../AGENTS.md) and [`git-workflow-and-versioning`](../skills/git-workflow-and-versioning/SKILL.md)) for tools that have no `attribution` setting (Cursor, Codex, opencode) and for hand-typed commits. These are plain shell scripts, not Python, so they run wherever git runs.

[`scripts/link-ai-agents.ps1`](../../scripts/link-ai-agents.ps1) / [`scripts/link-ai-agents.sh`](../../scripts/link-ai-agents.sh) install a git `prepare-commit-msg` hook at `<workspace>/.git/hooks/prepare-commit-msg`, a thin shim that calls `strip-ai-attribution.sh` via `sh` (git runs hooks through `sh` on every platform, including Windows git-bash, which bundles `awk`). Re-run a link script after clone to (re)install it. The hook is intentionally non-blocking: it edits the message in place and never fails the commit.

## Manual invocation examples

- `python3 .ai-agents/hooks/session-start.py`
- `echo '{"tool_input":{"url":"https://example.com"}}' | python3 .ai-agents/hooks/sdd-cache-pre.py`
- `echo '{}' | python3 .ai-agents/hooks/simplify-ignore.py`
