---
description: Harden AI assets: permissions, hooks, tool boundaries, secret safety, and orchestration risks
---

Run a focused hardening pass on AI assets, tool permissions, and orchestration boundaries.

## Scope

Review:

- [`.ai-agents/PERMISSIONS.md`](../PERMISSIONS.md)
- [`.claude/settings.json`](../../.claude/settings.json)
- [`.cursor/hooks.json`](../../.cursor/hooks.json)
- [`agents/`](../agents/)
- [`commands/`](../commands/)
- [`hooks/`](../hooks/)
- [`references/tool-safety-and-permissions.md`](../references/tool-safety-and-permissions.md)
- [`references/orchestration-patterns.md`](../references/orchestration-patterns.md)

## Checks

1. Broad allow rules and missing deny/ask controls.
2. Hook script existence, side effects, and tests/smoke checks.
3. Subagent `tools:` maps against router tool scopes.
4. Commands that imply risky operations without approval language.
5. Secret path exposure and logging risks.
6. Persona orchestration anti-patterns.

## Required output

```markdown
## AI Asset Hardening Report

**Verdict:** ACCEPTABLE | HARDENING NEEDED | UNSAFE

### Critical
### Important
### Suggestions
### Positive controls
### Verification run
```

## What

Review and improve safety boundaries for AI assets and tool use.

## Why

Reusable agent systems amplify permission mistakes; hardening keeps workflows portable and safe.

## How

Use the checks and severity report above.

## When

Invoke after adding commands, agents, hooks, permissions, or external-tool workflows.

## Routing & discovery

- Use for AI-asset/tool-safety review.
- Do not use for ordinary app security review; use [`security-and-hardening`](../skills/security-and-hardening/SKILL.md).
