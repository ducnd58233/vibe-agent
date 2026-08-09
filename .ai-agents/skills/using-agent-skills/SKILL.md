---
name: using-agent-skills
description: >-
  Meta-skill: discover current skills, agents, commands, references, hooks, and stack profiles through ROUTER tables instead of memorized lists; compose workflows and apply core behaviors. Use at session start or whenever choosing workflows for the current workspace.
disable-model-invocation: true
---

# Using Agent Skills

## Canonical source

<references>

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
</references>

## Precedence (check before routing)

<procedure>

The workspace root wins; this toolkit is the fallback. Resolve most specific first:

1. Explicit instruction in the current session.
2. Workspace-root agent rules — `AGENTS.md`, `CLAUDE.md`, `CLAUDE.local.md`, `.cursor/rules/`, or the harness equivalent.
3. Conventions already in the consumer repo — its own `TEMPLATE.md`, existing file/folder patterns, lint and formatter config.
4. This toolkit's `.ai-agents/` assets.

**Detect before assuming:** look for levels 2–3 at the workspace root before applying a toolkit default. On conflict, follow the local rule and state the divergence; a local rule may tighten a safety, permission, or verification boundary, but if it would weaken one, surface the conflict instead of applying it. Full rule: "Local-first precedence" in root [`../../../AGENTS.md`](../../../AGENTS.md).
</procedure>

## Mental model

<rules>

- **Skills** define reusable workflow: how to do work.
- **Agents/personas** define isolated perspective: who reviews or investigates.
- **Commands** define repeatable entrypoints: when to run a workflow.
- **References** define generic checklists and patterns.
- **Stack profiles** define pinned frameworks, tools, commands, and repo-specific conventions.
- **Hooks** define lifecycle automation.

The **user or command** orchestrates. Personas do not call personas; see [`../../references/orchestration-patterns.md`](../../references/orchestration-patterns.md).
</rules>

## Workflow

<procedure>

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
</procedure>

## Common routing shortcuts

<rules>

Use routers for the authoritative list, but these family-level shortcuts are stable:

- Need facts or current docs: route to research/source-driven skills and references.
- Need implementation: route to spec/plan/build/test skills and matching stack profiles.
- Need review/ship decision: route to commands and agents, especially `/review` or `/ship`.
- Need UI/design work: route to frontend/product-design skills plus design/front-end/mobile profiles.
- Need data, DevOps, QA, security, or performance: route to the matching specialized skill/profile rows.
- Need toolkit maintenance: route to `/doctor`, `/harden`, and `agent-systems-auditor`.
</rules>

## Verification

<verification>

- [ ] Workspace-root rules and templates checked before applying toolkit defaults; any divergence stated.
- [ ] Master router consulted.
- [ ] Relevant folder router consulted.
- [ ] Matching stack profiles loaded when implementation details are stack-specific.
- [ ] No stale hardcoded asset list was used instead of routers.
- [ ] Orchestration stayed user/command-driven, not persona-to-persona.
</verification>

## Routing & discovery

<routing>

- Use when choosing between skills, commands, agents, references, stack profiles, or hooks.
- Do not use when a downstream skill or command has already been explicitly selected and validated.

Use at session start or whenever workflow/asset selection is unclear.
</routing>

## Permissions & authority

<required>

- Tools: read-only documentation/routing guidance.
- Authority: no extra permissions beyond reading local markdown assets.
</required>
