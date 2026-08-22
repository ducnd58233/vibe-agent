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
</required>

## How

<procedure>

1. Read PLAN Mermaid and TASKS acceptance criteria.
2. Run the next host/CI step the plan names.
3. Refresh STATUS.md before returning.
4. Call `vibe_verify` at `experiment_monitor` (or let the host loop) until terminal.
</procedure>

## Routing & discovery

<routing>

- Use when the run is on `experiment_run` in `researcher-delivery`.
- Do not use for product `/build` work on `goal-delivery`.
</routing>
