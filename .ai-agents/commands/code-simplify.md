---
description: Simplify code without behavior change — clarity and maintainability
---

Follow [`code-simplification`](../skills/code-simplification/SKILL.md).

1. Read project charter [`AGENTS.md`](../../AGENTS.md) and [`stack-profiles/`](../stack-profiles/) when this repo pins a stack profile.
2. Scope: recent diff or user-specified paths.
3. Understand callers, tests, and edge cases before editing.
4. Reduce nesting, split long functions, improve names, dedupe carefully — **incrementally**, tests after each step.
5. Final pass with [`code-review-and-quality`](../skills/code-review-and-quality/SKILL.md) if requested.

Revert a step if tests fail.
