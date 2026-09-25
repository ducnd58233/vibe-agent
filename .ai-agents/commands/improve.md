---
description: Before a run ends, report confirmed memories reused enough to become a reviewed rule
---

At the `improve` node of the delivery graph, surface what the workspace keeps relying on, so a person
can turn it into a durable, reviewed rule. This command proposes; it never edits a rule file.

<references>

Graph node: `improve` in [`goal-delivery.yaml`](../graphs/goal-delivery.yaml), on both `/goal` and
`/auto`, after the last `task_complete` and before `done`.

Promotion logic: `ProposePromotions` in `runtime/internal/memory/domain/policy.go` (confirmed and
reused `PromotionThreshold` times). Adapted from ECC's `improve` step; not adopted: generating skills
or rules automatically from memory.
</references>

## Procedure

<procedure>

1. Run `vibe-agent memory promotions`.
2. For each promotion, report the memory, its target (for example `AGENTS.md` or a stack profile),
   and the reason, in the run's final summary. If the rule belongs in this repository, it lands as its
   own reviewed change, never as part of closing this run.
3. `.agent-state/MISTAKES.md` graduation (four or five repeats of one class) is the other half of
   improve; check it here too ([`mistakes-log.md`](../references/mistakes-log.md)).
4. Checkpoint the node: `vibe-agent checkpoint -slug <slug>`.

No promotions is a normal result.
</procedure>

## Routing & discovery

<routing>

- Use when the run is on `improve`.
- Do not use to edit `AGENTS.md`, a skill, or a command directly from memory.
</routing>
