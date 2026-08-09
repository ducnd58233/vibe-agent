---
description: TDD workflow — RED/GREEN; Prove-It pattern for bugs
---

**Runtime required (MUST).** Results are recorded with `vibe-agent verify --slug <slug>`, which runs what [`vibe-checks.yaml`](../../vibe-checks.yaml) declares. There is no `--passed`; a test suite cannot be marked green by saying so. Rules: [`goal.md`](goal.md) section "Runtime is required".

Follow [`test-driven-development`](../skills/test-driven-development/SKILL.md) and [`references/testing-patterns.md`](../references/testing-patterns.md).

**Features:** failing tests → implement → refactor — keep suite green.

**Bugs:** Prove-It — reproduction test fails → fix → passes → regression suite.

**Scope (MUST):** tests cover **core logic and business behavior** only. Do **not** generate discovery or infrastructure tests (file/folder/env existence, testcontainer or service "is up", config trivia, import-only smoke), trivial code (getters, setters, no-logic constructors), pass-through wrappers, or tautological assertions where a mock returns what the test supplied. Put infrastructure checks in CI or test setup, not in behavioral test cases.

**Required coverage (MUST):** when the change touches data ingestion or parsing of untrusted input, concurrency and replay, a security boundary, money or state transitions, or a failure path, write a test that fails when that behavior regresses. The scope rule bans low-value tests; it does not permit writing none.

**Redaction coverage (MUST):** when a boundary is required to withhold something, assert the negative. A test that a logger, serializer, or error mapper omits a password, token, or personal field is behavior, not discovery, and it is the only test that fails when someone removes the redaction. Assert on what the sink received, not on the happy path around it. See [`secure-by-default`](../skills/secure-by-default/SKILL.md) and [`sensitive-data-exposure.md`](../references/sensitive-data-exposure.md).

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
