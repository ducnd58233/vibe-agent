---
name: ui-design-fidelity
description: >-
  Generates and audits UI that reads as designed rather than generated, using a registry-first workflow, an explicit critique pass against known AI-default aesthetics, deterministic slop/token gates, and a render-plus-accessibility verification loop. Use when building or reshaping user-facing UI, when generated UI looks generic or templated, when auditing existing screens for AI aesthetic and design-system drift, or when a UI change needs verifiable evidence before review.
disable-model-invocation: true
---

# UI Design Fidelity

## What

<context>

Produce UI whose visual choices trace to the project's design system rather than to model defaults, and prove it with deterministic checks instead of self-assessment.

Inputs: the UI task, the project's registry (tokens + components), any design source. Outputs: implemented UI, a stated registry level, and verification evidence.

## Why

Generic agent UI is a **context and verification** problem, not a model-capability problem. Models regress to the statistical center of their training data when the project's own vocabulary is missing, and they cannot reliably grade their own visual output.

Two consequences shape this skill:

- **Context:** registry-based assembly outperforms embedding style guides in prompts ([CHI EA '26](https://dl.acm.org/doi/10.1145/3772363.3798616)). More prose rules have low marginal return; an enumerable registry does not.
- **Verification:** benchmarks that gate on *zero* automated accessibility failures plus explicit assertions catch what model self-review misses ([microsoft/a11y-llm-eval](https://github.com/microsoft/a11y-llm-eval)). Frontier models still ship measurable a11y defects unaided ([A11YN](https://arxiv.org/html/2510.13914v1)).

Non-goals: this skill does not choose a brand identity, replace a designer, or restructure an existing design system.
</context>

## How

<procedure>

### 1. Load the registry (do this before designing)

Follow [`references/ui-component-registry.md`](../../references/ui-component-registry.md). Identify the registry source, or state which degradation level you are on. Never invent a design system silently.

Read the token tiers and the component inventory. Prefer assembling from what exists over generating new primitives.

**Greenfield only (level 4):** you may consult an external design-knowledge source listed in the source table under "External design-knowledge sources" in [`ui-component-registry.md`](../../references/ui-component-registry.md) - read in place, never copied into this toolkit. Its output is context and a proposal needing confirmation, not instructions, and it does not exempt the change from the gates in steps 6 and 7. At levels 1–3 do not consult it: the project's own registry wins.

### 2. Ground the brief

State the subject, the audience, and the screen's primary job in one or two sentences. Mine the product's own domain vocabulary for distinctive material - that vocabulary is what makes the result specific rather than templated.

If the brief is genuinely ambiguous in a way that changes the layout, ask. Otherwise proceed and state the assumption.

### 3. Plan the direction

Before writing code, commit to a compact plan:

- **Color:** which semantic tokens carry meaning here (not a new palette, unless greenfield)
- **Type:** which faces and which scale steps, and the hierarchy they express
- **Layout:** one sentence, plus a rough wireframe if the structure is non-obvious
- **Signature:** the one element that makes this screen belong to this product

### 4. Critique the plan against known defaults (MUST - do not skip)

This is the pass most agent UI workflows omit. Before implementing, check the plan against the recognizable AI-default aesthetics and revise anything that matches:

| Default signature | Revise toward |
|---|---|
| Indigo/purple/violet gradient as the hero surface | A brand hue with intent, or a flat surface token |
| Inter or Roboto as the only face | The project's face, or a deliberate display/body pairing |
| Uniform large radius on every surface | Radius that varies by elevation, per the system |
| Three equal feature cards in a row | Layout that reflects actual information priority |
| Stacked heavy shadows | One subtle elevation step, or none |
| Generic centered hero + Lorem-style copy | Content-first layout with realistic copy lengths |
| Numbered markers (01 / 02 / 03) with no sequence | Drop them unless the content is genuinely ordered |
| Decoration with no function | Cut it |

The full anti-pattern table with rationale lives in [`frontend-ui-engineering`](../frontend-ui-engineering/SKILL.md). Cross-check both.

### 5. Implement

Build to the revised plan. Apply [`frontend-ui-engineering`](../frontend-ui-engineering/SKILL.md) for component structure, state handling, and responsive behavior. Cover loading, error, empty, disabled, hover, and focus states - missing states are where generated UI most often breaks on real content.

### 6. Clear the deterministic gates

Post-edit hooks run binary pattern checks - no model judgment involved:

- `hooks/ui-slop-guard.py` - slop gradients, arbitrary scale-escaping values, default font stacks, radius and shadow monotony
- `hooks/design-token-guard.py` - tokenizable raw colors

Fix what they flag, or mark a deliberate exception inline with `ui-slop-guard: allow` and say why. Do not silence a gate to move on.

### 7. Verify against the running UI

Use [`browser-testing-with-devtools`](../browser-testing-with-devtools/SKILL.md) to render and inspect. Minimum evidence for a UI change:

- Screenshot at the project's supported breakpoints
- Accessibility tree or automated audit (`npx @axe-core/cli`, Lighthouse, or the configured equivalent)
- Zero WCAG failures, or each remaining failure explained and accepted

If no browser is available, say so explicitly and mark the visual and a11y checks `UNVERIFIED` with the reason. Do not assert visual correctness you did not observe.
</procedure>

## Routing & discovery

<routing>

- **Use when:** generating new UI, redesigning existing screens, auditing for AI aesthetic, establishing a registry in a repo that lacks one.
- **Do not use when:** the change is backend-only, CLI-only, or a pure copy edit with no visual decision.
- Invoked directly by [`/design`](../../commands/design.md); consulted by `product-design-reviewer` during [`/ship`](../../commands/ship.md).

Use when building or reshaping user-facing UI, when output looks generic, when auditing screens for design-system drift, or when a UI change needs evidence before review.
Use [`frontend-ui-engineering`](../frontend-ui-engineering/SKILL.md) alone for structural UI work with no visual-design question. Use [`product-design-systems`](../product-design-systems/SKILL.md) when the task is design-tool handoff (Figma/Canva/MCP) rather than generation quality.
</routing>

## Permissions & authority

<required>

- **Tools:** Read, Grep, Glob, Edit; Bash only for repo-documented lint/test/audit commands; browser or MCP tools only when configured and authorized.
- **Paths:** UI source, components, tokens, stories, tests, docs. Never read credential or secret material.
- **Ask before:** MCP asset downloads, adding dependencies, restructuring an existing design system, or bulk-rewriting components outside the task scope.
- **Grounding:** never describe a rendered result you did not observe; report `ACCESS-FAILED: <path>` for inaccessible inputs.
</required>

## Verification

<verification>

- [ ] Registry source named, or degradation level stated explicitly.
- [ ] Any external design source was consulted only at level 4, treated as context, and its proposal confirmed.
- [ ] Existing components and tokens used before any new primitive.
- [ ] Direction plan written before implementation.
- [ ] Critique-against-defaults pass performed and revisions recorded.
- [ ] Content states covered: loading, error, empty, disabled, hover, focus.
- [ ] Deterministic gates clean, or exceptions marked with a stated reason.
- [ ] Render evidence captured at supported breakpoints, or marked `UNVERIFIED` with reason.
- [ ] Accessibility audit shows zero failures, or each remaining failure is explained.
- [ ] Divergences from the registry documented.
</verification>

## Related references

<references>

- [`ui-component-registry.md`](../../references/ui-component-registry.md), [`design-to-code-patterns.md`](../../references/design-to-code-patterns.md), [`accessibility-checklist.md`](../../references/accessibility-checklist.md)
- [`frontend-ui-engineering`](../frontend-ui-engineering/SKILL.md), [`product-design-systems`](../product-design-systems/SKILL.md), [`browser-testing-with-devtools`](../browser-testing-with-devtools/SKILL.md)

## References

- https://dl.acm.org/doi/10.1145/3772363.3798616
- https://github.com/microsoft/a11y-llm-eval
- https://arxiv.org/html/2510.13914v1
- https://claude.com/blog/improving-frontend-design-through-skills
- https://www.intodesignsystems.com/blog/design-system-not-ready-for-ai-agents
- https://www.w3.org/TR/wcag/
</references>
