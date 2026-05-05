---
description: Implement next task incrementally — test, build, verify, commit
---

Combine [`planning-and-task-breakdown`](../skills/planning-and-task-breakdown/SKILL.md) (task order, acceptance) with [`test-driven-development`](../skills/test-driven-development/SKILL.md).

This repo does **not** ship a separate `incremental-implementation` skill; use explicit vertical slices and TDD.

For each next task:

1. Read acceptance criteria.
2. Load context ([`context-engineering`](../skills/context-engineering/SKILL.md) as needed).
3. RED — failing test for new behavior.
4. GREEN — minimal implementation.
5. Run full tests and typecheck/build per project (`npm`/`pnpm`/`uv` as documented).
6. Commit with a conventional message ([`git-workflow-and-versioning`](../skills/git-workflow-and-versioning/SKILL.md)).
7. Mark task complete; proceed.

On failure: [`debugging-and-error-recovery`](../skills/debugging-and-error-recovery/SKILL.md).

## What

Implement the next planned task incrementally with TDD guardrails.

## Why

Maintains delivery momentum while minimizing regressions.

## How

Use the RED/GREEN + verify + commit loop defined above.

## When

Invoke when a task plan exists and coding should begin.

## Routing & discovery

- Use when moving from approved plan to implementation.
- Do not use when requirements/spec remain unresolved.

## Permissions & authority

Inherits session permissions; may use edit, test, build, and git operations within policy boundaries.
