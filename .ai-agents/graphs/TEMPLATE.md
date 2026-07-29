# Workflow graph authoring template

Use this contract when **adding a graph** under `.ai-agents/graphs/`. Every graph validates against [`schemas/workflow-graph.schema.json`](../../schemas/workflow-graph.schema.json).

Before writing one, read "When a graph is the wrong tool" in [`ROUTER.md`](ROUTER.md). A graph earns its cost only with lasting state, branching, retries, approvals, several phases, and resume.

---

## What

- **File path:** `.ai-agents/graphs/<graph-id>.yaml`
- **Graph id:** `metadata.id` MUST equal the filename stem. Hyphenated, like every other asset here.
- **Drives:** which command and skill this graph is the control flow for.

---

## Why

- **Problem:** what workflow was previously described in prose and interpreted differently each run?
- **Success criteria:** the transitions a reviewer can point at and check.
- **Non-goals:** what stays outside the graph and remains direct invocation.

---

## How

### Node types

Five, and no more. If a node does not fit, it is probably two nodes.

| Type | Executed by | Required fields | Produces |
|------|-------------|-----------------|----------|
| `agent` | Host coding agent | `command` | Free-form work. No automatic pass. |
| `artifact` | Host agent, then file assertion | `command`, `outputs` | Named files that must exist and be non-empty. |
| `verifier` | Runtime subprocess | `verifier`, `check` | Exit code or file assertion, written to `checks[]`. |
| `human_gate` | Human | `check`, `prompt` | An approval event. |
| `terminal` | Runtime | `status` | Run ends as `done`, `failed`, or `cancelled`. |

`command` nodes are `verifier` nodes with a command. Polling nodes are `verifier` nodes that poll. Do not add types for them.

### Guards, not expressions

Every edge condition is a **guard name** declared in `spec.guards`, optionally negated with a leading `!`. There is no expression syntax and the schema cannot represent one.

```yaml
guards:
  - name: unit_passed
    description: Unit and integration commands from the spec exited 0.
    source: check      # flag | check | result

edges:
  - from: test
    to: e2e
    when: unit_passed
  - from: test
    to: build
    when: "!unit_passed"
```

`source` says where the runtime reads the boolean:

| `source` | Read from |
|----------|-----------|
| `flag` | `flags[<name>]` in run state, set at intake or from the spec |
| `check` | `checks[<name>].passed` in run state, written only by real evidence |
| `result` | The outcome recorded for the node that just ran |

Name a guard for the condition, not the branch. `unit_passed` is right; `go_to_e2e` is not.

### Edge rules

- Conditional edges are evaluated in order. An edge with no `when` is the fallback.
- **At most one** unconditional edge may leave a node.
- A node whose edges are all conditional MUST pair a guard with its negation, or a run can strand when nothing matches.
- `terminal` nodes have no outgoing edges.
- Every node MUST be reachable from `spec.initial`, and every node MUST be able to reach some terminal.

### Human gates

Place a `human_gate` **before** the irreversible action, never after it. A gate that fires after a merge documents the merge; it does not govern it. Use the optional `guards` field to name what the gate stands in front of.

Write `prompt` as the sentence the human actually reads. State plainly what is being approved.

### Budget

`spec.maxTransitions` is the transition budget. Exceeding it sets run status to `budget_exceeded` rather than looping forever. Set it from the realistic worst case, not from optimism.

---

## When

- **Applies to:** which command invokes this workflow.
- **Does not apply to:** the single-step cases that stay direct invocation.

---

## Routing & discovery

Graphs are chosen by the command that owns them, not by intent matching. Document which command loads this graph and add the row to [`ROUTER.md`](ROUTER.md).

---

## Permissions & authority

| Topic | Notes |
|-------|--------|
| **Verifier commands** | Run as subprocesses outside the model's tool sandbox. The session must be allowed to run those patterns. |
| **Irreversible actions** | Merge, deploy, publish, and delete MUST sit behind a `human_gate`. |
| **Evidence** | A `check` is only `passed` from an exit code, a file assertion, a CI API response, or a human approval event. Model output can never set one. |

Record graph-related permission expectations in [`../PERMISSIONS.md`](../PERMISSIONS.md).

---

## After creating (MUST)

When you add, rename, or remove a graph:

1. Update **[`ROUTER.md`](ROUTER.md)** in this folder **in the same change**: add a row (workflow/use case, graph path, what it drives); delete stale rows on removal.
2. Validate against [`schemas/workflow-graph.schema.json`](../../schemas/workflow-graph.schema.json) and run the repository router check.
3. Note which command file references the graph, and make that command stop restating transitions in prose. Two copies of the control flow will disagree.
