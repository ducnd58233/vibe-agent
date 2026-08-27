# Codex feature map (for vibe-agent)

<context>

Maps OpenAI Codex CLI / IDE features to vibe-agent owners. Primary sources:
[config reference](https://learn.chatgpt.com/docs/config-file/config-reference),
[hooks](https://learn.chatgpt.com/docs/hooks), [MCP](https://learn.chatgpt.com/docs/mcp).
Local wiring: [`.codex/config.toml`](../../.codex/config.toml),
[`host-hook-contracts.md`](host-hook-contracts.md) (`codexContract`).

Use when importing an idea from Codex into the toolkit. Prefer linking here over
restating Codex docs. Domain product logic does not belong here.

**Parity bar host** (see [`AUTHORING.md`](../AUTHORING.md) section "Supported harness parity").
Peer maps: [`claude-code-feature-map.md`](claude-code-feature-map.md),
[`cursor-feature-map.md`](cursor-feature-map.md), [`opencode-feature-map.md`](opencode-feature-map.md).
</context>

## Feature map

<rules>

| Codex feature | Reuse | Reject | Gap / vibe owner |
|---------------|-------|--------|------------------|
| `config.toml` (approval_policy, sandbox_mode, features.*) | Document operator vocabulary | Treat Codex sandbox as vibe runner | [`.codex/config.toml`](../../.codex/config.toml); [`claude-code-feature-map.md`](claude-code-feature-map.md) §Sandbox truth |
| `AGENTS.md` rules file | Shared charter with toolkit | Duplicate long policy in config | [`AGENTS.md`](../../AGENTS.md) |
| Project `hooks.json` | `vibe-agent hook` events | Exit 2 block (ignored) | [`hooks/ROUTER.md`](../hooks/ROUTER.md) |
| `permissionDecision` JSON on PreToolUse | Align with danger list messaging | Rely on exit codes for block | [`tool-safety-and-permissions.md`](tool-safety-and-permissions.md) |
| Agent Skills (`.agents/skills`) | Same skill layout as Claude | Codex-only skill paths in toolkit assets | [`skills/ROUTER.md`](../skills/ROUTER.md) |
| MCP servers in config | `vibe-agent mcp serve` | New evidence source | [`token-efficiency.md`](token-efficiency.md) |
| Subagents (`[agents]` threads) | Persona agents + host threads | Embed Codex thread runtime | [`agents/ROUTER.md`](../agents/ROUTER.md) |
| `--full-auto` | Hooks still run; pair with dontAsk-like policy | Equate with vibe `/auto` graph | [`commands/auto.md`](../commands/auto.md) |
| `--dangerously-skip-permissions` | Bare-like eval only | Strip danger PreToolUse in prod | [`claude-code-feature-map.md`](claude-code-feature-map.md) §Minimal host |
| PostToolUse lifecycle | Journal successful tool runs | Journal failed shell (PostToolUse **does not fire**) | [`host-hook-contracts.md`](host-hook-contracts.md); session gap |

</rules>

## Sandbox truth (Codex vs vibe)

<rules>

Codex `sandbox_mode` (e.g. workspace-write, danger-full-access) is **host**
isolation inside the Codex process. It is not vibe-agent `sandbox.yaml`.

| Layer | What it is | Isolation |
|-------|------------|-----------|
| Codex `sandbox_mode` | OpenAI CLI sandbox policy | Host-enforced per Codex invocation |
| vibe `driver: local` | Host subprocess | **None** |
| vibe `driver: docker` | `docker run --rm` bind mount | Container only; see Claude map §Sandbox truth |

</rules>

## Host portability matrix (Codex adaptations)

<rules>

| Adaptation | Claude | Cursor | Codex | opencode |
|------------|--------|--------|-------|----------|
| Feature map / honesty docs | yes | yes | yes | yes |
| PreToolUse exit 2 block | yes | **No** | **No** | plugin gate |
| PostToolUse on failed command | yes | UNVERIFIED | **No** (measured) | partial |
| JSON permissionDecision | partial | partial | yes | partial |
| OS sandbox vocabulary | Claude `/sandbox` | host | Codex `sandbox_mode` | env/plugin |
| Skills under `.agents/skills` | yes | via link | yes | yes |

Portable work: docs, hook adapters, shared `.agents/skills`. Codex sandbox and
approval policies stay on the Codex host.
</rules>

## Routing & discovery

<routing>

- Local config: [`.codex/config.toml`](../../.codex/config.toml)
- Hook contracts: [`host-hook-contracts.md`](host-hook-contracts.md)
- Peer Claude map: [`claude-code-feature-map.md`](claude-code-feature-map.md)
- Peer Cursor map: [`cursor-feature-map.md`](cursor-feature-map.md)
- Peer opencode map: [`opencode-feature-map.md`](opencode-feature-map.md)
- Research slug: `feature-maps-all-supported`
</routing>
