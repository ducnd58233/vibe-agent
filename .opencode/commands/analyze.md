---
description: Analyze evidence into a confidence-labeled recommendation with a cited comparison table
---

Follow [`evidence-based-analysis`](../skills/evidence-based-analysis/SKILL.md).

Primary persona: [`data-analyst`](../agents/data-analyst.md).

## Inputs

- Existing digest/report, or scoped evidence set
- Decision objective and constraints

## Required output

1. Decision frame
2. Comparison table including `Source`
3. Recommendation with alternatives
4. Confidence per conclusion (`HIGH` / `MEDIUM` / `LOW` / `UNVERIFIED`)

## What

Transform gathered evidence into a recommendation with confidence labels.

## Why

Makes tradeoff logic transparent and auditable.

## How

Use the existing input/output contract and evidence-based analysis flow.

## When

Invoke after research/digest is available and a decision is required.

## Routing & discovery

- Use when comparing options with evidence.
- Do not use when source collection is incomplete.

## Permissions & authority

Inherits session permissions; primarily read/search/synthesis operations.
