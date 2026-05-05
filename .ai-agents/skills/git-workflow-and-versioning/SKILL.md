---
name: git-workflow-and-versioning
description: >-
  Structures Git workflow: trunk-based habits, atomic commits, branches, worktrees, bisect. Use for every change, merges, conflicts, or parallel agent streams.
disable-model-invocation: true
---

# Git Workflow and Versioning

## Stack profile for this repository

When working **in this monorepo**, open [`stack-profiles/ROUTER.md`](../../stack-profiles/ROUTER.md), select applicable profiles, and read those files. Product/domain expectations: root [`AGENTS.md`](../../../AGENTS.md).

Use package and language-tool commands from this repo’s scripts when illustrating verify steps.

## Overview

Git is the safety net for reversible, reviewable change — especially with AI-generated edits. Treat commits as save points, branches as short-lived sandboxes, and history as documentation.

## When to Use

Always. Every code change flows through Git.

## Core Principles

### Trunk-Based Development (recommended)

Keep `main` deployable. Prefer short-lived feature branches (merge within days). Prefer **feature flags** over weeks-long branches.

### Commit Early, Commit Often

Each successful increment gets its own commit. Avoid one giant unreviewable blob.

### Atomic Commits

Each commit does one logical thing: one feat, one fix, or one refactor — not mixed formatting + behavior.

### Descriptive Messages

Explain **why**, not only **what**:

```text
feat: add email validation to registration endpoint

Prevents invalid formats before persistence; aligns with request validation on auth routes.
```

Suggested types: `feat`, `fix`, `refactor`, `test`, `docs`, `chore`.

### Keep Concerns Separate

Do not combine large formatting-only edits with behavior changes in the same commit when it obscures review.

### Size Your Changes

Target small PRs (~100–300 lines per logical change when possible). Split large work across commits or stacked PRs; see [`code-review-and-quality`](../code-review-and-quality/SKILL.md).

## Branching

- Branch from the team default (`main`).
- Naming: `feature/…`, `fix/…`, `chore/…`, `refactor/…`.
- Delete merged branches.

## Worktrees (parallel work)

```bash
git worktree add ../my-feature-worktree feature/task-creation
# merge and remove when done
git worktree remove ../my-feature-worktree
```

Isolates experiments without switching branches in one working tree.

## Save Point Pattern

After each increment: tests green → commit → continue. If direction fails: reset to last good commit.

## Change Summaries (for agents)

After substantive edits, summarize:

```text
CHANGES MADE:
- …

DID NOT TOUCH (intentionally):
- …

POTENTIAL CONCERNS:
- …
```

## Pre-Commit Hygiene

Adapt to repo scripts, for example:

```bash
git diff --staged
# secrets scan (adapt for Windows/Linux)
npm test / npm run lint / npx tsc --noEmit   # frontend as configured
uv run pytest / uv run ruff check .          # backend as configured
```

Automate with husky/lint-staged or CI; align with project conventions.

## Generated Files

Commit lockfiles and migrations when policy requires. Do **not** commit `.env`, `.next/`, `dist/`, or secrets.

## Git for Debugging

```bash
git bisect start
git log --oneline -20
git blame path/to/file
```

## Verification

- [ ] Commit is one logical unit; message typed and explains intent
- [ ] Tests/lint/typecheck per project norms pass
- [ ] No secrets in the diff; `.gitignore` respected
