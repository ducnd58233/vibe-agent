---
name: using-agent-skills
description: >-
  Meta-skill: discover current skills, agents, commands, references, hooks, and stack profiles through ROUTER tables instead of memorized lists; compose workflows and apply core behaviors. Use at session start or whenever choosing workflows for the current workspace.
disable-model-invocation: true
---

# Using Agent Skills

## Canonical source

Do **not** rely on hardcoded skill/agent/command lists in this file. The current asset inventory lives in router tables:

1. Start with [`../../ROUTER.md`](../../ROUTER.md) to choose the asset family.
2. Open the relevant folder router:
   - Skills: [`../ROUTER.md`](../ROUTER.md)
   - Agents: [`../../agents/ROUTER.md`](../../agents/ROUTER.md)
   - Commands: [`../../commands/ROUTER.md`](../../commands/ROUTER.md)
   - References: [`../../references/ROUTER.md`](../../references/ROUTER.md)
   - Stack profiles: [`../../stack-profiles/ROUTER.md`](../../stack-profiles/ROUTER.md)
   - Hooks: [`../../hooks/ROUTER.md`](../../hooks/ROUTER.md)
3. Load only the matching assets and references needed for the task.

Product/domain expectations live in root [`../../../AGENTS.md`](../../../AGENTS.md). Skills stay stack-agnostic; stack-specific details live in matching stack profiles.

## Mental model

- **Skills** define reusable workflow: how to do work.
- **Agents/personas** define isolated perspective: who reviews or investigates.
- **Commands** define repeatable entrypoints: when to run a workflow.
- **References** define generic checklists and patterns.
- **Stack profiles** define pinned frameworks, tools, commands, and repo-specific conventions.
- **Hooks** define lifecycle automation.

The **user or command** orchestrates. Personas do not call personas; see [`../../references/orchestration-patterns.md`](../../references/orchestration-patterns.md).

## Workflow

1. **Route**
   - Read the master router, then the matching folder router.
   - Select every matching skill/profile/reference row by task intent.
2. **Compose**
   - Combine assets only when each adds a distinct role.
   - Prefer one skill plus matching stack profiles for ordinary work.
   - Use commands for repeated multi-step flows such as spec/plan/build/ship.
3. **Constrain context**
   - Read manifests and router rows before deep file reads.
   - Load optional references only when their router row or current task applies.
4. **Execute with guardrails**
   - Surface assumptions before non-trivial work.
   - Prefer simple, scoped changes.
   - Stop on conflicting instructions or unsafe ambiguity.
   - Verify with tests, checks, runtime evidence, or cited sources.
5. **Maintain routers**
   - After creating, renaming, or deleting assets, update the corresponding router in the same change.
   - Run `powershell -File scripts/check-ai-agents-routers.ps1` or `bash scripts/check-ai-agents-routers.sh`.

## Common routing shortcuts

Use routers for the authoritative list, but these family-level shortcuts are stable:

- Need facts or current docs: route to research/source-driven skills and references.
- Need implementation: route to spec/plan/build/test skills and matching stack profiles.
- Need review/ship decision: route to commands and agents, especially `/review` or `/ship`.
- Need UI/design work: route to frontend/product-design skills plus design/front-end/mobile profiles.
- Need data, DevOps, QA, security, or performance: route to the matching specialized skill/profile rows.
- Need toolkit maintenance: route to `/doctor`, `/harden`, and `agent-systems-auditor`.

## Verification

- [ ] Master router consulted.
- [ ] Relevant folder router consulted.
- [ ] Matching stack profiles loaded when implementation details are stack-specific.
- [ ] No stale hardcoded asset list was used instead of routers.
- [ ] Orchestration stayed user/command-driven, not persona-to-persona.

## What

Meta-skill for selecting and composing the current router-listed assets.

## Why

Prevents stale duplicated routing tables inside skills and keeps asset discovery centralized in `ROUTER.md` files.

## How

Use the router-first workflow and verification checklist above.

## When

Use at session start or whenever workflow/asset selection is unclear.

## Routing & discovery

- Use when choosing between skills, commands, agents, references, stack profiles, or hooks.
- Do not use when a downstream skill or command has already been explicitly selected and validated.

## Permissions & authority

- Tools: read-only documentation/routing guidance.
- Authority: no extra permissions beyond reading local markdown assets.
