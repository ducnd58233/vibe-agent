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
- **Tools (`tools:`):** Explicit map of tool names to `true` (OpenCode-compatible; matches [OpenCode `config.json` `tools` shape](https://opencode.ai/config.json)). This is the **authority boundary** for the subagent. Claude Code also accepts comma-separated or list-style `tools` in isolation; this repo standardizes on the boolean map for shared assets consumed by OpenCode.
- **Grounding rule (MUST):** every agent body must carry a no-fabrication rule — never describe a file, directory, or path not opened or listed via `Read`/`Grep`/`Glob`; report `ACCESS-FAILED: <path>` for inaccessible inputs instead of inferring structure. Subagents load individually (they do not read this template at runtime), so the rule must appear inline in each `agents/<name>.md`.

Map content into **`agents/<name>.md`**:

| Section | Where it goes |
|---------|----------------|
| What / Why / Routing | YAML **`description`** for discovery. |
| How | Markdown body (prompt). |
| Authority | YAML **`tools:`** — each allowed tool key set to `true`; all others unavailable. |

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
| **Subagent `tools:`** | Narrowest map that still succeeds (`ToolName: true`) — prefer explicit tools over “everything”. |
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

---

## Do not restate the defaults (MUST)

Write a `## Permissions & authority` section **only when this asset diverges** from the defaults in
[`PERMISSIONS.md`](../PERMISSIONS.md) "Default authority for skills, agents, and commands", and say
what diverges and why. Restating the default is what produced thirty-four identical copies that all
had to be edited together.

The same rule applies to any block that already lives somewhere canonical: the stack-profile pointer,
the grounding rule, the git gates. Link to the one home instead of copying it.
