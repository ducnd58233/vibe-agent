# Plan: harness plugin packaging (v1)

## Goal

Emit host plugin manifests from the link script so `.claude-plugin/plugin.json`, `.codex-plugin/plugin.json`, `.cursor-plugin/plugin.json`, and root `plugin.json` (Agent Plugins 1.0.0) are generated views of `.ai-agents/`, not hand-written copies that drift.

## Tasks

See [TASKS.md](TASKS.md).

## Order

1. Task 1: manifest emission in link scripts
2. Task 2: drift check in `check-generated-views.sh`
3. Task 3: verify all checks pass

## Branch

`harness-plugin/manifest-emission` (single task, single PR).

## Dependencies

None beyond existing link scripts and `.ai-agents/` tree.
