# Cursor feature map (for vibe-agent)

<context>

Maps Cursor Desktop / IDE agent features to vibe-agent owners. Primary sources:
[hooks](https://cursor.com/docs/agent/hooks), [rules](https://cursor.com/docs/context/rules),
[Bugbot](https://cursor.com/docs/bugbot), [MCP](https://cursor.com/docs/context/mcp),
[subagents](https://cursor.com/docs/agent/subagents). Local contract:
[`host-hook-contracts.md`](host-hook-contracts.md) (`cursorContract`).

Use when importing an idea from Cursor into the toolkit. Prefer linking here over
restating Cursor docs. Domain product logic does not belong here.

**Parity bar host** (see [`AUTHORING.md`](../AUTHORING.md) section "Supported harness parity").
Peer maps: [`claude-code-feature-map.md`](claude-code-feature-map.md),
[`codex-feature-map.md`](codex-feature-map.md), [`opencode-feature-map.md`](opencode-feature-map.md).
</context>

## Feature map

<rules>

| Cursor feature | Reuse | Reject | Gap / vibe owner |
|----------------|-------|--------|------------------|
| `.cursor/rules/*.mdc` | Always-on rules; consumer charter neutrality | Copy toolkit paths into consumer rules | [`CURSOR.md`](../../CURSOR.md), [`consumer-charter-authoring.md`](consumer-charter-authoring.md) |
| Linked skills / commands | `.ai-agents/` + link script | Assume symlinks on Windows (copies) | [`CURSOR.md`](../../CURSOR.md), `scripts/check-generated-views.sh` |
| Snake_case hooks (`preToolUse`, `postToolUse`, `stop`, `beforeSubmitPrompt`, `beforeMCPExecution`, `afterAgentResponse`) | `vibe-agent hook <event>` adapters | Exit 2 block semantics (Cursor ignores) | [`hooks/ROUTER.md`](../hooks/ROUTER.md); [`host-hook-contracts.md`](host-hook-contracts.md) |
| `beforeSubmitPrompt` | Gate dangerous prompts at submit | Inject context (not supported) | Document only; no graph evidence |
| Run context on `postToolUse` | Journal with run slug when host sends it | Assume run context on all events | `runtime/internal/harness` |
| Bugbot (PR review) | Intent matches `/review` + CI API on auto path | Bugbot as project rules file | [`commands/review.md`](../commands/review.md); `/auto` uses `ci_api` |
| MCP + dynamic tools | Host MCP; `vibe-agent mcp serve` | New evidence source | [`token-efficiency.md`](token-efficiency.md) |
| Subagents (Task tool) | Persona agents under `.ai-agents/agents` | Cursor-specific subagent schema as required | [`agents/ROUTER.md`](../agents/ROUTER.md) |
| Browser / computer-use MCP | Optional host capability | Embed browser in Go runtime | Decline; host-owned |
| Cloud agents / worktrees | Isolated branch pattern (`best-of-n-runner`) | Cloud VM inside control plane | [`commands/auto.md`](../commands/auto.md) outer loop only |
| SessionStart cwd injection | Not available | Fake SessionStart via rules | Gap: no `$CWD` hook variable |

</rules>

## Hook verification status

<rules>

Cursor hooks in this repo are **UNVERIFIED** (no measured hook fire in CI).
Observed risk: `cursor-agent` may read `.claude/settings.json` instead of
`.cursor/hooks.json`. Before claiming Cursor hook parity, run an observation
campaign and update [`host-hook-contracts.md`](host-hook-contracts.md).

</rules>

## Host portability matrix (Cursor adaptations)

<rules>

| Adaptation | Claude | Cursor | Codex | opencode |
|------------|--------|--------|-------|----------|
| Feature map / honesty docs | yes | yes | yes | yes |
| SessionStart + `$CWD` inject | yes | **No** | partial | **No** |
| PreToolUse exit 2 block | yes | **No** (ignored) | **No** | via plugin gate |
| PostToolUse on failed shell | yes | UNVERIFIED | **No** | partial (no exit in event) |
| Rules as charter | CLAUDE.md | `.mdc` rules | AGENTS.md | AGENTS.md |
| OS sandbox on Bash | Claude `/sandbox` | host default | Codex `sandbox_mode` | plugin/env |

Portable work: documentation, hook adapters, and charter files. Cursor-specific
UI (Bugbot, cloud agents) stays on the Cursor host.
</rules>

## Sandbox truth

<rules>

Cursor does not expose Claude-style Seatbelt/bubblewrap through vibe-agent.
vibe `sandbox.yaml` remains a **runner port** (local/docker), not Cursor isolation.
See [`claude-code-feature-map.md`](claude-code-feature-map.md) section "Sandbox truth".
</rules>

## Routing & discovery

<routing>

- Hook contracts: [`host-hook-contracts.md`](host-hook-contracts.md)
- Peer Claude map: [`claude-code-feature-map.md`](claude-code-feature-map.md)
- Peer Codex map: [`codex-feature-map.md`](codex-feature-map.md)
- Peer opencode map: [`opencode-feature-map.md`](opencode-feature-map.md)
- Research slug: `feature-maps-all-supported`
</routing>
