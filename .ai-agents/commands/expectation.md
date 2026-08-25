---
description: Write SPEC-tied expectation/REVIEW.md on the auto path, then verify expectation_ok
---

On the auto delivery path, after slop and before review, assert that the current
task still matches SPEC acceptance criteria. Evidence is a markdown file under
the run directory; the runtime only `file_assert`s it.

<references>

Graph node: `expectation_review` in [`goal-delivery.yaml`](../graphs/goal-delivery.yaml).

Check: `expectation_ok` in workspace `vibe-checks.yaml`.

Research basis: `docs/*/how-other-agent-harnesses/*/experiment/EXPECTATION-REVIEW-SKETCH.md`.
</references>

## When

<routing>

- Use when the run is on `expectation_review` (auto path only).
- Do not use on `/goal` without the auto flag; that path goes `slop` → `review`.
- Do not treat a model opinion as the check: write the file, then `vibe-agent verify`.
</routing>

## REVIEW.md contract (MUST)

<required>

Write:

`.agent-state/runs/<date>/<slug>/<version>/expectation/REVIEW.md`

```markdown
# Expectation review
status: pass
attempt: 1
updated: <RFC3339>

| AC id | Spec reference | Observed evidence path | result |
|-------|----------------|------------------------|--------|
| AC1   | SPEC §...      | unit/ or a file path   | pass   |
```

Rules:

1. One row per SPEC acceptance criterion the current task claims.
2. `result` is `pass` or `fail` only. A fail row requires a real evidence path (log, screenshot, missing file).
3. Top-level `status: pass` only when every row is `pass`; otherwise `status: fail`.
4. Increment `attempt` on each revisit after a miss. Soft cap is **2**. After the second fail, stop and ask a person rather than refining forever. The verifier notes the cap in its summary when `attempt` is above 2.

Then:

```sh
vibe-agent verify --slug <slug>
```

Pass advances to `review`. Fail resets plan approval and related checks and returns to `plan` so tasks can be refined.
</required>

## Soft attempt cap

<rules>

The graph may loop. The soft cap is a host rule, not a new evidence source. After two consecutive expectation fails on the same task, do not invent a third REVIEW that claims pass without new evidence. Record a blocker or ask the human.
</rules>
