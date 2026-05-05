# Subagent authoring template

Use this contract when **creating a new Claude Code subagent**. Implement as `agents/<name>.md` (single markdown file per subagent).

Reference: [Claude Code subagents](https://code.claude.com/docs/en/sub-agents).

---

## What

- **File name:** `agents/<name>.md`
- **Role:** One sentence — what this subagent is for.
- **Inputs:** What the parent agent passes (task description, paths).
- **Outputs:** Expected return (summary, artifacts, exit criteria).

---

## Why

- **Problem:** Why isolate this work in a subagent instead of the main loop?
- **Success criteria:** What does “done” look like?
- **Non-goals:** What it must not attempt.

---

## How

- **Behavior:** System prompt body — steps, tone, depth.
- **Frontmatter:** Required YAML keys per product docs (`name`, `description`, `tools`, etc.).
- **Tools (`tools:`):** Explicit list — this is the **authority boundary** for the subagent.

Map content into **`agents/<name>.md`**:

| Section | Where it goes |
|---------|----------------|
| What / Why / Routing | YAML **`description`** for discovery. |
| How | Markdown body (prompt). |
| Authority | YAML **`tools:`** — only listed tools are available. |

---

## When

- **Delegate when:** Parent should spawn this subagent (symptoms, task types).
- **Do not delegate when:** Cases that belong in the main agent or a different subagent.

---

## Routing & discovery

Draft for **`description`**:

- **Use when:** …
- **Do not use when:** …

---

## Permissions & authority

| Layer | Action |
|-------|--------|
| **Subagent `tools:`** | Narrowest set that still succeeds — prefer explicit tools over “everything”. |
| **Project `permissions`** | If the subagent only uses `Read` / `Grep`, parent session must still allow those globally unless Claude scopes differently; document any required [`.claude/settings.json`](../../.claude/settings.json) `allow` entries. |
| **Hooks** | If this subagent is always paired with a hook, note hook script paths and Bash rules. |
| **Cursor** | Subagents are Claude Code–specific; Cursor has no parallel — document “N/A” or link to [`.cursor/rules`](../../.cursor/rules) for similar guardrails. |

Sync material changes with [`.ai-agents/PERMISSIONS.md`](../PERMISSIONS.md).

---

## After creating (MUST)

When you add, rename, or remove a subagent file in `agents/`:

1. Update **[`ROUTER.md`](ROUTER.md)** in this folder **in the same change**: add a row (task / use case, filename, tool scope); delete or edit rows when removing agents.
2. Keep the table aligned with YAML **`tools:`** and **`description`**.

Same PR or commit as the new asset when possible.
