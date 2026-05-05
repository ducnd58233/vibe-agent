# Cursor (vibe-agent)

Cursor does **not** auto-load this filename the way Claude Code loads `CLAUDE.md`. Keep **project rules** in [`.cursor/rules/*.mdc`](.cursor/rules) and treat this file as the **onboarding index** for Cursor users and for `@`-references.

This repository is a **shared, domain-agnostic agent-assets toolkit** (skills, agents, commands, hooks), not an end-product app codebase.

## What to read first

1. [`AGENTS.md`](AGENTS.md) — shared project charter and conventions (includes **MUST** rules for templates and routers).
2. [`.ai-agents/ROUTER.md`](.ai-agents/ROUTER.md) — master router before choosing a skill, subagent, command, or hook; after adding assets, update the folder **`ROUTER.md`** table.
3. [`.ai-agents/README.md`](.ai-agents/README.md) — how `.ai-agents` maps to Claude, Cursor, and Codex.

When **authoring** under `.ai-agents`, follow each folder’s **`TEMPLATE.md`**. When changing tool requirements, see [`.ai-agents/PERMISSIONS.md`](.ai-agents/PERMISSIONS.md). Claude Code uses [`.claude/settings.json`](.claude/settings.json) for [permissions](https://code.claude.com/docs/en/permissions); Cursor relies on **`.cursor/rules`** and workspace trust.

## Cursor-specific paths

| What | Where |
|------|--------|
| Rules (recommended for persistent agent context) | [`.cursor/rules/`](.cursor/rules) |
| Skills (`SKILL.md` folders) | `.cursor/skills` → link to [`.ai-agents/skills`](.ai-agents/skills) via [`scripts/link-ai-agents.ps1`](scripts/link-ai-agents.ps1) or [`scripts/link-ai-agents.sh`](scripts/link-ai-agents.sh) |
| Slash commands (same sources as Claude) | `.cursor/commands` → junction/symlink to [`.ai-agents/commands`](.ai-agents/commands) via the same link script |
| Reference checklists | [`.ai-agents/references/`](.ai-agents/references) |
| Stack profiles (frameworks used here) | [`.ai-agents/stack-profiles/ROUTER.md`](.ai-agents/stack-profiles/ROUTER.md) |
| Hooks | [`.cursor/hooks.json`](.cursor/hooks.json) — hook commands may point at scripts under [`.ai-agents/hooks/`](.ai-agents/hooks) |

### Project `/review`, `/ship`, …

After **`scripts/link-ai-agents`**, editable markdown in [`.ai-agents/commands/`](.ai-agents/commands) is what both **`.claude/commands`** and **`.cursor/commands`** point at. In Cursor chat, type `/` to pick `review`, `ship`, etc. Author new commands only under `.ai-agents/commands/`.

## Other tools in this repo

- **Claude Code:** [`CLAUDE.md`](CLAUDE.md) and [`.claude/`](.claude)
- **Codex:** [`.codex/config.toml`](.codex/config.toml)

## Reuse in a consumer repo

When this toolkit is reused from another repository, prefer adding it as a submodule at `.vibe-agent`.

- Canonical shared assets path in the consumer repo: `.vibe-agent/.ai-agents`
- Consumer repo link scripts should point `.cursor/skills`, `.cursor/commands`, `.claude/*`, and `.opencode/*` to `.vibe-agent/.ai-agents/*`
- Keep this repo's own `scripts/link-ai-agents.*` semantics unchanged for this repository layout
