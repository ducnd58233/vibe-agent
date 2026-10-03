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

Update it whenever progress changes. The `experiment_monitor` verifier fails while `running` (or missing) and passes on `done` or `failed` **only when a `judgement:` line is also present and recognized** (added below `status:`):

```markdown
judgement: confirmed
```

Allowed `judgement` values, stated once the run is terminal:

| Value | Means |
|---|---|
| `confirmed` | The observed result matches the hypothesis/assumption this run was testing. |
| `refuted` | The observed result contradicts it. |
| `inconclusive` | The run finished but the evidence does not clearly support or contradict it. |
| `not_applicable` | There was no hypothesis to score against (a reproduce-and-fix cycle, not a research experiment). |

`not_applicable` is a legitimate answer, not a workaround: the point is stating whether a judgement
was made, never inventing one against a hypothesis that was never there. A terminal status with no
`judgement:` line, or one with a value outside this list, fails `experiment_monitor` the same way a
missing `status:` line always has - this is not a new evidence source, it is the same STATUS.md
contract the verifier already gates, asking one more honest question before it calls the file done.

When `status` becomes `done`, also write `experiment/METRICS.json`:

```json
{
  "metrics": {"ndcg_at_10": 0.84},
  "thresholds": {"ndcg_at_10": {"op": ">=", "value": 0.82}}
}
```

The `results_eval` verifier compares metrics to thresholds. Values below the bar route the graph back to `hypothesis` without human approval.

`METRICS.json` also carries an `integrity` block, and the verifier fails a file that has none. A run that fits or tunes a model, a prompt, or a rule and reports a number from held-out data writes:

```json
"integrity": {
  "kind": "held_out_eval",
  "selectionSplit": "validation",
  "reportedSplit": "test",
  "selectionMetrics": {"ndcg_at_10": 0.86},
  "maxGap": {"ndcg_at_10": 0.03},
  "trials": 14,
  "reportedSplitEvaluations": 1
}
```

A run with no model, split, or tuning step (a reproduce-and-fix cycle, a closed-form check) writes `{"kind": "not_applicable", "reason": "<why>"}`; a blank reason fails. The verifier fails a gap larger than `maxGap` in either direction, a reported split scored more than once, and a selection split equal to the reported one. Schema: [`experiment-run.schema.json`](../../schemas/experiment-run.schema.json).

**Comparing this run against earlier iterations (not just gating this one):** this STATUS.md/METRICS.json pair is scoped to the current graph run and stops mattering once it finishes. To keep a comparable record across many iterations for a paper, report, or competition writeup, also write `experiments/<project-slug>/<run-id>/` per [`researcher-harness`](../skills/researcher-harness/SKILL.md) section "Experiment ledger, across runs".
</required>

## Integrity (MUST)

<required>

Rules and reasons: [`research-integrity`](../references/research-integrity.md). The ones that bind this command:

- **Run the frozen PLAN.** Splits, metric, thresholds, `maxGap`, and trial budget come from the approved PLAN's Evaluation protocol. Copy them into `METRICS.json`; do not choose them after seeing a result.
- **Never edit the evaluator to pass.** No change to metric code, a grader, a threshold, labels, the eval set, or a test in the change that produces the result.
- **Count, do not recall.** `trials` and `reportedSplitEvaluations` come from the experiment ledger (`experiments/<project-slug>/`), counting failed and abandoned runs.
- **Report coverage.** Samples skipped by an error or timeout are reported as evaluated over total, not dropped from the average.
- **An `INTEGRITY:` failure is not a miss to retry.** Looping to `hypothesis` re-scores the reported split. Correct the record if it was wrong, re-split or bring fresh held-out data if the split is spent, or stop and ask a person. Never record it as `checkpoint --blocker`, and never re-run until it passes.
</required>

## How

<procedure>

1. Read PLAN Mermaid, Evaluation protocol, Data and terms, and TASKS acceptance criteria.
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
that fits the experiment, or, when the session must end, schedule a concrete resumption rather than
ending the turn on the strength of a single check and hoping a person reopens the session. A host
that cannot name how it will be checked again has not finished this step.

**This backstop is not the same on every host (MUST read before assuming it is).** vibe-agent
supports seven host clients; [`host-hook-contracts.md`](../references/host-hook-contracts.md) is the
generated, measured record of what each one's `Stop`/end-of-turn hook actually does, and it is not
uniform:

| Host | Can refuse to end a turn mid-graph | Concrete resumption mechanism |
|---|---|---|
| Claude Code | Yes, measured | `ScheduleWakeup`, or the `/loop` skill |
| Cursor | Wired, unverified whether it fires from this config | Whatever Cursor's own scheduling/background-task surface is; unverified here |
| Codex | Wired, unverified | Whatever Codex's own equivalent is; unverified here |
| opencode | **No end-of-turn hook at all** - confirmed, not merely untested | None available from the runtime; this written MUST is the *only* thing holding, see below |
| Antigravity | Wired, unverified | Unverified here |
| Kimi | Wired, unverified | Unverified here |
| Muse | Wired, unverified | Unverified here |

On **opencode**, or any host where the resumption mechanism is unverified, do not assume a hook will
catch an abandoned turn - `host-hook-contracts.md` already documents, per host, exactly what each one
does and does not provide; read that host's section before deciding there is nothing more to do. Where
no host-native scheduling exists, the only honest options are: keep polling synchronously within the
same turn until terminal, or end the turn only after writing a STATUS.md note that says plainly the
experiment is unattended and when a person should check back - never end it silently on the strength
of one check.

This is the same obligation [`auto.md`](auto.md)'s "Auto research host obligation" states for the
rest of the research/experiment loop; this is the one node in that loop where real wall-clock time,
not just another artifact, stands between here and terminal.
</required>

## Routing & discovery

<routing>

- Use when the run is on `experiment_run` in `researcher-delivery`.
- Do not use for product `/build` work on `goal-delivery`.
</routing>
