# Muse feature map (for vibe-agent)

<context>

Maps Meta Muse Code features to vibe-agent owners. Primary sources:
[extending Muse](https://dev.meta.ai/docs/muse-code/extending),
[hooks](https://dev.meta.ai/docs/muse-code/hooks). Local wiring:
[`.muse/hooks.json`](../../.muse/hooks.json),
[`host-hook-contracts.md`](host-hook-contracts.md) (`museContract`).

Use when importing an idea from Muse into the toolkit. Prefer linking here over
restating Muse docs. Domain product logic does not belong here.

**Research host** (not parity bar). Hooks are **UNVERIFIED** (beta builds).
See also [`host-contracts-researched.md`](host-contracts-researched.md).
</context>

## Feature map

<rules>

| Muse feature | Reuse | Reject | Gap / vibe owner |
|--------------|-------|--------|------------------|
| Claude-like `hooks.json` schema | Reuse hook adapter patterns from Claude | Assume 1:1 parity with Claude Code | [`host-hook-contracts.md`](host-hook-contracts.md) |
| `muse hooks trust` | Document trust step for operators | Skip trust in committed hooks | [`.muse/hooks.json`](../../.muse/hooks.json) README in AUTHORING |
| PreToolUse / PostToolUse / Stop | `vibe-agent hook` events | Block via exit 2 until measured | UNVERIFIED |
| SessionStart | If Muse adds it, align with Claude | Invent SessionStart | watch Muse changelog |
| AGENTS.md / rules | Shared charter pattern | Muse-only toolkit pointers | [`AGENTS.md`](../../AGENTS.md) |
| MCP | Host MCP when stable | New evidence source | [`token-efficiency.md`](token-efficiency.md) |
| Meta sandbox / isolation | Host concern | In-process sandbox in Go | Decline |

</rules>

## Verification status

<rules>

Muse hook contracts are **UNVERIFIED**. Beta hook behavior may change without
notice. Do not list Muse in supported harness parity until verified.
</rules>

## Host portability matrix (Muse adaptations)

<rules>

| Adaptation | Claude | Muse | Notes |
|------------|--------|------|-------|
| Feature map (this file) | yes | yes | research-only |
| hooks.json schema | yes | similar | trust step extra |
| Measured CI hooks | partial | **No** | UNVERIFIED |
| Skills / commands | yes | partial | follow Muse docs |

Portable work: hook JSON stub, contracts, documentation.
</rules>

## Routing & discovery

<routing>

- Researched contracts: [`host-contracts-researched.md`](host-contracts-researched.md)
- Hook contracts: [`host-hook-contracts.md`](host-hook-contracts.md)
- Local hooks: [`.muse/hooks.json`](../../.muse/hooks.json)
- Peer Claude map: [`claude-code-feature-map.md`](claude-code-feature-map.md)
- Research slug: `feature-maps-all-supported`
</routing>
