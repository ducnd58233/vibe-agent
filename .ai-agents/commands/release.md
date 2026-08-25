---
description: Write release/REVIEW.md on the auto path after ship, then verify release_ok
---

On the auto delivery path, after ship and before approve_merge, assert release
readiness gates. Evidence is a markdown file under the run directory; the
runtime only `file_assert`s it.

<references>

Graph node: `release_review` in [`goal-delivery.yaml`](../graphs/goal-delivery.yaml).

Check: `release_ok` in workspace `vibe-checks.yaml`.

Research basis: `sau-khi-merge-ci` experiment `RELEASE-READINESS-SKETCH.md`.
</references>

## When

<routing>

- Use when the run is on `release_review` (auto path only).
- Do not treat a model opinion as the check: write the file, then `vibe-agent verify`.
</routing>

## REVIEW.md contract (MUST)

<required>

Write:

`.agent-state/runs/<date>/<slug>/<version>/release/REVIEW.md`

```markdown
# Release readiness
status: pass
attempt: 1

| Gate id | Evidence source | Pointer | result |
|---------|-----------------|---------|--------|
| R1 | file_assert | ship/DECISION.md | pass |
| R2 | ci_api | main CI tip | pass |
```

Rules match [`expectation.md`](expectation.md): every result cell pass/fail; soft attempt cap **2**.

```sh
vibe-agent verify --slug <slug>
```

Pass advances to `approve_merge`. Fail routes to `build`.
</required>
