---
name: research-with-citations
description: >-
  Researches a topic using verifiable sources and produces a cited digest. Use
  when factual accuracy matters, when memory may be stale, or when decisions
  require source-backed claims and conflict handling.
disable-model-invocation: true
---

# Research With Citations

## How

<procedure>

1. PLAN
   - Restate the question and split into sub-questions.
   - Define required evidence types (official docs, regulatory filings, papers, news, local files).
2. RETRIEVE
   - Gather evidence with `WebSearch`, `WebFetch`, and repository reads.
   - Prefer primary sources first.
3. VERIFY
   - Cross-check critical claims across multiple sources.
   - If sources conflict, preserve the disagreement with citations.
4. CITE
   - Add a URL or file path citation for each non-trivial claim.
   - Numeric facts must always be cited.
5. DIGEST
   - Produce a concise synthesis: key findings, conflicts, open questions, and confidence.

No-fabrication rules:
- If a claim cannot be traced to a reachable URL or local file, label it `UNVERIFIED`.
- Do not synthesize numeric facts (prices, percentages, dates, statistics) from memory.
- When reputable sources conflict, present both and identify disagreement.

Evidence priority:
1. Regulatory / primary data
2. Official product documentation
3. Peer-reviewed publications
4. Reputable reporting (Reuters, Bloomberg, FT)
5. Engineering blogs with named authors
6. Tutorials / secondary summaries (mark weaker confidence)
</procedure>

## Routing & discovery

<routing>

- Pair with [`evidence-based-analysis`](../evidence-based-analysis/SKILL.md) to convert findings into decisions.
- Use [`context-engineering`](../context-engineering/SKILL.md) to keep only relevant evidence in context.
- Use [`source-driven-development`](../source-driven-development/SKILL.md) for framework API decisions.

Use for:
- Factual investigations
- Market/regulatory/standards questions
- “Compare X vs Y” requests requiring current citations
Avoid when:
- The task is purely stylistic editing with no factual assertions.
</routing>

## Permissions & authority

<required>

- Tools: `Read`, `Grep`, `Glob`, `WebSearch`, `WebFetch`
- Authority: read-only research workflow; no file modification or shell side effects.
</required>
