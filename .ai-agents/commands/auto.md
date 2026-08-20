Drive one objective to a merged pull request without stopping for a person, except where a written rule says stop.

<context>

`/auto` is `/goal` with the approval gates answered by evidence instead of by you. It composes the
same graph, the same verifiers, and the same refusals. What changes is who confirms the spec, the
plan, and the merge.

Read [`goal.md`](goal.md) first. Everything it says about the runtime, evidence provenance, and the
delivery gates applies here unchanged, and this file does not restate it.

Boundary this mode moves, and where that is recorded: [`AGENTS.md`](../../AGENTS.md) section
"Delivery gates", bullet **Merge approval**.
</context>

## Runtime is required (MUST)

<required>

`/auto` runs on the runtime. There is no markdown-only mode, and **you may not stand in for it**.

```sh
vibe-agent doctor          # preflight; stop here if it reports problems
vibe-agent auto init       # writes the opt-in, once per workspace
```

If the binary is not on `PATH`, or `doctor` reports problems, **stop. Run no phase.** Report:

```text
/auto requires the vibe-agent runtime, which is not installed.
  bash scripts/install-runtime.sh                                      # macOS, Linux, Git Bash
  powershell -ExecutionPolicy Bypass -File scripts/install-runtime.ps1 # Windows
Then run `vibe-agent doctor` and start /auto again.
```

**Three things you must not do, in order of how much damage they cause:**

1. **Do not simulate the mode.** Walking the phases yourself and reporting them as done is the
   failure this whole design exists to remove. The graph decides what runs next, from recorded
   evidence, and the only way to move it is `vibe-agent verify` or `vibe-agent checkpoint`.
2. **Do not record `human_event` on a person's behalf.** Auto mode passes the approval gates by
   graph edges guarded on a flag, never by writing evidence that claims someone approved. There is
   no source for model assertion, and inventing one through the human source would remove the one
   property this control plane has that no surveyed harness has.
3. **Do not merge without the opt-in.** A workspace with no `.agent-state/auto.yaml`, or one whose
   file still says `merge: false`, has not agreed to it. Absence is a no.
</required>

## The opt-in

<procedure>

`vibe-agent auto init` writes `.agent-state/auto.yaml`. The questions live in the file rather than
in a terminal prompt: a file is answered in a diff somebody can review, and it is still there to
re-read later.

It writes `merge: false`. Someone changes it, or auto mode stops at a green pull request and a
person merges.

`vibe-agent doctor` reports which of the three states a workspace is in: no file, file answered no,
file answered yes.
</procedure>

## What auto may decide, and what it may not

<rules>

Auto mode may pass `intake`, `approve_spec`, and `approve_plan` on its own when the objective is
specific enough to spec without guessing. It may pass `approve_merge` only when **every** condition
below already holds. Any one of them missing means it stops and a person decides.

| # | Condition | Where the evidence comes from |
|---|---|---|
| 1 | The workspace opted in | `.agent-state/auto.yaml`, answered by a person |
| 2 | Required PR checks passed | The CI API, not a reading of logs |
| 3 | Every test the spec names passed | `vibe-agent verify`, end-to-end included where in scope |
| 4 | The linter is clean | No rule suppressed, no baseline widened, no test skipped to get there |
| 5 | `/ship` returned GO | [`ship.md`](ship.md) |
| 6 | The diff touches nothing on the danger list | The `pre-tool-use` gate |

**The danger list stops auto mode every time**, whatever the other five say: schema and data
migrations, data destruction, production writes, credential changes, history rewrites,
infrastructure destruction, and outward publication. Those need a person, and the gate refuses with
exit 2 rather than asking.

**Ambiguity is the other stop.** A goal that cannot be specified without inventing an acceptance
criterion is ambiguous, and auto mode stops at `approve_spec` and asks. That is a test on the spec,
not a judgement call about the prompt.
</rules>

## Not yet built

<context>

Being honest about the seam rather than describing a mode that does not exist yet.

**Live today:** the opt-in file, `vibe-agent auto init`, `doctor` reporting the opt-in state, the
danger gate's mechanism, and every verifier `/goal` already runs.

**Not yet:** the graph edges that skip the approval gates, and a single `vibe-agent auto --goal`
entry point. Until they land, use [`goal.md`](goal.md) and answer the gates yourself. This file
describes the contract those pieces will implement, so the contract is reviewable before the
machinery arrives.
</context>

## Routing & discovery

<routing>

- Master: [`ROUTER.md`](../ROUTER.md) → [`commands/ROUTER.md`](ROUTER.md).
- Use when the objective is routine enough to run unattended and the workspace has opted in.
- Use [`goal.md`](goal.md) when you want the approval gates, when the workspace has not opted in,
  or when the objective is exploratory.
- Do **not** use for anything on the danger list. It will stop, and stopping late costs more than
  not starting.
</routing>
