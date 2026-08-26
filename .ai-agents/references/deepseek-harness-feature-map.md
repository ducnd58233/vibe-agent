# DeepSeek Harness feature map (for vibe-agent)

<context>

Maps DeepSeek Harness (`dsh`) / Cordis developer-preview features to vibe-agent
owners. Primary sources: [product page](https://deepseek.com/harness/en/),
[extension cookbook](https://deepseek-harness.github.io/deepseek-harness/en/reference/cookbook/extension-cookbook),
[GitHub deepseek-ai/deepseek-harness](https://github.com/deepseek-ai/deepseek-harness).
Prior workspace literature: `docs/2026-08-17/deepseek-harness-telemetry`.

Use this when importing an idea from DeepSeek Harness. Prefer linking here over
restating `dsh` docs. Cordis plugins are **not** Agent Skills; see
[AUTHORING.md](../AUTHORING.md). Domain product logic does not belong here.
Peer map for Claude Code: [`claude-code-feature-map.md`](claude-code-feature-map.md).
</context>

## Feature map

<rules>

| DeepSeek feature | Reuse | Reject | Gap / vibe owner |
|------------------|-------|--------|------------------|
| Cordis everything-is-plugin kernel | Extension-seam vocabulary | Vendor Cordis; replace Go control plane | Decline; outer loop stays in `runtime/` |
| Append-only session log (model-visible) | Host-gesture session append + Trajectory | Claim full parity on every host | [`host-hook-contracts.md`](host-hook-contracts.md), `runtime/internal/session` |
| Standard / Creator modes | Product context only | Mount Creator as vibe host | none |
| Code mode (TS multi-tool program) | Idea of fewer round trips | Implement Code mode in vibe | Decline; host inner loop |
| Minimal mode (bash + editor) | Bare-like eval recipe wording | Require Minimal as only host | [`claude-code-feature-map.md`](claude-code-feature-map.md) §Minimal host |
| Hook waterfalls (`tools/pre-execute`, …) | Compare to `vibe-agent hook` | Cordis listeners as graph evidence | [`hooks/ROUTER.md`](../hooks/ROUTER.md) |
| `dsh-hooks-claude-code` / `dsh-hooks-codex` | Proof that bridges are product-local | Depend on those packages | This file §Host portability |
| landlock / sandbox-exec (`dsh-bash-sandbox`) | Honesty language next to Claude OS sandbox | In-process OS sandbox in Go | [`claude-code-feature-map.md`](claude-code-feature-map.md) §Sandbox truth |
| Skills (Cordis skill plugins) | Keep skill converters separate | Treat Cordis plugin as `SKILL.md` | [AUTHORING.md](../AUTHORING.md); `vibe-agent skills` |
| MCP / tool registry plugins | Host MCP + `vibe-agent mcp serve` | New evidence source | [`token-efficiency.md`](token-efficiency.md) |
| Session telemetry / OTEL export | Local-first journal default | Ship OTEL as required path | Decline for now; journal stays local |
| Subagent providers (fork/ACP/Claude/Codex) | Persona + host Task pattern | Embed `dsh` subagent runtime | [`agents/ROUTER.md`](../agents/ROUTER.md) |

</rules>

## Sandbox layers (DeepSeek vs vibe)

<rules>

DeepSeek Harness documents a **subprocess OS sandbox** via `ctx.sandbox` /
`dsh-bash-sandbox` (landlock / sandbox-exec) ([extension cookbook](https://deepseek-harness.github.io/deepseek-harness/en/reference/cookbook/extension-cookbook)).
That is the same *class* of isolation as Claude Seatbelt/bubblewrap, and it is
**not** vibe-agent `sandbox.yaml`.

| Layer | What it is | Isolation |
|-------|------------|-----------|
| DeepSeek `dsh-bash-sandbox` | Cordis sandbox backend on shell tools | OS-enforced (landlock / sandbox-exec) inside `dsh` |
| Claude `/sandbox` | Seatbelt or bubblewrap on Bash | OS-enforced per command on Claude host |
| vibe `driver: local` | Host subprocess via `safexec` | **None** |
| vibe `driver: docker` | `docker run --rm` + workspace bind RW | Container namespaces only; not landlock |

Do not say vibe-agent “has DeepSeek sandbox.” Embedded container/GPU inside the
Go process stays declined ([`AGENTS.md`](../../AGENTS.md)).
</rules>

## Plugins vs skills

<procedure>

1. A **Cordis plugin** changes what the `dsh` runtime can do (model adapter,
   tool registry, session log, loop). It needs the Cordis kernel.
2. An **Agent Skill** (`SKILL.md`) is a portable workflow asset for Claude /
   Cursor / Codex / opencode discovery roots. `vibe-agent skills` installs those.
3. DeepSeek Harness is not a skill converter ([AUTHORING.md](../AUTHORING.md)).
   Do not route Cordis packages through `skills add`.
</procedure>

## Host portability matrix (DeepSeek adaptations)

<rules>

| Adaptation | Claude | Cursor | Codex | opencode |
|------------|--------|--------|-------|----------|
| Feature map / honesty docs | yes | yes | yes | yes |
| Cordis plugin (`apply(ctx)`) | **No** | **No** | **No** | **No** |
| Code mode TS tool SDK | **No** | **No** | **No** | **No** |
| landlock / `dsh-bash-sandbox` | **No** (Claude has its own OS sandbox) | **No** | **No** (Codex has host `sandbox_mode`) | **No** |
| `dsh-hooks-claude-code` bridge | **No** (bridge is inside `dsh`) | **No** | **No** | **No** |
| Session-log *idea* via host hooks | partial (hook surface) | partial / UNVERIFIED | partial (no failure journal) | partial (plugin) |
| Minimal / bare-like recipe (docs) | yes | yes | yes | yes |

Improving vibe-agent with DeepSeek **ideas** does not make Cordis features run
on every GenAI host this toolkit supports. Portable work is documentation,
honesty, and host-contract-limited capture.
</rules>

## Routing & discovery

<routing>

- Peer Claude map: [`claude-code-feature-map.md`](claude-code-feature-map.md)
- Hook contracts: [`host-hook-contracts.md`](host-hook-contracts.md)
- Authoring Cordis note: [AUTHORING.md](../AUTHORING.md)
- Prior research slug: `deepseek-harness-telemetry`
- Delivery research slug: `harness-deepseek-hi-n`
</routing>
