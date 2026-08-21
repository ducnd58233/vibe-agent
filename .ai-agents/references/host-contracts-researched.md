# Researched host contracts, not yet wired

<context>

Three coding agents this toolkit does not support, with what their vendors publish about hooks,
skills, and configuration.

**Separate from [`host-hook-contracts.md`](host-hook-contracts.md) on purpose.** That file is
generated from `runtime/internal/harness/contracts.go` and describes hosts the runtime actually
speaks to; a host only belongs there once it has a `Client` value and a branch in the refusal path.
Putting research there would mean extending the type the hook dispatch switches on, which is the
wiring this file exists to avoid claiming.

**Nothing here was run.** None of the three is installed on the machine this was written on:

```
antigravity    not installed
kimi           not installed
muse           not installed
```

Reading a vendor schema is not watching it fire. `host-hook-contracts.md` carries a Verified column
because host wiring written from an unverified reading is the defect class this repository has hit
most; task T1 of `harness-autonomy` was blocked rather than guessed when `cursor-agent` was missing,
and finding F7 stayed open for the same reason. This file is the reading. Someone with the binary
turns it into wiring.
</context>

## antigravity

<context>

- **Hooks:** `.agents/hooks.json` in a workspace, `~/.gemini/config/hooks.json` globally. The
  workspace file wins where both exist.
- **Source:** <https://antigravity.google/docs/hooks>
- **Assets:** `.agents/` is already the workspace directory this toolkit writes skills and commands
  into, so assets reach it today with no change.

| Host event key | Nearest vibe-agent event | Output keys | Inject | Refuse |
|---|---|---|---|---|
| `PreInvocation` | `user-prompt-submit` | `injectSteps[].ephemeralMessage`, `.userMessage`, `.toolCall` | yes | no |
| `PreToolUse` | `pre-tool-use` | `decision`, `reason`, `permissionOverrides` | no | yes |
| `PostToolUse` | `post-tool-use` | none documented | no | no |
| `PostInvocation` | none | none documented | no | no |
| `Stop` | `stop` | none documented | no | yes |

**Notes**

- `decision` takes `allow`, `deny`, `ask`, `force_ask`, or `deny_unless_prior_grant`. The first
  three are what Cursor accepts, so the branch that already answers Cursor is the shape to reuse.
- `PreToolUse` stdin carries `toolCall.name`, `toolCall.args`, `stepIdx`, `conversationId`,
  `workspacePaths`, `transcriptPath`, `artifactDirectoryPath`, `modelName`. A transcript path is
  present, which is what the grounding check needs.
- `workspacePaths` is an array. Multi-repo workspaces are a stated feature, so a wiring that assumes
  one path would be wrong on the case the vendor advertises.
- This is the most complete of the three and it is still unwired, deliberately.

</context>

## kimi

<context>

- **Config:** `~/.kimi/config.toml`; `~/.kimi/config.json` is accepted and migrated.
- **Skills:** `~/.config/agents/skills/<name>/SKILL.md`, with optional `scripts/`, `references/`,
  `assets/` beside it. **The global install writes this path.**
- **Source:** <https://moonshotai.github.io/kimi-cli/en/configuration/config-files.html>

**Hooks** are a `[[hooks]]` array with `event`, `command`, an optional `matcher` regex, and an
optional `timeout` in seconds defaulting to 30. Events named: `PreToolUse`, `PostToolUse`, `Stop`.

**What a hook receives on stdin, and what it prints to refuse, is not published.** Recorded as
undocumented rather than borrowed from a host that looks similar: a refusal shape guessed from a
neighbour is how a gate comes to fail open while looking wired.

</context>

## muse

<context>

- **Config:** `~/.config/muse/settings.json`, which must carry `"schema_version": 1` and holds a
  `hooks` block.
- **Skills:** read repo-locally from `.codex/skills` and `.claude/skills`. **The link script writes
  both**, so Muse can load this toolkit's skills today.
- **Source:** <https://dev.meta.ai/docs/muse-code>

`muse skills import --from claude` exists, which is the same conclusion reached from the vendor's
side. `muse exec` runs headlessly and streams JSONL, so an eval runner has something to talk to.

The hooks block binds shell commands to lifecycle points described in prose: session start, prompt
submission, tool use, permission requests, model calls, context compaction, subagent start and stop,
session stop. **The exact keys are not published**, which is why none is written here as a literal.

</context>

## Routing & discovery

<routing>

- Read this before adding a host. Read [`host-hook-contracts.md`](host-hook-contracts.md) for the
  hosts that are wired.
- A host moves from this file to that one by gaining a `Client` value, a contract in
  `runtime/internal/harness/contracts.go`, and someone who watched it refuse something.
</routing>
