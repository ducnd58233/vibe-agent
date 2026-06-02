# References router

Lookup table for shared checklists and pattern docs under this folder. These files support skills (they are not Agent Skills `SKILL.md`).

**After you add, rename, or remove a reference file, update this table in the same change.**

| Topic | File | Primary skills |
|-------|------|----------------|
| WCAG 2.1 AA, keyboard and screen readers | [`accessibility-checklist.md`](accessibility-checklist.md) | `frontend-ui-engineering` |
| Core Web Vitals, frontend + API + DB | [`performance-checklist.md`](performance-checklist.md) | `performance-optimization` |
| Auth, validation, headers, CORS, OWASP | [`security-checklist.md`](security-checklist.md) | `security-and-hardening` |
| AAA, mocks, RTL, API, E2E | [`testing-patterns.md`](testing-patterns.md) | `test-driven-development` |
| Fan-out `/ship`, sequential lifecycle, anti-patterns | [`orchestration-patterns.md`](orchestration-patterns.md) | `using-agent-skills`, slash commands |
| Skill/agent/command authoring quality | [`agent-authoring-patterns.md`](agent-authoring-patterns.md) | asset authors, `agent-systems-auditor` |
| Tool permissions, hooks, secret boundaries | [`tool-safety-and-permissions.md`](tool-safety-and-permissions.md) | `security-and-hardening`, `agent-systems-auditor` |
| Agent/skill evaluation and forward testing | [`agent-evaluation-patterns.md`](agent-evaluation-patterns.md) | `agent-systems-auditor`, `test-driven-development` |
| Context budgets and progressive disclosure | [`context-management-patterns.md`](context-management-patterns.md) | `context-engineering`, asset authors |
| Delivery and observability review patterns | [`ci-cd-observability-patterns.md`](ci-cd-observability-patterns.md) | `devops-platform-delivery`, `observability-monitoring` |
| SQL/NoSQL query diagnosis and optimization | [`database-query-patterns.md`](database-query-patterns.md) | `database-query-optimization`, `database-query-auditor` |
| Manual QA and automation strategy | [`qa-testing-strategy.md`](qa-testing-strategy.md) | `qa-testing-strategy`, `qa-tester` |
| Design-to-code, design systems, MCP handoff | [`design-to-code-patterns.md`](design-to-code-patterns.md) | `product-design-systems`, `product-design-reviewer` |

**Workspace-specific tooling** is indexed in [`../stack-profiles/ROUTER.md`](../stack-profiles/ROUTER.md). Checklists here stay generic; load matching `*.md` profiles from that router when needed.
