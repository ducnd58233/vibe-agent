# Loop and graph engineering

Use this reference when deciding who owns which loop in an agent system, and whether a workflow deserves an executable graph or should stay a direct invocation.

Companion to [`agent-harness-engineering.md`](agent-harness-engineering.md), which covers the harness responsibilities, and [`orchestration-patterns.md`](orchestration-patterns.md), which covers the ungraphed patterns.

## Two loops, two owners

<rules>

The single most useful distinction in this area is which loop belongs to whom.

| | Inner loop | Outer loop |
|---|---|---|
| Shape | perceive, reason, act, observe, repeat | plan, build, verify, review, ship, repeat |
| Owner | The coding agent (Claude Code, Codex, Cursor, opencode) | The surrounding system |
| Horizon | One turn | One task, often many sessions |
| State | Context window | Persisted run state |
| Terminates when | The model decides the turn is done | A gate produces evidence, or a budget runs out |

The inner loop is what a coding agent already runs on every turn. The outer loop is the system that runs that inner loop, feeds it work, checks the result, and decides the next thing ([Augment Code](https://www.augmentcode.com/guides/what-is-loop-engineering), [Arize](https://arize.com/blog/what-is-a-loop-in-ai-engineering-anyway/)).

**Do not rebuild the inner loop.** Vendors own it, it is well tuned, and replacing it buys nothing. Own the outer loop instead. That is where skipped phases, unverified completion, and lost progress actually come from.

## The four-loop stack

LangChain describes loop engineering as stacking feedback loops rather than building one ([The Art of Loop Engineering](https://www.langchain.com/blog/the-art-of-loop-engineering)).

| Loop | What it does | Cost to add |
|------|--------------|-------------|
| 1. Agent loop | A model calls tools repeatedly until a task is complete | Free, the vendor supplies it |
| 2. Verification loop | Output is scored against a rubric and retried with feedback on failure | Low, needs deterministic checks and a retry budget |
| 3. Event-driven loop | Events trigger agent runs that update a real system | Medium, needs triggers and idempotency |
| 4. Hill-climbing loop | Traces from production runs feed an analysis step that improves the harness config | High, needs tracing and an eval set first |

The important property is that the return arrow does not simply loop back to the top. It reaches inside and updates the agent loop's own configuration. A loop that only retries is not improving anything.

Build them in order. Loop 4 without loop 2 has nothing to learn from.

## When a graph earns its cost

A graph is worth it only when the workflow has **all** of these:

- Lasting state across steps and sessions
- Branching that depends on results
- Retries with a budget
- Approval gates
- Several distinct phases
- A need to resume after interruption

A single-file review, one research lookup, a lint run, and a small bug fix have none of these. Model them as direct invocation ([`orchestration-patterns.md`](orchestration-patterns.md)).

### The cost side

Structure is not free. Moving from loops to structured graphs buys explicit dependencies, parallelism, and predictability, and pays in rigidity: a DAG "imposes structure that may be restrictive for highly dynamic, unpredictable workflows," and scheduling gets harder as the graph grows ([arXiv 2604.11378](https://arxiv.org/pdf/2604.11378)).

The production guidance is blunt about sequencing: start with the simplest architecture that works, instrument it fully, and add complexity only in response to an observed failure mode ([Zylos, 2026](https://zylos.ai/research/2026-04-14-graph-based-agent-workflow-orchestration-production/)).
</rules>

## Graph anatomy

<context>

A node performs work. It might be a model call, a plain function, a test suite, a tool, or a human approval. An edge is a transition. State flows between them and is checkpointed so a run can resume ([LangGraph](https://docs.langchain.com/oss/python/langgraph/overview)).

### Node types in this toolkit

Five, defined in [`schemas/workflow-graph.schema.json`](../../schemas/workflow-graph.schema.json) and documented in [`graphs/TEMPLATE.md`](../graphs/TEMPLATE.md):

| Type | Executed by | Produces |
|------|-------------|----------|
| `agent` | Host coding agent | Free-form work, no automatic pass |
| `artifact` | Host agent, then file assertion | Named files that must exist and be non-empty |
| `verifier` | Runtime subprocess | Exit code or file assertion |
| `human_gate` | Human | An approval event |
| `terminal` | Runtime | Run ends |

Resist adding types. A polling node is a verifier that polls; a shell node is a verifier with a command.

### Guards, not expressions

Edge conditions here are **named booleans** declared in the graph, not expressions. `when: unit_passed` and `when: "!unit_passed"`, never `when: state.unit == true and state.iteration < 3`.

An expression language in a graph file means writing, testing, and hardening an evaluator over what is effectively executable input, for a workflow that has perhaps a dozen conditions. Named guards give the same branching with a map lookup, and a validator can reject a typo statically.

A guard is a **question** (`unit_passed`); a check is an **evidence slot** (`unit`). They are usually different words, so a guard declares which key it `reads`. Leaving that mapping implicit buries it in runtime code where nothing can check it.

### Evidence provenance

A check is `passed` only from a process exit code, a file assertion, a CI API response, or a recorded human approval. There is no provenance value for model assertion, so no code path lets model output mark its own work complete. This is enforced by [`schemas/run-state.schema.json`](../../schemas/run-state.schema.json), not by instruction.
</context>

## Production failure modes

<rules>

From a 2026 survey of graph-orchestrated systems in production ([Zylos](https://zylos.ai/research/2026-04-14-graph-based-agent-workflow-orchestration-production/)):

| Failure mode | What it looks like | Countermeasure here |
|---|---|---|
| Undefined coordination contracts | Rules live only in prompts, so failures look random | Transitions live in the graph; `check-graphs.py` validates them |
| Late observability | Tracing retrofitted after deployment is expensive | Run state and an append-only event log from the first run |
| Maximizing agent autonomy | Unconstrained agents underperform in production | Gates, budgets, and provenance rules constrain the loop |
| Improper interrupt placement | Approvals sit after the risky step instead of before it | `human_gate` nodes precede irreversible actions; the merge gate is the example |

The last one is easy to get backwards. A gate that fires after a merge documents the merge; it does not govern it.
</rules>

## Checklist

<verification>

Use when reviewing a workflow design.

- [ ] Inner loop left to the vendor; only the outer loop is owned here
- [ ] Loop 2 (verification) exists before anyone builds loop 4
- [ ] The workflow actually has state, branching, retries, gates, phases, and resume; otherwise no graph
- [ ] Node types kept to the five; no type added for a variation
- [ ] Edge conditions are guard names, never expressions
- [ ] Every guard declares what it reads, and something writes that key
- [ ] Every check has real provenance; nothing is passed by assertion
- [ ] Human gates precede irreversible actions
- [ ] A transition budget exists and terminates the run
- [ ] Every node is reachable and can reach a terminal
</verification>

## Related references

<references>

- [`agent-harness-engineering.md`](agent-harness-engineering.md) - harness responsibilities, guides and sensors, verification
- [`orchestration-patterns.md`](orchestration-patterns.md) - the patterns that need no graph, and the fan-out anti-patterns
- [`context-management-patterns.md`](context-management-patterns.md) - checkpointing to artifacts instead of chat memory
- [`agent-evaluation-patterns.md`](agent-evaluation-patterns.md) - what to measure before adding loop 4
- [`goal-verification-records.md`](goal-verification-records.md) - where run state and evidence are written

## Source notes

Sources were read in 2026-07. Verify against current material before relying on version-specific claims; this area is moving quickly.

- LangChain frames loop engineering as a four-loop stack and stresses that the outer loop must update the inner loop's configuration, not merely retry it.
- Production surveys report graph orchestration with per-node checkpointing as the common production shape, and name undefined contracts, late observability, excess autonomy, and misplaced interrupts as the recurring failures.
- Scheduler-theoretic work on converting agent loops to DAGs reports the parallelism and predictability gains alongside the rigidity and scheduling costs.
- Memory work has converged on an episodic, semantic, procedural split, and notes that declarative injection through `AGENTS.md`-style files is already an effective procedural-memory form for coding agents ([Zylos, memory architectures](https://zylos.ai/research/2026-04-05-ai-agent-memory-architectures-persistent-knowledge/)).
</references>
