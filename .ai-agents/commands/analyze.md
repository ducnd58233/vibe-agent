---
description: Analyze evidence into a confidence-labeled recommendation with a cited comparison table
---

<references>

Follow [`evidence-based-analysis`](../skills/evidence-based-analysis/SKILL.md).

When the analysis output includes diagrams, flows, timelines, or decision maps, follow [`diagram-authoring`](../references/diagram-authoring.md).

Primary persona: [`data-analyst`](../agents/data-analyst.md).
</references>

## Inputs

<inputs>

- Existing digest/report, or scoped evidence set
- Decision objective and constraints
</inputs>

## Required output

<outputs>

1. Decision frame
2. Comparison table including `Source`
3. Recommendation with alternatives
4. Confidence per conclusion (`HIGH` / `MEDIUM` / `LOW` / `UNVERIFIED`)
</outputs>

## Routing & discovery

<routing>

- Use when comparing options with evidence.
- Do not use when source collection is incomplete.

Invoke after research/digest is available and a decision is required.
</routing>
