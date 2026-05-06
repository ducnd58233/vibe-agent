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
- [`.claude/`](.claude): Claude settings; `skills` / `agents` / `commands` are **generated junctions/symlinks** (run link script after clone; see [`.gitignore`](.gitignore))
- [`.cursor/`](.cursor): Cursor rules/hooks; `skills` and `commands` are **generated links** (same script)
- [`.codex/`](.codex): Codex project config
- [`scripts/`](scripts): helper scripts (link setup and router validation)

## Useful commands

### Repository scripts

- `powershell -File scripts/link-ai-agents.ps1`
  - Creates/repairs Windows junctions from `.claude/.cursor/.opencode` to `.ai-agents` (defaults: workspace = repo root)
  - Consumer repo (submodule at `.vibe-agent`): `powershell -File .vibe-agent/scripts/link-ai-agents.ps1 -WorkspaceRoot $PWD -AssetsRoot (Join-Path $PWD '.vibe-agent\.ai-agents')`
- `bash scripts/link-ai-agents.sh`
  - Same for macOS/Linux; consumer: `bash .vibe-agent/scripts/link-ai-agents.sh --workspace "$PWD" --assets "$PWD/.vibe-agent/.ai-agents"`
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

## Example chat workflow

Orchestration stays **human-driven**: run slash commands in sequence (see [`orchestration-patterns`](.ai-agents/references/orchestration-patterns.md)). Below is a **sample dialogue** (placeholder feature and paths—swap for your consumer repo).

### Turn 1 — Spec (Cursor Ask or Plan; read-only)

**You:** `/spec` — Add OAuth refresh rotation to our API. Constraints: no new deps unless justified; must pass existing `pytest tests/` and `ruff`. Save spec to `docs/features/oauth-refresh/SPEC.md`.

**Assistant:** Clarifies scope → writes the SPEC → asks you to confirm before `/plan`.

### Turn 2 — Plan

**You:** `/plan` @ `docs/features/oauth-refresh/SPEC.md` — Break into vertical slices, **one PR per slice**. Output `docs/features/oauth-refresh/plan.md`.

**Assistant:** Dependency graph, slices (e.g. A: model + migration, B: refresh endpoint, C: revoke + tests), acceptance criteria, verification commands.

**You:** Approve plan before implementation.

### Turn 3 — Implement slice A (Cursor Agent)

**You:** Branch `feat/oauth-refresh-slice-a` from `main`. Implement **only Slice A** from the plan. Run verification commands when done.

**Assistant:** Implements, runs tests, summarizes diff and open questions.

### Turn 4 — Review before PR

**You:** `/review` — Review the diff on `feat/oauth-refresh-slice-a`; surface blockers only.

**Assistant:** Review notes; you fix or delegate fixes.

### Turn 5 — Open PR (you, outside the chat)

Open a PR whose description links the SPEC, plan section, and issue tracker as your team expects.

### Turn 6 — Next slice

**You:** Branch `feat/oauth-refresh-slice-b`. Implement Slice B only; rebase on `main` if Slice A merged. Repeat Turn 3–5.

### Turn 7 — Pre-merge gate (larger or risky integration)

**You:** `/ship` — Integration branch (or final PR) ready: run parallel quality + security + test perspectives and **Ship Decision: GO | NO-GO**.

**Assistant:** GO/NO-GO, blockers, rollback considerations.

### Turn 8 — CI failure loop (Agent)

**You:** CI failed on `main`: paste failing test name and log excerpt. Fix minimally; do not widen scope.

**Assistant:** Patch and re-run local verification commands.

### Turn 9 — Merge (you)

After green CI and policy checks, merge via your Git host (merge queue, squash, etc.—outside this toolkit).

### One-shot prompt variant

Use in **Agent** mode when the plan and branch already exist:

> Implement Slice A from `docs/features/oauth-refresh/plan.md` on branch `feat/oauth-refresh-slice-a`. Run `pytest tests/unit -q` and `ruff check src`. Do not implement Slice B. Summarize files changed and risks.

## How to use this toolkit in another repo

Mount this repository at a chosen `<toolkit-root>` (for example `.vibe-agent`) and point consumer tool folders to `<toolkit-root>/.ai-agents/*`. See:

- [`AGENTS.md`](AGENTS.md)
- [`.ai-agents/README.md`](.ai-agents/README.md)
