---
name: planning-and-task-breakdown
description: >-
  Breaks initiatives into vertical slices across frontend features and backend modules with clear dependencies. Use before large implementations or multi-PR work.
disable-model-invocation: true
---

# Planning and Task Breakdown

## Overview

<context>

Decompose work into small, verifiable tasks with explicit acceptance criteria. Good task breakdown is the difference between an agent that completes work reliably and one that produces a tangled mess. Every task should be small enough to implement, test, and verify in a single focused session.
</context>

## When to Use

<routing>

- You have a spec and need to break it into implementable units
- A task feels too large or vague to start
- Work needs to be parallelized across multiple agents or sessions
- You need to communicate scope to a human
- The implementation order isn't obvious

**When NOT to use:** Single-file changes with obvious scope, or when the spec already contains well-defined tasks.
</routing>

## The Planning Process

<procedure>

### Step 1: Enter Plan Mode

Before writing any code, operate in read-only mode:

- Read the spec and relevant codebase sections
- Identify existing patterns and conventions
- Map dependencies between components
- Note risks and unknowns

**Do NOT write code during planning.** The output is a plan document, not implementation.

### Step 2: Identify the Dependency Graph

Map what depends on what:

```
Database schema
    │
    ├── API models/types
    │       │
    │       ├── API endpoints
    │       │       │
    │       │       └── Frontend API client
    │       │               │
    │       │               └── UI components
    │       │
    │       └── Validation logic
    │
    └── Seed data / migrations
```

Implementation order follows the dependency graph bottom-up: build foundations first.

### Step 3: Slice Vertically

Instead of building all the database, then all the API, then all the UI - build one complete feature path at a time:

**Bad (horizontal slicing):**
```
Task 1: Build entire database schema
Task 2: Build all API endpoints
Task 3: Build all UI components
Task 4: Connect everything
```

**Good (vertical slicing):**
```
Task 1: User can create an account (schema + API + UI for registration)
Task 2: User can log in (auth schema + API + UI for login)
Task 3: User can create a task (task schema + API + UI for creation)
Task 4: User can view task list (query + API + UI for list view)
```

Each vertical slice delivers working, testable functionality.

### Step 4: Write Tasks

Every task carries a status, a description, and its own acceptance criteria. A task missing any of
the three is not a task yet: a status-less task cannot be reported on, a description-less task gets
reinterpreted by whoever picks it up, and a criteria-less task is finished whenever somebody says
so.

Each task follows this structure:

```markdown
### T[N]: [Short descriptive title]  [queued]

**Description:** One paragraph explaining what this task accomplishes.

**Acceptance criteria:**
- [ ] [Specific, testable condition]
- [ ] [Specific, testable condition]

**Verification:**
- [ ] Tests pass: `npm test -- --grep "feature-name"`
- [ ] Build succeeds: `npm run build`
- [ ] Manual check: [description of what to verify]

**Dependencies:** [Task numbers this depends on, or "None"]

**Files likely touched:**
- `src/path/to/file.ts`
- `tests/path/to/test.ts`

**Estimated scope:** [Small: 1-2 files | Medium: 3-5 files | Large: 5+ files]

**Delivery branch (for `/build`):** `feature/<slug>-task-<N>-<short-title>` (or `fix/…` / `chore/…` / `refactor/…`). One **planned** task maps to one branch and one PR. User feedback and fixes for **that same task** (logic, UI, tests, review blockers) add commits on the same branch; a **different** planned task or unrelated scope needs a new branch ([`build.md`](../../commands/build.md)).
```

### Step 5: Order and Checkpoint

Arrange tasks so that:

1. Dependencies are satisfied (build foundation first)
2. Each task leaves the system in a working state
3. Verification checkpoints occur after every 2-3 tasks
4. High-risk tasks are early (fail fast)

Add explicit checkpoints:

```markdown
## Checkpoint: After Tasks 1-3
- [ ] All tests pass
- [ ] Application builds without errors
- [ ] Core user flow works end-to-end
- [ ] Review with human before proceeding
```
</procedure>

## Task status (MUST)

<rules>

**This section is canonical.** [`plan.md`](../../commands/plan.md), [`build.md`](../../commands/build.md),
[`goal.md`](../../commands/goal.md), and [`auto.md`](../../commands/auto.md) point here rather than
restating it, so the rule lives in one place instead of five that drift.

### One list, two files, one vocabulary

A task list is written twice, and both copies are part of the same edit:

| File | Read by | Status lives in |
|---|---|---|
| `docs/<slug>/TASKS.md` | a person | the task heading: `### T1: Title  [queued]` |
| `docs/<slug>/tasks.json` | the runtime | the `status` field |

The vocabulary is the one [`schemas/tasks.schema.json`](../../../schemas/tasks.schema.json) already
defines, lowercase, in both files:

`queued` · `in_progress` · `blocked` · `done` · `canceled`

Lowercase and identical on both sides on purpose. A heading that says `[DONE]` beside a field that
says `done` is two spellings of one fact, which is how a reader ends up trusting the wrong one.

### Check the list before starting

Read `tasks.json` before picking up work, every time:

