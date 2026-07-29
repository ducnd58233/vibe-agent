---
description: Audit AI asset health: routers, links, hook paths, permissions posture, and generated discovery paths
---

Run a repo-health audit for this AI asset toolkit.

## Checks

1. Read [`ROUTER.md`](../ROUTER.md), folder routers, and [`PERMISSIONS.md`](../PERMISSIONS.md).
2. Run router validation:
   - Windows: `powershell -File scripts/check-ai-agents-routers.ps1`
   - Bash: `bash scripts/check-ai-agents-routers.sh`
2b. Run the graph, schema, and runtime checks:
   - `python3 scripts/check-schemas.py` and `python3 scripts/check-graphs.py` (need `scripts/requirements.txt`)
   - `python3 scripts/check-graphs-test.py` so the checker itself is still rejecting broken graphs
   - `vibe-agent doctor` when the binary is installed: graphs load, memory database opens, every `tmp/<slug>/manifest.json` is schema-valid, and `/tmp/` plus `/.agent-state/` are gitignored
   - `cd runtime && go vet ./... && go test ./...` when changing the runtime
   - Report the binary as **not installed** rather than assuming it is absent-by-design; the runtime is optional, so a missing binary is a finding, not a failure
3. Check hook command paths referenced in `.claude/settings.json` and `.cursor/hooks.json` exist.
4. Check link/discovery paths:
   - `.claude/skills`, `.claude/agents`, `.claude/commands`
   - `.cursor/skills`, `.cursor/commands`
   - `.opencode/agents`, `.opencode/commands`
   - `.agents/skills`, `.agents/commands`, `.codex/agents`
5. Check broad permission patterns:
   - `Bash(*)`, `Edit(*)`, `Write(*)`, `WebFetch(domain:*)`, `mcp__*`
6. Check routing evals: step 2's validation asserts every **Expected asset** link in [`routing-evals.md`](../references/routing-evals.md) resolves. Spot-check that the listed intents still route to the expected asset after scope/router changes.
7. Report stale router rows, missing files, missing hooks, broad permissions, broken routing-eval targets, and recommended fixes.

## Required output

```markdown
## AI Asset Doctor

**Verdict:** PASS | WARN | FAIL

### Checks
| Check | Result | Evidence |

### Findings
- Severity, file/path, issue, recommended fix

### Next actions
```

## What

Audit the health of reusable AI assets and discovery/config wiring.

## Why

Finds drift before agents fail at runtime.

## How

Use the checks and required output above.

## When

Invoke after adding/removing assets, changing hooks, changing permissions, or cloning/linking a consumer repo.

## Routing & discovery

- Use when validating toolkit health.
- Do not use for product-code review; use [`review.md`](review.md) or [`ship.md`](ship.md).

## Permissions & authority

Inherits session permissions; should remain read-mostly except for explicitly requested fixes.
