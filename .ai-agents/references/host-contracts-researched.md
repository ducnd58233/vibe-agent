# Researched host contracts

<context>

Historical reading notes for **Antigravity**, **Kimi**, and **Muse** before they gained
`Client` values in the runtime.

**Canonical contracts now live in [`host-hook-contracts.md`](host-hook-contracts.md)**,
generated from `runtime/internal/harness/contracts.go`. Edit that table, not this file,
when wiring or verification status changes.

Nothing here was run when this file was first written. Rows in the generated document stay
**UNVERIFIED** until someone watches a hook fire.
</context>

## Routing & discovery

<routing>

- Hook wiring, event keys, and doctor checks: [`host-hook-contracts.md`](host-hook-contracts.md)
- Workspace hook configs: `scripts/link-ai-agents.sh` (`.agents/hooks.json`, `.muse/hooks.json`,
  `.kimi/hooks.toml` snippet)
- Kimi merges the snippet into `~/.kimi/config.toml`; Muse requires `muse hooks trust`
</routing>
