# Agent harness engineering

Use this reference when designing or auditing the system around AI coding agents: instructions, routers, context selection, tool access, validation loops, observability, and intervention records.

## Definition

<rules>

An agent harness is the runtime and workflow substrate around an LLM or coding agent. It shapes what the agent can see, what it can do, how it gets feedback, and how humans verify or intervene.

Do not treat "harness" as only one thing. In this toolkit it includes:

- **Guides:** AGENTS.md, routers, skills, commands, stack profiles, references, prompts.
- **Sensors:** tests, linters, hooks, router checks, Codex generation checks, audits, evals, CI reports.
- **Actuators:** edit tools, shell commands, MCP tools, GitHub connectors, deployment tools.
- **Boundaries:** permissions, deny rules, secrets policy, approval gates, read/write scopes.
- **Records:** specs, plans, task state, verification evidence, failure notes, intervention logs.
</rules>

## Responsibility checklist

<verification>

Use this checklist for `/doctor`, `/harden`, `agent-systems-auditor`, or consumer-repo harness setup.

| Responsibility | Good signal | Failure mode |
|---|---|---|
| Task specification | Request is decomposed into intent, constraints, acceptance criteria, and non-goals | Agent guesses missing requirements |
| Context selection | Router-first reads; relevant files only; source docs cited when current facts matter | Context flooding, stale files, hallucinated APIs |
| Tool access | Least-privilege tools and path scopes match the task | Broad shell/write access by default |
| Project memory | Durable rules and stack profiles capture stable conventions | Important conventions live only in chat history |
| Task state | Specs/plans/checklists record progress and next action | Long sessions rely on memory and drift |
| Observability | Agent runs produce visible logs, diffs, checks, and traceable evidence | "Done" without proof |
| Failure attribution | Failures are classified as context, tool, permission, test, ambiguity, or model error | Repeated failures produce generic retries |
| Verification | Deterministic checks run before subjective review where possible | Review-only validation for testable behavior |
| Permissions | Dangerous actions require ask/approval; secrets paths are denied | Agents can read secrets or mutate production |
| Entropy auditing | Scheduled or pre-ship audits catch stale routers, dead links, debt, and drift | Harness quality decays as assets grow |
| Intervention recording | Human overrides and blocked decisions are recorded in specs, plans, or follow-up issues | Same ambiguity recurs without learning |
</verification>

## Design patterns

<rules>

### Guides plus sensors

Pair every important instruction with a feedback mechanism.

- Routing rule -> router check.
- Skill/agent source -> Codex generation check.
- Coding convention -> lint/static check where practical.
- Security rule -> deny pattern, scanner, or review checklist.
- Release rule -> CI gate, smoke test, dashboard, rollback checklist.

### Computational before inferential

Prefer deterministic tools for facts that can be computed:

- Use `git diff`, tests, linters, typecheckers, schema validators, explain plans, and link checkers before asking an LLM to judge quality.
- Use LLM review for semantic risk, missing cases, tradeoffs, and synthesis after deterministic evidence is collected.

### Narrow action windows

Keep the default harness read-heavy. Escalate to write, shell, network, deploy, or production tools only when the task and permissions justify it.

### Episode evidence

For non-trivial changes, preserve a compact episode package:

- task/spec link
- changed files
- commands/checks run
- failing evidence before fix, if any
- passing evidence after fix
- unresolved risks or deferred follow-ups

## Audit prompts

Use these as short prompts when reviewing a harness:

- What stable guidance is always loaded, and what is loaded only by router?
- Which deterministic sensors catch bad output before human review?
- Are generated tool-specific assets in sync with canonical `.ai-agents` sources?
- Where can the agent mutate files, shell, network, issues, or production state?
- Can a fresh session reproduce the intended routing and verification path?
- What failure categories are recurring, and should they become guides or sensors?
</rules>

## Related references

<references>

- [`agent-authoring-patterns.md`](agent-authoring-patterns.md)
- [`agent-evaluation-patterns.md`](agent-evaluation-patterns.md)
- [`context-management-patterns.md`](context-management-patterns.md)
- [`tool-safety-and-permissions.md`](tool-safety-and-permissions.md)
- [`orchestration-patterns.md`](orchestration-patterns.md)
- [`loop-and-graph-engineering.md`](loop-and-graph-engineering.md) - which loop the harness owns, and when a workflow earns an executable graph

## Source notes

- Thoughtworks describes harness engineering as the work around AI coding agents, including guides, sensors, tools, prompts, and workflow integration.
- The 2026 arXiv paper "AI Harness Engineering: A Runtime Substrate for Foundation-Model Software Agents" frames the harness as the model-harness-environment system and lists responsibilities including task specification, context selection, tool access, observability, verification, permissions, and intervention recording.
- Traditional test harness concepts still apply: drivers, stubs, test data, execution engines, and reports are examples of computational sensors around software under test.
</references>
