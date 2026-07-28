# Permissions and authority (project reference)

This file explains how **permission documentation** in `.ai-agents` assets connects to **Claude Code** configuration. **Cursor** does not use the same `settings.json` matrix; use [`.cursor/rules`](../.cursor/rules) and workspace trust.
Permissions described here are for reusable agent assets, not for domain-specific product workflows.

## Claude Code

| Mechanism | Location | Notes |
|-----------|----------|--------|
| Project permissions | [`.claude/settings.json`](../.claude/settings.json) | `permissions.allow`, `permissions.ask`, `permissions.deny` for tools (`Bash(...)`, `Read(...)`, `Edit(...)`, `WebFetch(...)`, MCP tools). **Deny overrides allow.** See [official permissions](https://code.claude.com/docs/en/permissions). |
| Subagent authority | `agents/*.md` YAML **`tools:`** | Map of tool name ? `true` (OpenCode-valid). Restricts which tools a subagent may use. |
| Hooks | `hooks` in settings + scripts under `.ai-agents/hooks/` | Hook commands may need shell access; align `Bash` rules with what scripts run. |

When you add or change a skill, agent, command, or hook:

1. Fill the **Permissions & authority** section in that folder's [`TEMPLATE.md`](./skills/TEMPLATE.md) (or equivalent).
2. If new tool patterns are required project-wide, extend `.claude/settings.json` in a dedicated PR and mention it here or in the PR description.
3. Prefer a **narrow default posture**: allow read/discovery and repo-local validation, ask for broad edits/network/package/deploy commands, and deny known secret/destructive patterns.

Current project posture:

- Allows common reads, router checks, link scripts, scoped edits under AI asset/config paths, and selected documentation fetch domains.
- Asks for broad shell/edit/web/MCP patterns and dependency/deploy/admin commands.
- Denies common secret files and force-push patterns.
- **Deny patterns must be specific enough to miss ordinary source files.** A broad `Read(**/*token*)` also blocked `hooks/design-token-guard.py` and design-token files such as `tokens.json`, and because deny overrides allow there is no way to grant an exception. It is now a set of credential-shaped patterns (`*access_token*`, `*refresh_token*`, `*auth_token*`, `*api_token*`, `*id_token*`, `.token*`, `token.json`). When adding a deny rule, check it against real repository paths before committing it.
- See [`references/tool-safety-and-permissions.md`](references/tool-safety-and-permissions.md) for the hardening checklist.

## Subagents (`agents/*.md`)

Specialist personas that inspect code/config usually declare **`tools`** with `Read`, `Grep`, `Glob`, and `Bash` set to `true`; research personas add `WebSearch` and `WebFetch`. Ensure [`.claude/settings.json`](../.claude/settings.json) allows those tools for sessions that spawn subagents, and scope `Bash` to repo-documented test/lint/check commands where possible.

After changing subagent `tools:` frontmatter, smoke-test delegation in Claude Code so allowlists still behave as expected. For OpenCode-only validation, run `opencode agent list` from the repo root (expects exit code 0).

## Cursor

- No duplicate of Claude `permissions` JSON.
- Encode guardrails in [`.cursor/rules/*.mdc`](../.cursor/rules).
- Refer to [`CURSOR.md`](../CURSOR.md) for onboarding.

## Review checklist

- [ ] Asset template **Permissions & authority** completed.
- [ ] Folder **`ROUTER.md`** updated if you added, renamed, or removed an asset (same change).
- [ ] Sensitive paths called out (`Read(./.env)`, etc.).
- [ ] Hook scripts: Bash / side effects documented.
- [ ] No missing hook script paths in `.claude/settings.json` or `.cursor/hooks.json`.
- [ ] Broad `allow` entries (`Bash(*)`, `Edit(*)`, `Write(*)`, `WebFetch(domain:*)`, `mcp__*`) are avoided or justified.
