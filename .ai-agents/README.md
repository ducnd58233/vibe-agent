# Centralized AI assets (`.ai-agents`)

This directory is the **single source of truth** for skills, subagents, commands, and hook scripts shared across AI coding tools. Link-based tool entrypoints (`.claude/`, `.cursor/`, `.opencode/`, `.agents/`) should **point here**—not copy—so changes stay in one place. Codex also reads skills via `.agents/skills` and project subagents via generated `.codex/agents/*.toml` after you run the link script.
It is intentionally **domain-agnostic** and should not contain product-domain logic.

## Layout

| Subfolder   | Purpose |
|------------|---------|
| `skills/`  | Folders with `SKILL.md` (Agent Skills style); **stack-agnostic** workflows. |
| `references/` | Generic checklists and orchestration patterns (markdown). |
| `stack-profiles/` | Repo-specific pinned stacks (markdown); skills link here instead of naming frameworks inline. |
| `agents/`  | Claude Code subagent definitions (`*.md`). |
| `commands/` | Slash-command prompts (`*.md`); `.claude/commands` and `.cursor/commands` link here (same as skills). |
| `hooks/`   | Hook scripts (shell, PowerShell, etc.) invoked by path from the active workspace root. |

| File | Purpose |
|------|---------|
| [`ROUTER.md`](ROUTER.md) | **Master router** — pick which subfolder applies. |
| [`skills/ROUTER.md`](skills/ROUTER.md) (and `agents/`, `commands/`, `hooks/`) | **Per-folder routers** — intent → asset; update tables when files change. |
| [`skills/TEMPLATE.md`](skills/TEMPLATE.md) (and same in other folders) | **Authoring contract** — required sections for new assets. |
| [`PERMISSIONS.md`](PERMISSIONS.md) | Claude **permissions / authority** vs [`.claude/settings.json`](../.claude/settings.json). |

## Authoring and routing (agents MUST)

1. **Creating** a new asset: follow the folder’s **`TEMPLATE.md`** (What, Why, How, When, Routing & discovery, Permissions & authority).
2. **After creating:** update that folder’s **`ROUTER.md`** table (same PR/commit) so use cases and paths stay accurate. From this toolkit repository root, run `bash scripts/check-ai-agents-routers.sh`, or on Windows `powershell -File scripts/check-ai-agents-routers.ps1`, to verify tables match disk (CI runs this on `.ai-agents` changes).
3. **Choosing** an existing asset: read [`ROUTER.md`](ROUTER.md) and the relevant subfolder **`ROUTER.md`**.
4. **Permissions:** update [`PERMISSIONS.md`](PERMISSIONS.md) and [`.claude/settings.json`](../.claude/settings.json) when tool needs change.

Skills stay **general** by default; for the **current workspace**, open [`stack-profiles/ROUTER.md`](stack-profiles/ROUTER.md) when present and read every profile file that applies to the task (see [`stack-profiles/TEMPLATE.md`](stack-profiles/TEMPLATE.md) when adding profiles).

Root [`AGENTS.md`](../AGENTS.md) and [`.cursor/rules/000-project-standards.mdc`](../.cursor/rules/000-project-standards.mdc) restate these rules for tools that load them automatically.

## How each tool uses this tree

| Tool         | What it reads | How it connects to `.ai-agents` |
|-------------|----------------|----------------------------------|
| **Claude Code** | `.claude/skills/`, `.claude/agents/`, `.claude/commands/`, hooks in `settings.json` | Run `scripts/link-ai-agents.ps1` or `scripts/link-ai-agents.sh` to create directory links (see below). |
| **Cursor**      | `.cursor/skills/`, `.cursor/commands/`, `.cursor/hooks.json` + hook scripts | Same link script for `skills` and `commands`; hook **commands** in `hooks.json` can point **directly** to `.ai-agents/hooks/...` when preferred. |
| **Codex**       | `.agents/skills`, `.agents/commands`, `.codex/agents/*.toml`, `.codex/config.toml`, `AGENTS.md` | Run the link script: `.agents/skills` and `.agents/commands` junctions; custom subagents generated into `.codex/agents/`. Slash commands in `/` still use skills until Codex supports repo `commands/`. |
| **opencode**    | Project `opencode.json`, root `AGENTS.md` (native rules file), `.opencode/agents/`, `.opencode/commands/` | Same link script creates `.opencode/agents` and `.opencode/commands` junctions. Skills are surfaced via the `instructions` glob in `opencode.json`, which points at the routers. |

