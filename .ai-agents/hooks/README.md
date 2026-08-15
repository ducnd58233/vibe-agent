# Hooks

<references>

Shared hook scripts live here. See **[`TEMPLATE.md`](TEMPLATE.md)** to author new hook scripts and **[`ROUTER.md`](ROUTER.md)** for routing; **after adding a script**, update **`ROUTER.md`** in this folder.

Most lifecycle behavior now lives in the Go runtime under `runtime/internal/harness` and is invoked as `vibe-agent hook <event>`. The files left here are scripts that still need to be separate: the source-driven `WebFetch` cache pair and the git attribution stripper.
</references>

## Cross-tool compatibility

<context>

- **Claude Code:** native hooks in [`.claude/settings.json`](../../.claude/settings.json).
- **Cursor:** native hooks in [`.cursor/hooks.json`](../../.cursor/hooks.json).
- **Codex:** native hooks in [`.codex/hooks.json`](../../.codex/hooks.json) for the events Codex exposes.
- **opencode:** no shell hook runtime is wired in this repository; policy still reaches it through commands, skills, and git-level hooks.

The runtime hook adapter accepts each host's payload shape. Keep host-specific envelope logic in `runtime/internal/harness`, and keep script-only behavior here.

### Current script hooks

- `sdd-cache-pre.py` and `sdd-cache-post.py` wrap `WebFetch` for source-driven cache revalidation and writes.
- `strip-ai-attribution.sh` is installed as the git `prepare-commit-msg` hook by `scripts/link-ai-agents.*`.
- `strip-ai-attribution.ps1` is the PowerShell equivalent for manual or PowerShell-driven use.

### Runtime advisory guards

`vibe-agent hook post-tool-use` owns the file-write guards that used to be Python scripts: `sensitive-data-guard`, `design-token-guard`, `ui-slop-guard`, and `core-logic-test-guard`. The built-in rule data is in `runtime/internal/harness/guards-default.yaml`, and a consumer can extend or disable rules with `.ai-agents/guards.yaml`.

`vibe-agent hook user-prompt-submit`, `session-start`, and `subagent-stop` own the prompt-time reminder, session bootstrap, and subagent grounding checks.

### Known divergence

Cursor's prompt hook cannot inject extra model context. Its documented output validates or blocks the submitted prompt instead. The runtime therefore returns no prompt-time context for Cursor rather than interrupting the user.

The Cursor CLI was tried and could not confirm live editor behavior: `cursor-agent` 2026.08.11 failed even a hook command of `true` before the hook process started. Treat live Cursor editor validation as still pending.
</context>

## Git-level hooks (all tools + manual commits)

<references>

Some policy is enforced below every harness, at the git layer. [`strip-ai-attribution.sh`](strip-ai-attribution.sh) implements the **No Agent Attribution** rule from [`AGENTS.md`](../../AGENTS.md) for tools that have no attribution setting and for hand-typed commits.

[`scripts/link-ai-agents.ps1`](../../scripts/link-ai-agents.ps1) / [`scripts/link-ai-agents.sh`](../../scripts/link-ai-agents.sh) install a git `prepare-commit-msg` hook at `<workspace>/.git/hooks/prepare-commit-msg`, a thin shim that calls `strip-ai-attribution.sh` through `sh`. Re-run a link script after clone to reinstall it. The hook edits the message in place and never fails the commit.
</references>

## Manual invocation examples

<procedure>

- `echo '{"tool_input":{"url":"https://example.com"}}' | python3 .ai-agents/hooks/sdd-cache-pre.py`
- `echo '{}' | vibe-agent hook session-start --workspace . --toolkit . --client claude`
</procedure>