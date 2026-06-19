---
name: shipping-and-launch
description: >-
  Pre-launch checklist, staged rollout, monitoring, rollback for production deploys. Use before risky releases, infra migration, or exposing new LLM tool surfaces to users.
disable-model-invocation: true
---

# Shipping and Launch

## Stack profile for current workspace

Concrete deploy/stack notes for the **current workspace**: start at [`stack-profiles/ROUTER.md`](../../stack-profiles/ROUTER.md); read every profile relevant to infra/runtime. Product/domain: [`AGENTS.md`](../../../AGENTS.md).

Cross-check detailed lists (generic guides; repo addenda in stack profile): [`references/security-checklist.md`](../../references/security-checklist.md), [`references/performance-checklist.md`](../../references/performance-checklist.md), [`references/accessibility-checklist.md`](../../references/accessibility-checklist.md).

## Overview

Ship safely: reversible deploys, observable behavior, incremental exposure — not only “it worked locally.”

## When to Use

- First production deploy or high-risk change.
- Data or infra migration.
- Enabling new AI/tool paths for users.

## Pre-Launch Checklist (summary)

### Code quality

- [ ] Tests pass (frontend + backend per CI).
- [ ] Lint and typecheck pass.
- [ ] Review complete; no stray debug logs in hot paths.
- [ ] **No AI/agent attribution** in release/tag/merge commits or PR body — no `Co-Authored-By: …` agent trailers or "Generated with …" lines ([`git-workflow-and-versioning`](../git-workflow-and-versioning/SKILL.md)).

### Security

- [ ] No secrets in repo; production env vars set.
- [ ] `npm audit` / Python audit pipeline clean per policy.
- [ ] AuthZ on protected routes; database queries never merge raw user objects into filters.
- [ ] LLM/tool endpoints bounded (timeouts, allowlists, rate limits).

### Performance

- [ ] Critical paths avoid N+1 persistence patterns; indexes or query plans for hot paths.
- [ ] Bundle and API latency within agreed budgets.

### Accessibility

- [ ] Keyboard flows and labels for new UI ([`references/accessibility-checklist.md`](../../references/accessibility-checklist.md)).

### Infrastructure

- [ ] Health check responds; logs and error reporting wired.
- [ ] TLS/DNS/CDN as designed.

### Documentation

- [ ] README/runbook updates; ADRs for architectural choices; changelog if user-facing.

## Feature Flags

Prefer flags to decouple **deploy** from **release**. Define owner and removal deadline; test both ON and OFF in CI when feasible.

## Staged Rollout

Typical sequence: staging validation → production deploy with flag OFF → internal → canary % → expand → full → remove flag after soak.

Use error rate and latency thresholds; roll back on regression (see [`references/orchestration-patterns.md`](../../references/orchestration-patterns.md) for team coordination, not automated persona chains).

## Monitoring

Track API errors, p95 latency, pool usage, LLM/tool failure rates, and client Web Vitals as applicable.

## Rollback Plan

Document: triggers (error rate, latency), steps (disable flag vs redeploy previous artifact), DB considerations (forward-only migrations policy), target time-to-recover.

## Verification

Before deploy:

- [ ] Checklists above addressed for this release.
- [ ] Rollback path documented; on-call aware.

After deploy:

- [ ] Health OK; error and latency dashboards nominal; smoke critical flows.

## What

Defines a release-readiness and launch workflow with risk-aware checks and explicit rollout discipline.

## Why

Reduces avoidable release risk by making verification, rollback readiness, and decision criteria explicit.

## How

Follow the existing instructions in this file as the execution flow; this section maps them to template headings without changing behavior.

## When

Use for deployment planning, go/no-go decisions, and release hardening.

## Routing & discovery

- Use when delivery scope includes rollout, launch, or release risk.
- Do not use when work is still in early ideation/spec stages without ship scope.

## Permissions & authority

- Tools: may require read/edit/test/build/deploy tools depending on project pipeline.
- Authority: inherit session/tooling permissions; do not bypass environment safeguards or policy gates.
