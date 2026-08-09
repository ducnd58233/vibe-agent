# Claude Code (vibe-agent)

[Claude Code](https://code.claude.com/docs) loads this file from the project root automatically each
session, alongside [`AGENTS.md`](AGENTS.md). **`AGENTS.md` holds the shared policy.** This file adds
only what is Claude-specific, so that a rule lives in one place rather than two that drift.

This repository is a shared, domain-agnostic agent-assets toolkit, not an end-product app codebase.

Sections are wrapped in XML tags so a model can address one block at a time; the tag set is
documented in [`.ai-agents/AUTHORING.md`](.ai-agents/AUTHORING.md).

<always_on>
The behavioral baseline is in [`AGENTS.md`](AGENTS.md) and applies here unchanged. In Claude sessions
it resolves to three skills the model may load on its own:

- [`karpathy-guardrails`](.ai-agents/skills/karpathy-guardrails/SKILL.md) - assumptions, simplicity,
  surgical diffs, verification-first completion, grounded claims.
- [`secure-by-default`](.ai-agents/skills/secure-by-default/SKILL.md) - write-time secrecy across
  bundles, storage, logs, UI, responses, telemetry, and `tmp/` evidence.
- [`token-efficient-execution`](.ai-agents/skills/token-efficient-execution/SKILL.md) - concise,
  low-noise output.

Every other skill sets `disable-model-invocation: true` and is user-invoked. That flag is the
whitelist of what loads without being asked for; adding a skill to this list is a deliberate act.

Increase verbosity and exploratory depth whenever the user asks for detail.
</always_on>

<claude_specific>
| Concern | Where |
|---|---|
| Settings, permissions, hooks | [`.claude/settings.json`](.claude/settings.json) |
| Generated views of skills, agents, commands | `.claude/skills`, `.claude/agents`, `.claude/commands`, produced by the link script |
| Private preferences kept out of git | `CLAUDE.local.md` in the project root, loaded alongside this file ([docs](https://code.claude.com/docs/en/claude-directory)) |
| Permission syntax and precedence | [`PERMISSIONS.md`](.ai-agents/PERMISSIONS.md) and the [official docs](https://code.claude.com/docs/en/permissions). Deny overrides allow |

**A generated view can be stale.** On Windows the link script writes copies, not symlinks, so an edit
under `.ai-agents/` does not reach the session until the script re-runs. Run
`bash scripts/check-generated-views.sh` before trusting what a command file says.
</claude_specific>

<other_harnesses>
- **Cursor:** [`CURSOR.md`](CURSOR.md), rules in [`.cursor/rules/`](.cursor/rules) as `.mdc` files.
- **Codex:** [`.codex/config.toml`](.codex/config.toml) plus [`AGENTS.md`](AGENTS.md).
- **opencode:** [`opencode.json`](opencode.json) for config and permissions, [`AGENTS.md`](AGENTS.md)
  as its rules file.

Authoring, setup, clone, and consumer-repo mounting: [`.ai-agents/AUTHORING.md`](.ai-agents/AUTHORING.md).
</other_harnesses>
