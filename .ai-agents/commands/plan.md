---
description: Break spec into ordered tasks with acceptance criteria
---

Break an approved spec into ordered tasks, each carrying acceptance criteria, a verification command, and the files it likely touches.

<procedure>

Follow [`planning-and-task-breakdown`](../skills/planning-and-task-breakdown/SKILL.md).

When the plan includes diagrams, dependency flows, timelines, or architecture sketches, follow [`diagram-authoring`](../references/diagram-authoring.md).

Read the spec (user-provided path) and relevant code. Prefer read-only planning unless the user wants edits.

1. Dependency graph and vertical slices (end-to-end thin slices).
2. Tasks with acceptance criteria, verification command, files likely touched.
3. Checkpoints between phases.

**Every task gets a status marker, a description, and its own acceptance criteria as checkboxes.**
The status vocabulary and the rule that TASKS.md and `tasks.json` are written together are in
[`planning-and-task-breakdown`](../skills/planning-and-task-breakdown/SKILL.md), section
**Task status (MUST)**, which is canonical. A task missing any of the three is not a task yet: a
status-less task cannot be reported on, a description-less task gets reinterpreted by whoever picks
it up, and a criteria-less task is finished whenever somebody says so.

Any task touching auth, user data, logging, error handling, or a client surface carries a **redaction acceptance criterion** stated in observable terms ("the audit log records the user ID and not the token"), so `/build` and `/test` have something to verify rather than a reminder to be careful. See [`secure-by-default`](../skills/secure-by-default/SKILL.md).

Write outputs to `docs/<slug>/PLAN.md`, `docs/<slug>/TASKS.md`, and `docs/<slug>/tasks.json` at the workspace root (the directory that contains `.vibe-agent/`; the repo root when this toolkit is used standalone), reusing the same `<slug>` as the spec for this work. See the "Generated docs output location" rule in [`AGENTS.md`](../../AGENTS.md). Present for human review before implementation.

`tasks.json` is the same list in the shape a verifier can read, against [`schemas/tasks.schema.json`](../../schemas/tasks.schema.json). It is what answers `tasks_remaining`, so a task missing from it is a task the run will not come back for. `TASKS.md` stays the file a person reviews; `doctor` reports when the two disagree on count and does not refuse.
</procedure>

## Routing & discovery

<routing>

- Use when execution steps are needed from written requirements.
- Do not use when only high-level brainstorming is requested.

Invoke after spec approval and before implementation.
</routing>
