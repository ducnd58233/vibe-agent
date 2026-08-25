# Workspace mistakes log

<context>

A plain markdown diary of agent failures and human corrections for the **current workspace**.
No tooling, no plugin, no vector store. Complements (does not replace) `.agent-state/memory.db`.

Path: **`.agent-state/MISTAKES.md`** at the workspace root (gitignored with the rest of
`.agent-state/`). Create the file on first append. Do not commit it.

Why not a tracked repo-root `MISTAKES.md`: consumer checkouts would accumulate agent noise in PRs.
Workspace-local state stays out of git the same way runs and `memory.db` do. Graduated hard rules
still land in tracked **`AGENTS.md`**, which every harness loads.
</context>

## When to write

<rules>

Append an entry when any of these happen:

- You broke a test, build, or verification the workspace already had green
- A human corrected a wrong approach, assumption, or edit
- You repeated a known failure after reading this log or `AGENTS.md`

Do **not** log routine first-attempt failures that were fixed in the same turn with no lasting
lesson. Do **not** put credentials, tokens, or personal data in the log; redact first
([`sensitive-data-exposure.md`](sensitive-data-exposure.md)).
</rules>

## Entry format (MUST)

<required>

Newest entry **first** (prepend below the title). One entry:

```markdown
## YYYY-MM-DD — short title

- **What happened:** …
- **Root cause:** …
- **Consequence:** …
- **Prevention:** the rule that would have stopped this (one sentence)
- **Class:** short kebab tag for counting repeats (e.g. `tasks-ac-checkboxes`, `forged-human-event`)
```

Optional: `**Related:**` run slug, PR number, or file path (workspace-relative).
</required>

## Graduation into AGENTS.md

<rules>

When the same **Class** appears about **four or five** times (count entries in this file), stop
treating it as diary-only:

1. Add a hard rule to the workspace-root **`AGENTS.md`** (or the consumer's local-first AGENTS.md).
2. Prepend a log entry noting the graduation (what rule, where it landed).
3. Prefer one sharp rule over restating every incident.

Graduation is a tracked charter change. The diary stays local evidence.
</rules>

## Versus runtime memory

<rules>

| | `.agent-state/MISTAKES.md` | `.agent-state/memory.db` |
|--|---------------------------|--------------------------|
| Shape | Human-readable narrative | Propose / confirm / forget records |
| Writer | Host agent (and humans) | Hooks + `vibe-agent memory` |
| Reader | Agents skimming before risky work | Session-start / prompt injection |
| Promotion | Repeat class → `AGENTS.md` rule | Confirm; expiry; forget |

Use the diary for deliberate "we got this wrong; here is the rule." Use memory for machine recall
of command outcomes. Do not copy every memory row into MISTAKES.
</rules>

## Consumer repos

<context>

When this toolkit is mounted in a consumer workspace, the log path is still
**`<workspace>/.agent-state/MISTAKES.md`**, not inside the toolkit checkout. Graduated rules go into
the **consumer** `AGENTS.md` under local-first precedence (root [`AGENTS.md`](../../AGENTS.md)).
</context>
