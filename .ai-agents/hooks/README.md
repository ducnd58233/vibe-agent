# Hooks

<references>

Shared hook scripts live here. See **[`TEMPLATE.md`](TEMPLATE.md)** to author new hooks and **[`ROUTER.md`](ROUTER.md)** for routing; **after adding a hook**, update **`ROUTER.md`** in this folder.

Wire paths from the repo root into [`.cursor/hooks.json`](../../.cursor/hooks.json) and Claude **`hooks`** in [`.claude/settings.json`](../../.claude/settings.json). Protocol details: [Cursor hooks](https://docs.cursor.com), [Claude Code hooks](https://code.claude.com/docs/en/hooks).
</references>

## Cross-tool compatibility

<context>

- **Cursor:** native runtime via [`.cursor/hooks.json`](../../.cursor/hooks.json).
- **Claude Code:** native runtime via `hooks` in [`.claude/settings.json`](../../.claude/settings.json).
- **opencode / codex:** no repository-native hook runtime is currently configured in this repo. Keep scripts in `.ai-agents/hooks/` as shared assets, and document/manual-run workflows when needed.

### Payload differences

The same script serves both harnesses, per the one-implementation rule below. Where the payloads differ, the scripts accept either shape rather than forking into per-tool copies:

| Script | Claude field | Cursor field |
|--------|--------------|--------------|
| [`subagent-grounding-guard.py`](subagent-grounding-guard.py) | `transcript_path` | `agent_transcript_path` |
| [`design-token-guard.py`](design-token-guard.py) | `tool_input.file_path` | `file_path` (top level) |
| [`ui-slop-guard.py`](ui-slop-guard.py) | `tool_name` + `tool_input.file_path` | `file_path`, no `tool_name` |

`ui-slop-guard.py` checks `tool_name` only when present. Claude's `PostToolUse` fires for every tool so the name must be checked; Cursor's `afterFileEdit` is already edit-only and sends none.

### Known divergence: no prompt-time context injection on Cursor

[`precedence-reminder.py`](precedence-reminder.py) is wired on Claude `UserPromptSubmit` and works by returning `hookSpecificOutput.additionalContext`, which Claude surfaces to the model alongside the prompt.

Cursor's nearest event, `beforeSubmitPrompt`, **cannot inject context**. Its documented output is `{"continue": bool, "user_message": string}`, which only validates and blocks. There is no field that adds text to the model's context.

So this hook is **not** wired on Cursor, and the reminder is not delivered there. This is a capability gap in the host, not an oversight in the wiring. Rewriting the hook to block submissions instead would be a different and worse behavior: it would interrupt the user rather than inform the model.

Cursor users get the same guidance through [`CURSOR.md`](../../CURSOR.md) and `.cursor/rules/`, which are always loaded, rather than per-prompt.

### Unverified

The wiring above follows the current Cursor hooks documentation, and the runtime's side of it is exercised against Cursor's real payload shapes. What has **not** been confirmed is a live Cursor **editor** session from this repository. The Cursor **CLI** was tried and cannot confirm it: `cursor-agent` 2026.08.11 fails every hook, including `"command": "true"`, with `--: eval: line 1: syntax error near unexpected token '&'` before the hook process starts, and treats that shell's exit 2 as a block. That is Cursor composing its own wrapper, not anything in this repository. Two things to check when someone next runs the editor here:

- Whether `"WebFetch"` matches anything. Cursor documents tool-type matchers such as `"Shell"`, `"Read"`, `"Write"`, `"Grep"`, `"Delete"`, `"Task"`, and `"MCP:<name>"`, and `"WebFetch"` is a Claude tool name absent from that list. The two `sdd-cache` hooks do not filter by tool themselves, so a wrong matcher here means they never run rather than running too often. Cursor's own name for its web tool is not in the documentation this was written from, so it was left alone rather than guessed at.
- The control-plane matchers were **not** left alone: `"Bash|Edit|NotebookEdit"` named no Cursor tool, so `post-tool-use` fired for `Write` and for nothing else, and no shell command in any Cursor session was ever journalled. They now use the documented names.
- Whether `afterFileEdit` surfaces a script's stdout. It is documented as observational with no output fields, so the two UI guards may run without their warnings reaching anyone.
</context>

## Git-level hooks (all tools + manual commits)

<references>

Some policy is enforced below every harness, at the git layer. [`strip-ai-attribution.sh`](strip-ai-attribution.sh) (with the PowerShell equivalent [`strip-ai-attribution.ps1`](strip-ai-attribution.ps1)) implements the **No Agent Attribution** rule (see [`AGENTS.md`](../../AGENTS.md) and [`git-workflow-and-versioning`](../skills/git-workflow-and-versioning/SKILL.md)) for tools that have no `attribution` setting (Cursor, Codex, opencode) and for hand-typed commits. These are plain shell scripts, not Python, so they run wherever git runs.

[`scripts/link-ai-agents.ps1`](../../scripts/link-ai-agents.ps1) / [`scripts/link-ai-agents.sh`](../../scripts/link-ai-agents.sh) install a git `prepare-commit-msg` hook at `<workspace>/.git/hooks/prepare-commit-msg`, a thin shim that calls `strip-ai-attribution.sh` via `sh` (git runs hooks through `sh` on every platform, including Windows git-bash, which bundles `awk`). Re-run a link script after clone to (re)install it. The hook is intentionally non-blocking: it edits the message in place and never fails the commit.
</references>

## Manual invocation examples

<procedure>

- `python3 .ai-agents/hooks/session-start.py`
- `echo '{"tool_input":{"url":"https://example.com"}}' | python3 .ai-agents/hooks/sdd-cache-pre.py`
</procedure>
