# Skill authoring template

Use this contract when **creating a new skill**. Implement the skill as `skills/<skill-name>/SKILL.md`. Copy the checklist below into your draft and fill every section before opening a PR.

Official skill shape reference: [Claude Code skills](https://code.claude.com/docs/en/skills), Cursor project skills (`SKILL.md` + YAML frontmatter).

---

## What

- **Skill name (folder):** `<skill-name>/`
- **Deliverable path:** `skills/<skill-name>/SKILL.md`
- **Purpose / scope:** What capability does this skill provide?
- **Inputs:** What context or artifacts does the agent need?
- **Outputs:** What does a successful run produce?

---

## Why

- **Problem:** What gap does this skill close?
- **Success criteria:** How do we know it worked?
- **Non-goals:** What this skill explicitly does not do.

---

## How

- **Steps:** Ordered instructions for the agent executing the skill.
- **Wiring:** Dependencies on repo paths, scripts, or external tools.
- **Frontmatter:** Must match Agent Skills rules (`name`, `description`, optional keys).

Map content into **`SKILL.md`** like this:

| Section in this template | Where it goes in `SKILL.md` |
|--------------------------|------------------------------|
| What / Why / When (summary) | YAML **`description`** — discovery and routing ([skills](https://code.claude.com/docs/en/skills)). |
| How | Markdown body under `# Instructions` (and optional `## Examples`). |

---

## When

- **Invocation:** Slash name, auto-invoke only if justified — state triggers explicitly.
- **Lifecycle:** One-shot workflow vs recurring pattern.

---

## Routing & discovery

Draft lines you can paste into YAML **`description`** and PR summaries:

- **Use when:** …
- **Do not use when:** …

Keep `description` third-person, concrete, and include trigger terms so Claude Code and Cursor can route correctly.

---

## Permissions & authority

Document tool and path needs so maintainers can align [`.claude/settings.json`](../../.claude/settings.json) [`permissions`](https://code.claude.com/docs/en/permissions).

| Topic | Notes |
|-------|--------|
| **Tools** | e.g. Read, Grep, Bash — list tools this skill expects to use. |
| **Paths** | Sensitive paths (`Read(./secrets/**)`) — call out so rules use `allow` / `ask` / `deny` appropriately. |
| **Suggested rules (examples)** | e.g. `Read(./src/**)`, `Bash(npm run test*)` — **deny overrides allow** in Claude Code. |
| **Cursor** | No `settings.json` permission matrix — note workspace trust and [`.cursor/rules`](../../.cursor/rules). |

After adding or tightening permissions in templates, update [`.ai-agents/PERMISSIONS.md`](../PERMISSIONS.md) if project-wide policy changes.

---

## After creating (MUST)

When you add, rename, or remove a skill folder under `skills/`:

1. Update **[`ROUTER.md`](ROUTER.md)** in this folder **in the same change**: add a row per skill (intent / use case, folder name, when to invoke); delete or edit rows when removing or renaming skills.
2. Keep column content aligned with each `SKILL.md` **`description`** and triggers.

Same PR or commit as the new asset when possible.