### Always-on baseline semantics

- The default always-on behavioral baseline is defined in root [`AGENTS.md`](../AGENTS.md).
- `opencode` applies this baseline via `opencode.json` `instructions` entries that include `AGENTS.md` and router docs.
- `codex` applies this baseline via [`.codex/config.toml`](../.codex/config.toml) `model_instructions_file = "../AGENTS.md"` (paths in project config are relative to `.codex/`).
- Keep always-on guidance concise; use skill routing for detail to avoid instruction bloat.

## Linking `skills` / `agents` / `commands`

Claude Code and Cursor can discover **skills** and **commands** through `.claude` / `.cursor` link paths. If your environment uses linked discovery paths, run the link script to create **symlinks** or **junctions**:

- `.claude/skills` → `.ai-agents/skills`
- `.claude/agents` → `.ai-agents/agents`
- `.claude/commands` → `.ai-agents/commands`
- `.cursor/skills` → `.ai-agents/skills`
- `.cursor/commands` → `.ai-agents/commands`
- `.opencode/agents` → `.ai-agents/agents`
- `.opencode/commands` → `.ai-agents/commands`
- `.agents/skills` → `.ai-agents/skills` (Codex skill discovery)
- `.agents/commands` → `.ai-agents/commands` (forward-compatible Codex command discovery; no effect until Codex supports `.agents/commands/`)
- `.codex/agents/*.toml` — generated from `.ai-agents/agents/*.md` (Codex custom subagents; re-run link after agent edits)

### Parameters (toolkit dev vs consumer repo)

| Parameter | PowerShell | Bash | Default |
|-----------|------------|------|---------|
| Workspace root (where `.claude`, `.cursor`, `.opencode` are created) | `-WorkspaceRoot` | `--workspace` / `-w` / `--workspace=DIR` | Directory that contains `scripts/` (this toolkit checkout) |
| Assets root (folder that already contains `skills/`, `agents/`, `commands/`) | `-AssetsRoot` | `--assets` / `-a` / `--assets=DIR` | `<workspace default>/.ai-agents` |
| Same as above via environment | `LINK_WORKSPACE`, `LINK_ASSETS` | `LINK_WORKSPACE`, `LINK_ASSETS` | Used only when the matching flag/parameter is omitted |

Symlink targets on Unix and junction targets on Windows are resolved to **absolute** paths so links stay valid regardless of current working directory.

**Git Bash on Windows:** do not put Windows paths with backslashes inside **double-quoted** strings passed to `bash -lc "..."` — Bash treats `\\` sequences there and paths like `D:\\projects` can turn into `D:projects`. Prefer: run the script as argv (`bash .vibe-agent/scripts/link-ai-agents.sh --workspace ...`), use **forward slashes** (`D:/projects/...`), use `--workspace=D:/...` form, or set `LINK_WORKSPACE` / `LINK_ASSETS` and run the script with no path flags.

### Windows (PowerShell)

From **this** repository root (creates junctions here):

```powershell
./scripts/link-ai-agents.ps1
```

From a **consumer** repository root, with this toolkit as submodule `.vibe-agent` (creates junctions at the consumer root):

```powershell
powershell -ExecutionPolicy Bypass -File .vibe-agent/scripts/link-ai-agents.ps1 -WorkspaceRoot $PWD -AssetsRoot (Join-Path $PWD '.vibe-agent\.ai-agents')
```

