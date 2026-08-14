# vibe-agent

Reusable toolkit for AI-agent workflows and AI asset management: shared skills, subagents, slash commands, routing guides, hooks, permissions policy, stack profiles, and cross-tool interoperability patterns.

This repository is intentionally **domain-agnostic**. Product-specific behavior belongs in the consuming repository's local `AGENTS.md`.

## What this repository provides

- A single canonical asset tree under [`.ai-agents/`](.ai-agents)
- Explicit router tables so agents choose the right workflow before acting
- Reusable skills for engineering, research, security, QA, design systems, DevOps/SRE, AI/ML model engineering, MLOps, databases, realtime systems, and product lifecycle work
- Specialist subagents for review, security, testing, research, AI/ML research, AI/ML engineering, architecture, DevOps/SRE, database query audits, QA, product design review, and AI-asset audits
- Slash commands for repeatable workflows such as `/goal`, `/spec`, `/plan`, `/build`, `/review`, `/ship`, `/doctor`, and `/harden`
- Stack profiles for pinned frameworks and operational domains
- Shared hook scripts and permission guidance for safer tool use
- Link scripts and validation checks that wire Claude Code, Cursor, Codex, and opencode discovery paths back to `.ai-agents`

## Recent hardening and expansion

The toolkit has been expanded with these capabilities:

### New specialist domains

- **Backend/runtime:** Rust Axum/Tokio, realtime/concurrency/high-traffic systems
- **Frontend/mobile:** React.js, React Native, Flutter, native Android, native iOS
- **Operations:** DevOps platform, CI/CD, system administration, observability/monitoring, product lifecycle
- **Data/ML/AI:** AI model engineering, AI research methodology, MLOps, SQL databases, NoSQL databases, database query optimization
- **Design:** design systems, design-to-code, Figma/Canva MCP handoff, visual QA
- **Quality:** manual QA, automation QA, exploratory testing, release signoff

### New skills

- `concurrency-realtime-systems`
- `devops-platform-delivery`
- `system-administration-ops`
- `observability-monitoring`
- `ai-model-engineering`
- `ai-research-methodology`
- `mlops-lifecycle`
- `product-lifecycle-management`
- `database-query-optimization`
- `qa-testing-strategy`
- `product-design-systems`
- `agent-harness-engineering`

Core existing skills such as `backend-engineering`, `frontend-ui-engineering`, `security-and-hardening`, `performance-optimization`, and `test-driven-development` were kept reusable and composed with the new domain-specific skills.

### New subagents

- `agent-systems-auditor` - audits skills, commands, hooks, routers, permissions, and context hygiene
- `architect-planner` - plans architecture, module/API boundaries, tradeoffs, and implementation slices
- `devops-sre-auditor` - reviews CI/CD, infra, observability, deploy, rollback, and operational readiness
- `database-query-auditor` - reviews SQL/NoSQL query correctness, indexes, migrations, locks, hot keys, and performance
- `qa-tester` - plans manual QA, automation QA, exploratory testing, test matrices, and release signoff
- `product-design-reviewer` - reviews design-system alignment, Figma/Canva handoff, visual QA, and UI fidelity
- `ai-engineer` - reviews and plans AI/ML model build, train, eval, serve, monitor, model-card, and dataset-card work
- `ai-researcher` - researches AI/ML papers, models, benchmarks, reproduction plans, and research-to-engineering handoffs

Existing review agents remain available: `code-reviewer`, `security-auditor`, `test-engineer`, `research-investigator`, `data-analyst`, and `source-auditor`.

### New commands

- `/goal` - end-to-end delivery from ambiguous objective through clarify, optional research, spec, plan, per-task build, test, review, ship, and fix loops until requirements are met ([`goal-driven-delivery`](.ai-agents/skills/goal-driven-delivery/SKILL.md))
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
- `agent-harness-engineering.md`
- `ai-model-development-patterns.md`
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
- Added Codex asset validation for `.agents/skills`, `.agents/commands`, generated `.codex/agents/*.toml`, stale generated links, and UTF-8 mojibake
- Updated Codex agent generation to rewrite shared asset links to workspace-root `.ai-agents/...` paths and read source personas as UTF-8
- Added optional `design-token-guard.py` hook to warn on raw color values in UI files

## Folder structure

