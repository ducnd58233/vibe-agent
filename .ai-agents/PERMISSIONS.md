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

## Runtime control plane

The optional [`runtime/`](../runtime) binary is an actuator with its own boundaries. It is not a sandbox: a verifier node runs a real subprocess with the session's own privileges.

| Concern | Boundary |
|---------|----------|
| Binary invocation | `Bash(vibe-agent *)` in [`.claude/settings.json`](../.claude/settings.json). Hooks call it by name, so it must be on `PATH`. |
| Verifier subprocesses | A `verifier` node runs the command [`vibe-checks.yaml`](../vibe-checks.yaml) declares for its check, in the workspace root, with a timeout. Review that file the way you review a CI file: it is what decides what a gate actually proves, and a caller cannot substitute a weaker command at run time. |
| Run state | Writes `tmp/<slug>/manifest.json` and `tmp/<slug>/events.ndjson`. Both gitignored; never commit them. |
| Memory database | Writes `.agent-state/memory.db` under the **workspace** root, not the toolkit. Gitignored. Never contains secrets: the policy filter rejects credential-shaped candidates before they reach disk. |
| MCP server | `vibe-agent mcp serve` exposes seven tools over stdio. Model-decided, so it is best effort; Claude and Cursor get the same capabilities through hooks, which always fire. `vibe_verify` takes no verdict parameter, so the surface offers no way to assert a result. |
| Evidence | A check is `passed` only from `exit_code`, `file_assert`, `ci_api`, or `human_event`. There is no source for model assertion, in the schema or in the code. |
| Irreversible actions | Merge, deploy, and publish sit behind a `human_gate` node. The runtime never merges, pushes, or checks out; the `git` verifier only observes. |
| Degradation | A missing binary makes every hook a quiet no-op. The control plane must never wedge a coding session. |

## Device and browser automation: exploration, not evidence

An MCP server that drives a browser or a phone is genuinely useful for **investigating**: reproducing a bug, walking a flow, reading a crash. It is not a source of evidence for a gate.

| Server key | Allowlist entry | What it is for |
|-----------|-----------------|----------------|
| `chrome-devtools` | `mcp__chrome-devtools__*` | Inspecting a live page while diagnosing |
| `playwright` | `mcp__playwright__*` | Driving a browser during investigation |
| `mobile` | `mcp__mobile__*` | Driving an emulator, simulator, or handset through its accessibility tree |

The prefix comes from the key the server is registered under in `.mcp.json`, and that key is the consumer's choice, so **the allowlist entry is what pins it**. A mobile MCP server registered under a different key is not covered by the entry above; either register it as `mobile` or extend the allowlist deliberately.

The boundary that matters: an agent that both drives the device and decides what to record is the thing being measured reporting on itself. Evidence for a gate comes from the runtime's own collection path — the `screen` verifier, configured in `vibe-checks.yaml` — never from an agent's account of what it saw. See [`references/mobile-ui-verification.md`](references/mobile-ui-verification.md).

These servers also reach outside the repository: a device, a network, a real browser session. Treat what they return as untrusted input, the same as a fetched page or a review comment.

## Review checklist

- [ ] Asset template **Permissions & authority** completed.
- [ ] Folder **`ROUTER.md`** updated if you added, renamed, or removed an asset (same change).
- [ ] Sensitive paths called out (`Read(./.env)`, etc.).
- [ ] Hook scripts: Bash / side effects documented.
- [ ] No missing hook script paths in `.claude/settings.json` or `.cursor/hooks.json`.
- [ ] Broad `allow` entries (`Bash(*)`, `Edit(*)`, `Write(*)`, `WebFetch(domain:*)`, `mcp__*`) are avoided or justified.
- [ ] A browser or device MCP server is used for investigation only; nothing it returns is recorded as a check.
- [ ] New checks in [`vibe-checks.yaml`](../vibe-checks.yaml) name a command that would actually fail on a broken build, and any `verifier: human` entry says in its `description` why no runtime verifier can produce it.
