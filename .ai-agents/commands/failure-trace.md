---
description: Write failure/TRACE.md on verifier fail or third-strike blocker, then choose refine_target and patch tasks before rebuild
---

On fail, write a failure TRACE so the next step resumes from the origin, not from
blind retry. Evidence stays files and existing checkpoint sources.

<references>

Schema: [`../references/failure-trace.md`](../references/failure-trace.md).

Research: `docs/*/when-delivery-or-research/*/WRITEUP-*.md`.
</references>

## When

<routing>

- Use when a verifier fails, or when recording a third-strike blocker, on `/auto` or `/goal`.
- Write TRACE **before** `vibe-agent checkpoint` / `verify` that leaves the failed node toward `build` or `plan`.
</routing>

## TRACE.md contract (MUST)

<required>

Write:

`.agent-state/runs/<date>/<slug>/<version>/failure/TRACE.md`

Include every required field from [`failure-trace.md`](../references/failure-trace.md), including
`refine_target` from the FailureClass map.

Then:

1. If `refine_target` is `plan` (or assumption ids are invalidated), patch `tasks.json` / `TASKS.md`.
2. Continue the graph (`checkpoint` / `verify`) toward rebuild or replan.
3. Stop at budget / `MaxBlockerAttempts` without inventing evidence.

```sh
vibe-agent run status --slug <slug>
# write TRACE, optionally patch tasks
vibe-agent checkpoint --slug <slug> --blocker "<reason>" --class <class>
# or vibe-agent verify when still on a verifier
```
</required>

## Routing & discovery

<routing>

- Master: [`ROUTER.md`](ROUTER.md).
- Pair with [`expectation.md`](expectation.md) / [`bug-hunt.md`](bug-hunt.md) when those verifiers fail (still write TRACE first).
- Do not use a judge LLM as Passed evidence.
</routing>
