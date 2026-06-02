# vibe-agent

Reusable toolkit for AI-agent workflows and AI asset management: shared skills, subagents, slash commands, routing guides, hooks, permissions policy, stack profiles, and cross-tool interoperability patterns.

This repository is intentionally **domain-agnostic**. Product-specific behavior belongs in the consuming repository's local `AGENTS.md`.

## What this repository provides

- A single canonical asset tree under [`.ai-agents/`](.ai-agents)
- Explicit router tables so agents choose the right workflow before acting
- Reusable skills for engineering, research, security, QA, design systems, DevOps/SRE, MLOps, databases, realtime systems, and product lifecycle work
- Specialist subagents for review, security, testing, research, architecture, DevOps/SRE, database query audits, QA, product design review, and AI-asset audits
- Slash commands for repeatable workflows such as `/spec`, `/plan`, `/build`, `/review`, `/ship`, `/doctor`, and `/harden`
- Stack profiles for pinned frameworks and operational domains
- Shared hook scripts and permission guidance for safer tool use
- Link scripts that wire Claude Code, Cursor, Codex, and opencode discovery paths back to `.ai-agents`

## Recent hardening and expansion

The toolkit has been expanded with these capabilities:

### New specialist domains

- **Backend/runtime:** Rust Axum/Tokio, realtime/concurrency/high-traffic systems
- **Frontend/mobile:** React.js, React Native, Flutter, native Android, native iOS
- **Operations:** DevOps platform, CI/CD, system administration, observability/monitoring, product lifecycle
- **Data/ML:** MLOps, SQL databases, NoSQL databases, database query optimization
- **Design:** design systems, design-to-code, Figma/Canva MCP handoff, visual QA
- **Quality:** manual QA, automation QA, exploratory testing, release signoff

### New skills

- `concurrency-realtime-systems`
- `devops-platform-delivery`
- `system-administration-ops`
- `observability-monitoring`
- `mlops-lifecycle`
- `product-lifecycle-management`
- `database-query-optimization`
- `qa-testing-strategy`
- `product-design-systems`

Core existing skills such as `backend-engineering`, `frontend-ui-engineering`, `security-and-hardening`, `performance-optimization`, and `test-driven-development` were kept reusable and composed with the new domain-specific skills.

### New subagents

- `agent-systems-auditor` - audits skills, commands, hooks, routers, permissions, and context hygiene
- `architect-planner` - plans architecture, module/API boundaries, tradeoffs, and implementation slices
- `devops-sre-auditor` - reviews CI/CD, infra, observability, deploy, rollback, and operational readiness
- `database-query-auditor` - reviews SQL/NoSQL query correctness, indexes, migrations, locks, hot keys, and performance
- `qa-tester` - plans manual QA, automation QA, exploratory testing, test matrices, and release signoff
- `product-design-reviewer` - reviews design-system alignment, Figma/Canva handoff, visual QA, and UI fidelity

Existing review agents remain available: `code-reviewer`, `security-auditor`, `test-engineer`, `research-investigator`, `data-analyst`, and `source-auditor`.

### New commands

- `/doctor` - audit AI asset health: routers, hooks, link paths, permissions, and discovery wiring
- `/harden` - harden AI assets: permissions, hooks, tool boundaries, secret safety, and orchestration risks

`/ship` now supports conditional specialist fan-out:

- Always: `code-reviewer`, `security-auditor`, `test-engineer`
- Add `devops-sre-auditor` for CI/CD, infra, deploy, observability, or rollout changes
- Add `database-query-auditor` for SQL/NoSQL, migration, index, cache/datastore, or query-performance changes
- Add `qa-tester` for manual QA, release signoff, exploratory testing, platform matrix, or E2E strategy
- Add `product-design-reviewer` for design-system changes, Figma/Canva handoff, visual QA, tokens, or UI fidelity

### New references

- `agent-authoring-patterns.md`
- `tool-safety-and-permissions.md`
- `agent-evaluation-patterns.md`
- `context-management-patterns.md`
- `ci-cd-observability-patterns.md`
- `database-query-patterns.md`
- `qa-testing-strategy.md`
- `design-to-code-patterns.md`

### Hardening updates

- `.claude/settings.json` now uses a narrower default permission posture
- Removed stale Claude hook references to missing `.claude/hooks/scripts/hooks.py`
- Disabled automatic loading of all project MCP servers by default
- Added secret-path deny patterns
- Updated hook router entries to link Python hook scripts
- Updated router validation to check `.py`, `.ps1`, and `.sh` hooks
- Added optional `design-token-guard.py` hook to warn on raw color values in UI files

## Folder structure

- [`.ai-agents/`](.ai-agents): canonical shared assets
  - [`skills/`](.ai-agents/skills): reusable workflows (`SKILL.md`)
  - [`agents/`](.ai-agents/agents): persona files for subagent-style delegation
  - [`commands/`](.ai-agents/commands): slash-command prompts
  - [`stack-profiles/`](.ai-agents/stack-profiles): pinned stack and domain profiles
  - [`references/`](.ai-agents/references): generic checklists and pattern references
  - [`hooks/`](.ai-agents/hooks): shared hook scripts
  - [`ROUTER.md`](.ai-agents/ROUTER.md): top-level asset routing index
  - [`PERMISSIONS.md`](.ai-agents/PERMISSIONS.md): permission and authority guidance
