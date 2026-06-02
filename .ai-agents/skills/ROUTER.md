# Skills router

Lookup table for skills under this folder. **After you create, rename, or delete a skill folder, update this table in the same change.**

| Intent / use case | Skill folder | When to invoke |
|-------------------|--------------|----------------|
| Meta: discover skills, routers, orchestration rules | [`using-agent-skills`](using-agent-skills) | Session start; unsure which workflow applies |
| AI coding-agent harness design and hardening | [`agent-harness-engineering`](agent-harness-engineering) | Instructions, routers, skills, agents, commands, context selection, tools, permissions, hooks, sensors, evals, generated Codex assets |
| Always-on coding reliability guardrails | [`karpathy-guardrails`](karpathy-guardrails) | Non-trivial implementation; avoid assumptions/overengineering/scope drift |
| Token-efficient execution and concise delivery | [`token-efficient-execution`](token-efficient-execution) | High-volume loops; minimize verbosity without losing correctness |
| Multi-axis PR review, merge quality | [`code-review-and-quality`](code-review-and-quality) | Before merge; evaluating agent output |
| HTTP/API contracts, OpenAPI alignment | [`api-and-interface-design`](api-and-interface-design) | New endpoints; schema design |
| Backend modular layering, repos, txs | [`backend-engineering`](backend-engineering) | Services layout; transactions; persistence boundaries |
| Browser verification, DevTools | [`browser-testing-with-devtools`](browser-testing-with-devtools) | Runtime/UI bugs; performance in browser |
| Concurrency, realtime, streaming, high-traffic systems | [`concurrency-realtime-systems`](concurrency-realtime-systems) | WebSockets, SSE, WebRTC, video/live streaming, fan-out, async runtimes, slow consumers, overload |
| Reduce complexity without behavior change | [`code-simplification`](code-simplification) | Refactor for clarity after tests exist |
| Load right context, avoid token flood | [`context-engineering`](context-engineering) | Large tasks; scoped reads |
| Diverge/converge on product ideas | [`idea-refine`](idea-refine) | Vague features; prioritization |
| UI patterns, a11y, client/server boundaries | [`frontend-ui-engineering`](frontend-ui-engineering) | Components, layouts, UX |
| Product design systems and design-to-code | [`product-design-systems`](product-design-systems) | Figma/Canva/MCP handoff, tokens, visual QA, reusable UI components |
| Citation-first topic investigation and digest | [`research-with-citations`](research-with-citations) | Facts must be source-backed; web research needed |
| AI/ML model engineering across CV/NLP/speech/multimodal | [`ai-model-engineering`](ai-model-engineering) | Model training, fine-tuning, inference, evals, model/data cards, monitoring, AI product/model quality |
| AI/ML research methodology and paper-to-experiment handoff | [`ai-research-methodology`](ai-research-methodology) | Literature review, model/method comparison, benchmark analysis, reproduction plans, ablations |
| Tradeoff analysis with confidence and evidence | [`evidence-based-analysis`](evidence-based-analysis) | Choosing options from gathered evidence |
| Measure before optimizing | [`performance-optimization`](performance-optimization) | Latency, bundles, persistence slow paths |
| Decompose specs into tasks | [`planning-and-task-breakdown`](planning-and-task-breakdown) | Multi-step features |
| Git commits, branches, worktrees | [`git-workflow-and-versioning`](git-workflow-and-versioning) | Any change; parallel agents |
| Official-docs-backed implementation | [`source-driven-development`](source-driven-development) | Framework APIs; version-sensitive code |
| Written spec before code | [`spec-driven-development`](spec-driven-development) | New features; ambiguous scope |
| RED/GREEN, Prove-It bugs | [`test-driven-development`](test-driven-development) | New logic; regressions |
| Production checklist, rollout | [`shipping-and-launch`](shipping-and-launch) | Deploys; risky releases |
| Systematic debug | [`debugging-and-error-recovery`](debugging-and-error-recovery) | Failures; flakiness |
| Docs and ADRs | [`documentation-and-adrs`](documentation-and-adrs) | Decisions; runbooks |
| OWASP-style hardening, document DB + LLM routes | [`security-and-hardening`](security-and-hardening) | Auth, data, dependencies |
| SQL/NoSQL query analysis and optimization | [`database-query-optimization`](database-query-optimization) | Query errors, slow queries, indexes, explain plans, locks, hot keys, cache/datastore issues |
| Manual and automation QA strategy | [`qa-testing-strategy`](qa-testing-strategy) | Test charters, exploratory QA, regression, E2E automation, release signoff |
| DevOps platform, CI/CD, IaC, deploy automation | [`devops-platform-delivery`](devops-platform-delivery) | Pipelines, containers, Terraform/Kubernetes, release gates, rollback |
| System administration and host operations | [`system-administration-ops`](system-administration-ops) | systemd, Ansible, shell scripts, services, backups, host diagnostics |
| Observability, monitoring, alerting, dashboards | [`observability-monitoring`](observability-monitoring) | OpenTelemetry, Prometheus/Grafana, logs/traces/metrics, SLOs, runbooks |
| MLOps lifecycle and model operations | [`mlops-lifecycle`](mlops-lifecycle) | ML pipelines, model registry, evaluation, serving, drift, retraining |
| Product lifecycle and rollout management | [`product-lifecycle-management`](product-lifecycle-management) | Success metrics, feature flags, staged launch, feedback, deprecation |

Pinned stack for the **current workspace**: [`stack-profiles/ROUTER.md`](../stack-profiles/ROUTER.md) â€” compose matching profile files per task.

**Authoring:** [`TEMPLATE.md`](TEMPLATE.md).
