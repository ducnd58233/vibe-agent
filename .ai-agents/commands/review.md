---
description: Five-axis code review — correctness, readability, architecture, security, performance
---

Follow the [`code-review-and-quality`](../skills/code-review-and-quality/SKILL.md) skill.

Review current changes (staged diff, branch, or paths the user specifies) across:

1. **Correctness** — Spec alignment, edge cases, adequate tests.
2. **Readability** — Names, structure, clarity.
3. **Architecture** — Patterns, boundaries, coupling.
4. **Security** — Deep pass references [`security-and-hardening`](../skills/security-and-hardening/SKILL.md); query/injection safety for your persistence layer.
5. **Performance** — N+1, bundle, caching; see [`performance-optimization`](../skills/performance-optimization/SKILL.md).

Categorize findings as **Critical**, **Important**, or **Suggestion**. Include `file:line` and concrete fixes.

Optional: spawn the **`code-reviewer`** subagent ([`agents/code-reviewer.md`](../agents/code-reviewer.md)) for a dedicated review session.
