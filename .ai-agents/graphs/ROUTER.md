# Graphs router

<context>

Lookup table for executable workflow graphs in this folder. **After you add, rename, or remove a `.yaml` graph, update this table in the same change.**

A graph is the control flow for a multi-phase workflow: which node runs next, and on what evidence. The matching command file stays the human-facing description and the matching skill stays the phase semantics. The graph is the only place transitions are defined.

| Workflow / use case | Graph | Drives |
|---------------------|-------|--------|
| User objective to verified, shipped work with human gates | [`goal-delivery.yaml`](goal-delivery.yaml) | [`/goal`](../commands/goal.md), [`goal-driven-delivery`](../skills/goal-driven-delivery/SKILL.md) |

**Contract:** every graph validates against [`schemas/workflow-graph.schema.json`](../../schemas/workflow-graph.schema.json).

**Authoring:** [`TEMPLATE.md`](TEMPLATE.md).
</context>

## When a graph is the wrong tool

<context>

Most work needs no graph. Reach for one only when the workflow has lasting state, branching, retries, approval gates, several phases, and a need to resume. A single-file review, one research lookup, a lint run, or a small fix are all direct invocation. Adding nodes to model them buys structure and pays in rigidity. See [`orchestration-patterns.md`](../references/orchestration-patterns.md) for the ungraphed patterns and [`loop-and-graph-engineering.md`](../references/loop-and-graph-engineering.md) for the tradeoff.
</context>