- A task already `done` is **not** restarted. If it needs more work, that is a new task or a fix on
  its existing branch, not a status reversal.
- A task `blocked` needs its blocker named and resolved, or the task re-planned. Starting it anyway
  produces work that cannot land.
- Take the first `queued` task whose dependencies are all `done`.

### Update the status when it changes, before the verifier reads it

Set `in_progress` when work starts and `done` when the acceptance criteria pass, in **both** files,
and tick the acceptance checkboxes as they are met.

Timing is load-bearing, not tidiness. The `task_complete` verifier reads `tasks.json` to decide
whether another task remains. Marking a task `done` **after** that verifier has run means the graph
saw a task that was already finished and sent the run back through a full build cycle for nothing.
That has happened, and it costs a complete loop each time.

A `canceled` task carries a note saying why. So does a `blocked` one. "The count reflects what is
actually outstanding" is the whole reason the field exists.
</rules>

## Task Sizing Guidelines

<rules>

| Size | Files | Scope | Example |
|------|-------|-------|---------|
| **XS** | 1 | Single function or config change | Add a validation rule |
| **S** | 1-2 | One component or endpoint | Add a new API endpoint |
| **M** | 3-5 | One feature slice | User registration flow |
| **L** | 5-8 | Multi-component feature | Search with filtering and pagination |
| **XL** | 8+ | **Too large - break it down further** | - |

If a task is L or larger, it should be broken into smaller tasks. An agent performs best on S and M tasks.

**When to break a task down further:**
- It would take more than one focused session (roughly 2+ hours of agent work)
- You cannot describe the acceptance criteria in 3 or fewer bullet points
- It touches two or more independent subsystems (e.g., auth and billing)
- You find yourself writing "and" in the task title (a sign it is two tasks)
</rules>

## Plan Document Template

<context>

**Output location:** Write the plan to `docs/<slug>/PLAN.md` and the task list to `docs/<slug>/TASKS.md` at the workspace root (the directory containing `.vibe-agent/`, or the repo root when this toolkit is standalone), reusing the same `<slug>` as the spec. See the "Generated docs output location" rule in [`AGENTS.md`](../../../AGENTS.md).

```markdown
# Implementation Plan: [Feature/Project Name]

## Overview
[One paragraph summary of what we're building]

## Architecture Decisions
- [Key decision 1 and rationale]
- [Key decision 2 and rationale]

## Task List

The plan lists the tasks; TASKS.md carries each one in full. Tick a box here when that task's
status reaches `done` in both TASKS.md and tasks.json.

### Phase 1: Foundation
- [ ] T1: ...
- [ ] T2: ...

### Checkpoint: Foundation
- [ ] Tests pass, builds clean

### Phase 2: Core Features
- [ ] T3: ...
- [ ] T4: ...

### Checkpoint: Core Features
- [ ] End-to-end flow works

### Phase 3: Polish
- [ ] T5: ...
- [ ] T6: ...

### Checkpoint: Complete
- [ ] All acceptance criteria met
- [ ] Ready for review

## Risks and Mitigations
| Risk | Impact | Mitigation |
|------|--------|------------|
| [Risk] | [High/Med/Low] | [Strategy] |

## Open Questions
- [Question needing human input]
```
</context>

## Parallelization Opportunities

<rules>

When multiple agents or sessions are available:

- **Safe to parallelize:** Independent feature slices, tests for already-implemented features, documentation
- **Must be sequential:** Database migrations, shared state changes, dependency chains
- **Needs coordination:** Features that share an API contract (define the contract first, then parallelize)
</rules>

## Common Rationalizations

<antipatterns>

| Rationalization | Reality |
|---|---|
| "I'll figure it out as I go" | That's how you end up with a tangled mess and rework. 10 minutes of planning saves hours. |
| "The tasks are obvious" | Write them down anyway. Explicit tasks surface hidden dependencies and forgotten edge cases. |
| "Planning is overhead" | Planning is the task. Implementation without a plan is just typing. |
| "I can hold it all in my head" | Context windows are finite. Written plans survive session boundaries and compaction. |

## Red Flags

- Starting implementation without a written task list
- Tasks that say "implement the feature" without acceptance criteria
- A task with no status marker, or a status that differs between TASKS.md and tasks.json
- Marking a task done after the verifier that reads the list has already run
- No verification steps in the plan
- All tasks are XL-sized
- No checkpoints between tasks
- Dependency order isn't considered
</antipatterns>

## Verification

<verification>

Before starting implementation, confirm:

- [ ] Every task has a status marker, a description, and its own acceptance criteria
- [ ] Every task's status uses the schema vocabulary, lowercase, in TASKS.md and tasks.json alike
- [ ] Every task has a verification step
- [ ] Task dependencies are identified and ordered correctly
- [ ] No task touches more than ~5 files
- [ ] Checkpoints exist between major phases
- [ ] Plan and tasks written to `docs/<slug>/` at the workspace root (sibling of `.vibe-agent/`)
- [ ] The human has reviewed and approved the plan
</verification>

## Related references

<references>

- [`orchestration-patterns.md`](../../references/orchestration-patterns.md) (sequential pipelines vs parallel fan-out)
</references>
