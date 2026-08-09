---
description: Simplify code without behavior change — clarity and maintainability
---

Follow [`code-simplification`](../skills/code-simplification/SKILL.md).

1. Read project charter [`AGENTS.md`](../../AGENTS.md) and [`stack-profiles/`](../stack-profiles/) when the current workspace pins a stack profile.
2. Scope: recent diff or user-specified paths.
3. Understand callers, tests, and edge cases before editing.
4. Reduce nesting, split long functions, improve names, dedupe carefully — **incrementally**, tests after each step.
5. **Disclosure check (MUST):** a refactor may not widen what any sink receives. Behavior-preserving is not disclosure-preserving, and the test suite will not catch the difference: no test asserts what a log line, response body, or event payload must *not* contain. Re-check any step that merges DTOs, hoists an error object, moves a log statement, or broadens a serializer. See [`secure-by-default`](../skills/secure-by-default/SKILL.md).
6. Final pass with [`code-review-and-quality`](../skills/code-review-and-quality/SKILL.md) if requested.

Revert a step if tests fail.

## Routing & discovery

<routing>

- Use when clarity/complexity reduction is the main objective.
- Do not use when introducing new behavior without specification.

Invoke after behavior is understood and tests exist to protect refactors.
</routing>
