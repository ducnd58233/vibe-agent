# Agent instructions (vibe-agent)

**vibe-agent** is a reusable codebase for **agent workflows and AI asset management**: shared skills, subagents, slash commands, routing guides, hooks, permissions policy, and cross-tool interoperability patterns.

This file is the **tool-agnostic** project charter for AI assistants.

## Product scope and assistant behavior

- **Purpose:** Build and maintain a portable **AI-agent toolkit** that can be reused across repositories without duplicating assets.
- **Assistant stance:** Prioritize reusable patterns, explicit routing, stable permissions boundaries, and minimal duplication across tools.
- **Scope boundary:** This repository is **not** an application/product domain codebase. Domain-specific behavior belongs in each consuming repo via its local `AGENTS.md`.

## Project layout for AI assets

| Location | Role |
|----------|------|
| [`.ai-agents/README.md`](.ai-agents/README.md) | **Index** of shared skills, agents, commands, and hooks. |
| [`.ai-agents/ROUTER.md`](.ai-agents/ROUTER.md) | **Master router** — start here to pick which family (skills, agents, commands, hooks) applies. |
| [`.ai-agents/skills/ROUTER.md`](.ai-agents/skills/ROUTER.md) (and peers under `agents/`, `commands/`, `hooks/`) | **Per-folder routers** — intent → concrete asset; **must** stay in sync when assets change. |
| [`.ai-agents/skills/TEMPLATE.md`](.ai-agents/skills/TEMPLATE.md) (and `agents/`, `commands/`, `hooks/`) | **Authoring contract** for new files — you **MUST** follow the template for that folder. |
| [`.ai-agents/PERMISSIONS.md`](.ai-agents/PERMISSIONS.md) | How **permissions / authority** map to [`.claude/settings.json`](.claude/settings.json) and subagent `tools:`. |
| [`.ai-agents/skills/`](.ai-agents/skills) | Canonical **skills** (`SKILL.md` per folder). |
| [`.ai-agents/references/`](.ai-agents/references) | Shared **generic** checklists and patterns (a11y, security, testing, orchestration). |
| [`.ai-agents/stack-profiles/`](.ai-agents/stack-profiles) | **Repo-pinned stacks** — index [`ROUTER.md`](.ai-agents/stack-profiles/ROUTER.md); author with [`TEMPLATE.md`](.ai-agents/stack-profiles/TEMPLATE.md). |
| [`.ai-agents/agents/`](.ai-agents/agents) | **Claude subagents** (`*.md`). |
| [`.ai-agents/commands/`](.ai-agents/commands) | **Slash commands** (`*.md`); `.claude/commands` and `.cursor/commands` link here (run `scripts/link-ai-agents`). |
| [`.ai-agents/hooks/`](.ai-agents/hooks) | **Shared hook scripts**; referenced from Cursor and Claude config. |
| [`.claude/`](.claude) | Claude Code **settings**; `skills` / `agents` / `commands` are **linked** to `.ai-agents` (see link script). |
| [`.cursor/`](.cursor) | Cursor **rules** and **hooks**; **`commands`** and **`skills`** link to `.ai-agents` (same link script). |
| [`.opencode/`](.opencode) | opencode link directory; **`agents`** and **`commands`** link to `.ai-agents` (same link script). |
| [`opencode.json`](opencode.json) | opencode config — `instructions` (rule files), `permission` posture, shell. |
| [`CLAUDE.md`](CLAUDE.md) | Claude Code entry (auto-loaded by Claude). |
| [`CURSOR.md`](CURSOR.md) | Cursor entry (conventions; rules live under `.cursor/rules`). |

## Conventions

- **Single source of truth:** Edit skills, agents, and commands under [`.ai-agents/`](.ai-agents), not in duplicated copies.
- **After clone:** Run `scripts/link-ai-agents.ps1` (Windows) or `scripts/link-ai-agents.sh` (macOS/Linux) so `.claude`, `.cursor`, and `.opencode` see the same trees.
- **Authoring (MUST):** When **creating** a new skill, subagent, command, or hook, follow the folder’s **`TEMPLATE.md`** and complete every section (What, Why, How, When, Routing & discovery, Permissions & authority).
- **Routing (MUST):** When **choosing** which asset to use, read [`.ai-agents/ROUTER.md`](.ai-agents/ROUTER.md) and the relevant subfolder **`ROUTER.md`**.
- **Router tables (MUST):** After **creating, renaming, or deleting** any skill, agent, command, or hook file, update that folder’s **`ROUTER.md`** table in the **same change** (intent / use case, path, notes). Remove stale rows when deleting assets. Run `bash scripts/check-ai-agents-routers.sh` before push, or on Windows `powershell -File scripts/check-ai-agents-routers.ps1` (wraps the same Bash script). CI enforces the same check.
- **Permissions:** After changing tool or path requirements, align [`.claude/settings.json`](.claude/settings.json) and [`.ai-agents/PERMISSIONS.md`](.ai-agents/PERMISSIONS.md) as needed. See [Claude permissions](https://code.claude.com/docs/en/permissions) (`deny` overrides `allow`).
- **Tool-specific rules:** Keep Cursor rule files in [`.cursor/rules/`](.cursor/rules). They are not interchangeable with Claude `rules` without editing.
- **Security:** Do not commit secrets. Use local ignore files and environment variables for credentials.

## Reuse in another repository (consumer repo)

- Keep the consumer repo as its own repository and source of product code.
- Recommended mounting strategy: add this toolkit repo as a submodule at `.vibe-agent`, then treat `.vibe-agent/.ai-agents` as the canonical shared assets path.
- In the consumer repo, create its own root `AGENTS.md` with product/domain constraints specific to that repo.
- In the consumer repo, add a local link script that points `.claude`, `.cursor`, and `.opencode` directories to `.vibe-agent/.ai-agents/*` (skills/agents/commands).
- Keep this repository's `scripts/link-ai-agents.*` for this repo layout; do not assume they work unchanged in consumer repos.
- Treat `opencode.json` permissions as repository-local policy: if the consumer repo uses different paths than `src/**` and `tests/**`, update its permission map accordingly.

## Stack and quality

- Document **build, test, and lint** commands here as the codebase grows so every tool applies the same workflow.
- Keep shared assets **tool-agnostic by default**; move repo-specific constraints into `stack-profiles/` and local consumer docs.
- Respect **secrets boundaries**: never commit credentials; read secrets only via configured secure paths or environment variables (see **Security** above).

## Related docs

- [`.ai-agents/README.md`](.ai-agents/README.md) — detailed tool mapping and linking instructions.
- [`.ai-agents/ROUTER.md`](.ai-agents/ROUTER.md) — master router for choosing assets.
- [`.ai-agents/PERMISSIONS.md`](.ai-agents/PERMISSIONS.md) — permissions and authority.
- [`CLAUDE.md`](CLAUDE.md) — Claude Code–specific entry.
- [`CURSOR.md`](CURSOR.md) — Cursor-specific entry.
