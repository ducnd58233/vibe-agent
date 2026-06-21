---
description: Break spec into ordered tasks with acceptance criteria
---

Follow [`planning-and-task-breakdown`](../skills/planning-and-task-breakdown/SKILL.md).

When the plan includes diagrams, dependency flows, timelines, or architecture sketches, follow [`diagram-authoring`](../references/diagram-authoring.md).

Read the spec (user-provided path) and relevant code. Prefer read-only planning unless the user wants edits.

1. Dependency graph and vertical slices (end-to-end thin slices).
2. Tasks with acceptance criteria, verification command, files likely touched.
3. Checkpoints between phases.

Write outputs to `docs/<slug>/PLAN.md` and `docs/<slug>/TASKS.md` at the workspace root (the directory that contains `.vibe-agent/`; the repo root when this toolkit is used standalone), reusing the same `<slug>` as the spec for this work. See the "Generated docs output location" rule in [`AGENTS.md`](../../AGENTS.md). Present for human review before implementation.

## What

Convert a spec into ordered implementation tasks with verification criteria.

## Why

Improves execution predictability and reduces missed dependencies.

## How

Use the existing dependency/task/checkpoint flow above.

## When

Invoke after spec approval and before implementation.

## Routing & discovery

- Use when execution steps are needed from written requirements.
- Do not use when only high-level brainstorming is requested.

## Permissions & authority

Inherits session permissions; primarily read/planning documentation authoring.
