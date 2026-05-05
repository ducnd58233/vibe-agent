# Skills router

Lookup table for skills under this folder. **After you create, rename, or delete a skill folder, update this table in the same change.**

| Intent / use case | Skill folder | When to invoke |
|-------------------|--------------|----------------|
| Meta: discover skills, routers, orchestration rules | [`using-agent-skills`](using-agent-skills) | Session start; unsure which workflow applies |
| Multi-axis PR review, merge quality | [`code-review-and-quality`](code-review-and-quality) | Before merge; evaluating agent output |
| HTTP/API contracts, OpenAPI alignment | [`api-and-interface-design`](api-and-interface-design) | New endpoints; schema design |
| Backend modular layering, repos, txs | [`backend-engineering`](backend-engineering) | Services layout; transactions; persistence boundaries |
| Browser verification, DevTools | [`browser-testing-with-devtools`](browser-testing-with-devtools) | Runtime/UI bugs; performance in browser |
| Reduce complexity without behavior change | [`code-simplification`](code-simplification) | Refactor for clarity after tests exist |
| Load right context, avoid token flood | [`context-engineering`](context-engineering) | Large tasks; scoped reads |
| Diverge/converge on product ideas | [`idea-refine`](idea-refine) | Vague features; prioritization |
| UI patterns, a11y, client/server boundaries | [`frontend-ui-engineering`](frontend-ui-engineering) | Components, layouts, UX |
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

Pinned stack for **this** monorepo: [`stack-profiles/ROUTER.md`](../stack-profiles/ROUTER.md) — compose matching profile files per task.

**Authoring:** [`TEMPLATE.md`](TEMPLATE.md).
