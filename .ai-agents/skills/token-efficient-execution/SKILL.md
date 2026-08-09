---
name: token-efficient-execution
description: >-
  Reduces unnecessary token usage while preserving correctness and signal. Use
  for high-volume execution loops, automation pipelines, and repetitive coding
  workflows where concise outputs and strict scope discipline reduce cost and
  noise.
---

# Token-Efficient Execution

## Overview

<context>

This skill applies practical token-efficiency rules without sacrificing correctness:

- keep responses concise by default
- avoid repetitive restatement and conversational filler
- constrain edits to requested scope
- prefer deterministic outputs over verbose explanation in routine flows

The objective is **net token savings with no quality regression**.
</context>

## Inputs

<inputs>

- User task and acceptance criteria
- Relevant repo files and routing docs
- Execution context (interactive explanation vs automation/high-volume loop)
</inputs>

## Outputs

<outputs>

- Minimal, actionable responses
- Focused code/file edits with low churn
- Verification evidence (tests/checks) instead of long narrative
</outputs>

## When to use

<routing>

Use when:

- Running repetitive engineering workflows where output volume compounds cost.
- Producing machine-consumable or parse-friendly summaries.
- The user asks for concise, direct execution.

Do not use when:

- The user explicitly requests deep explanation, alternatives, or teaching mode.
- Architectural decisions require richer tradeoff discussion.
</routing>

## Core rules

<procedure>

1. **Concise by default**
   - Do not add greetings, praise, or unnecessary restatements.
   - Lead with result, then only essential context.
2. **Scope control**
   - Touch only files and behavior required for the task.
   - Avoid speculative refactors and optional extras unless requested.
3. **Single-pass context**
   - Read what is needed; avoid re-reading unchanged files.
   - Prefer targeted lookups over broad scans once location is known.
4. **Structured brevity**
   - Prefer compact bullet summaries over long prose.
   - Use explicit checklists for verification outcomes.
5. **Override on demand**
   - If the user asks for details, expand depth immediately.

## Execution flow

1. Clarify the deliverable and acceptance criteria.
2. Gather only the context required to execute safely.
3. Implement or analyze with minimal surface area.
4. Verify using concrete checks.
5. Report outcome in concise, high-signal form.
</procedure>

## Verification

<verification>

- [ ] Output is concise and directly task-aligned.
- [ ] No unsolicited scope expansion.
- [ ] Verification evidence is present where applicable.
- [ ] User-requested verbosity overrides are respected.
</verification>

## Routing & discovery

<routing>

- Use when output efficiency, low-noise delivery, and strict scope are primary.
- Do not use as a hard cap when the user requests deep reasoning detail.
- This skill governs **output** economy; to curate **input** context (which files/retrieval load into the window), use [`context-engineering`](../context-engineering/SKILL.md).

Invoke in high-volume or execution-focused workflows where concise outputs are
beneficial and detail can remain on-demand.
</routing>
