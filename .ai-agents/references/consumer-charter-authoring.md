# Consumer charter authoring

<context>

Use when creating or editing workspace-root agent rules in a **consumer/product repository**. The
hard rule lives in root [`AGENTS.md`](../../AGENTS.md) (**Consumer charter neutrality**); this file
shows shape, not policy.
</context>

## Good (harness-neutral)

<rules>

```markdown
# Agent instructions (my-product)

This repo is a TypeScript API for billing. Domain rules live here; shared delivery workflows may
exist on the machine but are not named in this file.

## Stack

- Node 22, Fastify, PostgreSQL
- Tests: `npm test`; lint: `npm run lint`

## Security

- Never log card numbers or tokens.
- Read secrets from environment variables only.
```

```markdown
---
description: API handler conventions
globs: src/routes/**/*.ts
alwaysApply: false
---

Validate request bodies with Zod at the route boundary. Handlers orchestrate; business rules live in
`src/domain/`.
```
</rules>

## Bad (toolkit-coupled)

<rules>

Do **not** commit prose like:

- "Follow vibe-agent `AGENTS.md` at `.vibe-agent/AGENTS.md`."
- "Run `vibe-agent doctor` before every session."
- "See `.ai-agents/skills/engineering-principles` for SOLID rules."
- "Local-first precedence: this file wins over the toolkit charter in `AGENTS.md`."

Those belong in hook-injected session context or in this toolkit's own docs, not in a product repo's
charter.
</rules>
