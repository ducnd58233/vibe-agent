# Failure TRACE schema

<context>

Host convention for answering "a graph node failed; from where?" without a new
checkpoint `--source`. Research: `docs/2026-08-26/graph-node-failure-provenance/1/`.
</context>

## When to write

<rules>

Write `.agent-state/runs/<date>/<slug>/<version>/failure/TRACE.md` (or `.json`
with the same fields) when recording a blocker, especially on the third strike
(`MaxBlockerAttempts`), or when a verifier fails and the host stops to diagnose.
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

## Non-goals

<rules>

- Do not record "assumption still true" as a Passed check.
- Do not invent a fifth `--source`.
- Do not embed a language-specific root-cause engine in the Go control plane.
</rules>

## Assumption ids

SPEC and PLAN may list `A1`, `A2`, ... with a short statement. TRACE may cite
those ids when a failure invalidates them. Invalidation creates a task or graph
replan; it never calls `checkpoint --passed` on assumption validity.

## CLI

```sh
vibe-agent checkpoint --slug <slug> --blocker "<reason>" --class <class>
```

`--class` is required with `--blocker`.
