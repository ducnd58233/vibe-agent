---
description: Run or advance a host/CI experiment and keep STATUS.md current until done or failed
---

Execute or advance one research experiment on **host or CI compute**, and keep the run's STATUS file honest so `researcher-delivery` can monitor continuously.

<references>

Follow [`researcher-harness`](../skills/researcher-harness/SKILL.md).

Graph: [`researcher-delivery`](../graphs/researcher-delivery.yaml).

MCP: `vibe_experiment_status` reads STATUS; it does not start a sandbox.
</references>

## What

<context>

- **Inputs:** experiment PLAN/TASKS for the slug; host commands or CI jobs the plan names.
- **Outputs:** updated `.agent-state/runs/<date>/<slug>/<version>/experiment/STATUS.md` and any logs the plan requires.
- **Non-goal:** in-process GPU or container sandbox (declined by charter). Use host/CI.
</context>

## STATUS.md contract (MUST)

<required>

Write this file under the run directory:

```markdown
# Experiment status
status: running
updated: <RFC3339>
note: <short progress>
```

Allowed `status` values: `running`, `done`, `failed`.

Update it whenever progress changes. The `experiment_monitor` verifier fails while `running` (or missing) and passes on `done` or `failed`.

When `status` becomes `done`, also write `experiment/METRICS.json`:

```json
{
  "metrics": {"ndcg_at_10": 0.84},
  "thresholds": {"ndcg_at_10": {"op": ">=", "value": 0.82}}
}
```

The `results_eval` verifier compares metrics to thresholds. Values below the bar route the graph back to `hypothesis` without human approval.

**Comparing this run against earlier iterations (not just gating this one):** this STATUS.md/METRICS.json pair is scoped to the current graph run and stops mattering once it finishes. To keep a comparable record across many iterations for a paper, report, or competition writeup, also write `experiments/<project-slug>/<run-id>/` per [`researcher-harness`](../skills/researcher-harness/SKILL.md) section "Experiment ledger, across runs".
</required>

## How

<procedure>

1. Read PLAN Mermaid and TASKS acceptance criteria.
2. Run the next host/CI step the plan names.
3. Refresh STATUS.md before returning.
4. Call `vibe_verify` at `experiment_monitor`.
</procedure>

## Watch it to completion (MUST)

<required>

One honest `vibe_verify` that comes back `running` is a true reading of the experiment at that
instant. It is not evidence that anything will check again, and on its own it is not grounds to end
the turn - the same discipline as watching a model-training run, not glancing at it once and walking
away. Before ending a turn with `experiment_monitor` reporting anything other than terminal
(`done`/`failed`), arrange real continued monitoring: keep checking in the same turn at a cadence
that fits the experiment, or, when the session must end, schedule a concrete resumption (Claude Code:
`ScheduleWakeup`, or the `/loop` skill; another host: whatever its own equivalent is) rather than
ending the turn on the strength of a single check and hoping a person reopens the session. A host
that cannot name how it will be checked again has not finished this step.

This is the same obligation [`auto.md`](auto.md)'s "Auto research host obligation" states for the
rest of the research/experiment loop; this is the one node in that loop where real wall-clock time,
not just another artifact, stands between here and terminal.
</required>

## Routing & discovery

<routing>

- Use when the run is on `experiment_run` in `researcher-delivery`.
- Do not use for product `/build` work on `goal-delivery`.
</routing>
