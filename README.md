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

| Tool | How to use |
|---|---|
| Claude Code | Use `/vibe-<command>` after global install, or `/<command>` in a workspace after `link-ai-agents`. |
| Cursor | Use `/vibe-<command>` after global install, or `/<command>` in a linked workspace if Cursor command discovery is enabled. |
| opencode | Global install writes `~/.config/opencode/commands/vibe-*.md`; workspace install writes `.opencode/commands`. Use opencode's command picker or ask by command name if the picker is unavailable. |
| Codex | Codex loads vibe-agent skills from `.agents/skills` or `~/.agents/skills`. This Codex surface does not accept `/prompts:vibe-doctor`; use `$vibe-<skill>` where skills are available, or ask by intent, for example "run vibe doctor". The prompt files under `$CODEX_HOME/prompts` are best-effort files for ChatGPT desktop surfaces that support custom prompts. |

Available toolkit commands:

| Command | Use it for |
|---|---|
| `goal` | Run the delivery loop from objective to ship decision. Requires the `vibe-agent` runtime. |
| `spec` | Write an implementation spec before coding. |
| `plan` | Break a spec into ordered tasks. |
| `build` | Implement the next planned task with tests. Requires the runtime. |
| `test` | Prove behavior with tests or a focused validation loop. Requires the runtime. |
| `review` | Review code across correctness, readability, architecture, security, and performance. Requires the runtime. |
| `ship` | Produce a GO or NO-GO ship decision. Requires the runtime. |
| `research` | Gather citation-backed evidence for one research lane. |
| `analyze` | Turn evidence into a recommendation with tradeoffs. |
| `investigate` | Run multi-lane research, analysis, and source audit. |
| `doctor` | Check AI asset health: routers, links, hooks, permissions, generated views. |
| `harden` | Tighten AI asset safety: permissions, hooks, tool boundaries, secret paths. |
| `code-simplify` | Reduce complexity without changing behavior. |
| `design` | Build or audit UI with design-system and accessibility checks. |

Examples:

```text
/vibe-doctor
/vibe-research current Codex slash command support
/vibe-goal finish the payments retry work and open a PR
```

In a linked workspace that exposes unprefixed project commands, use the same names without `vibe-`:

```text
/doctor
/research current Codex slash command support
/goal finish the payments retry work and open a PR
```

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
