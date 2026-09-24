---
name: researcher-harness
description: >-
  Domain-agnostic research loop for vibe-agent: literature with mandatory
  Applicability and Mermaid, experiment plans with Mermaid, host/CI STATUS
  monitoring until done or failed, findings that cite runs. Use for AI, eng,
  finance, or non-code research on researcher-delivery. Not for product ship
  (use goal-driven-delivery).
disable-model-invocation: true
---

# Researcher harness

## How

<procedure>

1. **Literature** - citation-first digest. MUST include:
   - **Applicability** - how each source maps to *this* topic (reuse / reject / gap).
   - **Refine** - what to change before experiments.
   - A fenced `mermaid` literature or claim→method diagram.
2. **Hypothesis** - testable questions derived from Refine.
3. **Experiment design** - PLAN with Mermaid setup (data → protocol → metrics → stop), plus TASKS.
4. **Run** - host or CI only. Keep `experiment/STATUS.md` (`running|done|failed`).
5. **Monitor** - `vibe_verify` / `vibe_experiment_status` until terminal.
6. **Findings + writeup** - cite STATUS and artifacts; no orphan claims.

Anti-fabrication: no model assertion as check evidence. Gates use `file_assert` / `human_event` / `exit_code` / `ci_api` only.

GPU/sandbox: unsupported in-process. Document host/CI as the compute port.
</procedure>

## Experiment ledger, across runs (MUST when comparing iterations)

<required>

`experiment/STATUS.md` and `METRICS.json` under a delivery run's own
`.agent-state/runs/<date>/<slug>/<version>/` are scoped to **one graph
traversal** - they answer "is this run's experiment done, and did it pass,"
then the run finishes and that evidence stops mattering. Comparing many
iterations of a method for a paper, report, or competition submission needs a
record that outlives any one run.

Use `experiments/<project-slug>/<run-id>/`, a separate top-level convention -
not under `.agent-state/` (ephemeral, gitignored, local; wrong home for
something meant to be compared later) and not under one run's
`docs/<date>/<slug>/<version>/` (scoped to a single delivery, not a series):

```text
experiments/<project-slug>/<run-id>/
  config.json    # hyperparameters, model/version, dataset version, seed
  metrics.json    # {"metrics": {...}, "thresholds": {...}}
  SUMMARY.md       # one paragraph: what changed since the last run-id, and why
  code.sha         # git commit this run executed against
```

`config.json` and `metrics.json` each validate against
[`schemas/experiment-run.schema.json`](../../../schemas/experiment-run.schema.json)
(`$defs/config`, `$defs/metrics`) - the same `{metrics, thresholds}` shape
`experiment/METRICS.json` already uses, so one parser reads both, and every
run in a series shares the same fields to diff against. A worked example:
[`experiments/_example/001/`](../../../experiments/_example/001/).

Whether `experiments/` is gitignored is this project's own `AGENTS.md`
choice, the same as `docs/` already is - a research or competition repo will
usually track it, since comparing runs later for a writeup is the point.
</required>

## Routing & discovery

<routing>

- Graph: [`researcher-delivery`](../../graphs/researcher-delivery.yaml)
- Commands: [`research.md`](../../commands/research.md), [`experiment.md`](../../commands/experiment.md), [`findings.md`](../../commands/findings.md)
- Cursor rule: research Applicability + Mermaid MUST
- Prefer [`ai-research-methodology`](../ai-research-methodology/SKILL.md) only for AI/ML method detail overlays

Use for researcher workflows. Avoid when shipping product code through `goal-delivery`.
</routing>

## Permissions & authority

<required>

- Tools: Read, Grep, Glob, WebSearch, WebFetch, Bash (host experiment commands), MCP `vibe_experiment_status` / `vibe_verify`
- No forging `human_event`; auto gates use document structure tests
</required>
