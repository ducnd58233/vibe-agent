---
description: Parallel investigation with investigator, analyst, and source auditor, then merged verdict
---

Follow [`references/orchestration-patterns.md`](../references/orchestration-patterns.md) parallel fan-out rules and compose:

1. [`research-investigator`](../agents/research-investigator.md)
2. [`data-analyst`](../agents/data-analyst.md)
3. [`source-auditor`](../agents/source-auditor.md)

## Phase A — Fan-out

Run three independent tracks on the same scoped topic.

## Phase B — Merge

Merge into:
- Evidence summary
- Analytical recommendation
- Source-audit findings

## Phase C — Final report

Return:
1. Investigator section
2. Analyst section
3. Auditor section
4. Consolidated verdict with confidence and follow-up actions

## What

Execute parallel investigation lanes and merge into one evidence-backed verdict.

## Why

Combines complementary perspectives while keeping orchestration explicit.

## How

Use the existing Phase A/B/C fan-out and merge workflow.

## When

Invoke for multi-faceted questions needing research, analysis, and source audit.

## Routing & discovery

- Use when one persona is insufficient for confidence.
- Do not use for small, single-lane tasks.

## Permissions & authority

Inherits session permissions; orchestration must respect persona boundaries and allowed tools.
