---
name: git-workflow-and-versioning
description: >-
  Structures Git workflow: trunk-based habits, atomic commits, branches, worktrees, bisect. Use for every change, merges, conflicts, or parallel agent streams.
disable-model-invocation: true
---

# Git Workflow and Versioning

## Stack profile for current workspace

When working **in a repository that includes this toolkit**, open [`stack-profiles/ROUTER.md`](../../stack-profiles/ROUTER.md) when present, select applicable profiles, and read those files. Product/domain expectations: root [`AGENTS.md`](../../../AGENTS.md).

Use package and language-tool commands from the current workspace scripts when illustrating verify steps.

## Overview

Git is the safety net for reversible, reviewable change — especially with AI-generated edits. Treat commits as save points, branches as short-lived sandboxes, and history as documentation.

## When to Use

Always. Every code change flows through Git.

## What

Defines safe, incremental Git practices for collaborative and AI-assisted workflows.

## Why

Protects change quality through atomic history, easier review, and reversible steps.

## How

Use the principles and workflow sections in this file as the operational guidance.

## When

Use for all code changes, merges, rebases, and parallel workstreams.

## Routing & discovery

- Use when workflow concerns center on branching, commits, conflicts, or history quality.
- Do not use as a replacement for feature-spec or implementation skills.

## Permissions & authority

- Tools: Git and shell operations within repository policy boundaries.
- Authority: follow branch protections and avoid destructive commands unless explicitly requested.

## Core Principles

### Trunk-Based Development (recommended)

Keep `main` deployable. Prefer short-lived feature branches (merge within days). Prefer **feature flags** over weeks-long branches.

### Commit Early, Commit Often

Each successful increment gets its own commit. Avoid one giant unreviewable blob.

### Atomic Commits

Each commit does one logical thing: one feat, one fix, or one refactor — not mixed formatting + behavior.

### Descriptive Messages (MUST)

Write commit messages a human can read at a glance. Rules:

- **Conventional prefix is required:** `type(scope): subject`. Types: `feat`, `fix`, `refactor`, `test`, `docs`, `chore`, `perf`, `build`, `ci`. The `(scope)` is optional but preferred (the module/area touched).
- **Human-friendly subject:** imperative, lower-case, concise; no trailing period.
- **No AI wording:** do not use filler words such as ensure, enhance, simplify, leverage, utilize, seamless, robust, comprehensive. Say what changed in plain terms.
- **No decorative characters:** no emojis, icons, or em-dash characters in the subject or body. Use a normal hyphen or comma.
- **Body explains why,** not only what. Keep it short.
- **Match the branch:** the type/scope should line up with the branch name (see Branching). A `fix/...` branch produces `fix(...)` commits.

```text
feat(auth): add email validation to registration endpoint

Reject invalid formats before persistence. Matches the request validation
already used on the other auth routes.
```

### No AI words anywhere in git artifacts

The same plain-language rule applies to branch names, tags, and PR titles/descriptions: no AI-tell filler words, no emojis or icons, no em-dash. Attribution rules are below.

### No Agent Attribution

Do **not** append AI/agent co-author trailers (`Co-Authored-By: …`) or "Generated with …" lines to commits or PR bodies. Every commit and PR is attributed only to the human contributor's git identity. For Claude Code this is enforced by the empty `attribution` block in [`.claude/settings.json`](../../../.claude/settings.json); other agents (Cursor, Codex, opencode) must honor the same rule by convention.

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
