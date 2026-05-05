---
description: TDD workflow — RED/GREEN; Prove-It pattern for bugs
---

Follow [`test-driven-development`](../skills/test-driven-development/SKILL.md) and [`references/testing-patterns.md`](../references/testing-patterns.md).

**Features:** failing tests → implement → refactor — keep suite green.

**Bugs:** Prove-It — reproduction test fails → fix → passes → regression suite.

For browser/runtime issues, add [`browser-testing-with-devtools`](../skills/browser-testing-with-devtools/SKILL.md) when DevTools verification is needed.

Optional subagent: **`test-engineer`** ([`agents/test-engineer.md`](../agents/test-engineer.md)).

## What

Run TDD/Prove-It verification workflow for features and bugfixes.

## Why

Ensures behavioral correctness and guards against regressions.

## How

Use the existing feature/bug workflow above and apply browser validation when needed.

## When

Invoke during implementation, bugfix, and pre-merge verification.

## Routing & discovery

- Use when test strategy or prove-it evidence is required.
- Do not use as a substitute for architecture/security review.

## Permissions & authority

Inherits session permissions; may execute test tooling allowed by the active environment.
