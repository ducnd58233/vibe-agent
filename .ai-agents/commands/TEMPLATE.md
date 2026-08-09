# Slash command authoring template

Use this contract when **creating a slash command** (single-file markdown prompt). Implement as `.ai-agents/commands/<name>.md`. **Claude Code** picks it up via `.claude/commands` → this folder; **Cursor** picks it up via `.cursor/commands` → this folder ([`CURSOR.md`](../../CURSOR.md)).

Reference: [Claude Code directory — commands](https://code.claude.com/docs/en/claude-directory); Cursor `/` picker uses [`.cursor/commands`](https://docs.cursor.com).

If your environment uses linked discovery paths, run [`scripts/link-ai-agents.ps1`](../../scripts/link-ai-agents.ps1) or [`scripts/link-ai-agents.sh`](../../scripts/link-ai-agents.sh) after clone so junctions/symlinks exist.

---

## What

- **File name:** `commands/<name>.md`
- **Command intent:** What the user gets when they invoke `/`-style routing for this file (per product behavior).
- **Inputs:** User-supplied args or implicit context.
- **Outputs:** Expected assistant behavior or artifacts.

---

## Why

- **Problem:** Why a dedicated command instead of a skill folder?
- **Success criteria:** …
- **Non-goals:** …

---

## How

- **Prompt body:** Full markdown content — instructions the command injects or follows.
- **Optional frontmatter:** If your toolchain supports metadata for commands, align with team convention.
- **Doc output location (MUST, if the command writes a markdown deliverable):** Write it under `docs/<slug>/` at the workspace root (the directory containing `.vibe-agent/`, or the repo root when this toolkit is standalone), using a short kebab-case `<slug>` for the work. Do not write into `.vibe-agent/`. See the "Generated docs output location" rule in [`AGENTS.md`](../../AGENTS.md).

---

## When

- **Invoke when:** User goals or keywords.
- **Avoid when:** Better handled by a skill or subagent.

---

## Routing & discovery

- **Use when:** …
- **Do not use when:** …

---

## Permissions & authority

Commands inherit the **same tool permissions** as the session that runs them. Document:

| Topic | Notes |
|-------|--------|
| **Tools likely used** | Read, Edit, Bash patterns — so reviewers can add [`.claude/settings.json`](../../.claude/settings.json) rules. |
| **Risky operations** | Network, deploy, `rm` — prefer `ask` or `deny` per [permissions](https://code.claude.com/docs/en/permissions). |
| **Cursor** | Same `*.md` file — no duplicate folder; commonly discovered via `.cursor/commands` symlink/junction ([`CURSOR.md`](../../CURSOR.md)). |

Update [`.ai-agents/PERMISSIONS.md`](../PERMISSIONS.md) when commands imply new default allows.

---

## After creating (MUST)

When you add, rename, or remove a command file in `commands/`:

1. Update **[`ROUTER.md`](ROUTER.md)** in this folder **in the same change**: add a row (user goal / use case, filename, preconditions); delete or edit rows when removing commands.

Same PR or commit as the new asset when possible.

---

## Do not restate the defaults (MUST)

Write a `## Permissions & authority` section **only when this asset diverges** from the defaults in
[`PERMISSIONS.md`](../PERMISSIONS.md) "Default authority for skills, agents, and commands", and say
what diverges and why. Restating the default is what produced thirty-four identical copies that all
had to be edited together.

The same rule applies to any block that already lives somewhere canonical: the stack-profile pointer,
the grounding rule, the git gates. Link to the one home instead of copying it.
