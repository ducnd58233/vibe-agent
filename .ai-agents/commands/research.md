---
description: Run citation-first research on a scoped topic and return a digest with Applicability, Refine, Mermaid, sources, and unresolved questions
---

Research a scoped topic citation-first and return a digest of what the sources support, where they conflict, what applies to *this* topic, and what stays unresolved.

<references>

Follow [`research-with-citations`](../skills/research-with-citations/SKILL.md) and, for the full loop, [`researcher-harness`](../skills/researcher-harness/SKILL.md).

Primary persona: [`research-investigator`](../agents/research-investigator.md).

Diagrams: [`diagram-authoring`](../references/diagram-authoring.md). Cursor rule: research Applicability + Mermaid MUST.
</references>

## Inputs

<inputs>

- Topic or question (the user's research topic is the Applicability target)
- Optional scope constraints (time range, geography, source type)
</inputs>

## Required output

<outputs>

1. Scoped question breakdown
2. Findings with citations
3. Conflicts across sources
4. **Applicability (MUST)** - table or section mapping each source to this topic: reuse / reject / gap
5. **Refine (MUST)** - what to change in method, data, or scope before experiments
6. **Mermaid diagram (MUST)** - literature map or claim→method flow in a ` ```mermaid ` fence
7. `UNVERIFIED` items
8. Final digest section

When writing under `docs/<date>/<slug>/<version>/`, use basename `RESEARCH-<date>.md`.

**Every cited URL must resolve (MUST).** Run `vibe-agent docs check-citations <RESEARCH file>`
before checkpointing. `checkpoint` runs the same check at `auto_research` and `literature` and
refuses to leave the node while any cited URL (outside fenced code) fails; it fails closed offline,
and it refuses private and loopback addresses. A live link is necessary, not sufficient: open the
primary source and check the claim beside it, because across 14 models link validity stayed above
94% while factual accuracy against the cited source was 39-77% (arXiv:2605.06635), and 3-13% of
citation URLs are fabricated outright (arXiv:2604.03173).
</outputs>

```sh
vibe-agent doctor
vibe-agent research "<topic from the user>"
vibe-agent run status --slug <slug from start output>
```

Host agents derive slug and graph; do not ask the user for flags. For unattended
research loops with metric gates, use `vibe-agent auto research "<topic>"` after
`vibe-agent auto init`.

## Routing & discovery

<routing>

- Use when the user invokes `/research` or asks for a command-style citation digest.
- On `researcher-delivery`, this command drives the `literature` (and optionally writeup) nodes.
- Do not use when evidence is already collected and only synthesis is needed.
- Use [`investigate.md`](investigate.md) when the question needs parallel evidence lanes.
- Use [`experiment.md`](experiment.md) after an approved experiment design.
</routing>
