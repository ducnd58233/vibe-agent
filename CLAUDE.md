# Claude Code (vibe-agent)

[Claude Code](https://code.claude.com/docs) loads this file from the project root **automatically** each session. For shared policy, start with [`AGENTS.md`](AGENTS.md); this file only adds Claude-oriented pointers.

This repository is a **shared, domain-agnostic agent-assets toolkit** (skills, agents, commands, hooks), not an end-product app codebase.

## Where things live

| What | Where |
|------|--------|
| Shared skills, subagents, commands, hook scripts | [`.ai-agents/`](.ai-agents) (see [`.ai-agents/README.md`](.ai-agents/README.md)) |
| Project settings, permissions, hooks | [`.claude/settings.json`](.claude/settings.json) |
| Linked views of skills / agents / commands | `.claude/skills`, `.claude/agents`, `.claude/commands` → run [`scripts/link-ai-agents.ps1`](scripts/link-ai-agents.ps1) or [`scripts/link-ai-agents.sh`](scripts/link-ai-agents.sh) after clone |

## Private overrides

To keep personal preferences out of git, use `CLAUDE.local.md` in the project root (see [Claude directory](https://code.claude.com/docs/en/claude-directory)). It is loaded alongside this file.

## Authoring and routing (MUST)

- **New assets:** Follow the **`TEMPLATE.md`** in the relevant [`.ai-agents/`](.ai-agents) subfolder (`skills`, `agents`, `commands`, `hooks`).
- **Choosing assets:** Read [`.ai-agents/ROUTER.md`](.ai-agents/ROUTER.md) and the subfolder **`ROUTER.md`** before picking a skill, subagent, command, or hook.
- **After creating assets:** Update that folder’s **`ROUTER.md`** in the same change (tables track files and use cases).
- **Permissions:** See [`.ai-agents/PERMISSIONS.md`](.ai-agents/PERMISSIONS.md) and align [`.claude/settings.json`](.claude/settings.json) with documented tool needs ([official docs](https://code.claude.com/docs/en/permissions)).

## Other tools in this repo

- **Cursor:** [`CURSOR.md`](CURSOR.md) and [`.cursor/rules/`](.cursor/rules)
- **Codex:** [`.codex/config.toml`](.codex/config.toml) and [`AGENTS.md`](AGENTS.md)
- **opencode:** [`opencode.json`](opencode.json) (config + permissions) and [`AGENTS.md`](AGENTS.md) (native rules file); `.opencode/agents` and `.opencode/commands` link to `.ai-agents/` via the same link script.

## Reuse in a consumer repo

When reused from another repository, prefer adding this toolkit as a submodule at `.vibe-agent`.

- Canonical shared assets path in the consumer repo: `.vibe-agent/.ai-agents`
- Consumer repo link scripts should map `.claude/*`, `.cursor/*`, and `.opencode/*` to `.vibe-agent/.ai-agents/*`
- Keep this repository's own `scripts/link-ai-agents.*` behavior unchanged for local usage
