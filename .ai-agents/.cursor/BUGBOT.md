# Bugbot rules for `.ai-agents/` changes

<context>

Apply these in addition to the root `.cursor/BUGBOT.md` when reviewing files under `.ai-agents/`.
</context>

## Authoring contract

<rules>

- New or updated assets under `skills/`, `agents/`, `commands/`, `hooks/`, `references/`, or `stack-profiles/` must follow that folder's `TEMPLATE.md`.
- Every create, rename, or delete under those trees must update the folder `ROUTER.md` in the **same** change.
- Read `.ai-agents/ROUTER.md` before adding overlapping assets; prefer extending existing skills over duplicating policy.
</rules>

## Routing and discovery

<routing>

- Skill `description` frontmatter must state when to use the skill and when not to.
- Commands must not silently replace implementation workflows they are not meant to own.
- Cross-links use repo-relative paths, not generated link paths (`.claude/`, `.cursor/commands` symlinks).
</routing>

## Permissions

<required>

- If an asset documents new tool, path, or shell needs, `.ai-agents/PERMISSIONS.md` and consumer-facing config notes must stay aligned.
- Subagent `tools:` maps must match documented authority; do not grant `Bash` or write tools without a stated reason in the asset.
</required>

## Plain writing

<rules>

- No AI-tell filler (ensure, leverage, robust, seamless, and similar).
- No emojis or em-dashes in authored content.
- Comments explain why, not what.
</rules>

## Verification

<verification>

For hook or router-check changes, expect evidence that `scripts/check-ai-agents-routers.sh` or `scripts/check-ai-agents-routers.ps1` was run, or file an **Important** finding.
</verification>
