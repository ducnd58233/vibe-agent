# Hook script authoring template

Use this contract when **adding a hook script** under `.ai-agents/hooks/`. Reference the same path from [`.cursor/hooks.json`](../../.cursor/hooks.json) and/or [`.claude/settings.json`](../../.claude/settings.json) hooks.

References: [Cursor hooks](https://docs.cursor.com), [Claude Code hooks](https://code.claude.com/docs/en/hooks).

---

## What

- **Script path:** `.ai-agents/hooks/<script-name>.<ext>`
- **Language:** Shell / PowerShell / Node — must be executable in CI and locally.
- **Event:** Which hook event(s) invoke this script (`afterFileEdit`, `beforeShellExecution`, Claude-specific events, etc.).

---

## Why

- **Problem:** What automation or guardrail does this hook provide?
- **Success criteria:** Valid JSON on stdout (for JSON protocols), exit code expectations.
- **Non-goals:** …

---

## How

- **Stdin/stdout:** Summarize the JSON contract your script implements (link product docs).
- **Exit codes:** Document success vs failure (`failClosed` implications in Cursor).
- **Wiring:** Exact entries added to `hooks.json` or Claude `settings.json` (event + `command` path from repo root).

---

## When

- **Runs on:** Event name(s); optional matchers (tool name, glob).
- **Does not run on:** …

---

## Routing & discovery

Hooks are not “invoked by intent” like skills — document **which events** and **which repos/teams** should enable this hook.

---

## Permissions & authority

Hook processes often run **outside** the model’s tool sandbox and may spawn shells.

| Topic | Notes |
|-------|--------|
| **Bash / shell** | If the script runs `bash` or `cmd`, the **user/session** must be allowed to run those patterns — document suggested `Bash(...)` [permission rules](https://code.claude.com/docs/en/permissions). |
| **File read** | Hooks that read repo files may need matching `Read(...)` rules if enforced in your setup. |
| **Secrets** | Never log secrets; minimize env exposure. |
| **Cursor vs Claude** | Same script path can be shared; JSON protocols may differ — branch or separate scripts if needed. |

Record project-wide hook permission expectations in [`.ai-agents/PERMISSIONS.md`](../PERMISSIONS.md).

---

## After creating (MUST)

When you add, rename, or remove a hook script in `hooks/`:

1. Update **[`ROUTER.md`](ROUTER.md)** in this folder **in the same change**: add a row (event / concern / use case, script path, permission notes); delete or edit rows when removing hooks.
2. Note which **`hooks.json`** / Claude settings entries reference the script.

Same PR or commit as the new asset when possible.
