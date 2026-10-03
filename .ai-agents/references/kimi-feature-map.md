# Kimi CLI feature map (for vibe-agent)

<context>

Maps Moonshot Kimi CLI features to vibe-agent owners. Primary sources:
[config files](https://moonshotai.github.io/kimi-code/en/configuration/config-files.html),
[hooks](https://moonshotai.github.io/kimi-code/en/customization/hooks.html). Local
snippet: [`.kimi-code/hooks.toml`](../../.kimi-code/hooks.toml),
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
| User `~/.kimi-code/config.toml` | Operator docs for global config | Commit secrets or user paths | Snippet only in repo |
| Repo `.kimi-code/hooks.toml` | Hook command wiring pattern | Assume repo config is read (hooks are user-level only) | [`.kimi-code/hooks.toml`](../../.kimi-code/hooks.toml) |
| SessionStart / UserPromptSubmit / PreToolUse / PostToolUse / PostToolUseFailure / Stop | Map to `vibe-agent hook` events | Inject at SessionStart (observation-only) | `contracts.go` (`kimiContract`), UNVERIFIED |
| TOML hook config | Document alongside JSON hosts | Single config format in harness | `contracts.go` |
| Agent skills / rules | `.kimi-code/skills`, shared `.agents/skills`, `AGENTS.md` | Kimi-only skill copies in `.ai-agents/` | `internal/hosts` catalog |
| MCP | Host MCP when available | New evidence source | [`token-efficiency.md`](token-efficiency.md) |
| Moonshot model routing | Product concern | Embed in toolkit runtime | Decline |

</rules>

## Verification status

<rules>

Kimi Code hook contracts are **UNVERIFIED** here: no hook has been watched firing from this
config. The shapes come from the vendor's own documentation
([`MoonshotAI/kimi-code` docs/en/customization/hooks.md](https://github.com/MoonshotAI/kimi-code/blob/main/docs/en/customization/hooks.md)),
read on 2026-10-03: user-level `~/.kimi-code/config.toml` only; six events wired (SessionStart is
observation-only); UserPromptSubmit appends stdout text to context; exit 2 blocks with stderr as
the reason; skills at `.kimi-code/skills` and `.agents/skills`.
</rules>

## Host portability matrix (Kimi adaptations)

<rules>

| Adaptation | Claude | Kimi | Notes |
|------------|--------|------|-------|
| Feature map (this file) | yes | yes | research-only |
| hooks.json | yes | **No** (TOML) | adapter in contracts.go |
| SessionStart injection | yes | **No** (observation-only) | context arrives with the first prompt |
| CI observation | partial | **No** | UNVERIFIED |

Portable work: TOML hook snippet, contract documentation, research notes.
</rules>

## Routing & discovery

<routing>

- Researched contracts: [`host-contracts-researched.md`](host-contracts-researched.md)
- Hook contracts: [`host-hook-contracts.md`](host-hook-contracts.md)
- Local snippet: [`.kimi-code/hooks.toml`](../../.kimi-code/hooks.toml)
- Research slug: `feature-maps-all-supported`
</routing>
