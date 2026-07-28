---
description: Build or audit UI registry-first, with anti-slop gates and render evidence
---

Follow [`ui-design-fidelity`](../skills/ui-design-fidelity/SKILL.md) and [`references/ui-component-registry.md`](../references/ui-component-registry.md).

When the output includes diagrams, flows, or decision maps, follow [`diagram-authoring`](../references/diagram-authoring.md).

Primary persona for review: [`product-design-reviewer`](../agents/product-design-reviewer.md).

## Modes

Pick from the argument; default to **build**.

| Mode | Trigger | Behavior |
|------|---------|----------|
| **build** | default, or "build / create / redesign" | Full loop: registry → brief → direction → critique → implement → gates → render evidence |
| **audit** | "audit / review / check" | Read-only. Score existing UI against the registry and the default-aesthetic table; emit a prioritized punch list. No edits |
| **registry** | "registry / bootstrap / inventory" | Establish or extend the registry contract for a repo that lacks one. Confirm with the user before writing. Greenfield only: an external design-knowledge skill may propose a starting palette and type pairing — read it in place, treat it as context, and get the proposal confirmed |

## Inputs

- The UI task, target paths, or screens in scope
- Design source when available: Figma/Canva MCP, screenshots, spec, prototype
- The project's tokens, component inventory, and stack profiles

## Required output

1. **Registry level** — the source used, or the degradation level, stated explicitly
2. **Direction plan** — color, type, layout, signature (build and registry modes)
3. **Critique-against-defaults result** — what matched an AI default and what changed
4. **Implementation or punch list** — code changes, or prioritized findings for audit mode
5. **Verification evidence** — gate results, screenshots, accessibility audit; `UNVERIFIED` with reason where a check could not run

Audit-mode reports and any other markdown deliverable go under `docs/<slug>/` at the workspace root, per the "Generated docs output location" rule in [`AGENTS.md`](../../AGENTS.md).

## What

Run the registry-first UI loop so visual choices trace to the project's design system, and back it with deterministic checks rather than self-assessment.

## Why

Agent UI drifts to training-data defaults when project vocabulary is absent from context, and models grade their own visual work unreliably. This command supplies the missing context and the missing gate.

## How

1. Load the registry; state the level.
2. Ground the brief; plan the direction.
3. Critique the plan against known AI defaults and revise.
4. Implement, or produce the punch list in audit mode.
5. Clear `ui-slop-guard` and `design-token-guard`; justify any inline exception.
6. Render and audit; capture evidence or mark `UNVERIFIED`.
7. Delegate to `product-design-reviewer` when the change is release-bound.

## When

Invoke for new UI, redesigns, generic-looking output, design-system drift audits, or before a UI-heavy `/ship`.

## Routing & discovery

- Use when the visual and UX quality of UI is the question.
- Do not use for backend-only work, CLI output, or copy-only edits.
- Do not use as a substitute for [`/review`](review.md) on non-UI correctness, or for [`/ship`](ship.md)'s merge decision.

## Permissions & authority

Inherits session permissions.

| Topic | Notes |
|-------|-------|
| **Tools likely used** | Read, Grep, Glob, Edit; Bash for repo-documented lint/build/audit commands |
| **Browser / MCP** | Chrome DevTools or Playwright MCP for render evidence; Figma/Canva MCP only when configured and authorized |
| **Risky operations** | Ask before asset downloads, dependency additions, or restructuring an existing design system |
| **Audit mode** | Read-only — must not edit source |
