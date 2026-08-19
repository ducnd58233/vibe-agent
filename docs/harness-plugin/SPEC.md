# Spec: harness plugin packaging (v1)

## Open questions

1. Confirm slug **`harness-plugin`** (distinct from `harness-plugins-session-ux`, which tracks session UX + manifests in parallel).
2. Confirm v1 ships **manifest emission + docs only**; marketplace publish to Anthropic/Cursor/OpenAI directories is out of scope.
3. Confirm Superpowers-style **skills + commands** is the reference shape for asset plugins (intake example: https://github.com/obra/superpowers).

If unanswered, defaults below stand.

## ASSUMPTIONS

1. "Plugin" means packaging this toolkit for host install (manifest + generated views), not executing third-party JS inside `runtime/`.
2. `.ai-agents/` remains canonical; host files are generated or thin pointers.
3. OpenCode keeps the existing JS hook adapter at `.opencode/plugin/vibe-agent.js`.
4. Research digest lives in [`RESEARCH.md`](RESEARCH.md); implementation detail may overlap [`docs/harness-plugins-session-ux/`](../harness-plugins-session-ux/) — prefer one source of truth per file, link instead of copy.
5. Web UI changes (session sort/delete) are **not** required for this slug unless plan explicitly pulls them in.

Correct now or these stand.

## Objective

A developer can install vibe-agent as a **host plugin** (Claude, Cursor Agent Plugin, Codex) from emitted manifests, while the clone+link path keeps working unchanged.

Measurable done:

- `scripts/link-ai-agents.sh` (and Windows sibling) emit valid plugin manifest stubs pointing at `.ai-agents/` content.
- Docs under `docs/harness-plugin/` describe install paths for each host and the Superpowers-style skills model.
- `go -C runtime test ./...` and `bash scripts/check-generated-views.sh` pass after manifest changes.
- `/ship` GO on a task PR after human `spec_approved` and `plan_approved`.

## Users

- Toolkit maintainers cutting a release.
- Developers mounting `.vibe-agent` in a consumer repo (current path).
- Developers preferring `/plugin install` or `codex plugin add`.

## Tech stack

Pinned in `runtime/go.mod`: Go 1.26.5, stdlib HTTP for web (unchanged). Asset trees under `.ai-agents/`. Link scripts: `scripts/link-ai-agents.sh`, `scripts/link-ai-agents.ps1`. Host JSON must match docs at implement time ([Claude plugins](https://code.claude.com/docs/en/plugins.md), [Cursor plugins](https://cursor.com/docs/plugins.md), [Codex plugins](https://developers.openai.com/codex/plugins/build), [Agent Plugins](https://agent-plugins.org/)).

## Commands

```text
bash scripts/check-generated-views.sh
bash scripts/link-ai-agents.sh --workspace "$PWD" --assets "$PWD/.ai-agents"
go -C runtime test ./...
vibe-agent doctor
```

Manifest dry-run proof: link script writes `.claude-plugin/plugin.json`, Agent Plugin `plugin.json`, and `.codex-plugin/plugin.json` (exact paths per host docs) without duplicating skill bodies.

## Project structure (this workspace)

| Path | Role |
|---|---|
| `.ai-agents/` | Canonical skills, agents, commands, hooks, graphs |
| `scripts/link-ai-agents.*` | Generated host views + plugin manifests |
| `.opencode/plugin/vibe-agent.js` | OpenCode hook adapter (keep) |
| `docs/harness-plugin/` | This spec, research, later PLAN/TASKS |
| `docs/harness-plugins-session-ux/` | Related UX + task breakdown (reference) |

Do not add `runtime/internal/plugin/` evaluator.

## Data classification

| Data | Allowed | Never |
|---|---|---|
| Skill/command markdown | Host load paths, git | Committed secrets in manifests |
| MCP config templates | Example stubs with env var refs | Literal API keys in JSON |
| Session logs under `tmp/` | Local verification only | PR bodies, committed artifacts |

## Code style

Match existing Go and shell in `scripts/` and `runtime/`: small functions, wrapped errors, plain comments. Manifest JSON: host-required fields only, no speculative keys.

## Testing strategy

- Unit: any Go helpers added for manifest validation.
- Script: `check-generated-views.sh` fails if manifests drift from `.ai-agents/`.
- Manual: install stub into a throwaway host config dir per host doc (record under `tmp/harness-plugin/`).

## Boundaries

**Always:** edit `.ai-agents/` source, regenerate views, run tests before checkpoint.

**Ask:** changing command namespaces for marketplace-only install; publishing to public marketplaces.

**Never:** model-only checkpoint without artifact or command output; duplicate long policy into manifests; run third-party plugin code inside `runtime/`.

## Success criteria

1. RESEARCH.md and SPEC.md exist at `docs/harness-plugin/`.
2. PLAN.md + TASKS.md follow human approval of this spec.
3. At least one task PR adds manifest emission with tests.
4. `vibe-agent doctor` OK on workspace with generated manifests.

## Out of scope (v1)

- Public marketplace listing or review submission.
- Replacing clone+link with plugin-only workflow.
- In-process plugin sandbox inside Go runtime.
