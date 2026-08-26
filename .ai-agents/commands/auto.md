---
description: Unattended delivery loop - /goal with the approval gates answered by evidence, behind a workspace opt-in
---

Drive one objective to a merged pull request without stopping for a person, except where a written rule says stop.

<context>

`/auto` is `/goal` with the approval gates answered by evidence instead of by you. It composes the
same graph, the same verifiers, and the same refusals. What changes is who confirms the spec, the
plan, and the merge.

Read [`goal.md`](goal.md) first. Everything it says about the runtime, evidence provenance, and the
delivery gates applies here unchanged, and this file does not restate it.

Boundary this mode moves, and where that is recorded: [`AGENTS.md`](../../AGENTS.md) section
"Delivery gates", bullets **Merge approval** and **Auto `reviews` and `ship` (reversal)**. On
`/auto` only, `reviews` and `ship` pass from `ci_api` / `file_assert` evidence rather than from
`human_event`; `/goal` stays human at those nodes, and no new checkpoint evidence source was added.
</context>

## Runtime is required (MUST)

<required>

`/auto` runs on the runtime. There is no markdown-only mode, and **you may not stand in for it**.

```sh
vibe-agent doctor          # preflight; stop here if it reports problems
vibe-agent auto init       # writes the opt-in, once per workspace
vibe-agent auto "<objective from the user>"
vibe-agent auto research "<research topic from the user>"
vibe-agent auto gate --slug <slug from start output>
```

Host agents pass the user's text as plain arguments. Slug and graph are derived;
do not ask the user for `--goal`, `--graph`, or `--slug` unless resuming an
existing run.

If the binary is not on `PATH`, **stop. Run no phase.** Report:

```text
/auto requires the vibe-agent runtime, which is not installed.
  bash scripts/install-runtime.sh                                      # macOS, Linux, Git Bash
  powershell -ExecutionPolicy Bypass -File scripts/install-runtime.ps1 # Windows
Then run `vibe-agent doctor` and start /auto again.
```

If the binary runs but `doctor` reports problems, **stop. Run no phase.** Do **not**
claim the runtime is missing. Report the doctor failures and fix that workspace
first (check plan, docs layout, leftover `tmp/`, hooks). `merge: false` in the
auto opt-in is a note, not a doctor failure: auto may still run and stops at a
green PR for a person to merge.

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

## The task list (MUST)

<required>

Everything [`goal.md`](goal.md) says in its **The task list (MUST)** section applies here unchanged,
and unattended running makes it matter more rather than less: nobody is watching to notice that a
task was ticked late or not at all.

Two consequences specific to this mode:

- **A stale list stalls or loops the run.** `task_complete` decides on `tasks.json`. With a person
  driving, a wrong answer there gets spotted; with auto driving, the run simply walks another build
  cycle for a task that is already finished, and keeps doing it.
- **`auto gate` reads the plan document.** A task without acceptance criteria gives the spec and
  plan gates nothing to test, and a plan that still says `TBD` keeps the gate closed - which is the
  gate working, not a fault to route around.

Canonical rules: [`planning-and-task-breakdown`](../skills/planning-and-task-breakdown/SKILL.md),
section **Task status (MUST)**.
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

## What is built, and what auto does not do

<context>

Being honest about the seam rather than describing a mode that does not exist yet.

**Live today:** the opt-in file, `vibe-agent auto init`, `doctor` reporting the opt-in state, the
danger list refused before anything runs, every verifier `/goal` already runs, and the graph itself
- the `auto` flag routes through `simplify`, `lint`, `commit`, `bug_hunt` (`bug_hunt/FINDINGS.md`),
`expectation_review` (SPEC-tied `expectation/REVIEW.md`), `release_review` (`release/REVIEW.md`)
via `file_assert` (misses reopen `plan` or `build`), and a watch on the default branch after the
merge lands, and the `intake`, `approve_spec`, and `approve_plan` gates skip when the run's flags
say a person is not needed. A skipped gate records `skipped`, never `passed`, so run state still
says which gates a person answered. Host docs: [`bug-hunt.md`](bug-hunt.md),
[`expectation.md`](expectation.md), [`release.md`](release.md),
[`failure-trace.md`](failure-trace.md) (write TRACE on verifier fail or third-strike before replan).

```sh
vibe-agent auto "<one objective>"   # delivery graph; slug derived
vibe-agent auto research "<topic>"  # researcher-delivery graph
vibe-agent auto gate --slug <slug>  # answers gates from documents
vibe-agent auto init                # writes the opt-in, once per checkout
```

Host agents pass plain text; users never pass `--graph` or `--goal`.

`auto gate` sets the flag only when the document declares nothing open: a populated **Open
questions** section, or a `TBD` left in the prose. For `approve_applicability` it also requires
Applicability, Refine, and a Mermaid fence; for `approve_design` it also requires a Mermaid fence
on PLAN. An empty result is not a promise the document is complete - it is the most a text search
can claim - which is why the gate it opens records `skipped` and never `passed`.

On the auto path, `vibe-agent checkpoint` runs the same document tests when a run
lands on `approve_applicability` or `approve_design`, then walks past the gate.
Auto research therefore continues from literature to hypothesis and from
experiment design to `experiment_run` without a separate gate step. [`goal.md`](goal.md)
and `vibe-agent research` still park at those gates until a person approves.

A goal that arrives over MCP is fenced where it enters run state, with the warning before the
content, and content cannot close its own fence. Whoever filed that ticket is not the person
running this.

**Not yet:** nothing in this contract. What auto mode does **not** do is the work - the host coding
agent still runs every agent node exactly as it does under [`goal.md`](goal.md), and the runtime
still holds the evidence. Auto is a route through the same graph, not a second implementation of
it.
</context>

## Auto research host obligation (MUST)

<required>

On `vibe-agent auto research`, the host agent still runs every node. It must **not** stop after
literature and ask the human what to do next. Walk the graph through `hypothesis`,
`experiment_design`, `experiment_run`, `findings`, and `writeup`, calling `vibe-agent checkpoint`
after each artifact and `vibe-agent verify` at verifiers.

Stop only when:

- run status is terminal (`done`, `failed`, `budget_exceeded`), or
- a gate document leaves open markers (Open questions, TBD, or missing Applicability / Refine /
  Mermaid on RESEARCH, or missing Mermaid on PLAN).

When RESEARCH and PLAN are settled, `vibe-agent checkpoint` and `vibe-agent auto gate` both skip
the approval gates and advance the run. Report results when the loop finishes; do not poll the
human mid-pipeline.
</required>

## Routing & discovery

<routing>

- Master: [`ROUTER.md`](../ROUTER.md) → [`commands/ROUTER.md`](ROUTER.md).
- Use when the objective is routine enough to run unattended and the workspace has opted in.
- Use `vibe-agent auto research "<topic>"` for literature → experiment → findings loops.
- Use [`goal.md`](goal.md) when you want the approval gates, when the workspace has not opted in,
  or when the objective is exploratory.
- Do **not** use for anything on the danger list. It will stop, and stopping late costs more than
  not starting.
</routing>
