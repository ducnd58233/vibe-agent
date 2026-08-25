---
description: Write bug_hunt/FINDINGS.md on the auto path after e2e, then verify bug_hunt_ok
---

On the auto delivery path, after e2e and before slop, record bug-hunt findings.
Evidence is a markdown file; the runtime only `file_assert`s it. Compute stays
in consumer CI or an external runner; the runtime does not own a sandbox.

<references>

Graph node: `bug_hunt` in [`goal-delivery.yaml`](../graphs/goal-delivery.yaml).

Check: `bug_hunt_ok` in workspace `vibe-checks.yaml`.

Research basis: `sau-khi-merge-ci` experiment `BUG-HUNT-SKETCH.md`.
</references>

## When

<routing>

- Use when the run is on `bug_hunt` (auto path only).
- Host may run fuzz/property/SWE jobs first; this command only records the verdict file.
</routing>

## FINDINGS.md contract (MUST)

<required>

Write:

`.agent-state/runs/<date>/<slug>/<version>/bug_hunt/FINDINGS.md`

```markdown
# Bug hunt
status: pass
attempt: 1

| Case | Evidence | result |
|------|----------|--------|
| none new | e2e green | pass |
```

Same pass/fail and soft-cap rules as [`expectation.md`](expectation.md).

```sh
vibe-agent verify --slug <slug>
```

Pass continues to `slop`. Fail reopens `plan`.
</required>
