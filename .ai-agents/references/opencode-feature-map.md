# opencode feature map (for vibe-agent)

<context>

Maps opencode agent features to vibe-agent owners. Primary sources:
[plugins](https://opencode.ai/docs/plugins/), [permissions](https://opencode.ai/docs/permissions/),
[config](https://opencode.ai/docs/config/). Local wiring: [`.opencode/plugin/`](../../.opencode/plugin/),
[`opencode.json`](../../opencode.json), [`host-hook-contracts.md`](host-hook-contracts.md)
(`opencodeContract`).

Use when importing an idea from opencode into the toolkit. Prefer linking here
over restating opencode docs. Domain product logic does not belong here.

**Parity bar host** (see [`AUTHORING.md`](../AUTHORING.md) section "Supported harness parity").
Peer maps: [`claude-code-feature-map.md`](claude-code-feature-map.md),
[`cursor-feature-map.md`](cursor-feature-map.md), [`codex-feature-map.md`](codex-feature-map.md).
</context>

## Feature map

<rules>

| opencode feature | Reuse | Reject | Gap / vibe owner |
|------------------|-------|--------|------------------|
| JS/TS plugin (`.opencode/plugin/`) | Hook-equivalent via `tool.execute.before` / `after` | Shell `hooks.json` as required path | [`hooks/ROUTER.md`](../hooks/ROUTER.md); plugin in repo |
| `permission.ask` / deny | Align with danger list at tool gate | Treat ask UI as graph evidence | [`tool-safety-and-permissions.md`](tool-safety-and-permissions.md) |
| `opencode.json` permissions | Document allow/deny patterns | Widen allowlist to pass CI | [`.ai-agents/PERMISSIONS.md`](../PERMISSIONS.md) |
| Agent Skills (`.agents/skills`) | Shared skill tree with Claude/Codex | opencode-only skill format in toolkit | [`skills/ROUTER.md`](../skills/ROUTER.md) |
| `AGENTS.md` rules | Shared charter | Duplicate toolkit policy | [`AGENTS.md`](../../AGENTS.md) |
| MCP | Host MCP + `vibe-agent mcp serve` | New evidence source | [`token-efficiency.md`](token-efficiency.md) |
| Stop / session hooks | **No** Stop hook in plugin API | Fake stop via plugin hack | Gap: use runtime journal + verify |
| Failed tool exit status | `tool.execute.after` without reliable exit code | Journal failures like Claude PostToolUse | [`host-hook-contracts.md`](host-hook-contracts.md) |
| Subagents | Host-native if available | Require opencode-only agent schema | [`agents/ROUTER.md`](../agents/ROUTER.md) |

</rules>

## Plugin vs shell hooks

<procedure>

opencode has **no** project-level shell hook runner like Claude or Codex.
Cross-host features that depend on `vibe-agent hook stop` or PreToolUse exit 2
must be reimplemented in the opencode plugin or documented as host gaps.

Shipped adapter: `.opencode/plugin/` (journal + permission patterns). Edit
contracts in `runtime/internal/harness/contracts.go`, not only the plugin file.
</procedure>

## Host portability matrix (opencode adaptations)

<rules>

| Adaptation | Claude | Cursor | Codex | opencode |
|------------|--------|--------|-------|----------|
| Feature map / honesty docs | yes | yes | yes | yes |
| Shell hooks.json | yes | yes | yes | **No** (plugin only) |
| PreToolUse exit 2 | yes | **No** | **No** | permission gate |
| Stop hook | yes | yes | partial | **No** |
| PostToolUse failure journal | yes | UNVERIFIED | **No** | partial |
| Skills `.agents/skills` | yes | linked | yes | yes |

Portable work: plugin adapter, shared skills, charter docs. opencode-specific
permission UI stays on the opencode host.
</rules>

## Routing & discovery

<routing>

- Plugin: [`.opencode/plugin/`](../../.opencode/plugin/)
- Config: [`opencode.json`](../../opencode.json)
- Hook contracts: [`host-hook-contracts.md`](host-hook-contracts.md)
- Peer Claude map: [`claude-code-feature-map.md`](claude-code-feature-map.md)
- Research slug: `feature-maps-all-supported`
</routing>
