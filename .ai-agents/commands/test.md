---
description: TDD workflow — RED/GREEN; Prove-It pattern for bugs
---

Follow [`test-driven-development`](../skills/test-driven-development/SKILL.md) and [`references/testing-patterns.md`](../references/testing-patterns.md).

**Features:** failing tests → implement → refactor — keep suite green.

**Bugs:** Prove-It — reproduction test fails → fix → passes → regression suite.

**Scope (MUST):** tests cover **core logic and business behavior** only. Do **not** generate discovery or infrastructure tests (file/folder/env existence, testcontainer or service "is up", config trivia, import-only smoke), trivial code (getters, setters, no-logic constructors), pass-through wrappers, or tautological assertions where a mock returns what the test supplied. Put infrastructure checks in CI or test setup, not in behavioral test cases.

**Required coverage (MUST):** when the change touches data ingestion or parsing of untrusted input, concurrency and replay, a security boundary, money or state transitions, or a failure path, write a test that fails when that behavior regresses. The scope rule bans low-value tests; it does not permit writing none.

**Judge semantically:** a test may read a file, the clock, or the environment when that reading is the claim. "Saving writes a manifest" and "this input creates no file" are behavior; "the config file exists" is discovery.

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
