---
description: "Audit AI asset health: routers, links, hook paths, permissions posture, and generated discovery paths"
---

Run a repo-health audit for this AI asset toolkit.

## Checks

<verification>

1. Read [`ROUTER.md`](../ROUTER.md), folder routers, and [`PERMISSIONS.md`](../PERMISSIONS.md).
2. Run router validation:
   - Windows: `powershell -File scripts/check-ai-agents-routers.ps1`
   - Bash: `bash scripts/check-ai-agents-routers.sh`
2b. Run the graph, schema, and runtime checks:
   - `python3 scripts/check-schemas.py` and `python3 scripts/check-graphs.py` (need `scripts/requirements.txt`)
   - `python3 scripts/check-graphs-test.py` so the checker itself is still rejecting broken graphs
   - `vibe-agent doctor` when the binary is installed: graphs load, routing-eval fixtures validate and report coverage, hook wiring is complete for every host, memory database opens, every `tmp/<slug>/manifest.json` is schema-valid, and `/tmp/` plus `/.agent-state/` are gitignored
   - `cd runtime && go vet ./... && go test ./...` when changing the runtime
   - Report the binary as **not installed** rather than assuming it is absent-by-design. A missing binary is a **blocking** finding for the delivery pipeline (`/goal`, `/build`, `/test`, `/review`, `/ship` refuse without it) and a non-blocking note for reference-only use. See [`goal.md`](goal.md) section "Runtime is required"
3. Check hook command paths referenced in `.claude/settings.json` and `.cursor/hooks.json` exist.
4. Check link/discovery paths:
   - `.claude/skills`, `.claude/agents`, `.claude/commands`
   - `.cursor/skills`, `.cursor/commands`
   - `.opencode/agents`, `.opencode/commands`
   - `.agents/skills`, `.agents/commands`, `.codex/agents`
5. Check broad permission patterns:
   - `Bash(*)`, `Edit(*)`, `Write(*)`, `WebFetch(domain:*)`, `mcp__*`
6. Check routing evals: `vibe-agent doctor` (step 2b) validates [`routing-evals.md`](../references/routing-evals.md) - link resolution, family-to-folder agreement, duplicate intents, intents that name their own asset - and prints intent coverage. Coverage is a number to watch, not a gate. What no check covers is whether the router *actually* selects the listed asset, so spot-check that by hand after scope or router changes.
7. Report stale router rows, missing files, missing hooks, broad permissions, broken routing-eval targets, and recommended fixes.
</verification>

## Required output

<outputs>

```markdown
## AI Asset Doctor

**Verdict:** PASS | WARN | FAIL

### Checks
| Check | Result | Evidence |

### Findings
- Severity, file/path, issue, recommended fix

### Next actions
```
</outputs>

## Routing & discovery

<routing>

- Use when validating toolkit health.
- Do not use for product-code review; use [`review.md`](review.md) or [`ship.md`](ship.md).

Invoke after adding/removing assets, changing hooks, changing permissions, or cloning/linking a consumer repo.
</routing>
