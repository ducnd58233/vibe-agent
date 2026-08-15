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

Global install writes commands with the `vibe-` prefix. Workspace install writes unprefixed project commands for tools that read workspace command folders. Codex custom prompts are global only in Codex 0.147.0 and use the `/prompts:` namespace.

| Command | What it is | Use case | Claude Code | Cursor | Codex | opencode |
|---|---|---|---|---|---|---|
| `goal` | End-to-end delivery loop | Drive one objective through clarify, spec, plan, build, review, and ship. Requires the runtime. | `/vibe-goal ...` or `/goal ...` | `/vibe-goal ...` or `/goal ...` | `/prompts:vibe-goal ...` | `/vibe-goal ...` or `/goal ...` |
| `spec` | Spec-first prompt | Write objective, scope, acceptance criteria, risks, and open questions before coding. | `/vibe-spec ...` or `/spec ...` | `/vibe-spec ...` or `/spec ...` | `/prompts:vibe-spec ...` | `/vibe-spec ...` or `/spec ...` |
| `plan` | Task breakdown prompt | Turn an approved spec into ordered tasks with checks and likely files. | `/vibe-plan ...` or `/plan ...` | `/vibe-plan ...` or `/plan ...` | `/prompts:vibe-plan ...` | `/vibe-plan ...` or `/plan ...` |
| `build` | Implementation prompt | Implement the next planned task on the right branch with tests. Requires the runtime. | `/vibe-build ...` or `/build ...` | `/vibe-build ...` or `/build ...` | `/prompts:vibe-build ...` | `/vibe-build ...` or `/build ...` |
| `test` | TDD and proof prompt | Start with a failing test, prove a bug, or run a focused validation loop. Requires the runtime. | `/vibe-test ...` or `/test ...` | `/vibe-test ...` or `/test ...` | `/prompts:vibe-test ...` | `/vibe-test ...` or `/test ...` |
| `review` | Five-axis review prompt | Review a diff for correctness, readability, architecture, security, and performance. Requires the runtime. | `/vibe-review ...` or `/review ...` | `/vibe-review ...` or `/review ...` | `/prompts:vibe-review ...` | `/vibe-review ...` or `/review ...` |
| `ship` | Pre-ship decision prompt | Fan out final review and return GO or NO-GO with blockers named. Requires the runtime. | `/vibe-ship ...` or `/ship ...` | `/vibe-ship ...` or `/ship ...` | `/prompts:vibe-ship ...` | `/vibe-ship ...` or `/ship ...` |
| `research` | Citation-first research prompt | Gather source-backed evidence for one scoped question. | `/vibe-research ...` or `/research ...` | `/vibe-research ...` or `/research ...` | `/prompts:vibe-research ...` | `/vibe-research ...` or `/research ...` |
| `analyze` | Evidence synthesis prompt | Turn collected evidence into a recommendation with tradeoffs and confidence. | `/vibe-analyze ...` or `/analyze ...` | `/vibe-analyze ...` or `/analyze ...` | `/prompts:vibe-analyze ...` | `/vibe-analyze ...` or `/analyze ...` |
| `investigate` | Multi-lane investigation prompt | Run investigator, analyst, and source-auditor lanes, then merge the verdict. | `/vibe-investigate ...` or `/investigate ...` | `/vibe-investigate ...` or `/investigate ...` | `/prompts:vibe-investigate ...` | `/vibe-investigate ...` or `/investigate ...` |
| `doctor` | AI asset health prompt | Check routers, generated views, hooks, permissions, runtime wiring, and drift. | `/vibe-doctor` or `/doctor` | `/vibe-doctor` or `/doctor` | `/prompts:vibe-doctor` | `/vibe-doctor` or `/doctor` |
| `harden` | AI asset safety prompt | Audit permissions, hooks, tool boundaries, and secret-handling risks. | `/vibe-harden ...` or `/harden ...` | `/vibe-harden ...` or `/harden ...` | `/prompts:vibe-harden ...` | `/vibe-harden ...` or `/harden ...` |
| `code-simplify` | Behavior-preserving cleanup prompt | Reduce complexity while keeping tests green and behavior unchanged. | `/vibe-code-simplify ...` or `/code-simplify ...` | `/vibe-code-simplify ...` or `/code-simplify ...` | `/prompts:vibe-code-simplify ...` | `/vibe-code-simplify ...` or `/code-simplify ...` |
| `design` | UI build or audit prompt | Work registry-first, then check accessibility, design drift, and rendered evidence. | `/vibe-design ...` or `/design ...` | `/vibe-design ...` or `/design ...` | `/prompts:vibe-design ...` | `/vibe-design ...` or `/design ...` |

If `/prompts:vibe-doctor` is unrecognized in Codex after restart, run `codex doctor` and confirm it reports the same `CODEX_HOME` where `vibe-*.md` files were installed. Top-level `/vibe-<command>` is not file-installable in Codex CLI 0.147.0.

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
