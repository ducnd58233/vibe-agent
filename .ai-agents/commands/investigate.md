---
description: Parallel investigation with investigator, analyst, and source auditor, then merged verdict
---

Follow [`references/orchestration-patterns.md`](../references/orchestration-patterns.md) parallel fan-out rules and compose:

1. [`research-investigator`](../agents/research-investigator.md)
2. [`data-analyst`](../agents/data-analyst.md)
3. [`source-auditor`](../agents/source-auditor.md)

When the final report includes diagrams, flows, timelines, or evidence maps, follow [`diagram-authoring`](../references/diagram-authoring.md).

## Phase 0 - Scope & repository-access preflight (MUST run first)

<procedure>

1. **Pick the right instrument.** These three lanes are **web/citation** personas. If the question is primarily about a **local code repository** (its tree, code, or behavior), do **not** fan them out over the filesystem - prefer the built-in read-only `Explore` agent or direct `Read`/`Grep`/`Glob` (orchestration-patterns section 5). Reserve this command for **evidence/citation** questions (web, docs, mixed) where the lanes each add a *different kind* of finding.
2. **Resolve and pass an absolute working directory.** Compute the repo root as an absolute path and pass it explicitly to every lane. Never let a lane guess its own CWD.
3. **Require a ground-truth anchor.** Each lane must begin by listing the real top-level tree (`Glob`/`Read`) and echo it back. A lane that reports `ACCESS-FAILED: <path>` returns zero findings and is treated as non-authoritative.
4. **Authoritative-fallback rule.** If your own tools verifiably access the repo but a lane cannot (or its anchor does not match the real tree), treat that lane's output as non-authoritative and investigate directly. Do not merge unverified structural claims.

## Phase A - Fan-out

Run three independent tracks on the same scoped topic. Each lane operates only on paths it has anchored in Phase 0; redundant lanes reading one shared local tree are an anti-pattern (see orchestration-patterns section E).

## Phase B - Merge

Merge into:
- Evidence summary
- Analytical recommendation
- Source-audit findings

## Phase C - Final report

Return:
1. Investigator section
2. Analyst section
3. Auditor section
4. Consolidated verdict with confidence and follow-up actions
</procedure>

## Routing & discovery

<routing>

- Use when one persona is insufficient for confidence on an **evidence/citation** question.
- Do not use for small, single-lane tasks.
- Do not use for repository/code investigation grounded in a single local tree - route to `Explore` or direct tools instead (see Phase 0).

Invoke for multi-faceted questions needing research, analysis, and source audit.
</routing>
