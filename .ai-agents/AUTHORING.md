# Authoring and setup

Everything an agent needs **only when creating toolkit assets or setting a repo up**. It lived in the
root [`AGENTS.md`](../AGENTS.md), which every session loads in full, so a rule needed once per month
was costing tokens on every turn. AGENTS.md keeps the rules that govern behavior each turn and points
here for the rest.

Read this file when you are: adding or changing an asset, wiring a harness, or mounting the toolkit
into a consumer repo. Skip it otherwise.

## Project layout

| Location | Role |
|----------|------|
| [`.ai-agents/README.md`](README.md) | Index of shared skills, agents, commands, stack profiles, references, and hooks. |
| [`.ai-agents/ROUTER.md`](ROUTER.md) | Master router - start here to pick which asset family applies. |
| [`.ai-agents/*/ROUTER.md`](skills/ROUTER.md) | Per-folder routers - intent to concrete asset; **must** stay in sync when assets change. |
| [`.ai-agents/*/TEMPLATE.md`](skills/TEMPLATE.md) | Authoring contracts for folders that define one. |
| [`.ai-agents/PERMISSIONS.md`](PERMISSIONS.md) | Permissions and authority mapping to [`.claude/settings.json`](../.claude/settings.json), hooks, and subagent `tools:`. |
| [`.ai-agents/skills/`](skills) | Canonical skills (`SKILL.md` per folder), stack-agnostic by default. |
| [`.ai-agents/agents/`](agents) | Subagent/persona definitions (`*.md`). |
| [`.ai-agents/commands/`](commands) | Slash-command prompts (`*.md`). |
| [`.ai-agents/references/`](references) | Generic checklists and patterns. |
| [`.ai-agents/stack-profiles/`](stack-profiles) | Repo-pinned stack and domain profiles. |
| [`.ai-agents/graphs/`](graphs) | Executable workflow graphs (`*.yaml`). |
| [`.ai-agents/hooks/`](hooks) | Shared hook scripts. |
| [`schemas/`](../schemas) | JSON Schema contracts for graphs, run state, and memory records. |
| [`runtime/`](../runtime) | Go control plane. Required by the delivery pipeline. |
| [`.claude/`](../.claude), [`.cursor/`](../.cursor), [`.opencode/`](../.opencode), [`.codex/`](../.codex), [`.agents/`](../.agents) | Harness config plus generated links, produced by `scripts/link-ai-agents`. |
| [`opencode.json`](../opencode.json), [`CLAUDE.md`](../CLAUDE.md), [`CURSOR.md`](../CURSOR.md) | Per-harness entry points. |

## Authoring rules (MUST)

- **Follow the folder's `TEMPLATE.md`** where present when creating a skill, subagent, command, hook,
  reference, or stack profile.
- **Update that folder's `ROUTER.md` in the same change** after creating, renaming, or deleting any
  asset under `skills/`, `agents/`, `commands/`, `hooks/`, `references/`, `stack-profiles/`, or
  `graphs/`.
- **Do not restate a rule that already has a home.** Link to it. The stack-detection rule, the
  permissions defaults, the git gates, and the runtime requirement each live in exactly one place;
  copies drift and the copy is what a reader trusts.
- **Tool naming and currency:** shared skills, references, and commands stay tool-agnostic and
  describe the capability, not a product. Concrete tools, packages, and libraries are named only in
  [`stack-profiles/`](stack-profiles), and even there as non-exhaustive examples the agent must
  verify against current official docs. Prefer detecting what the repo uses over any hardcoded list.
  See [`stack-profiles/TEMPLATE.md`](stack-profiles/TEMPLATE.md).
- **Tool-specific rules:** Cursor rule files live in [`.cursor/rules/`](../.cursor/rules) and are not
  interchangeable with Claude rules without editing.
- **Permissions:** after changing tool or path requirements, align
  [`.claude/settings.json`](../.claude/settings.json) and [`PERMISSIONS.md`](PERMISSIONS.md). Deny
  overrides allow.

## Checks to run

| After changing | Run |
|---|---|
| Any asset under `.ai-agents/` | `bash scripts/check-ai-agents-routers.sh` or `powershell -File scripts/check-ai-agents-routers.ps1` (dependency-free) |
| Anything, before trusting a harness read | `bash scripts/check-generated-views.sh` - a canonical edit does not reach the harness until the link script re-runs |
| `.ai-agents/graphs/*.yaml` or `schemas/*.json` | `python3 scripts/check-graphs.py` and `python3 scripts/check-schemas.py` |
| [`runtime/`](../runtime) | `cd runtime && make check` |
| `.ai-agents/agents/*.md` | re-run the link script, then `powershell -File scripts/check-codex-assets.ps1` so `.codex/agents/*.toml` stays loadable |

The python checks need `python3 -m pip install -r scripts/requirements.txt`. Full table:
[`README.md`](README.md).

## After clone

Run `scripts/link-ai-agents.ps1` (Windows) or `scripts/link-ai-agents.sh` (macOS, Linux) so
`.claude`, `.cursor`, `.opencode`, `.agents`, and `.codex/agents` point at `.ai-agents`. The script
also installs the git `prepare-commit-msg` attribution hook and refreshes the runtime binary.

**Re-run it after every asset edit.** On Windows the script writes copies rather than symlinks, so
until it runs again the harness serves the previous text while every other check reports green.
`scripts/check-generated-views.sh` exists because that happened.

## Reuse in a consumer repository

- Keep the consumer repo as its own repository and the source of product code.
- Mount this toolkit as a submodule at a chosen path, for example `.vibe-agent`.
- Treat `<toolkit-root>/.ai-agents` as the canonical shared assets path.
- Give the consumer repo its own root `AGENTS.md` with product and domain constraints. It wins over
  this toolkit; see local-first precedence in [`AGENTS.md`](../AGENTS.md).
- Run the link scripts from the submodule with `-WorkspaceRoot` / `--workspace` set to the consumer
  root and `-AssetsRoot` / `--assets` set to `<toolkit-root>/.ai-agents`.
- Treat tool permissions as repository-local policy: adapt `opencode.json`, `.claude/settings.json`,
  and local rules to that repo's layout and risk profile.

## Asset inventory

There is no list here on purpose. The router files are authoritative, and a second list in prose is
a list that goes stale without anything failing. Start at [`ROUTER.md`](ROUTER.md).
