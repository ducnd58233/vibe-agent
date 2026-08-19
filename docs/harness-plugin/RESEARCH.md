# Research: what "harness plugin" means (2026)

Question: what are coding-agent harnesses shipping as "plugins" worldwide, and what should vibe-agent adopt without inventing a fifth runtime?

Intake example: [obra/superpowers](https://github.com/obra/superpowers) — an agentic **skills framework** distributed as markdown skills + workflow commands, not as executable code inside the host. That matches the **asset plugin** family below.

## Findings

### 1. Two plugin families (do not mix)

| Kind | Examples | Host runs code? | vibe-agent stance |
|---|---|---|---|
| **Asset plugin** | Claude `.claude-plugin/`, Cursor Agent Plugin, Codex `.codex-plugin/` | No (manifest + markdown + optional MCP config) | **Target.** Same material as `.ai-agents/`. |
| **In-process plugin** | OpenCode JS module, host hook scripts | Yes | **Keep one adapter:** `.opencode/plugin/vibe-agent.js`. Do not reimplement as MCP. |

Superpowers sits in the asset family: skills and slash-style workflows the model loads, comparable to this repo's `.ai-agents/skills` and `.ai-agents/commands`.

### 2. Major hosts (bundle shape)

| Host | Manifest | Bundles | Install |
|---|---|---|---|
| Claude Code | `.claude-plugin/plugin.json` | skills, agents, hooks, MCP | `/plugin` + git marketplaces |
| Cursor | `plugin.json` or `.cursor-plugin/plugin.json` | skills, MCP (Agent Plugin); + rules, hooks (Cursor Plugin) | Customize / Marketplace |
| Codex | `.codex-plugin/plugin.json` | skills, hooks, `.mcp.json` | `codex plugin` CLI |
| OpenCode | npm or `.opencode/plugins/` JS | lifecycle hooks, custom tools | config + local dir |

Agent Plugins 1.0.0 ([agent-plugins.org](https://agent-plugins.org/)) is the portable floor Cursor, OpenAI, and others align on.

### 3. Marketplaces are catalogs, not evaluators

Claude, Cursor, and Codex install from **marketplace JSON + git**, with team policy. Distribution ≠ loading arbitrary code inside vibe-agent's Go runtime.

### 4. Namespacing vs clone path

Marketplace installs namespace commands (`/plugin-name:goal`). Clone+link keeps `/vibe-goal` and `$vibe-goal`. **Dual distribution:** generated views for mounted toolkits; thin manifests for marketplace install.

### 5. Related work in this repo

[`docs/harness-plugins-session-ux/RESEARCH.md`](../harness-plugins-session-ux/RESEARCH.md) already digests host docs and local session-list gaps. This slug owns the **concept + packaging spec**; that slug owns **concrete UX + manifest emission tasks** unless merged later.

## Recommendation

1. Keep `.ai-agents/` as the only asset source of truth.
2. Extend `scripts/link-ai-agents.*` to emit host manifests (Claude, Cursor Agent Plugin, Codex) from the same tree.
3. Do not build a vibe-agent marketplace or Go plugin VM in v1.
4. Prove packaging with link-script dry-run + `vibe-agent doctor` + host doc field checks at implement time.

## Confidence

High on the two-family split and Superpowers as a skills-framework reference. Medium on exact marketplace.json field names — verify against host docs at implement time.
