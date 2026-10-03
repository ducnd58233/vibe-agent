---
description: Write findings that cite experiment STATUS and run artifacts; no orphan claims
---

Synthesize experiment outcomes into FINDINGS with citations to the run STATUS and artifacts. Every claim must point at evidence from this run or from RESEARCH.

<references>

Follow [`researcher-harness`](../skills/researcher-harness/SKILL.md) and [`research-with-citations`](../skills/research-with-citations/SKILL.md).

Diagrams: [`diagram-authoring`](../references/diagram-authoring.md).
</references>

## Required output

<outputs>

Write `docs/<date>/<slug>/<version>/FINDINGS-<date>.md` with:

1. Summary of outcomes vs hypothesis
2. Evidence table (claim → STATUS/log/path)
3. Failures and what they falsify
4. Next experiments (optional)
5. At least one Mermaid summary when the result set has more than one stage
6. **Cost of the number (MUST):** trials tried, failed and abandoned runs, spread across seeds, coverage (evaluated over total), the selection-versus-held-out gap, and which results are exploratory because their split was reused. Rules: [`research-integrity`](../references/research-integrity.md)

When this work also writes a durable ledger entry under
`experiments/<project-slug>/<run-id>/` ([`researcher-harness`](../skills/researcher-harness/SKILL.md)
"Experiment ledger, across runs"), item 1 cites that run-id's `JUDGEMENT.md` rather than restating
it - `JUDGEMENT.md` is the durable, cross-run record; `FINDINGS.md` is this delivery's synthesis of
possibly several run-ids, and should point at each rather than duplicate its text.
</outputs>

## Routing & discovery

<routing>

- Use on the `findings` node of `researcher-delivery`.
- Do not invent metrics that STATUS or logs do not support; mark `UNVERIFIED`.
- A claim backed by one favourable run, or by a split that was scored more than once, is `UNVERIFIED` or exploratory, never a finding.
- Report failed and refuted runs with the same prominence as the one that worked.
</routing>
