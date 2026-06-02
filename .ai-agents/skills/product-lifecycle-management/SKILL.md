---
name: product-lifecycle-management
description: >-
  Connects product discovery, specs, delivery plans, feature flags, launch readiness, observability, feedback loops, deprecation, and lifecycle metrics. Use when planning product lifecycle, defining success/guardrail metrics, rollout strategy, release gates, post-launch review, or end-of-life for features and products.
disable-model-invocation: true
---

# Product Lifecycle Management

## What

Guide a product or feature from idea to spec, build, launch, observe, iterate, deprecate, or retire.

## Why

Shipping code is not the same as delivering product value. Lifecycle discipline ties implementation to user outcomes, operational safety, feedback, and explicit end states.

## How

1. **Load stack context**
   - Read root `AGENTS.md`, relevant specs/ADRs/plans, and [`product-lifecycle-delivery.md`](../../stack-profiles/product-lifecycle-delivery.md).
2. **Clarify lifecycle stage**
   - Discovery, spec, build, beta, launch, growth, maintenance, migration, deprecation, or retirement.
3. **Define success and guardrails**
   - Product metrics: activation, adoption, retention, conversion, task success, support load, revenue/cost where applicable.
   - Guardrails: error rate, latency, accessibility, privacy, abuse, support burden, rollback thresholds.
4. **Connect plan to delivery**
   - Link spec, tasks, tests, feature flags, release gates, dashboards, owner, and timeline.
   - Define “done” for both engineering and user outcome.
5. **Launch deliberately**
   - Use staged rollout: internal, beta/canary, percentage ramp, full launch, post-launch review.
   - Define rollback/disable criteria before exposure.
6. **Close the loop**
   - Review metrics and qualitative feedback.
   - Decide iterate, scale, maintain, migrate, deprecate, or remove.
   - Remove stale flags and update docs/runbooks.

## When

Use for lifecycle planning, rollout strategy, success metrics, product operations, and feature retirement. Do not use as a substitute for domain-specific product strategy in consuming repos.

## Routing & discovery

- Pair with [`spec-driven-development`](../spec-driven-development/SKILL.md) for written specs.
- Pair with [`planning-and-task-breakdown`](../planning-and-task-breakdown/SKILL.md) for execution plans.
- Pair with [`shipping-and-launch`](../shipping-and-launch/SKILL.md) and [`observability-monitoring`](../observability-monitoring/SKILL.md) for release and feedback loops.

## Permissions & authority

- Tools: Read, Grep, Glob, Edit; Shell only for documented validation.
- Paths: docs, specs, ADRs, analytics/observability configs, release notes.
- Do not add telemetry collecting new user data without privacy/security review.

## Verification

- [ ] Lifecycle stage, owner, users, and decision criteria are explicit.
- [ ] Success metrics and guardrails are measurable.
- [ ] Rollout, rollback/disable, and post-launch review are defined.
- [ ] Feature flags have owner, default, ramp criteria, and removal date.
- [ ] Docs/specs/runbooks link to dashboards or feedback mechanisms.

## References

- https://dora.dev/guides/dora-metrics/
- https://opentelemetry.io/docs/concepts/observability-primer/
- https://docs.github.com/en/actions/learn-github-actions/understanding-github-actions
