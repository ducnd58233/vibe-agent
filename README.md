# vibe-agent

vibe-agent is a shared toolkit of AI-agent assets: skills, subagents, commands, routers, hooks, runtime graphs, and validation scripts. It is meant to be mounted into other repositories, while each consumer repo keeps its own product rules in `AGENTS.md`.

## Table of contents

- [Install commands](#install-commands)
- [Commands loaded in GenAI tools](#commands-loaded-in-genai-tools)
- [Folder structure](#folder-structure)

## Install commands

Run these from the toolkit checkout unless the command says it is for a consumer repo.

### Global install

Global install puts shared assets under your home directory with the `vibe-` prefix, for example `vibe-doctor` and `vibe-research`.

| Shell | Command |
|---|---|
| PowerShell | `powershell -ExecutionPolicy Bypass -File scripts/install-global.ps1` |
| Bash, macOS, Linux, Git Bash, WSL | `sh scripts/install-global.sh` |

Check an existing global install:

```powershell
powershell -ExecutionPolicy Bypass -File scripts/install-global.ps1 -Check
```

```bash
sh scripts/install-global.sh --check
```

### Workspace install

Workspace install wires the current repository to `.ai-agents` and tool-specific folders.

| Shell | Command |
|---|---|
| PowerShell | `powershell -ExecutionPolicy Bypass -File scripts/link-ai-agents.ps1` |
| Bash, macOS, Linux, Git Bash, WSL | `bash scripts/link-ai-agents.sh` |

### Consumer repo install

If this toolkit is mounted in another repo as `.vibe-agent`, run from the consumer repo root:

```powershell
powershell -ExecutionPolicy Bypass -File .vibe-agent/scripts/link-ai-agents.ps1 -WorkspaceRoot $PWD -AssetsRoot (Join-Path $PWD '.vibe-agent\.ai-agents')
```

```bash
bash .vibe-agent/scripts/link-ai-agents.sh --workspace "$PWD" --assets "$PWD/.vibe-agent/.ai-agents"
```

## Commands loaded in GenAI tools

Canonical command prompts live in `.ai-agents/commands/`.

Global install writes commands with the `vibe-` prefix. Workspace install writes unprefixed commands for the current repo. In the table below, the first form is global and the second form is workspace-local.

Codex CLI does not load custom slash prompts in current releases, so Codex uses command skill mentions instead of `/vibe-*` slash commands.

| Command | Command file | What it does | Use case | Claude Code | Cursor | Codex | opencode |
|---|---|---|---|---|---|---|---|
| `goal` | `.ai-agents/commands/goal.md` | Runs the full delivery loop. | Take one objective through clarify, spec, plan, build, review, and ship. Requires the runtime. | Global: `/vibe-goal ...`<br>Workspace: `/goal ...` | Global: `/vibe-goal ...`<br>Workspace: `/goal ...` | Global: `$vibe-goal ...`<br>Workspace: `$goal ...` | Global: `/vibe-goal ...`<br>Workspace: `/goal ...` |
| `spec` | `.ai-agents/commands/spec.md` | Writes the work spec first. | Capture scope, acceptance criteria, risks, and open questions before coding. | Global: `/vibe-spec ...`<br>Workspace: `/spec ...` | Global: `/vibe-spec ...`<br>Workspace: `/spec ...` | Global: `$vibe-spec ...`<br>Workspace: `$spec ...` | Global: `/vibe-spec ...`<br>Workspace: `/spec ...` |
| `plan` | `.ai-agents/commands/plan.md` | Breaks a spec into tasks. | Turn an approved spec into ordered tasks with checks and likely files. | Global: `/vibe-plan ...`<br>Workspace: `/plan ...` | Global: `/vibe-plan ...`<br>Workspace: `/plan ...` | Global: `$vibe-plan ...`<br>Workspace: `$plan ...` | Global: `/vibe-plan ...`<br>Workspace: `/plan ...` |
| `build` | `.ai-agents/commands/build.md` | Implements the next task. | Build one planned task on the right branch with tests. Requires the runtime. | Global: `/vibe-build ...`<br>Workspace: `/build ...` | Global: `/vibe-build ...`<br>Workspace: `/build ...` | Global: `$vibe-build ...`<br>Workspace: `$build ...` | Global: `/vibe-build ...`<br>Workspace: `/build ...` |
| `test` | `.ai-agents/commands/test.md` | Runs a TDD or proof loop. | Start with a failing test, prove a bug, or run focused validation. Requires the runtime. | Global: `/vibe-test ...`<br>Workspace: `/test ...` | Global: `/vibe-test ...`<br>Workspace: `/test ...` | Global: `$vibe-test ...`<br>Workspace: `$test ...` | Global: `/vibe-test ...`<br>Workspace: `/test ...` |
| `review` | `.ai-agents/commands/review.md` | Reviews a diff from five angles. | Check correctness, readability, architecture, security, and performance. Requires the runtime. | Global: `/vibe-review ...`<br>Workspace: `/review ...` | Global: `/vibe-review ...`<br>Workspace: `/review ...` | Global: `$vibe-review ...`<br>Workspace: `$review ...` | Global: `/vibe-review ...`<br>Workspace: `/review ...` |
| `ship` | `.ai-agents/commands/ship.md` | Makes the pre-ship call. | Run final review and return GO or NO-GO with blockers named. Requires the runtime. | Global: `/vibe-ship ...`<br>Workspace: `/ship ...` | Global: `/vibe-ship ...`<br>Workspace: `/ship ...` | Global: `$vibe-ship ...`<br>Workspace: `$ship ...` | Global: `/vibe-ship ...`<br>Workspace: `/ship ...` |
| `research` | `.ai-agents/commands/research.md` | Gathers cited evidence. | Answer one scoped research question with sources. | Global: `/vibe-research ...`<br>Workspace: `/research ...` | Global: `/vibe-research ...`<br>Workspace: `/research ...` | Global: `$vibe-research ...`<br>Workspace: `$research ...` | Global: `/vibe-research ...`<br>Workspace: `/research ...` |
| `analyze` | `.ai-agents/commands/analyze.md` | Turns evidence into a recommendation. | Compare options, tradeoffs, confidence, and likely next steps. | Global: `/vibe-analyze ...`<br>Workspace: `/analyze ...` | Global: `/vibe-analyze ...`<br>Workspace: `/analyze ...` | Global: `$vibe-analyze ...`<br>Workspace: `$analyze ...` | Global: `/vibe-analyze ...`<br>Workspace: `/analyze ...` |
| `investigate` | `.ai-agents/commands/investigate.md` | Runs parallel evidence lanes. | Use investigator, analyst, and source-auditor lanes for a multi-part question. | Global: `/vibe-investigate ...`<br>Workspace: `/investigate ...` | Global: `/vibe-investigate ...`<br>Workspace: `/investigate ...` | Global: `$vibe-investigate ...`<br>Workspace: `$investigate ...` | Global: `/vibe-investigate ...`<br>Workspace: `/investigate ...` |
| `doctor` | `.ai-agents/commands/doctor.md` | Checks AI asset health. | Check routers, generated views, hooks, permissions, runtime wiring, and drift. | Global: `/vibe-doctor`<br>Workspace: `/doctor` | Global: `/vibe-doctor`<br>Workspace: `/doctor` | Global: `$vibe-doctor`<br>Workspace: `$doctor` | Global: `/vibe-doctor`<br>Workspace: `/doctor` |
| `harden` | `.ai-agents/commands/harden.md` | Audits AI asset safety. | Review permissions, hooks, tool boundaries, and secret-handling risks. | Global: `/vibe-harden ...`<br>Workspace: `/harden ...` | Global: `/vibe-harden ...`<br>Workspace: `/harden ...` | Global: `$vibe-harden ...`<br>Workspace: `$harden ...` | Global: `/vibe-harden ...`<br>Workspace: `/harden ...` |
| `code-simplify` | `.ai-agents/commands/code-simplify.md` | Cleans up code without behavior change. | Cut complexity while keeping tests green and behavior unchanged. | Global: `/vibe-code-simplify ...`<br>Workspace: `/code-simplify ...` | Global: `/vibe-code-simplify ...`<br>Workspace: `/code-simplify ...` | Global: `$vibe-code-simplify ...`<br>Workspace: `$code-simplify ...` | Global: `/vibe-code-simplify ...`<br>Workspace: `/code-simplify ...` |
| `design` | `.ai-agents/commands/design.md` | Builds or audits UI. | Work registry-first, then check accessibility, design drift, and rendered evidence. | Global: `/vibe-design ...`<br>Workspace: `/design ...` | Global: `/vibe-design ...`<br>Workspace: `/design ...` | Global: `$vibe-design ...`<br>Workspace: `$design ...` | Global: `/vibe-design ...`<br>Workspace: `/design ...` |

For Codex, type the `$...` form in the prompt text. `/vibe-<command>` and `/prompts:vibe-<command>` are not available in Codex CLI 0.147.0.

## Folder structure

| Path | Purpose |
|---|---|
| `.ai-agents/` | Canonical toolkit assets. Edit here first. |
| `.ai-agents/commands/` | Command prompts such as `doctor.md`, `research.md`, and `goal.md`. |
| `.ai-agents/skills/` | Reusable skills with `SKILL.md`. |
| `.ai-agents/agents/` | Specialist subagent persona files. |
| `.ai-agents/references/` | Shared patterns, checklists, and deeper guidance. |
| `.ai-agents/stack-profiles/` | Stack-specific rules and pinned framework notes. |
| `.ai-agents/graphs/` | Runtime workflow graphs for delivery loops. |
| `.ai-agents/hooks/` | Hook scripts used by tool configs. |
| `runtime/` | Go control plane behind `vibe-agent`. |
| `scripts/` | Install, link, validation, and release helper scripts. |
| `schemas/` | JSON schemas for graphs, run state, and memory records. |
| `.claude/`, `.cursor/`, `.opencode/`, `.codex/`, `.agents/` | Tool-facing generated views and configs. Do not treat them as the source of truth. |
