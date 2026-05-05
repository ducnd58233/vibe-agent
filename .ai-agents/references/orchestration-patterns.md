# Orchestration Patterns

Reference catalog of agent orchestration patterns this toolkit endorses, plus anti-patterns to avoid. Read this before adding a slash command that coordinates multiple personas, or before introducing a persona that wraps existing ones.

**Canonical assets:** [`.ai-agents/`](../README.md) — skills (`skills/*/SKILL.md`), personas (`agents/*.md`), commands (`commands/*.md`). [Master router](../ROUTER.md) lists families; each subfolder has a `ROUTER.md`.

The governing rule: **the user (or a slash command) is the orchestrator. Personas do not invoke other personas.** Skills are mandatory hops inside a persona’s workflow.

---

## Endorsed patterns

### 1. Direct invocation (no orchestration)

Single persona, single perspective, single artifact.

```
user → code-reviewer → report → user
```

**Use when:** one perspective on one artifact.

**Examples:** “Review this PR” → `code-reviewer`; “Audit `auth.ts`” → `security-auditor`.

---

### 2. Single-persona slash command

A command wraps one persona with the project’s skills.

```
/review → code-reviewer (with code-review-and-quality skill) → report
```

**Use when:** the same invocation repeats with the same setup.

**Cost:** same as direct invocation; the command is a saved prompt.

**Anti-signal:** if the command body is mostly “decide which persona to call,” delete it and route via [`ROUTER.md`](../ROUTER.md) instead.

---

### 3. Parallel fan-out with merge

Multiple personas on the same input; main session merges into go/no-go.

```
                    ┌─→ code-reviewer    ─┐
/ship → fan out  ───┼─→ security-auditor ─┤→ merge → decision + rollback plan
                    └─→ test-engineer    ─┘
```

**Use when:** sub-tasks are independent, each needs its own focus, merge is small.

**Validation checklist:**

- [ ] Sub-agents can run without ordering dependencies
- [ ] Each persona produces a different *kind* of finding
- [ ] Merge fits in the main context

---

### 4. Sequential pipeline (user-driven commands)

```
/spec → /plan → /build → /test → /review → /ship
```

**Use when:** steps depend on prior outputs and human judgment matters between steps.

**Cost:** no meta-orchestrator agent; the user carries context.

---

### 5. Research isolation (context preservation)

Spawn read-only exploration that returns a digest (large read → small summary).

**In Cursor:** use codebase search / exploration tools or a dedicated readonly pass; **in Claude Code:** prefer the built-in `Explore` subagent when applicable.

---

## Claude Code and Cursor

| Concern | Claude Code | Cursor |
|---------|-------------|--------|
| Skills discovery | `.claude/skills` junction → `.ai-agents/skills` | `.cursor/skills` junction → same |
| Commands | `.ai-agents/commands/*.md` as slash prompts | `.cursor/commands` junction → same files; Cursor `/` menu |
| Subagents | `agents/*.md` in `.ai-agents/agents` | Same files as behavioral prompts; composer does not auto-load slash syntax |

Platform rules from upstream still apply on Claude Code (e.g. subagents cannot spawn subagents). See upstream [orchestration doc source](https://github.com/addyosmani/agent-skills/blob/main/references/orchestration-patterns.md) for Agent Teams and plugin frontmatter details.

---

## Anti-patterns

### A. Router persona (“meta-orchestrator”)

Pure routing with no domain value — adds hops and token cost. Use [`ROUTER.md`](../ROUTER.md) and commands instead.

### B. Persona calls another persona

Chaining defeats single-perspective design. **Recommend** a follow-up; user or command runs it.

### C. Sequential LLM orchestrator for `/spec` → `/plan` → …

Summarization drift and lost checkpoints. Keep the human as orchestrator.

### D. Deep persona trees

Orchestration depth should stay ≤ 1 from a slash command to leaf personas; merge in main agent.

---

## Decision flow

```
One perspective on one artifact?
├── Yes → Direct invocation. Stop.
└── No  → Sub-tasks independent?
         ├── No → Sequential user-driven commands.
         └── Yes → Parallel fan-out with merge (validate checklist).
```

---

## When to add a new pattern here

Add only after repeated real use, a concrete repo artifact, and a clear anti-pattern shadow — same bar as [upstream catalog](https://github.com/addyosmani/agent-skills/blob/main/references/orchestration-patterns.md).
