---
description: Run citation-first research on a scoped topic and return a digest with sources and unresolved questions
---

Follow [`research-with-citations`](../skills/research-with-citations/SKILL.md).

Primary persona: [`research-investigator`](../agents/research-investigator.md).

When the output includes docs with diagrams or flows, follow [`diagram-authoring`](../references/diagram-authoring.md).

## Inputs

- Topic or question
- Optional scope constraints (time range, geography, source type)

## Required output

1. Scoped question breakdown
2. Findings with citations
3. Conflicts across sources
4. `UNVERIFIED` items
5. Final digest section

## What

Run citation-first research and return a structured digest.

## Why

Improves factual reliability for downstream decisions.

## How

Use the scoped input and required-output contract above.

## When

Invoke when current, verifiable evidence is required.

## Routing & discovery

- Use when research depth is needed before analysis/implementation.
- Do not use when evidence is already collected and only synthesis is needed.
