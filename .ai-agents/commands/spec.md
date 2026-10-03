---
description: Spec-first - structured specification before implementation
---

Write the structured specification before any implementation starts: objective, stack, commands, structure, testing, boundaries, open questions.

<required>

Follow [`spec-driven-development`](../skills/spec-driven-development/SKILL.md).

When the spec includes diagrams, flows, state maps, timelines, or architecture sketches, follow [`diagram-authoring`](../references/diagram-authoring.md).

clarify objective, users, acceptance criteria, stack constraints (from manifests, [`AGENTS.md`](../../AGENTS.md), [`stack-profiles/`](../stack-profiles/) when present), and boundaries (Always / Ask / Never).

Produce a spec covering: objective, tech stack, **commands** (real workspace scripts), project structure **for the current workspace**, code style pointer, testing strategy aligned with configured runners, boundaries, success criteria, open questions.

Every figure in the spec, including every budget, threshold, and limit it states is computed with `vibe-agent calc` and logged in a fenced `calc` block, and `checkpoint` refuses to leave the node while a logged line does not recompute. Give each figure its unit, currency, as-of date, and source. Rules: [`quantitative-accuracy`](../skills/quantitative-accuracy/SKILL.md).

**Data classification (MUST):** name the sensitive data classes the feature touches (credentials, personal data, internal structure) and, for each, where it is allowed to appear and where it is not. A feature whose spec never says which fields are secret produces code that guesses. See [`secure-by-default`](../skills/secure-by-default/SKILL.md) and [`sensitive-data-exposure.md`](../references/sensitive-data-exposure.md).

**Non-code deliverable (MUST when the run has the `task_required` flag):** run `vibe-agent run status --slug <slug>` and read its `flags` line. If it lists `task_required`, the objective also has something for people that is not code, and the SPEC must carry three more sections, written exactly as [`task.md`](task.md) section `spec` shows: **Deliverable** (what it is, its format, and where it is saved), **Acceptance rows** (one checkable statement each, with how it is checked), and **Outward actions** (one `OA` row for every message, post, payment, share, or write to another system, or a single `none` row). The non-code part runs after the last code task, and a person approves it row by row before anything leaves the workspace. An action that is not listed in the SPEC is not allowed.

Write the spec to `docs/<date>/<slug>/<version>/SPEC-<date>.md` at the workspace root (the directory that contains `.vibe-agent/`; the repo root when this toolkit is used standalone). `<slug>` is a short kebab-case name for the work; confirm it with the user when it is not obvious. See the "Generated docs location" rule in [`AGENTS.md`](../../AGENTS.md). Confirm the spec before coding.
</required>

## Routing & discovery

<routing>

- Use when implementation is not yet fully specified.
- Do not use when a reviewed spec already exists and execution is requested.

Invoke for new features, ambiguous requests, or major refactors.
</routing>