- [`.ai-agents/`](.ai-agents): canonical shared assets
  - [`skills/`](.ai-agents/skills): reusable workflows (`SKILL.md`)
  - [`agents/`](.ai-agents/agents): persona files for subagent-style delegation
  - [`commands/`](.ai-agents/commands): slash-command prompts
  - [`stack-profiles/`](.ai-agents/stack-profiles): pinned stack and domain profiles
  - [`references/`](.ai-agents/references): generic checklists and pattern references
  - [`graphs/`](.ai-agents/graphs): executable workflow graphs (`*.yaml`)
  - [`hooks/`](.ai-agents/hooks): shared hook scripts
  - [`ROUTER.md`](.ai-agents/ROUTER.md): top-level asset routing index
  - [`PERMISSIONS.md`](.ai-agents/PERMISSIONS.md): permission and authority guidance
- [`schemas/`](schemas): JSON Schema contracts for graphs, run state, and memory records
- [`runtime/`](runtime): Go control plane (see below); **required** by the delivery pipeline (`/goal`, `/build`, `/test`, `/review`, `/ship`), optional for everything else
- [`.claude/`](.claude): Claude settings; `skills`, `agents`, and `commands` are generated links after running the link script
- [`.cursor/`](.cursor): Cursor rules/hooks; `skills` and `commands` are generated links after running the link script
- [`.codex/`](.codex): Codex project config; `.codex/agents/*.toml` is generated from `.ai-agents/agents`
- [`.agents/`](.agents): generated Codex-compatible skills/commands links
- [`.opencode/`](.opencode): generated opencode agent/command links
- [`scripts/`](scripts): helper scripts for linking, validation, and runtime install

## Runtime (optional)

The toolkit is markdown-first for **reference** assets: every skill and reference reads fine with nothing installed. The **delivery pipeline** is the exception. `/goal`, `/build`, `/test`, `/review`, and `/ship` require the runtime and refuse to run without it, because tracking delivery phases by reading a markdown file is the model marking its own work complete. See [`commands/goal.md`](.ai-agents/commands/goal.md) section "Runtime is required".

The **runtime** adds what markdown cannot do: it enforces workflow transitions instead of describing them, persists run state so a lost session resumes, and keeps evidence-backed memory across runs. It owns only the **outer** loop. Claude Code, Codex, Cursor, and opencode keep their own model and tool loops.

### Install

You need **no Go, no C compiler, and no SQLite**. Download a prebuilt binary:

```bash
bash scripts/install-runtime.sh                # macOS, Linux, Git Bash
```

```powershell
powershell -ExecutionPolicy Bypass -File scripts/install-runtime.ps1   # Windows
```

Both scripts detect your platform, verify the SHA-256 checksum, and put `vibe-agent` on `PATH`. Confirm with:

```bash
vibe-agent version
vibe-agent doctor
```

Both write to `$VIBE_INSTALL_DIR`, defaulting to `~/.local/bin`, and so does `make install` in `runtime/`. One location on purpose: they used to write to three, which left a machine holding several binaries at different versions with `PATH` order deciding which one the hooks called.

**Rerunning is how you update.** Both scripts fetch every time; neither skips because a binary is already there, and `link-ai-agents` runs the installer on every link so a consumer repo stays current. Skipping used to be the behaviour, and it meant a repo that installed once never got another version: the hooks kept calling a binary that fell behind the configs registering them, which fails invisibly because a stale binary answers the events it knows and refuses the rest. `vibe-agent doctor` reports that mismatch if it happens anyway.

If no release is reachable and Go happens to be installed, the scripts build from `runtime/` instead and say so. That fallback is what keeps a fresh clone usable; a release is the normal path and needs no Go.

### Where binaries come from

Two channels, both published by [`runtime-release.yml`](.github/workflows/runtime-release.yml):

| Channel | Trigger | Tag | Use it for |
|---------|---------|-----|------------|
| Rolling | every push to `main` touching `runtime/**` | `runtime/latest`, prerelease | Always current with `main`. What the installer falls back to before any version is cut. |
| Stable | a `runtime/v*` tag | `runtime/v0.1.0` | Pinning. Immutable. |

Both run `make check` before publishing, so nothing ships that has not passed the same gates as a commit.

Binaries are **not committed to this repository**. Each target is about 7 MB and there are six of them; committing every version would add roughly 43 MB to git history per release, forever, with no way to diff or review a binary blob. Release assets sit outside git objects, carry SHA-256 checksums, and can be deleted.

