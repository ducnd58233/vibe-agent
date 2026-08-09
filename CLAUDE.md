# Claude Code (vibe-agent)

[Claude Code](https://code.claude.com/docs) loads this file from the project root **automatically** each session. For shared policy, start with [`AGENTS.md`](AGENTS.md); this file only adds Claude-oriented pointers.

This repository is a **shared, domain-agnostic agent-assets toolkit** (skills, agents, commands, hooks), not an end-product app codebase.

## Always-on behavior baseline

Use these skills as default behavioral guardrails in Claude sessions:

- [`karpathy-guardrails`](.ai-agents/skills/karpathy-guardrails/SKILL.md) for assumption handling, simplicity, surgical changes, verifiable completion, and grounded claims (never describe a file/path/result you have not opened or run — report `ACCESS-FAILED: <path>` instead of inferring; applies to primary agents and subagents).
- [`secure-by-default`](.ai-agents/skills/secure-by-default/SKILL.md) for write-time secrecy: no credentials, tokens, or user data in client bundles, browser or device storage, consoles and device logs, rendered UI, API responses, server logs, telemetry, build artifacts, or `tmp/` evidence. Redact at the boundary, not at the call site. Applies to `/spec`, `/plan`, `/build`, `/test`, `/code-simplify`, `/review`, and `/goal` alike, because review only sees code that already exists.
- [`token-efficient-execution`](.ai-agents/skills/token-efficient-execution/SKILL.md) for concise, low-noise responses in repetitive execution workflows.

Increase verbosity and exploratory depth whenever the user explicitly asks for detail.

## Where things live

| What | Where |
|------|--------|
| Shared skills, subagents, commands, hook scripts | [`.ai-agents/`](.ai-agents) (see [`.ai-agents/README.md`](.ai-agents/README.md)) |
| Project settings, permissions, hooks | [`.claude/settings.json`](.claude/settings.json) |
| Linked views of skills / agents / commands | `.claude/skills`, `.claude/agents`, `.claude/commands` → run [`scripts/link-ai-agents.ps1`](scripts/link-ai-agents.ps1) or [`scripts/link-ai-agents.sh`](scripts/link-ai-agents.sh) after clone |

## Private overrides

To keep personal preferences out of git, use `CLAUDE.local.md` in the project root (see [Claude directory](https://code.claude.com/docs/en/claude-directory)). It is loaded alongside this file.

## Authoring and routing (MUST)

- **Local-first precedence:** The workspace root wins. Check its own rules and templates (`AGENTS.md`, `CLAUDE.md`, `CLAUDE.local.md`, its own `TEMPLATE.md`, existing file patterns) before applying any toolkit default; on conflict follow the local rule and state the divergence. Full rule in [`AGENTS.md`](AGENTS.md).
- **New assets:** Follow the **`TEMPLATE.md`** in the relevant [`.ai-agents/`](.ai-agents) subfolder (`skills`, `agents`, `commands`, `hooks`).
- **Choosing assets:** Read [`.ai-agents/ROUTER.md`](.ai-agents/ROUTER.md) and the subfolder **`ROUTER.md`** before picking a skill, subagent, command, or hook.
- **After creating assets:** Update that folder’s **`ROUTER.md`** in the same change (tables track files and use cases).
- **Permissions:** See [`.ai-agents/PERMISSIONS.md`](.ai-agents/PERMISSIONS.md) and align [`.claude/settings.json`](.claude/settings.json) with documented tool needs ([official docs](https://code.claude.com/docs/en/permissions)).

## Other tools in this repo

- **Cursor:** [`CURSOR.md`](CURSOR.md) and [`.cursor/rules/`](.cursor/rules)
- **Codex:** [`.codex/config.toml`](.codex/config.toml) and [`AGENTS.md`](AGENTS.md)
- **opencode:** [`opencode.json`](opencode.json) (config + permissions) and [`AGENTS.md`](AGENTS.md) (native rules file); `.opencode/agents` and `.opencode/commands` link to `.ai-agents/` via the same link script.

## Reuse in a consumer repo

When reused from another repository, prefer adding this toolkit as a submodule at a chosen path (for example `.vibe-agent`).

- Canonical shared assets path in the consumer repo: `<toolkit-root>/.ai-agents`
- From the consumer workspace root, run [`scripts/link-ai-agents.ps1`](scripts/link-ai-agents.ps1) or [`scripts/link-ai-agents.sh`](scripts/link-ai-agents.sh) with workspace = consumer root and assets = `<toolkit-root>/.ai-agents` (see [`.ai-agents/README.md`](.ai-agents/README.md)). Defaults (no parameters) keep the usual behavior when run from a checkout of this toolkit alone.
