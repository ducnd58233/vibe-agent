---
name: spec-driven-development
description: >-
  Produces a written spec before coding for new features or ambiguous work. Use when requirements are unclear, scope crosses modules, or architectural choices need agreement across frontend and backend.
disable-model-invocation: true
---

# Spec-Driven Development

## Stack profile for current workspace

When working **in a repository that includes this toolkit**, open [`stack-profiles/ROUTER.md`](../../stack-profiles/ROUTER.md) when present, select applicable profiles for the planned work, and read those files. Product/domain expectations: root [`AGENTS.md`](../../../AGENTS.md).

## Overview

Write a structured specification before implementation. The spec is the shared truth for **what**, **why**, and **done means**.

## When to Use

- New feature or significant change.
- Requirements ambiguous or multi-module.
- Architectural decision needed.

**When NOT to use:** One-line fixes; changes with obvious, local scope.

## What

Defines a spec-first workflow that turns ambiguous requests into validated implementation contracts.

## Why

Aligns stakeholders on scope, constraints, and acceptance criteria before coding.

## How

Use the gated workflow and phase details in this file to move from specification to implementation.

## When

Use for non-trivial work requiring shared requirements and sequencing clarity.

## Routing & discovery

- Use when requirements are unclear, cross-cutting, or architecture-affecting.
- Do not use for tiny local fixes with obvious behavior and scope.

## Permissions & authority

- Tools: read/write documentation and planning artifacts, plus project context discovery.
- Authority: require human validation at gated checkpoints before implementation progression.

## Gated Workflow

```text
SPECIFY → PLAN → TASKS → IMPLEMENT
```

Do not advance without human validation when the process calls for it.

### Phase 1: Specify

List assumptions **before** detailed spec content:

```text
ASSUMPTIONS:
1. …
→ Correct now or I proceed.
```

Cover at minimum:

1. **Objective** — users, problem, success.
2. **Tech stack** — align with repo manifests, [`stack-profiles/`](../../stack-profiles/), and [`AGENTS.md`](../../../AGENTS.md); name frameworks only after reading files in-tree.
3. **Commands** — real commands from the current workspace (for example `npm run dev`, `uv run pytest`), not placeholders.
4. **Project structure** — where UI, API, tests, and docs live in **this** monorepo layout.
5. **Code style** — pointer to existing conventions + one short example.
6. **Testing strategy** — unit/component, API integration, and E2E per layer using runners configured for the current workspace.
7. **Boundaries** — Always / Ask first / Never (schema changes, new deps, CI, secrets).

Include **Success criteria** as testable checks (latency, validation rules, UX states).

**Output location:** Write the spec to `docs/<slug>/SPEC.md` at the workspace root (the directory containing `.vibe-agent/`, or the repo root when this toolkit is standalone). Reuse the same `<slug>` for the plan and tasks. See the "Generated docs output location" rule in [`AGENTS.md`](../../../AGENTS.md).

**Template sketch:**

```markdown
# Spec: [Name]

## Objective
## Tech stack (pinned where relevant)
## Commands (from repo)
## Project structure (current workspace)
## Code style
## Testing strategy
## Boundaries (Always / Ask / Never)
## Success criteria
## Open questions
```

### Phase 2: Plan

Technical plan: components, dependencies, order, risks, parallel vs sequential work, checkpoints.

### Phase 3: Tasks

Discrete tasks with acceptance criteria, verification command, and expected files touched. Prefer tasks that avoid gigantic single PRs.

### Phase 4: Implement

Execute using [`planning-and-task-breakdown`](../planning-and-task-breakdown/SKILL.md), [`test-driven-development`](../test-driven-development/SKILL.md), and [`context-engineering`](../context-engineering/SKILL.md) as appropriate.

## Living Spec

Update the spec when scope or decisions change; link specs from PRs.

## Verification

Before implementation:

- [ ] Spec covers objective, stack, commands, layout, testing, boundaries, success criteria
- [ ] Human reviewed when required by team process
- [ ] Spec written to `docs/<slug>/SPEC.md` at the workspace root (sibling of `.vibe-agent/`)
