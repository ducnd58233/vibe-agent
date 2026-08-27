# Antigravity feature map (for vibe-agent)

<context>

Maps Google Antigravity agent features to vibe-agent owners. Primary sources:
[hooks](https://antigravity.google/docs/hooks). Local wiring:
[`.agents/hooks.json`](../../.agents/hooks.json) (shared path with other hosts),
[`host-hook-contracts.md`](host-hook-contracts.md) (`antigravityContract`).

Use when importing an idea from Antigravity into the toolkit. Prefer linking
here over restating Antigravity docs. Domain product logic does not belong here.

**Research host** (not parity bar). Hooks are **UNVERIFIED** in this repo.
See also [`host-contracts-researched.md`](host-contracts-researched.md).
Peer parity maps: [`cursor-feature-map.md`](cursor-feature-map.md),
[`codex-feature-map.md`](codex-feature-map.md), [`opencode-feature-map.md`](opencode-feature-map.md).
</context>

## Feature map

<rules>

| Antigravity feature | Reuse | Reject | Gap / vibe owner |
|---------------------|-------|--------|------------------|
| `.agents/hooks.json` | Same path as Claude/Codex pattern | Assume identical stdin schema | [`host-hook-contracts.md`](host-hook-contracts.md) |
| PreInvocation / injectSteps | Context injection idea | Replace SessionStart on other hosts | Document only; no `$CWD` equivalent |
| PreToolUse / PostToolUse | `vibe-agent hook` mapping | Exit 2 block until measured | UNVERIFIED |
| Stop hook | Run finalization journal | Stop as graph evidence without verify | UNVERIFIED |
| `workspacePaths` array on stdin | Multi-root workspace awareness | Hard-code single cwd in hooks | `runtime/internal/harness` |
| Agent rules / skills | `.agents/` layout vocabulary | Require Antigravity-only assets in toolkit | [`AUTHORING.md`](../AUTHORING.md) |
| MCP | Host MCP when documented | New evidence source | [`token-efficiency.md`](token-efficiency.md) |
| Google-specific sandbox | Host isolation if shipped | In-process sandbox in Go | Decline; see Claude map §Sandbox truth |

</rules>

## Verification status

<rules>

All Antigravity hook rows in [`host-hook-contracts.md`](host-hook-contracts.md)
are **UNVERIFIED**. Do not list Antigravity in the supported harness parity
checklist until an observation campaign updates contract status.
</rules>

## Host portability matrix (Antigravity adaptations)

<rules>

| Adaptation | Claude | Antigravity | Notes |
|------------|--------|-------------|-------|
| Feature map (this file) | yes | yes | research-only |
| SessionStart hook | yes | **No** (PreInvocation inject) | different lifecycle |
| Measured hook fire in CI | yes | **No** | UNVERIFIED |
| Shared `.agents/hooks.json` path | yes | yes | schema may differ |

Portable work: documentation and contract stubs only until verified.
</rules>

## Routing & discovery

<routing>

- Researched contracts: [`host-contracts-researched.md`](host-contracts-researched.md)
- Hook contracts: [`host-hook-contracts.md`](host-hook-contracts.md)
- Peer Claude map: [`claude-code-feature-map.md`](claude-code-feature-map.md)
- Research slug: `feature-maps-all-supported`
</routing>
