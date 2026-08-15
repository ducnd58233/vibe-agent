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

Per-host event names, output field casing, workspace-root mechanisms, and what each host does **not** provide are recorded in [`host-hook-contracts.md`](../references/host-hook-contracts.md), generated from `runtime/internal/harness/contracts.go`. That table is the source; this section only carries what does not fit a row.

Cursor's prompt hook cannot inject extra model context. Its documented output validates or blocks the submitted prompt instead. The runtime therefore returns no prompt-time context for Cursor, and delivers the run's current node on `postToolUse` instead, once per node change.

**Cursor, measured 2026-08-15.** `cursor-agent` 2026.08.11 now completes ordinary turns, so the earlier note that it failed before the hook process started no longer holds. Forcing a shell call showed two things instead:

- It ran a hook out of [`.claude/settings.json`](../../.claude/settings.json), not [`.cursor/hooks.json`](../../.cursor/hooks.json). The command carried `--workspace ${CLAUDE_PROJECT_DIR}` and no `--client cursor`, and that string exists in exactly one config in this repository. So `--client cursor` never reaches the runtime, and the reply is built in Claude's shape, which Cursor discards.
- It wraps the command in PowerShell (`$OutputEncoding`, `Get-Content -LiteralPath`, `| & { $input | ... }`) and executes it with a POSIX shell, which refuses it: ``eval: syntax error near unexpected token `&'``. The hook process never starts on Windows.

The second is a defect in the host and cannot be wired around from here. The **Cursor editor** was not tested and may behave differently, so every Cursor row in the contract stays `UNVERIFIED`. Evidence: `tmp/<slug>/runtime/host-measurements.md`.
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