- [`.claude/`](.claude): Claude settings; `skills`, `agents`, and `commands` are generated links after running the link script
- [`.cursor/`](.cursor): Cursor rules/hooks; `skills` and `commands` are generated links after running the link script
- [`.codex/`](.codex): Codex project config; `.codex/agents/*.toml` is generated from `.ai-agents/agents`
- [`.agents/`](.agents): generated Codex-compatible skills/commands links
- [`.opencode/`](.opencode): generated opencode agent/command links
- [`scripts/`](scripts): helper scripts for linking and router validation

## Routing rules

Agents should route through the asset tree before acting:

1. Read [`.ai-agents/ROUTER.md`](.ai-agents/ROUTER.md).
2. Open the relevant folder router:
   - skills: [`.ai-agents/skills/ROUTER.md`](.ai-agents/skills/ROUTER.md)
   - agents: [`.ai-agents/agents/ROUTER.md`](.ai-agents/agents/ROUTER.md)
   - commands: [`.ai-agents/commands/ROUTER.md`](.ai-agents/commands/ROUTER.md)
   - references: [`.ai-agents/references/ROUTER.md`](.ai-agents/references/ROUTER.md)
   - stack profiles: [`.ai-agents/stack-profiles/ROUTER.md`](.ai-agents/stack-profiles/ROUTER.md)
   - hooks: [`.ai-agents/hooks/ROUTER.md`](.ai-agents/hooks/ROUTER.md)
3. Load every matching stack profile for the current task.
4. Keep skills stack-agnostic; keep pinned framework/tool details in stack profiles.

## Useful commands

### Link AI assets after clone

Windows:

```powershell
powershell -File scripts/link-ai-agents.ps1
```

macOS/Linux:

```bash
bash scripts/link-ai-agents.sh
```

Consumer repo with this toolkit mounted as `.vibe-agent`:

```powershell
powershell -File .vibe-agent/scripts/link-ai-agents.ps1 -WorkspaceRoot $PWD -AssetsRoot (Join-Path $PWD '.vibe-agent\.ai-agents')
```

```bash
bash .vibe-agent/scripts/link-ai-agents.sh --workspace "$PWD" --assets "$PWD/.vibe-agent/.ai-agents"
```

### Validate routers

Windows:

```powershell
powershell -File scripts/check-ai-agents-routers.ps1
```

macOS/Linux:

```bash
bash scripts/check-ai-agents-routers.sh
```

The router check validates skills, agents, commands, references, stack profiles, and hook scripts (`.py`, `.ps1`, `.sh`) against their folder `ROUTER.md` tables.

## Slash commands

- `/spec`: produce a structured implementation spec
- `/plan`: break a spec into actionable tasks
- `/build`: implement the next planned task with TDD discipline
- `/test`: run test-driven or prove-it validation
- `/review`: run focused code review
- `/ship`: run pre-ship specialist fan-out and produce GO/NO-GO
- `/research`: run citation-first research
- `/analyze`: synthesize evidence into a recommendation
- `/investigate`: run parallel investigation, analysis, and source audit
- `/doctor`: audit AI asset health and discovery/config wiring
- `/harden`: review and improve AI asset safety boundaries
- `/code-simplify`: simplify safely under test protection

## Example workflow

```text
/spec -> /plan -> /build -> /test -> /review -> /ship
```

For a database-heavy feature, `/ship` can include `database-query-auditor`. For an infra-heavy change, it can include `devops-sre-auditor`. For release signoff, it can include `qa-tester`.

## Authoring new assets

When adding a skill, agent, command, hook, reference, or stack profile:

1. Follow that folder's `TEMPLATE.md`.
2. Complete What / Why / How / When / Routing & discovery / Permissions & authority.
3. Update the folder `ROUTER.md` in the same change.
4. Run the router check.
5. Update [`.ai-agents/PERMISSIONS.md`](.ai-agents/PERMISSIONS.md) and [`.claude/settings.json`](.claude/settings.json) if tool authority changes.

## How to use this toolkit in another repo

Mount this repository at a chosen toolkit path, commonly `.vibe-agent`, then point consumer tool folders to `<toolkit-root>/.ai-agents/*` using the link script.

The consumer repository should keep its own root `AGENTS.md` for product/domain rules while this toolkit supplies shared workflows.

See also:

- [`AGENTS.md`](AGENTS.md)
- [`.ai-agents/README.md`](.ai-agents/README.md)
- [`.ai-agents/PERMISSIONS.md`](.ai-agents/PERMISSIONS.md)
- [`.ai-agents/references/orchestration-patterns.md`](.ai-agents/references/orchestration-patterns.md)