Contributors to the runtime need Go:

```bash
cd runtime && make check && make install
```

### How it gets triggered

Three paths, in descending order of reliability:

| Host | Mechanism | Fires |
|------|-----------|-------|
| Claude Code | hooks in [`.claude/settings.json`](.claude/settings.json) | **Always.** The harness runs them; the model does not choose. |
| Codex | hooks in [`.codex/hooks.json`](.codex/hooks.json), for a project marked trusted in [`.codex/config.toml`](.codex/config.toml) | **Always**, except that Codex fires no event at all for a failed command, so failures are not journalled there. |
| Cursor | hooks in [`.cursor/hooks.json`](.cursor/hooks.json), MCP in [`.cursor/mcp.json`](.cursor/mcp.json) | **Always** in the editor, for the events Cursor exposes. |
| opencode | MCP server, `vibe-agent mcp serve` | **When the model decides to call a tool.** Its tool lifecycle is JS/TS plugins rather than shell commands, so there is no hook to wire. |
| You, or an agent in a shell | the CLI directly | On demand. |

The wiring already exists in this repo. Once the binary is on `PATH`, a new session picks it up; before that, every hook is a deliberate no-op rather than an error, so a missing binary never wedges a session.

### The one hook that refuses

Hooks inform. One does not.

`pre-tool-use` refuses two commands while a run is active and has not recorded the `merge_approved` evidence its graph requires:

- a `git push` whose destination is `main` or `master`
- `gh pr merge`

Everything else passes, including pushing a task branch, which happens at `open_pr` long before the ship gate. Blocking that would wedge the loop this exists to protect.

The gate does not invent a rule. [`goal-delivery.yaml`](.ai-agents/graphs/goal-delivery.yaml) already calls `approve_merge` "the only gate in front of an irreversible action"; this is the process that enforces the sentence.

It refuses with **exit 2** on Claude Code, and a `deny` decision on Cursor, which uses JSON rather than exit codes for this event. Exit 2 is deliberate: a JSON `permissionDecision` fails open, because one stray line on stdout makes the JSON unparseable and the command proceeds. In front of an irreversible action a guard has to fail closed.

Two ways it stays out of the way: a workspace with **no active run is never gated**, and a workspace with **no binary installed** gets a non-blocking error rather than a refusal. On Codex it refuses with the same JSON `permissionDecision` shape as Cursor: exit 2 was measured being ignored there, and the command ran. opencode reaches the runtime over MCP, which the model chooses to call, so it gets no gate.

### Using it

```bash
vibe-agent run start --slug webhook-idempotency --goal "make delivery idempotent"
vibe-agent run status --slug webhook-idempotency     # current node and what completes it
vibe-agent checkpoint --slug webhook-idempotency \
  --check unit --source exit_code --passed           # record evidence, advance the graph
```

Run state lands in `tmp/<slug>/manifest.json` with an append-only log at `tmp/<slug>/events.ndjson`, beside the human-readable `RECORD.md`. Memory lives in `.agent-state/memory.db`. All of it is gitignored.

`--source` is one of `exit_code`, `file_assert`, `ci_api`, `human_event`. There is deliberately **no source for model assertion**, so nothing can mark its own work complete by claiming it did.

When the toolkit is mounted as a submodule, point the two roots separately:

```bash
vibe-agent run status --slug my-feature --workspace . --toolkit .vibe-agent
```

Full detail in [`runtime/README.md`](runtime/README.md) and [`references/loop-and-graph-engineering.md`](.ai-agents/references/loop-and-graph-engineering.md).

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

### Validate Codex generated assets

Run this after changing `.ai-agents/agents`, after clone/link setup, or before relying on Codex custom agents:

```powershell
powershell -File scripts/check-codex-assets.ps1
```

The Codex check verifies that `.agents/skills` and `.agents/commands` exist, every source agent in `.ai-agents/agents/*.md` has a generated `.codex/agents/*.toml`, there are no stale generated agents, generated agent instructions use workspace-root `.ai-agents/...` links, and generated TOML does not contain common UTF-8 mojibake markers.

## Slash commands

- `/goal`: full delivery loop (clarify, research, spec, plan, build, test, review, ship until done); waits for PR CI and external bot reviews; E2E when in scope; saves evidence under `tmp/<slug>/` (gitignored)
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
/goal
```

Or step by step:

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
