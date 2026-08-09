# Orchestration Patterns

<context>

Reference catalog of agent orchestration patterns this toolkit endorses, plus anti-patterns to avoid. Read this before adding a slash command that coordinates multiple personas, or before introducing a persona that wraps existing ones.

**Canonical assets:** [`.ai-agents/`](../README.md) - skills (`skills/*/SKILL.md`), personas (`agents/*.md`), commands (`commands/*.md`). [Master router](../ROUTER.md) lists families; each subfolder has a `ROUTER.md`.

The governing rule: **the user (or a slash command) is the orchestrator. Personas do not invoke other personas.** Skills are mandatory hops inside a persona’s workflow.

---
</context>

## Endorsed patterns

<context>

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
- [ ] **Repo-access preflight:** lanes get an absolute working directory and must echo a real top-level listing before structural claims are trusted; a lane that cannot access the path returns `ACCESS-FAILED` and is non-authoritative (the main agent investigates directly if its own tools verify access)

---

### 4. Sequential pipeline (user-driven commands)

```
/spec → /plan → /build → /test → /review → /ship → (human: merge to main)
              ↑ one branch + one PR per planned task; same-task feedback on same branch; /build never merges to main
```

**Use when:** steps depend on prior outputs and human judgment matters between steps.

**Git gates:** `/build` uses one branch + one PR per planned task; same-task feedback fixes stay on that branch; unrelated work needs a new branch. `/build` never merges to `main`. `/ship` emits GO/NO-GO; merge to `main` only after GO **and** explicit human approval ([`git-workflow-and-versioning`](../skills/git-workflow-and-versioning/SKILL.md), [`commands/build.md`](../commands/build.md), [`commands/ship.md`](../commands/ship.md)).

**Cost:** no meta-orchestrator agent; the user carries context.

---

### 4b. Goal orchestration (`/goal`)

Single slash command that walks the sequential pipeline with clarify-first intake, optional research, and iterate-until-done loops ([`commands/goal.md`](../commands/goal.md), [`goal-driven-delivery`](../skills/goal-driven-delivery/SKILL.md)).

```
/goal → INTAKE → [RESEARCH → ANALYZE] → /spec → /plan → (/build → /test → /review → /ship)* → done
```

**Use when:** the user states an outcome, not a single command step; requirements may be ambiguous.

**Not the same as:** Claude Code or Codex native `/goal` harness loops (evaluator-driven turns). Toolkit `/goal` composes existing commands with checkpoints.

**Anti-drift:** re-read `docs/<slug>/SPEC.md` and `TASKS.md` each phase; report `GOAL STATUS` after each phase.

**Executable form:** the transitions above are defined in [`graphs/goal-delivery.yaml`](../graphs/goal-delivery.yaml) and validated by `scripts/check-graphs.py`. When the graph and a prose description disagree, the graph wins; two copies of the control flow drift. See [`loop-and-graph-engineering.md`](loop-and-graph-engineering.md) for when a workflow earns a graph at all, and [`graphs/TEMPLATE.md`](../graphs/TEMPLATE.md) for the authoring contract.

---

### 5. Research isolation (context preservation)

Spawn read-only exploration that returns a digest (large read → small summary).

**In Cursor:** use codebase search / exploration tools or a dedicated readonly pass; **in Claude Code:** prefer the built-in `Explore` subagent when applicable.

---
</context>

## Claude Code and Cursor

<rules>

| Concern | Claude Code | Cursor |
|---------|-------------|--------|
| Skills discovery | `.claude/skills` junction → `.ai-agents/skills` | `.cursor/skills` junction → same |
| Commands | `.ai-agents/commands/*.md` as slash prompts | `.cursor/commands` junction → same files; Cursor `/` menu |
| Subagents | `agents/*.md` in `.ai-agents/agents` | Same files as behavioral prompts; composer does not auto-load slash syntax |

Platform rules from upstream still apply on Claude Code (e.g. subagents cannot spawn subagents). For Agent Teams and plugin frontmatter details, consult the `agent-assets` sources in [`external-source-registry.md`](external-source-registry.md) - read in place, and verify against current official docs before relying on upstream platform claims.

---
</rules>

## Anti-patterns

<antipatterns>

### A. Router persona (“meta-orchestrator”)

Pure routing with no domain value - adds hops and token cost. Use [`ROUTER.md`](../ROUTER.md) and commands instead.

### B. Persona calls another persona

Chaining defeats single-perspective design. **Recommend** a follow-up; user or command runs it.

### C. Sequential LLM orchestrator for `/spec` → `/plan` → …

Summarization drift and lost checkpoints. Keep the human as orchestrator.

### D. Deep persona trees

Orchestration depth should stay ≤ 1 from a slash command to leaf personas; merge in main agent.

### E. Fan-out over a single local source of truth

Fanning multiple personas across the **same** local repository tree is redundant (they read the same files) and *multiplies* the chance that ≥1 lane fails its sandbox/path resolution or hallucinates structure - then forces the merge step to adjudicate reliability. Fan-out requires each lane to add a *different kind* of finding (checklist above); pure re-reading does not qualify. For repository-grounded investigation prefer the read-only `Explore` agent or direct `Read`/`Grep`/`Glob`, and apply the repo-access preflight before trusting any lane's structural claims.

---
</antipatterns>

## Decision flow

<context>

```
One perspective on one artifact?
├── Yes → Direct invocation. Stop.
└── No  → Sub-tasks independent?
         ├── No → Sequential user-driven commands.
         └── Yes → Parallel fan-out with merge (validate checklist).
```

---
</context>

## Token & performance levers (evidence-backed)

<references>

Levers from Anthropic's agent-engineering reports, applied to this toolkit. Use them to keep fan-out fast and cheap.

- **Fan out only for independent threads.** Multi-agent systems consume ~15× the tokens of a single chat, and token usage alone explains ~80% of performance variance; the win materializes only when subtasks are genuinely independent - shared-context and most coding tasks are a poor fit ([multi-agent research system](https://www.anthropic.com/engineering/multi-agent-research-system)). This is the quantitative basis for anti-pattern §E and the §3 checklist.
- **Concise output formats.** Switching agent/tool outputs to a concise format cut token use by up to ~65% in Anthropic's measurements. Keep each lane's return to its digest/verdict - not raw file dumps or transcripts ([multi-agent research system](https://www.anthropic.com/engineering/multi-agent-research-system)). Pair with [`token-efficient-execution`](../skills/token-efficient-execution/SKILL.md).
- **Filesystem hand-off, not telephone.** For large investigations, have lanes write digests to `./reports` (this repo's `plansDirectory`) and pass file references, rather than funneling full content through the orchestrator's context window ([multi-agent research system](https://www.anthropic.com/engineering/multi-agent-research-system)).
- **Right-size the worker model.** Run parallel lanes on a cheaper/faster model and reserve the strongest model for synthesis - Anthropic ran Sonnet workers under an Opus lead. Use the Agent tool's `model` override per lane.
- **Progressive disclosure.** Keep agent/skill metadata lean (~100 tokens) and push detail into bodies/references loaded on demand, so unused reference material never enters context ([building effective agents](https://www.anthropic.com/research/building-effective-agents)).
</references>

## When to add a new pattern here

<context>

Add only after repeated real use, a concrete repo artifact, and a clear anti-pattern shadow - the same bar the `agent-assets` upstreams in [`external-source-registry.md`](external-source-registry.md) apply.
</context>
