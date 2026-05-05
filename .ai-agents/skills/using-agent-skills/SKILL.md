---
name: using-agent-skills
description: >-
  Meta-skill: discover skills via ROUTER tables, compose workflows, and apply core behaviors (assumptions, verification). Use at session start or when choosing workflows and slash commands for this codebase.
disable-model-invocation: true
---

# Using Agent Skills

## Canonical locations and stack profile

Canonical AI assets live under [`.ai-agents/`](../../README.md). Read [`ROUTER.md`](../../ROUTER.md) first, then the subfolder **`ROUTER.md`** for skills, agents, commands, or hooks.

Skills stay **stack-agnostic** by default; for pinned frameworks **here**, open [`stack-profiles/ROUTER.md`](../../stack-profiles/ROUTER.md), select every matching profile row, and read those files. Product/domain charter: [`AGENTS.md`](../../../AGENTS.md).

## Overview

Skills encode **how** to execute a workflow. Personas (`agents/`) encode **who** (single perspective). Commands (`commands/`) encode **when** (repeatable entrypoints). The **user or command** orchestrates — personas do not call personas ([`references/orchestration-patterns.md`](../../references/orchestration-patterns.md)).

## Discovery

1. Open [`.ai-agents/ROUTER.md`](../../ROUTER.md).
2. Open the relevant [`skills/ROUTER.md`](../ROUTER.md), [`agents/ROUTER.md`](../../agents/ROUTER.md), or [`commands/ROUTER.md`](../../commands/ROUTER.md).
3. Pick the skill or persona whose **description** matches the task.

### Phase → skill hints

Not every task needs every skill. Common mappings:

| Situation | Start here |
|-----------|------------|
| Vague idea | [`idea-refine`](../idea-refine/SKILL.md) |
| No written requirements | [`spec-driven-development`](../spec-driven-development/SKILL.md) |
| Spec → tasks | [`planning-and-task-breakdown`](../planning-and-task-breakdown/SKILL.md) |
| Right files in context | [`context-engineering`](../context-engineering/SKILL.md) |
| Doc-accurate APIs | [`source-driven-development`](../source-driven-development/SKILL.md) |
| UI work | [`frontend-ui-engineering`](../frontend-ui-engineering/SKILL.md) |
| HTTP/schemas | [`api-and-interface-design`](../api-and-interface-design/SKILL.md) |
| Backend modules / clean layers | [`backend-engineering`](../backend-engineering/SKILL.md) |
| Tests | [`test-driven-development`](../test-driven-development/SKILL.md) |
| Browser/runtime debug | [`browser-testing-with-devtools`](../browser-testing-with-devtools/SKILL.md) |
| Incidents | [`debugging-and-error-recovery`](../debugging-and-error-recovery/SKILL.md) |
| Pre-merge | [`code-review-and-quality`](../code-review-and-quality/SKILL.md) |
| Security pass | [`security-and-hardening`](../security-and-hardening/SKILL.md) |
| Perf pass | [`performance-optimization`](../performance-optimization/SKILL.md) |
| Git hygiene | [`git-workflow-and-versioning`](../git-workflow-and-versioning/SKILL.md) |
| Docs/ADRs | [`documentation-and-adrs`](../documentation-and-adrs/SKILL.md) |
| Deploy | [`shipping-and-launch`](../shipping-and-launch/SKILL.md) |
| Simplify diff | [`code-simplification`](../code-simplification/SKILL.md) |

**Parallel ship-style review:** use command [`ship`](../../commands/ship.md) to compose reviewer + auditor + test-engineer reports — see [`orchestration-patterns.md`](../../references/orchestration-patterns.md).

## Core Behaviors (always)

1. **Surface assumptions** before non-trivial work.
2. **Stop on confusion** — name conflicts; ask instead of guessing.
3. **Push back** with concrete downsides when an approach is risky.
4. **Prefer simplicity** — fewer lines when clarity allows.
5. **Scope discipline** — touch only what the task needs.
6. **Verify** — tests, build, or runtime evidence; not “looks fine.”

## Skill Rules

1. Check [`skills/ROUTER.md`](../ROUTER.md) for an applicable skill before inventing ad hoc process.
2. Follow the skill’s steps including **Verification** sections.
3. Multiple skills may apply sequentially; avoid skipping spec/plan when requirements are unclear.

## Typical Feature Sequence (example)

```text
idea-refine → spec-driven-development → planning-and-task-breakdown
→ context-engineering → source-driven-development → test-driven-development
→ code-review-and-quality → git-workflow-and-versioning → documentation-and-adrs → shipping-and-launch
```

Bugfix might be: `debugging-and-error-recovery` → `test-driven-development` → `code-review-and-quality` only.

## Verification

- [ ] Correct `ROUTER.md` consulted for the asset type
- [ ] Chosen skill matches triggers in its YAML `description`
- [ ] Orchestration patterns respected ([`references/orchestration-patterns.md`](../../references/orchestration-patterns.md))
