---
description: After a task merges, propose what it taught to memory.db through the memory policy
---

At the `remember` node of the delivery graph, write down what this task taught that a later session,
or another harness sharing this workspace, should recall. It goes to `memory.db`, never to a file.

<references>

Graph node: `remember` in [`goal-delivery.yaml`](../graphs/goal-delivery.yaml), on both `/goal` and
`/auto`, after the merge and before `task_complete`.

Boundary and rules: [`runtime/AGENTS.md`](../../runtime/AGENTS.md) section "Agent memory boundary".

Adapted from ECC's `remember` lifecycle step. Not adopted: ECC's model-assigned confidence and
background extraction, because model assertion is not evidence here.
</references>

## Procedure

<procedure>

1. Look back over this task's run: blockers, failed verifiers, workarounds that worked, anything you
   had to rediscover. `vibe-agent run status --slug <slug>` and the run's events show them.
2. For each lesson worth keeping, propose one atomic claim with the observation that supports it:

   ```sh
   vibe-agent memory propose --kind correction \
     --content "gh pr checks exits 8 while checks are pending, not on failure" \
     --evidence "gh pr checks 184 printed pending and exited 8" \
     --source-type command_result --client <your host> --model <your model>
   ```

   MCP hosts can use `vibe_memory_propose` with the same fields.
3. A rejection (no evidence, a hedge, a credential) is the policy working; fix the claim or drop it.
4. Checkpoint the node: `vibe-agent checkpoint -slug <slug>`.

Nothing learned is a valid outcome. Do not invent a lesson to fill the step, and do not repeat what
the hook journal already recorded for a failed command.
</procedure>

## Routing & discovery

<routing>

- Use when the run is on `remember`.
- Do not use to write notes into `docs/`, a README, or a comment; that is the boundary this command
  exists to keep.
</routing>
