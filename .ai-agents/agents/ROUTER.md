# Subagents router

<routing>

Lookup table for Claude subagent files in this folder. **After you add, rename, or remove a `*.md` subagent, update this table in the same change.**

Subagents are primarily for **Claude Code** (typically via `.claude/agents` linked to this folder). Cursor users may `@`-reference the same files as prompts.

| Task type / use case | Subagent file | Tool scope (YAML `tools` map, values `true`) |
|----------------------|---------------|---------------------------|
| Five-axis code review | [`code-reviewer.md`](code-reviewer.md) | Read, Grep, Glob, Bash |
| Security audit, OWASP + document DB + LLM surfaces | [`security-auditor.md`](security-auditor.md) | Read, Grep, Glob, Bash |
| Tests, coverage, Prove-It | [`test-engineer.md`](test-engineer.md) | Read, Grep, Glob, Bash |
| Topic evidence gathering and digest | [`research-investigator.md`](research-investigator.md) | Read, Grep, Glob, WebSearch, WebFetch |
| Evidence synthesis and recommendation | [`data-analyst.md`](data-analyst.md) | Read, Grep, Glob, WebSearch, WebFetch |
| Citation integrity and source quality audit | [`source-auditor.md`](source-auditor.md) | Read, Grep, Glob, WebSearch, WebFetch |
| AI/ML research and paper-to-experiment handoff | [`ai-researcher.md`](ai-researcher.md) | Read, Grep, Glob, WebSearch, WebFetch, Bash |
| AI/ML model engineering and production-readiness | [`ai-engineer.md`](ai-engineer.md) | Read, Grep, Glob, Bash, WebSearch, WebFetch |
| Judgment-level agent harness audit: orchestration, context hygiene, evaluation coverage (deterministic checks are `/doctor`) | [`agent-systems-auditor.md`](agent-systems-auditor.md) | Read, Grep, Glob, Bash |
| DevOps/SRE operational audit | [`devops-sre-auditor.md`](devops-sre-auditor.md) | Read, Grep, Glob, Bash |
| Architecture planning and design risk | [`architect-planner.md`](architect-planner.md) | Read, Grep, Glob, Bash |
| SQL/NoSQL query and datastore audit | [`database-query-auditor.md`](database-query-auditor.md) | Read, Grep, Glob, Bash |
| Manual and automation QA | [`qa-tester.md`](qa-tester.md) | Read, Grep, Glob, Bash |
| Product design and design-system review | [`product-design-reviewer.md`](product-design-reviewer.md) | Read, Grep, Glob, Bash |

**Authoring:** [`TEMPLATE.md`](TEMPLATE.md). Overview: [`README.md`](README.md).
</routing>
