# Stack profile: Product lifecycle and delivery

## Scope

Applies to consumer repositories that connect product discovery, specifications, delivery planning, feature flags, release readiness, launch, monitoring, feedback loops, and lifecycle decisions.

## When to load

- Writing specs, delivery plans, release plans, rollout plans, or lifecycle runbooks
- Defining product success metrics, adoption metrics, guardrails, feature flags, or feedback loops
- Planning staged rollout, launch, deprecation, migration, or end-of-life
- Connecting engineering delivery metrics with product outcomes and operational health

## Detection

- `docs/specs/`, `docs/features/`, `docs/adr/`, `roadmap`, `release`, `launch`, `runbook`
- Feature flag configs, analytics events, experimentation plans, changelogs, migration/deprecation docs
- Mentions of DORA metrics, SLIs/SLOs, adoption, activation, retention, conversion, support burden, lifecycle

## Framework and tooling

- Spec-first and task breakdown workflows from this repository
- Feature flags and staged rollouts
- Product analytics, experimentation, support feedback, and observability dashboards
- DORA-style delivery metrics plus product outcome metrics

## Repo layout conventions

- Keep product-domain decisions in the consumer repo's local `AGENTS.md`, specs, and ADRs
- Keep reusable lifecycle workflow in skills; keep repo-specific metrics/events/tooling in this profile or local docs
- Link specs to implementation plans, tests, release gates, dashboards, and rollback/deprecation criteria
- Track feature ownership, launch date, removal date for flags, and follow-up review date

## Commands

- Use repo-documented docs/test/build commands
- Typical examples: `npm run test`, `npm run build`, `pytest`, `cargo test`, link/check scripts for AI assets
- Analytics/dashboard validation commands depend on the consuming repo and should be documented locally

## Boundaries

- Do not confuse shipping a deploy with launching value to users
- Do not add feature flags without owner, default state, rollout criteria, and removal plan
- Do not optimize DORA metrics as vanity metrics without product and reliability context
- Do not encode domain-specific product strategy in shared toolkit assets

## Security / performance appendix

- Pair launch plans with privacy/security review when collecting new telemetry or user data
- Define guardrail metrics that can stop rollout
- Define operational owner, support path, rollback/disable procedure, and post-launch review

## References

- https://dora.dev/guides/dora-metrics/
- https://docs.github.com/en/actions/learn-github-actions/understanding-github-actions
- https://opentelemetry.io/docs/concepts/observability-primer/
