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

## What

Run a five-axis code review on scoped changes and return severity-ranked findings.

## Why

Standardizes review coverage and reduces merge risk.

## How

Use the existing review flow above as the command procedure.

## When

Invoke before merge, ship, or when quality concerns are raised.

## Routing & discovery

- Use when review intent is explicit.
- Do not use as a replacement for implementation commands.

## Permissions & authority

Inherits session permissions; typically uses read/search and optional test/build tooling.
