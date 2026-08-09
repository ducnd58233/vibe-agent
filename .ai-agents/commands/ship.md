---
description: Pre-ship parallel review - specialist fan-out, then GO/NO-GO
---

**Runtime required (MUST).** GO is recorded as the `ship` check, which [`vibe-checks.yaml`](../../vibe-checks.yaml) declares `verifier: human` because no exit code means GO. Merge stays refused by `pre-tool-use` until `merge_approved` is recorded. Rules: [`goal.md`](goal.md) section "Runtime is required".

Follow [`shipping-and-launch`](../skills/shipping-and-launch/SKILL.md) and [`references/orchestration-patterns.md`](../references/orchestration-patterns.md).

`/ship` fans out selected personas on the same change, then merges in the main session.

## Phase A - Parallel fan-out

Spawn the baseline three subagents in **one turn** when using Claude Code's Agent tool (`subagent_type` matches YAML `name`):

1. **`code-reviewer`** - Five-axis review; template in [`agents/code-reviewer.md`](../agents/code-reviewer.md); grounded in [`code-review-and-quality`](../skills/code-review-and-quality/SKILL.md).
2. **`security-auditor`** - [`agents/security-auditor.md`](../agents/security-auditor.md) + [`security-and-hardening`](../skills/security-and-hardening/SKILL.md).
3. **`test-engineer`** - [`agents/test-engineer.md`](../agents/test-engineer.md) + [`test-driven-development`](../skills/test-driven-development/SKILL.md).

Add conditional specialists when the change touches their risk area:

4. **`devops-sre-auditor`** - CI/CD, infrastructure, deploy scripts, observability, alerts, runtime config, or rollout mechanics.
5. **`database-query-auditor`** - SQL/NoSQL schemas, migrations, indexes, query code, cache/datastore behavior, or database performance.
6. **`qa-tester`** - manual QA, release signoff, exploratory testing, cross-browser/mobile coverage, or E2E automation strategy.
7. **`product-design-reviewer`** - Figma/Canva handoff, design-system changes, visual QA, tokens, accessibility, or UI fidelity.

Without Agent tool: run the selected perspectives sequentially but merge as below.

Personas **must not** delegate to each other.

## Phase B - Merge

Synthesize: quality blockers, security blockers, coverage gaps, design-system/visual QA risks, database/query risks, QA signoff gaps, accessibility ([`references/accessibility-checklist.md`](../references/accessibility-checklist.md) if UI changed), infra/env/feature flags, deploy/rollback/observability readiness.

### UI evidence gate (MUST, when the change touches UI)

Treat these as **blockers**, not observations — they are deterministic, so a missing result is a NO-GO input rather than a judgment call:

- `ui-slop-guard` and `design-token-guard` are clean, or each exception is marked inline with a stated reason.
- An automated accessibility audit reports **zero** WCAG failures, or every remaining failure is explained and explicitly accepted.
- Render evidence exists at the supported breakpoints.

Where a check could not run, it must be reported as `UNVERIFIED: <reason>` — never as a pass. Run [`/design`](design.md) in audit mode to produce the missing evidence before re-running `/ship`.

## Phase C - Decision

Emit **Ship Decision: GO | NO-GO** with blockers, recommended fixes, acknowledged risks, **rollback plan**, and appended specialist reports.

### Merge gate (MUST)

`/ship` is the **only** command in the delivery pipeline that may **authorize** merging a task branch into `main` (or the team default branch).

- **GO** means the change is fit to merge from a quality, security, and test perspective. It does **not** mean auto-merge.
- **MUST NOT** merge, push to `main`, or complete a PR merge unless **both** are true: (1) **Ship Decision: GO**, and (2) the human explicitly asks to merge (or to open/update a PR and merge when checks pass).
- **NO-GO** means do not merge. Stay on the **same task branch**; fix blockers (logic, UI, tests, security) on that branch and re-run `/ship`. Only unrelated new work needs a new branch.
- [`build.md`](build.md) must never merge to `main`. If `/build` was run on `main`, treat that as a workflow violation: move work to a task branch before ship.

**Skip fan-out** only if the change is trivial: <=2 files, ~<=50 lines, and no auth/payments/data/config - otherwise default to full `/ship`.

## What

Run pre-ship parallel specialist review and produce a GO/NO-GO decision.

## Why

Improves release safety through independent quality, security, testing, data, QA, and operational perspectives.

## How

Use the existing Phase A/B/C orchestration and merge flow.

## When

Invoke before risky merges/releases or when blast radius is non-trivial.

## Routing & discovery

- Use when ship-readiness is the core question.
- Do not use for trivial changes that do not warrant fan-out review.
