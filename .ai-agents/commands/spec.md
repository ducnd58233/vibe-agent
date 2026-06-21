---
description: Spec-first — structured specification before implementation
---

Follow [`spec-driven-development`](../skills/spec-driven-development/SKILL.md).

When the spec includes diagrams, flows, state maps, timelines, or architecture sketches, follow [`diagram-authoring`](../references/diagram-authoring.md).

clarify objective, users, acceptance criteria, stack constraints (from manifests, [`AGENTS.md`](../../AGENTS.md), [`stack-profiles/`](../stack-profiles/) when present), and boundaries (Always / Ask / Never).

Produce a spec covering: objective, tech stack, **commands** (real workspace scripts), project structure **for the current workspace**, code style pointer, testing strategy aligned with configured runners, boundaries, success criteria, open questions.

Write the spec to `docs/<slug>/SPEC.md` at the workspace root (the directory that contains `.vibe-agent/`; the repo root when this toolkit is used standalone). `<slug>` is a short kebab-case name for the work; confirm it with the user when it is not obvious. See the "Generated docs output location" rule in [`AGENTS.md`](../../AGENTS.md). Confirm the spec before coding.

## What

Produce a structured implementation spec before coding.

## Why

Aligns scope, constraints, and acceptance criteria early.

## How

Follow the existing spec workflow and output expectations above.

## When

Invoke for new features, ambiguous requests, or major refactors.

## Routing & discovery

- Use when implementation is not yet fully specified.
- Do not use when a reviewed spec already exists and execution is requested.

## Permissions & authority

Inherits session permissions; usually read/write markdown and repo discovery tools.
