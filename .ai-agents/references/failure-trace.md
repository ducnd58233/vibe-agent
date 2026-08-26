# Failure TRACE schema

<context>

Host convention for answering "a graph node failed; from where?" and where to
resume fixing, without a new checkpoint `--source`. Research:
`docs/2026-08-26/graph-node-failure-provenance/1/` and
`docs/2026-08-26/when-delivery-or-research/1/`.
</context>

## When to write

<rules>

Write `.agent-state/runs/<date>/<slug>/<version>/failure/TRACE.md` when recording
a blocker (especially the third strike) or when a verifier fails and the host
will replan or rebuild. On `/auto` and `/goal`, write TRACE **before** the
checkpoint that leaves the failed node. See [`../commands/failure-trace.md`](../commands/failure-trace.md).
</rules>

## Required fields

| Field | Filled from |
|-------|-------------|
| `run_id` | `manifest.json` `runId` |
| `slug` | manifest |
| `failed_node` | `Blocker.Node` or `currentNode` |
| `failure_class` | `Blocker.Class` (`context`, `tool`, `permission`, `test`, `ambiguity`, `model`) |
| `symptom` | `Blocker.Reason` or verifier log basename |
| `events_ref` | `events.ndjson#<sequence>` |
| `upstream_artifacts` | Relative paths under `docs/<date>/<slug>/<version>/` |
| `assumption_ids` | Ids (`A1`..) listed in those artifacts; empty if none |
| `checks_failed` | Check keys with `passed: false` and not skipped |
| `evidence_sources` | Only `exit_code`, `file_assert`, `ci_api`, `human_event` already on those checks |
| `refine_target` | `build` \| `plan` \| `retry` (see map below) |

## FailureClass → refine_target

| Class | Default `refine_target` |
|-------|-------------------------|
| test, model | `build` |
| ambiguity | `plan` |
| tool, permission | `retry` (same node after env fix) |
| context | `retry` after restoring SPEC/PLAN context |

## Task patch

When `refine_target` is `plan`, or when assumption ids are invalidated, patch
`tasks-<date>.json` and `TASKS.md` before rebuild: insert or reorder tasks that
name the broken assumption or failing acceptance criterion. Do not treat
assumption truth as a Passed check. Done tasks settle only when each section has
an `**Acceptance criteria:**` block with every checkbox checked; status alone
does not clear `tasks_remaining`.

## Non-goals

<rules>

- Do not record "assumption still true" as a Passed check.
- Do not invent a fifth `--source`.
- Do not embed a language-specific root-cause engine in the Go control plane.
</rules>

## CLI

```sh
vibe-agent checkpoint --slug <slug> --blocker "<reason>" --class <class>
```

`--class` is required with `--blocker`.

