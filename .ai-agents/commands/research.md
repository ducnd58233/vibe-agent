---
description: Run citation-first research on a scoped topic and return a digest with sources and unresolved questions
---

Research a scoped topic citation-first and return a digest of what the sources support, where they conflict, and what stays unresolved.

<references>

Follow [`research-with-citations`](../skills/research-with-citations/SKILL.md).

Primary persona: [`research-investigator`](../agents/research-investigator.md).

When the output includes docs with diagrams or flows, follow [`diagram-authoring`](../references/diagram-authoring.md).
</references>

## Inputs

<inputs>

- Topic or question
- Optional scope constraints (time range, geography, source type)
</inputs>

## Required output

<outputs>

1. Scoped question breakdown
2. Findings with citations
3. Conflicts across sources
4. `UNVERIFIED` items
5. Final digest section
</outputs>

## Routing & discovery

<routing>

- Use when research depth is needed before analysis/implementation.
- Do not use when evidence is already collected and only synthesis is needed.

Invoke when current, verifiable evidence is required.
</routing>
