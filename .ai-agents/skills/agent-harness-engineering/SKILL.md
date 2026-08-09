---
name: agent-harness-engineering
description: >-
  Designs, audits, and improves AI coding-agent harnesses: instructions, routers, skills, agents, commands, context selection, tool access, permissions, hooks, deterministic sensors, evaluation loops, observability, failure attribution, and intervention records. Use when building or hardening agent workflows, Codex/Claude/Cursor/opencode discovery paths, MCP/tool boundaries, generated agent assets, or reusable AI-agent toolkit infrastructure.
disable-model-invocation: true
---

# Agent Harness Engineering

## How

<procedure>

1. **Route and scope**
   - Start at [`../../ROUTER.md`](../../ROUTER.md), then open relevant folder routers.
   - Identify whether the task is authoring, hardening, tool wiring, evaluation, or generated-asset validation.
2. **Map the harness**
   - List guides: `AGENTS.md`, routers, skills, commands, stack profiles, references, tool-specific configs.
   - List sensors: tests, linters, hooks, validation scripts, audits, CI checks, eval prompts.
   - List actuators: edit tools, shell commands, MCP servers, app connectors, deployment tools.
   - List boundaries: permissions, deny rules, path scopes, approval gates, secret rules.
3. **Check responsibilities**
   - Open [`../../references/agent-harness-engineering.md`](../../references/agent-harness-engineering.md).
   - Audit task specification, context selection, tool access, task state, observability, failure attribution, verification, permissions, entropy auditing, and intervention recording.
4. **Prefer computational sensors**
   - Add or improve deterministic checks before adding more prompt text when the rule can be validated by script, test, typecheck, linter, schema, router check, or generated-asset check.
   - Use inferential review for semantic judgment after deterministic evidence exists.
4b. **Audit the control plane when one exists**
   - Read [`loop-and-graph-engineering.md`](../../references/loop-and-graph-engineering.md) for the inner/outer loop boundary and when a workflow earns a graph.
   - Check who owns the outer loop: are transitions enforced, or described in prose and interpreted?
   - Check evidence provenance: can any path mark a check passed from model output? There must be none.
   - Check run state: can a fresh session resume from a file, or does progress live in chat history?
   - Check human gates: do they precede irreversible actions, or merely record them afterwards?
   - Check budgets: does every loop terminate, and is every node able to reach a terminal?
   - Check memory writes: does anything store secrets, task state, or unevidenced guesses?
5. **Keep assets canonical**
   - Edit canonical files under `.ai-agents/`.
   - After creating, renaming, or deleting assets, update the relevant `ROUTER.md`.
   - After agent changes, regenerate linked/generated tool assets with `scripts/link-ai-agents.*`.
6. **Verify**
   - Run `powershell -File scripts/check-ai-agents-routers.ps1` or `bash scripts/check-ai-agents-routers.sh`.
   - For Codex-facing assets, run `powershell -File scripts/check-codex-assets.ps1`.
   - For graphs and schemas, run `python3 scripts/check-graphs.py` and `python3 scripts/check-schemas.py`.
   - For runtime changes, run `cd runtime && go vet ./... && go test ./...`.
   - Include checks run, failures found, and residual risks in the final report.
</procedure>

## Routing & discovery

<routing>

- Pair with [`using-agent-skills`](../using-agent-skills/SKILL.md) when asset selection is unclear.
- Pair with [`context-engineering`](../context-engineering/SKILL.md) for large-context or retrieval problems.
- Pair with [`security-and-hardening`](../security-and-hardening/SKILL.md) for secret boundaries and dangerous tools.
- Pair with [`devops-platform-delivery`](../devops-platform-delivery/SKILL.md) when sensors run in CI/CD.
- Delegate review to [`../../agents/agent-systems-auditor.md`](../../agents/agent-systems-auditor.md) for independent harness audit.

Use for AI-agent toolkit changes, consumer-repo harness setup, MCP/tool integration boundaries, routing drift, generated Codex agent issues, hook/permission hardening, and evaluation coverage.
Do not use for ordinary application implementation unless the agent workflow or tooling around that implementation is the thing being changed.
</routing>

## Permissions & authority

<required>

- Tools: Read, Grep, Glob, Edit; Shell only for repo-documented validation or link scripts.
- Paths: `.ai-agents/**`, tool config directories, scripts, docs, CI config; never secret paths.
- Ask before changing broad permissions, enabling new MCP servers, running destructive commands, deploying, or mutating external systems.
</required>

## Verification

<verification>

- [ ] Relevant routers consulted and updated.
- [ ] Guides and sensors mapped.
- [ ] Deterministic checks preferred where possible.
- [ ] Permissions and approval boundaries documented.
- [ ] Generated tool assets validated when applicable.
- [ ] Final report includes evidence and unresolved risks.
</verification>