Uses directory junctions on Windows—no admin usually needed.

### macOS / Linux

From **this** repository root:

```bash
./scripts/link-ai-agents.sh
```

From a **consumer** repository root (submodule at `.vibe-agent`):

```bash
bash .vibe-agent/scripts/link-ai-agents.sh --workspace "$PWD" --assets "$PWD/.vibe-agent/.ai-agents"
```

Windows paths without MSYS `$PWD` (forward slashes or `cygpath`-friendly):

```bash
bash .vibe-agent/scripts/link-ai-agents.sh --workspace=D:/path/to/consumer --assets=D:/path/to/consumer/.vibe-agent/.ai-agents
```

Or:

```bash
export LINK_WORKSPACE="D:/path/to/consumer"
export LINK_ASSETS="D:/path/to/consumer/.vibe-agent/.ai-agents"
bash .vibe-agent/scripts/link-ai-agents.sh
```

### Git and symlinks

**Canonical content** lives under `.ai-agents/`. In this toolkit repository, the mirrored link paths under `.claude/`, `.cursor/`, and `.opencode/` are **generated only** (see [`.gitignore`](../.gitignore)) and are not committed—run the link script after each clone. Consumer repos typically gitignore the same link paths at their root and run the consumer invocation above.

**Maintenance note:** If a path is a **junction** into `.ai-agents`, remove it with `rmdir .\\claude\\skills` (Windows) or `rm .claude/skills` (Unix symlink) before using `git rm` on that path, so Git does not follow the junction and delete files under `.ai-agents/`. The link script removes junctions safely before recreating them.

### Reuse this setup in another repository

If you want another repository to reuse this setup without copying asset files:

1. Add this toolkit repository as a submodule at a chosen path (for example `.vibe-agent`) in the consumer repo.
2. Use `<toolkit-root>/.ai-agents` as the canonical shared assets path.
3. From the **consumer** workspace root, run [`scripts/link-ai-agents.ps1`](../scripts/link-ai-agents.ps1) or [`scripts/link-ai-agents.sh`](../scripts/link-ai-agents.sh) with `-WorkspaceRoot` / `--workspace` set to the consumer root and `-AssetsRoot` / `--assets` set to `<toolkit-root>/.ai-agents` (see examples above). You do not need a separate pasted copy of the junction logic.
4. Keep a consumer-repo-specific `AGENTS.md` for product/domain constraints while shared workflows remain under `<toolkit-root>/.ai-agents`.
5. Add or adapt tool config at the consumer root as needed (for example `.claude/settings.json`, `.cursor/hooks.json`, `.cursor/rules/`, `opencode.json`, `.codex/config.toml`)—the link script only wires `skills` / `agents` / `commands` discovery paths.
6. Review the consumer repo `opencode.json` permission paths (`src/**`, `tests/**`, etc.) and adapt them to that repo layout.

## Hooks (shared scripts)

- Implement hook logic **once** under `.ai-agents/hooks/`.
- Reference the same path from:
  - [`.cursor/hooks.json`](../.cursor/hooks.json) (`command` paths are relative to the project root), and/or
  - [`.claude/settings.json`](../.claude/settings.json) (per [Claude Code hooks](https://code.claude.com/docs/en/hooks)).
- For toolchains without native project hook runtime wiring in this repo (currently `opencode` and `codex`), treat `.ai-agents/hooks/` as shared scripts and invoke them manually or from external automation.

Do not duplicate script bodies in `.cursor/hooks/` unless a tool requires that path; prefer one shared file under `.ai-agents/hooks/`.

## Rules and markdown style

- **Cursor** project rules use `.cursor/rules/*.mdc` (see [Cursor rules](https://docs.cursor.com)). Keep tool-specific or editor-specific rules there.
- **Claude Code** optional `rules/*.md` live under `.claude/` in that product’s format; do not assume the same files work in Cursor without adaptation.
- **Shared policy** for humans and all tools: [../AGENTS.md](../AGENTS.md).
