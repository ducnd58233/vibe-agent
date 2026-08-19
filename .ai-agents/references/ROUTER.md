# References router

<routing>

Lookup table for shared checklists and pattern docs under this folder. These files support skills (they are not Agent Skills `SKILL.md`).

**After you add, rename, or remove a reference file, update this table in the same change.**

| Topic | File | Primary skills |
|-------|------|----------------|
| WCAG 2.1 AA, keyboard and screen readers | [`accessibility-checklist.md`](accessibility-checklist.md) | `frontend-ui-engineering` |
| Core Web Vitals, frontend + API + DB | [`performance-checklist.md`](performance-checklist.md) | `performance-optimization` |
| Auth, validation, headers, CORS, OWASP | [`security-checklist.md`](security-checklist.md) | `security-and-hardening` |
| Leak channels: bundle, client storage, console/device logs, UI/DOM, API response, server logs, CI artifacts | [`sensitive-data-exposure.md`](sensitive-data-exposure.md) | `secure-by-default`, `security-and-hardening` |
| AAA, mocks, RTL, API, E2E | [`testing-patterns.md`](testing-patterns.md) | `test-driven-development` |
| Fan-out `/ship`, sequential lifecycle, anti-patterns | [`orchestration-patterns.md`](orchestration-patterns.md) | `using-agent-skills`, slash commands |
| End-to-end `/goal` delivery loop | [`goal-driven-delivery`](../skills/goal-driven-delivery/SKILL.md) skill + [`goal.md`](../commands/goal.md) command | `/goal`, `goal-driven-delivery` |
| `/goal` verification artifacts (`tmp/`, PR wait, E2E records) | [`goal-verification-records.md`](goal-verification-records.md) | `/goal`, `qa-testing-strategy` |
| Skill/agent/command authoring quality | [`agent-authoring-patterns.md`](agent-authoring-patterns.md) | asset authors, `agent-systems-auditor` |
| AI agent harness responsibilities, guides, sensors, verification | [`agent-harness-engineering.md`](agent-harness-engineering.md) | `agent-harness-engineering`, `agent-systems-auditor` |
| Inner vs outer loop, when a workflow deserves a graph, guard design | [`loop-and-graph-engineering.md`](loop-and-graph-engineering.md) | `agent-harness-engineering`, graph authors |
| Per-host hook contracts: event keys, output field casing, workspace root, what each host does not provide. **Generated; edit `runtime/internal/harness/contracts.go`** | [`host-hook-contracts.md`](host-hook-contracts.md) | `agent-harness-engineering`, `agent-systems-auditor`, hook authors |
| Tool permissions, hooks, secret boundaries | [`tool-safety-and-permissions.md`](tool-safety-and-permissions.md) | `security-and-hardening`, `agent-systems-auditor` |
| Agent/skill evaluation and forward testing | [`agent-evaluation-patterns.md`](agent-evaluation-patterns.md) | `agent-systems-auditor`, `test-driven-development` |
| Context budgets and progressive disclosure | [`context-management-patterns.md`](context-management-patterns.md) | `context-engineering`, asset authors |
| Measured token and memory savings: extraction, indexes, cache keys, what a runtime may own | [`token-efficiency.md`](token-efficiency.md) | `token-efficient-execution`, `agent-harness-engineering` |
| AI/ML model development, evaluation, documentation, monitoring | [`ai-model-development-patterns.md`](ai-model-development-patterns.md) | `ai-model-engineering`, `ai-research-methodology`, `ai-engineer`, `ai-researcher` |
| Delivery and observability review patterns | [`ci-cd-observability-patterns.md`](ci-cd-observability-patterns.md) | `devops-platform-delivery`, `observability-monitoring` |
| SQL/NoSQL query diagnosis and optimization | [`database-query-patterns.md`](database-query-patterns.md) | `database-query-optimization`, `database-query-auditor` |
| Manual QA and automation strategy | [`qa-testing-strategy.md`](qa-testing-strategy.md) | `qa-testing-strategy`, `qa-tester` |
| Design-to-code, design systems, MCP handoff | [`design-to-code-patterns.md`](design-to-code-patterns.md) | `product-design-systems`, `product-design-reviewer` |
| Registry contract for UI generation: tokens, component inventory, degradation ladder | [`ui-component-registry.md`](ui-component-registry.md) | `ui-design-fidelity`, `frontend-ui-engineering`, `product-design-reviewer` |
| External repos agents may consult in place: source table, consumption rules, admission checklist | [`external-source-registry.md`](external-source-registry.md) | any skill citing an external repo; asset authors, `agent-systems-auditor` |
| Diagram authoring with Mermaid, render checks, readability | [`diagram-authoring.md`](diagram-authoring.md) | docs-writing commands, `architect-planner`, `research-investigator`, `data-analyst` |
| Proving a mobile app rendered: crash buffer, view hierarchy, blank-frame check | [`mobile-ui-verification.md`](mobile-ui-verification.md) | `qa-testing-strategy`, `test-driven-development`, `qa-tester`, mobile stack profiles |
| Routing fixtures: intent → expected asset, checked by `/doctor` | [`routing-evals.md`](routing-evals.md) | `agent-systems-auditor`, `using-agent-skills` |
| Graph path fixtures: outcomes → expected node path, checked by `/doctor` and `vibe-agent eval graph` | [`graph-path-evals.yaml`](graph-path-evals.yaml) | `agent-systems-auditor`, graph authors |

**Workspace-specific tooling** is indexed in [`../stack-profiles/ROUTER.md`](../stack-profiles/ROUTER.md). Checklists here stay generic; load matching `*.md` profiles from that router when needed.
</routing>
