---
name: spec-driven-development
description: >-
  Produces a written spec before coding for new features or ambiguous work. Use when requirements are unclear, scope crosses modules, or architectural choices need agreement across frontend and backend.
disable-model-invocation: true
---

# Spec-Driven Development

## Overview

<context>

Write a structured specification before implementation. The spec is the shared truth for **what**, **why**, and **done means**.
</context>

## When to Use

<routing>

- New feature or significant change.
- Requirements ambiguous or multi-module.
- Architectural decision needed.

**When NOT to use:** One-line fixes; changes with obvious, local scope.

## Routing & discovery

- Use when requirements are unclear, cross-cutting, or architecture-affecting.
- Do not use for tiny local fixes with obvious behavior and scope.

Use for non-trivial work requiring shared requirements and sequencing clarity.
</routing>

## Permissions & authority

<required>

- Tools: read/write documentation and planning artifacts, plus project context discovery.
- Authority: require human validation at gated checkpoints before implementation progression.
</required>

## Gated Workflow

<procedure>

```text
SPECIFY → PLAN → TASKS → IMPLEMENT
```

Do not advance without human validation when the process calls for it.

### Phase 1: Specify

List assumptions **before** detailed spec content. Prefer stable ids so a later
failure TRACE can cite them (see [`failure-trace.md`](../../references/failure-trace.md)):

```text
ASSUMPTIONS:
A1. …
A2. …
→ Correct now or I proceed.
```

Assumption statements are authoring aids and TRACE links. They are **not**
checkpoint evidence: never record assumption truth as `Passed` with any
`--source`. Invalidation belongs in TRACE / tasks / replan edges.

Cover at minimum:

1. **Objective** - users, problem, success.
2. **Tech stack** - align with repo manifests, [`stack-profiles/`](../../stack-profiles/), and [`AGENTS.md`](../../../AGENTS.md); name frameworks only after reading files in-tree.
3. **Commands** - real commands from the current workspace (for example `npm run dev`, `uv run pytest`), not placeholders.
4. **Project structure** - where UI, API, tests, and docs live in **this** monorepo layout.
5. **Code style** - pointer to existing conventions + one short example.
6. **Testing strategy** - unit/component, API integration, and E2E per layer using runners configured for the current workspace.
7. **Boundaries** - Always / Ask first / Never (schema changes, new deps, CI, secrets).

Include **Success criteria** as testable checks (latency, validation rules, UX states).

**Output location:** Write the spec to `docs/<slug>/SPEC.md` at the workspace root (the directory containing `.vibe-agent/`, or the repo root when this toolkit is standalone). Reuse the same `<slug>` for the plan and tasks. See the "Generated docs output location" rule in [`AGENTS.md`](../../../AGENTS.md).

**Template sketch:**

```markdown
# Spec: [Name]

## Open questions
```

### Phase 2: Plan

Technical plan: components, dependencies, order, risks, parallel vs sequential work, checkpoints.

### Phase 3: Tasks

Discrete tasks with acceptance criteria, verification command, and expected files touched. Prefer tasks that avoid gigantic single PRs.

### Phase 4: Implement

Execute using [`planning-and-task-breakdown`](../planning-and-task-breakdown/SKILL.md), [`test-driven-development`](../test-driven-development/SKILL.md), and [`context-engineering`](../context-engineering/SKILL.md) as appropriate.
</procedure>

## Living Spec

<context>

Update the spec when scope or decisions change; link specs from PRs.
</context>

## Verification

<verification>

Before implementation:

- [ ] Spec covers objective, stack, commands, layout, testing, boundaries, success criteria
- [ ] Human reviewed when required by team process
- [ ] Spec written to `docs/<slug>/SPEC.md` at the workspace root (sibling of `.vibe-agent/`)
</verification>
