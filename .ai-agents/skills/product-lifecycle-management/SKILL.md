---
name: product-lifecycle-management
description: >-
  Coordinates the product lifecycle end to end (discovery, specs, delivery, launch, feedback, deprecation), owning lifecycle metrics, guardrails, feature-flag governance, and end-of-life decisions while delegating idea shaping to idea-refine and launch/rollback mechanics to shipping-and-launch. Use when planning lifecycle stages, defining success/guardrail metrics, governing feature flags, running post-launch review, or retiring features and products.
disable-model-invocation: true
---

# Product Lifecycle Management

## How

<procedure>

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
   - Delegate rollout/rollback mechanics to [`shipping-and-launch`](../shipping-and-launch/SKILL.md); own the lifecycle-level go/no-go criteria, owner, and post-launch review.
   - Confirm staged exposure (internal → canary → ramp → full) and rollback/disable criteria are defined before exposure.
6. **Close the loop**
   - Review metrics and qualitative feedback.
   - Decide iterate, scale, maintain, migrate, deprecate, or remove.
   - Remove stale flags and update docs/runbooks.
</procedure>

## Routing & discovery

<routing>

- This skill **coordinates** the lifecycle and owns metrics, guardrails, flag governance, and deprecation; it does not replace the focused skills it delegates to.
- Delegate early idea shaping and option framing to [`idea-refine`](../idea-refine/SKILL.md).
- Delegate launch execution, staged rollout, and rollback mechanics to [`shipping-and-launch`](../shipping-and-launch/SKILL.md).
- Pair with [`spec-driven-development`](../spec-driven-development/SKILL.md) for written specs and [`planning-and-task-breakdown`](../planning-and-task-breakdown/SKILL.md) for execution plans.
- Pair with [`observability-monitoring`](../observability-monitoring/SKILL.md) for feedback and post-launch signals.

Use for lifecycle planning, rollout strategy, success metrics, product operations, and feature retirement. Do not use as a substitute for domain-specific product strategy in consuming repos.
</routing>

## Permissions & authority

<required>

- Tools: Read, Grep, Glob, Edit; Shell only for documented validation.
- Paths: docs, specs, ADRs, analytics/observability configs, release notes.
- Do not add telemetry collecting new user data without privacy/security review.
</required>

## Verification

<verification>

- [ ] Lifecycle stage, owner, users, and decision criteria are explicit.
- [ ] Success metrics and guardrails are measurable.
- [ ] Rollout, rollback/disable, and post-launch review are defined.
- [ ] Feature flags have owner, default, ramp criteria, and removal date.
- [ ] Docs/specs/runbooks link to dashboards or feedback mechanisms.
</verification>

## References

<references>

- https://dora.dev/guides/dora-metrics/
- https://opentelemetry.io/docs/concepts/observability-primer/
- https://docs.github.com/en/actions/learn-github-actions/understanding-github-actions
</references>
