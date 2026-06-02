# Agent personas (Claude subagents)

Markdown files in this folder define **single-role** specialists consumed as Claude Code subagents (commonly via `.claude/agents` linked to this folder). Cursor does not load these automatically; use the same instructions by `@`-referencing a file in chat.

## Three-layer model

| Layer | Role | Location |
|-------|------|----------|
| **Skill** | Workflow (how) | [`skills/`](../skills/) |
| **Persona** | Perspective (who) | This folder |
| **Command** | Entry point (when) | [`commands/`](../commands/) |

Personas **do not** call other personas. Composition is via user or slash commands â€” see [`references/orchestration-patterns.md`](../references/orchestration-patterns.md).

## Personas

| Persona | File | Best for |
|---------|------|----------|
| Code review (five axes) | [`code-reviewer.md`](code-reviewer.md) | Pre-merge PR review |
| Security audit | [`security-auditor.md`](security-auditor.md) | Auth, data paths, dependencies |
| Tests / coverage | [`test-engineer.md`](test-engineer.md) | TDD, gaps, Prove-It bugs |
| Research investigation | [`research-investigator.md`](research-investigator.md) | Source-backed topic discovery |
| AI research | [`ai-researcher.md`](ai-researcher.md) | AI/ML papers, benchmarks, reproduction, method comparison |
| AI engineering | [`ai-engineer.md`](ai-engineer.md) | Model build/train/eval/serve/monitor, model/data cards |
| Data analysis | [`data-analyst.md`](data-analyst.md) | Evidence synthesis and recommendations |
| Source audit | [`source-auditor.md`](source-auditor.md) | Citation quality and source integrity |
| Agent systems audit | [`agent-systems-auditor.md`](agent-systems-auditor.md) | Skills, commands, hooks, routers, permissions, harnesses |
| DevOps/SRE audit | [`devops-sre-auditor.md`](devops-sre-auditor.md) | CI/CD, infra, observability, deployment risk |
| Architecture planning | [`architect-planner.md`](architect-planner.md) | Design options, boundaries, implementation slices |
| Database query audit | [`database-query-auditor.md`](database-query-auditor.md) | SQL/NoSQL query correctness, performance, indexes |
| QA testing | [`qa-tester.md`](qa-tester.md) | Manual QA, automation strategy, release signoff |
| Product design review | [`product-design-reviewer.md`](product-design-reviewer.md) | Design systems, visual QA, Figma/Canva handoff |

## Authoring

Use [`TEMPLATE.md`](TEMPLATE.md). After adding or renaming a file, update [`ROUTER.md`](ROUTER.md).
