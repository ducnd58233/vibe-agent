---
description: Spec-first — structured specification before implementation
---

Follow [`spec-driven-development`](../skills/spec-driven-development/SKILL.md).

clarify objective, users, acceptance criteria, stack constraints (from manifests, [`AGENTS.md`](../../AGENTS.md), [`stack-profiles/`](../stack-profiles/) when present), and boundaries (Always / Ask / Never).

Produce a spec covering: objective, tech stack, **commands** (real workspace scripts), project structure **for the current workspace**, code style pointer, testing strategy aligned with configured runners, boundaries, success criteria, open questions.

Save to a path agreed with the maintainer (for example `docs/SPEC.md` or `docs/features/<name>/SPEC.md`) and confirm before coding.

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
