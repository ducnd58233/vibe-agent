---
description: Five-axis code review - correctness, readability, architecture, security, performance
---

Review the current change across five axes: correctness, readability, architecture, security, and performance.

<prerequisites>

**Runtime required (MUST).** Run `vibe-agent doctor` first. Rules: [`goal.md`](goal.md) section "Runtime is required".
</prerequisites>

<procedure>

Follow the [`code-review-and-quality`](../skills/code-review-and-quality/SKILL.md) skill.

Review current changes (staged diff, branch, or paths the user specifies) across:

1. **Correctness** - Spec alignment, edge cases, adequate tests.
2. **Readability** - Names, structure, clarity.
3. **Architecture** - Patterns, boundaries, coupling.
4. **Security** - Deep pass references [`security-and-hardening`](../skills/security-and-hardening/SKILL.md); query/injection safety for your persistence layer. **Also review the sinks, not only the logic:** every log call, response body, client-storage write, error path, and build-time env var the diff touched, against [`sensitive-data-exposure.md`](../references/sensitive-data-exposure.md). Disclosure is the defect class review misses most often, because the code works.
5. **Performance** - N+1, bundle, caching; see [`performance-optimization`](../skills/performance-optimization/SKILL.md).

Categorize findings as **Critical**, **Important**, or **Suggestion**. Include `file:line` and concrete fixes.

Optional: spawn the **`code-reviewer`** subagent ([`agents/code-reviewer.md`](../agents/code-reviewer.md)) for a dedicated review session.
</procedure>

## Routing & discovery

<routing>

- Use when review intent is explicit.
- Do not use as a replacement for implementation commands.

Invoke before merge, ship, or when quality concerns are raised.
</routing>
