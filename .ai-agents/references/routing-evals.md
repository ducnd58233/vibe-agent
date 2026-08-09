# Routing evals (intent → expected asset)

Lightweight, deterministic fixtures for the toolkit's routing layer. Each row pairs a
representative **user intent** with the asset the router should select.

`/doctor` (via `scripts/check-ai-agents-routers.sh`) asserts that every **Expected asset**
link below resolves to a real file, so a renamed or deleted target fails fast. The
intent→asset mapping is also a human-checkable regression list: after changing routers or
skill scopes, confirm these still hold.

This is **not** a model-graded eval — it verifies the routing target exists and stays
named as documented. For model/skill behavioral evaluation patterns see
[`agent-evaluation-patterns.md`](agent-evaluation-patterns.md).

## Fixtures

<references>

| User intent | Expected family | Expected asset |
|-------------|-----------------|----------------|
| Write a failing test first, then implement | skill | [`test-driven-development`](../skills/test-driven-development/SKILL.md) |
| Review a diff across five axes before merge | command | [`review.md`](../commands/review.md) |
| Research a topic with verifiable citations | skill | [`research-with-citations`](../skills/research-with-citations/SKILL.md) |
| Choose between options from gathered evidence | skill | [`evidence-based-analysis`](../skills/evidence-based-analysis/SKILL.md) |
| Ground version-sensitive framework APIs in official docs | skill | [`source-driven-development`](../skills/source-driven-development/SKILL.md) |
| Curate which files/retrieval load into the window | skill | [`context-engineering`](../skills/context-engineering/SKILL.md) |
| Keep output concise in a high-volume loop | skill | [`token-efficient-execution`](../skills/token-efficient-execution/SKILL.md) |
| Design CI/CD, IaC, deploy gates, rollback | skill | [`devops-platform-delivery`](../skills/devops-platform-delivery/SKILL.md) |
| Add OpenTelemetry traces, metrics, SLOs, alerts | skill | [`observability-monitoring`](../skills/observability-monitoring/SKILL.md) |
| Train/fine-tune and evaluate a model | skill | [`ai-model-engineering`](../skills/ai-model-engineering/SKILL.md) |
| Build ML pipelines, registry, drift monitoring | skill | [`mlops-lifecycle`](../skills/mlops-lifecycle/SKILL.md) |
| Plan lifecycle metrics, feature flags, deprecation | skill | [`product-lifecycle-management`](../skills/product-lifecycle-management/SKILL.md) |
| Staged rollout and rollback for a risky release | skill | [`shipping-and-launch`](../skills/shipping-and-launch/SKILL.md) |
| Shape a vague feature idea into scoped options | skill | [`idea-refine`](../skills/idea-refine/SKILL.md) |
| Optimize a slow SQL/NoSQL query or explain plan | skill | [`database-query-optimization`](../skills/database-query-optimization/SKILL.md) |
| Audit toolkit asset health (routers, links, hooks) | command | [`doctor.md`](../commands/doctor.md) |
| Harden toolkit permissions, hooks, tool boundaries | command | [`harden.md`](../commands/harden.md) |
| Parallel investigation with merged verdict | command | [`investigate.md`](../commands/investigate.md) |
| Security review: auth, injection, secrets, LLM tool surface | agent | [`security-auditor.md`](../agents/security-auditor.md) |
| Isolated five-axis code review persona | agent | [`code-reviewer.md`](../agents/code-reviewer.md) |
| Audit the agent/skill system itself | agent | [`agent-systems-auditor.md`](../agents/agent-systems-auditor.md) |
</references>

## Maintenance

<rules>

- Add a row when you add a routable asset users will ask for by intent.
- When you rename or move an asset, update its row in the same change; the `/doctor`
  router check fails on a dangling **Expected asset** link.
- Keep intents phrased as a user would ask, not as the asset name.
- Boundary-sensitive pairs are intentionally pinned here (model-engineering vs MLOps,
  context-engineering vs token-efficient-execution, product-lifecycle vs shipping/idea)
  so seam drift surfaces as a failing route.
</rules>
