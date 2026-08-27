# Kimi CLI feature map (for vibe-agent)

<context>

Maps Moonshot Kimi CLI features to vibe-agent owners. Primary sources:
[config files](https://moonshotai.github.io/kimi-cli/en/configuration/config-files.html),
[hooks](https://moonshotai.github.io/kimi-cli/en/configuration/hooks.html). Local
snippet: [`.kimi/hooks.toml`](../../.kimi/hooks.toml),
[`host-hook-contracts.md`](host-hook-contracts.md) (`kimiContract`).

Use when importing an idea from Kimi CLI into the toolkit. Prefer linking here
over restating Kimi docs. Domain product logic does not belong here.

**Research host** (not parity bar). Hooks are **UNVERIFIED** in this repo.
See also [`host-contracts-researched.md`](host-contracts-researched.md).
</context>

## Feature map

<rules>

| Kimi feature | Reuse | Reject | Gap / vibe owner |
|--------------|-------|--------|------------------|
| User `~/.kimi/config.toml` | Operator docs for global config | Commit secrets or user paths | Snippet only in repo |
| Repo `.kimi/hooks.toml` | Hook command wiring pattern | Assume repo config overrides user without docs | [`.kimi/hooks.toml`](../../.kimi/hooks.toml) |
| PreToolUse / PostToolUse / Stop | Map to `vibe-agent hook` events | SessionStart / prompt hooks (not documented) | UNVERIFIED |
| TOML hook config | Document alongside JSON hosts | Single config format in harness | `contracts.go` |
| Agent skills / rules | Vocabulary only until wired | Kimi-only skill paths in `.ai-agents/` | Decline until parity request |
| MCP | Host MCP when available | New evidence source | [`token-efficiency.md`](token-efficiency.md) |
| Moonshot model routing | Product concern | Embed in toolkit runtime | Decline |

</rules>

## Verification status

<rules>

Kimi hook contracts are **UNVERIFIED**. No measured hook fire in this workspace.
Do not require Kimi in supported harness parity until verification lands.
</rules>

## Host portability matrix (Kimi adaptations)

<rules>

| Adaptation | Claude | Kimi | Notes |
|------------|--------|------|-------|
| Feature map (this file) | yes | yes | research-only |
| hooks.json | yes | **No** (TOML) | adapter in contracts.go |
| SessionStart | yes | **No** | gap |
| CI observation | partial | **No** | UNVERIFIED |

Portable work: TOML hook snippet, contract documentation, research notes.
</rules>

## Routing & discovery

<routing>

- Researched contracts: [`host-contracts-researched.md`](host-contracts-researched.md)
- Hook contracts: [`host-hook-contracts.md`](host-hook-contracts.md)
- Local snippet: [`.kimi/hooks.toml`](../../.kimi/hooks.toml)
- Research slug: `feature-maps-all-supported`
</routing>
