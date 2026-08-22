# Cursor (vibe-agent)

<scope>

Cursor does **not** auto-load this filename the way Claude Code loads `CLAUDE.md`. Keep **project rules** in [`.cursor/rules/*.mdc`](.cursor/rules) and treat this file as the **onboarding index** for Cursor users and for `@`-references.

This repository is a **shared, domain-agnostic agent-assets toolkit** (skills, agents, commands, hooks), not an end-product app codebase.
</scope>

## What to read first

<prerequisites>

1. [`AGENTS.md`](AGENTS.md) - shared project charter and conventions (includes **MUST** rules for templates and routers).
2. [`.ai-agents/ROUTER.md`](.ai-agents/ROUTER.md) - master router before choosing a skill, subagent, command, or hook; after adding assets, update the folder **`ROUTER.md`** table.
3. [`.ai-agents/README.md`](.ai-agents/README.md) - how `.ai-agents` maps to Claude, Cursor, and Codex.

When **authoring** under `.ai-agents`, follow each folder’s **`TEMPLATE.md`**. When changing tool requirements, see [`.ai-agents/PERMISSIONS.md`](.ai-agents/PERMISSIONS.md). Claude Code uses [`.claude/settings.json`](.claude/settings.json) for [permissions](https://code.claude.com/docs/en/permissions); Cursor relies on **`.cursor/rules`** and workspace trust.
</prerequisites>

## Cursor-specific paths

<context>

| What | Where |
|------|--------|
| Rules (recommended for persistent agent context) | [`.cursor/rules/`](.cursor/rules) (includes research Applicability + Mermaid MUST) |
| Skills (`SKILL.md` folders) | `.cursor/skills` → link to [`.ai-agents/skills`](.ai-agents/skills) via [`scripts/link-ai-agents.ps1`](scripts/link-ai-agents.ps1) or [`scripts/link-ai-agents.sh`](scripts/link-ai-agents.sh) |
| Slash commands (same sources as Claude) | `.cursor/commands` → junction/symlink to [`.ai-agents/commands`](.ai-agents/commands) via the same link script |
| Reference checklists | [`.ai-agents/references/`](.ai-agents/references) |
| Stack profiles (frameworks used here) | [`.ai-agents/stack-profiles/ROUTER.md`](.ai-agents/stack-profiles/ROUTER.md) |
| Hooks | [`.cursor/hooks.json`](.cursor/hooks.json) - hook commands may point at scripts under [`.ai-agents/hooks/`](.ai-agents/hooks) |

### Project `/review`, `/ship`, …

After **`scripts/link-ai-agents`**, editable markdown in [`.ai-agents/commands/`](.ai-agents/commands) is what both **`.claude/commands`** and **`.cursor/commands`** point at. In Cursor chat, type `/` to pick `goal`, `review`, `ship`, etc. Author new commands only under `.ai-agents/commands/`.

### Cursor Bugbot (PR reviews)

PR **Bugbot** does **not** load `.cursor/rules`, `AGENTS.md`, or linked skills/commands automatically. It reads:

| Source | Path |
|--------|------|
| Project rules (always) | [`.cursor/BUGBOT.md`](.cursor/BUGBOT.md) |
| Scoped rules | Nested `.cursor/BUGBOT.md` (for example [`.ai-agents/.cursor/BUGBOT.md`](.ai-agents/.cursor/BUGBOT.md)) |
| Org/repo rules | [Bugbot dashboard](https://cursor.com/dashboard/bugbot) |

This repo's `BUGBOT.md` files distill the same policy as `/review`, `/test`, and `code-review-and-quality`. Keep them in sync when review standards change.

**Local pre-push review:** run `/review-bugbot` in Cursor chat (uses the `review-bugbot` skill). Optional `Custom Instructions` can point at a specific command or skill for one-off runs.

**After merge readiness:** `/goal` and `/ship` may wait on the **Cursor Bugbot** GitHub check when configured (see `goal.md` and `goal-verification-records.md`).
</context>

## Other tools in this repo

<other_harnesses>

- **Claude Code:** [`CLAUDE.md`](CLAUDE.md) and [`.claude/`](.claude)
- **Codex:** [`.codex/config.toml`](.codex/config.toml)
</other_harnesses>

## Reuse in a consumer repo

<procedure>

When this toolkit is reused from another repository, prefer adding it as a submodule at a chosen path (for example `.vibe-agent`).

- Canonical shared assets path in the consumer repo: `<toolkit-root>/.ai-agents`
- From the consumer workspace root, run [`scripts/link-ai-agents.ps1`](scripts/link-ai-agents.ps1) or [`scripts/link-ai-agents.sh`](scripts/link-ai-agents.sh) with workspace = consumer root and assets = `<toolkit-root>/.ai-agents` so `.cursor/skills`, `.cursor/commands`, `.claude/*`, and `.opencode/*` resolve to the shared trees (see [`.ai-agents/README.md`](.ai-agents/README.md)).
</procedure>
