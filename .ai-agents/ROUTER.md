# Router (master index)

Use this file to **choose which asset family** to open before implementing work. Each subfolder has its own **`ROUTER.md`** with a concrete lookup table.
All assets are reusable and **domain-agnostic**; keep product-domain implementations in consumer repositories.

**Agents MUST (precedence):** Before applying anything here, check the **workspace root** for its own rules and templates (`AGENTS.md`, `CLAUDE.md`, `CLAUDE.local.md`, `.cursor/rules/`, its own `TEMPLATE.md`, existing file patterns). Those win; this toolkit is the fallback. On conflict, follow the local rule and state the divergence. See "Local-first precedence" in root [`AGENTS.md`](../AGENTS.md).

**Agents MUST (routing):** Read this file when deciding which skill, subagent, command, or hook applies; then open the listed **`ROUTER.md`** in that subfolder.

**Agents MUST (after creating):** When you add or remove an asset under `skills/`, `agents/`, `commands/`, `references/`, `stack-profiles/`, or `hooks/`, update that folder’s **`ROUTER.md`** in the **same change** (same PR or commit): for `stack-profiles/*.md`, add or remove matching rows in **`stack-profiles/ROUTER.md`** per **`stack-profiles/TEMPLATE.md`**; elsewhere add a row per new file and delete stale rows when removing assets.

| Goal / intent | Open next |
|---------------|-----------|
| Reusable workflow, SKILL-style instructions | [`skills/ROUTER.md`](skills/ROUTER.md) |
| Checklists and orchestration reference docs | [`references/ROUTER.md`](references/ROUTER.md) |
| Pinned frameworks for the **current workspace** | [`stack-profiles/ROUTER.md`](stack-profiles/ROUTER.md) (overview: [`README.md`](stack-profiles/README.md); authoring: [`TEMPLATE.md`](stack-profiles/TEMPLATE.md)) |
| Isolated specialist worker (Claude subagent) | [`agents/ROUTER.md`](agents/ROUTER.md) |
| Slash-style command prompt (Claude Code / Cursor after link script) | [`commands/ROUTER.md`](commands/ROUTER.md) |
| Full delivery from user objective (`/goal`) | [`commands/goal.md`](commands/goal.md) + [`skills/goal-driven-delivery/SKILL.md`](skills/goal-driven-delivery/SKILL.md) |
| Lifecycle automation (format on save, CI hooks, guardrails) | [`hooks/ROUTER.md`](hooks/ROUTER.md) |

**Creating new assets:** Follow that folder’s **`TEMPLATE.md`** (e.g. [skills/TEMPLATE.md](skills/TEMPLATE.md), [stack-profiles/TEMPLATE.md](stack-profiles/TEMPLATE.md), [agents/TEMPLATE.md](agents/TEMPLATE.md), [commands/TEMPLATE.md](commands/TEMPLATE.md), [hooks/TEMPLATE.md](hooks/TEMPLATE.md)).

**Permissions:** See [`PERMISSIONS.md`](PERMISSIONS.md) and align [`.claude/settings.json`](../.claude/settings.json) when assets need new tool patterns.

**Meta:** How routing fits the skill system — [`skills/using-agent-skills/SKILL.md`](skills/using-agent-skills/SKILL.md).
