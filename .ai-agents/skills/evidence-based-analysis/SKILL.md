---
name: evidence-based-analysis
description: >-
  Synthesizes recommendations from gathered evidence using explicit criteria,
  confidence labels, and cited comparisons. Use when choosing between options
  and when tradeoffs must be justified with traceable evidence.
disable-model-invocation: true
---

# Evidence-Based Analysis

## How

<procedure>

1. FRAME
   - Define decision objective, constraints, and criteria up front.
   - State assumptions explicitly.
2. COMPARE
   - Build option-by-option comparison.
   - Each non-trivial cell is cited or labeled `UNVERIFIED`.
3. JUDGE
   - Apply criteria consistently and show weighting rationale.
   - Challenge your own leading hypothesis with counter-evidence.
4. RECOMMEND
   - Name the preferred option.
   - Include risks, mitigations, alternatives, and dissent notes.

Confidence labels per conclusion:
- `HIGH`: strong, consistent, primary evidence
- `MEDIUM`: mostly consistent with minor gaps
- `LOW`: limited or indirect evidence
- `UNVERIFIED`: missing direct supporting evidence
</procedure>

## Routing & discovery

<routing>

- Input source typically comes from [`research-with-citations`](../research-with-citations/SKILL.md).
- Hand off to [`idea-refine`](../idea-refine/SKILL.md) for alternative generation.
- Hand off to [`spec-driven-development`](../spec-driven-development/SKILL.md) when a recommendation becomes implementation scope.
- Hand off to [`documentation-and-adrs`](../documentation-and-adrs/SKILL.md) for final decision records.

Use for:
- Option comparisons
- Architecture or tool selection
- Tradeoff memos / ADR preparation
Avoid when:
- No evidence exists yet (run research first).
</routing>

## Permissions & authority

<required>

- Tools: `Read`, `Grep`, `Glob`, `WebSearch`, `WebFetch`
- Authority: read-only analysis and synthesis.
</required>
