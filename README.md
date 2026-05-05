# vibe-agent

Reusable toolkit for AI coding workflows: shared skills, subagents, slash commands, routing docs, stack profiles, and hook scripts.

## What this repository does

- Centralizes reusable AI assets under [`.ai-agents/`](.ai-agents)
- Keeps asset discovery explicit with router tables (`ROUTER.md`)
- Supports multiple tools (Claude Code, Cursor, Codex, opencode) with one canonical source
- Provides link scripts so tool-specific folders can point to shared assets

## Folder structure

- [`.ai-agents/`](.ai-agents): canonical shared assets
  - [`skills/`](.ai-agents/skills): reusable workflows (`SKILL.md`)
  - [`agents/`](.ai-agents/agents): persona files for subagent-style delegation
  - [`commands/`](.ai-agents/commands): slash-command prompts
  - [`stack-profiles/`](.ai-agents/stack-profiles): workspace stack playbooks
  - [`references/`](.ai-agents/references): generic checklists/patterns
  - [`hooks/`](.ai-agents/hooks): shared hook scripts
  - [`ROUTER.md`](.ai-agents/ROUTER.md): top-level asset routing index
- [`.claude/`](.claude): Claude settings and linked discovery paths
- [`.cursor/`](.cursor): Cursor rules/hooks and linked discovery paths
- [`.codex/`](.codex): Codex project config
- [`scripts/`](scripts): helper scripts (link setup and router validation)

## Useful commands

### Repository scripts

- `powershell -File scripts/link-ai-agents.ps1`
  - Creates/repairs Windows junctions from `.claude/.cursor/.opencode` to `.ai-agents`
- `bash scripts/link-ai-agents.sh`
  - Same as above for macOS/Linux
- `bash scripts/check-ai-agents-routers.sh`
  - Validates folder `ROUTER.md` tables match files on disk
- `powershell -File scripts/check-ai-agents-routers.ps1`
  - Windows wrapper for the same router check

### Slash commands (from `.ai-agents/commands`)

- `/spec`: produce scoped implementation spec
- `/plan`: break spec into actionable steps
- `/build`: implement next planned task with test discipline
- `/test`: run test-driven/prove-it style validation
- `/review`: run focused code review
- `/ship`: parallel ship decision flow (review + security + tests)
- `/research`: run citation-first topic investigation
- `/analyze`: produce evidence-based recommendation
- `/investigate`: run parallel investigator + analyst + source-audit merge

## How to use this toolkit in another repo

Mount this repository at a chosen `<toolkit-root>` (for example `.vibe-agent`) and point consumer tool folders to `<toolkit-root>/.ai-agents/*`. See:

- [`AGENTS.md`](AGENTS.md)
- [`.ai-agents/README.md`](.ai-agents/README.md)
