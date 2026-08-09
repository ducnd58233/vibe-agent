---
description: Implement next task on a dedicated branch; test, build, verify, commit; never merge to main
---

**Runtime required (MUST).** Run `vibe-agent doctor` first; if it fails, stop and report the install commands. Rules, command surface, and why: [`goal.md`](goal.md) section "Runtime is required". Do not restate them here.

Combine [`planning-and-task-breakdown`](../skills/planning-and-task-breakdown/SKILL.md) (task order, acceptance) with [`test-driven-development`](../skills/test-driven-development/SKILL.md) and [`git-workflow-and-versioning`](../skills/git-workflow-and-versioning/SKILL.md).

When a task creates or updates docs with diagrams or flows, follow [`diagram-authoring`](../references/diagram-authoring.md).

This repo does **not** ship a separate `incremental-implementation` skill; use explicit vertical slices and TDD.

## Git rules (MUST follow before and during every task)

`/build` implements work on **task branches only**. Merging to `main` (or the team default branch) is **out of scope** for `/build`.

### Same branch/PR vs new branch/PR

| Stay on the **existing** task branch and PR | **MUST** open a **new** branch and PR |
|---------------------------------------------|---------------------------------------|
| User feedback on the current task (fix logic, fix UI, copy, a11y, tests for that slice) | A **different** task from `TASKS.md` or plan |
| Review or `/ship` blockers for **this** PR (address findings, re-run ship) | New scope, feature, or unrelated bug |
| Iteration until the **same** acceptance criteria pass | Work that does not trace to the current task or open PR |

When in doubt: if the change would belong in the **same PR description** as the current task, use the same branch. If it needs its own task line or its own PR summary, create a new branch.

1. **One planned task = one branch = one PR** (follow-up fixes for that task stay on that branch). Do not put **multiple unrelated tasks** from `TASKS.md` on one branch or in one pull request.
2. **Never implement on `main` / `master`.** If the working tree is on the default branch, create and check out a task branch first (or check out the existing branch for the task you are continuing).
3. **Branch naming:** `feature/<slug>-task-<n>-<short-title>`, `fix/...`, `chore/...`, or `refactor/...` aligned with the task and commit type. Branch from an up-to-date default branch for **new** tasks only.
4. **MUST NOT** during `/build`: merge or rebase into `main`, fast-forward `main`, push to `main`, bundle unrelated tasks in one PR, or treat `/build` as complete because code landed on `main`.
5. **After the task passes verification:** commit on the task branch (same branch for same-task feedback), push if the human uses a remote, and stop. Open or update **one** PR for that task when the human asks. Do not merge.
6. **Next unrelated task:** check out the default branch, pull if appropriate, create a **new** branch. Do not stack a new task onto a branch that already serves a different task.

Merge to `main` happens only after [`ship.md`](ship.md) returns **Ship Decision: GO** and the human agrees to merge (see ship command). `/review` and `/test` may run on the task branch before `/ship`.

## Per-task loop

For **one** task only (then stop or ask before starting the next task on a new branch):

1. Read acceptance criteria from `docs/<slug>/TASKS.md` (or the path the human gave).
2. Create or confirm the dedicated task branch (rules above).
3. Load context ([`context-engineering`](../skills/context-engineering/SKILL.md) as needed).
4. RED — failing test for new behavior.
5. GREEN — minimal implementation.
6. Run full tests and typecheck/build per project (`npm`/`pnpm`/`uv` as documented).
7. **Disclosure pass (MUST, before commit):** apply [`secure-by-default`](../skills/secure-by-default/SKILL.md) to the diff. For every sink the task added or changed (log call, response body, client storage, analytics event, error path, env var), name what goes into it. A clean [`sensitive-data-guard`](../hooks/sensitive-data-guard.py) run is a floor, not evidence. Channel detail: [`sensitive-data-exposure.md`](../references/sensitive-data-exposure.md).
8. Commit with a human-friendly conventional message, `type(scope): subject`, that matches the branch. Use plain words, no AI-tell filler, no emojis/icons, no em-dash. **MUST NOT** add AI/agent co-author trailers (`Co-Authored-By: ...`) or "Generated with ..." lines; attribute commits solely to the human's git identity.
9. Mark the task complete in `TASKS.md` if that file is in scope; report branch name and PR link if created. **Do not merge to `main`.**

On failure: [`debugging-and-error-recovery`](../skills/debugging-and-error-recovery/SKILL.md).

## What

Implement the next planned task incrementally with TDD guardrails.

## Why

Maintains delivery momentum while minimizing regressions.

## How

Use the RED/GREEN + verify + commit loop defined above.

## When

Invoke when a task plan exists and coding should begin.

## Routing & discovery

- Use when moving from approved plan to implementation.
- Do not use when requirements/spec remain unresolved.
