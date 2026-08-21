# Permissions and authority (project reference)

<context>

This file explains how **permission documentation** in `.ai-agents` assets connects to each host's
configuration: [`.claude/settings.json`](../.claude/settings.json) for Claude Code,
[`.cursor/cli.json`](../.cursor/cli.json) for `cursor-agent`, [`.codex/config.toml`](../.codex/config.toml)
for Codex, [`opencode.json`](../opencode.json) for OpenCode. All four share one posture: a broad
allow, with the `pre-tool-use` danger-list hook — command-precise, unbypassable, fires for every
host — as the real safety backstop, not an interactive `ask` tier. `ask` (or its host-specific
equivalent) is kept minimal on purpose: nobody answers a prompt in `/auto`, so an `ask` entry there
stalls an unattended run while doing nothing a command-precise refusal wouldn't already catch.
Permissions described here are for reusable agent assets, not for domain-specific product workflows.
</context>

## Claude Code

<procedure>

| Mechanism | Location | Notes |
|-----------|----------|--------|
| Project permissions | [`.claude/settings.json`](../.claude/settings.json) | `permissions.allow`, `permissions.ask`, `permissions.deny` for tools (`Bash(...)`, `Read(...)`, `Edit(...)`, `WebFetch(...)`, MCP tools). Use `Edit(path)` for file-write path scopes; `Write(path)` is not matched by Claude's file permission checks. **Deny overrides allow.** See [official permissions](https://code.claude.com/docs/en/permissions). |
| Subagent authority | `agents/*.md` YAML **`tools:`** | Map of tool name ? `true` (OpenCode-valid). Restricts which tools a subagent may use. |
| Hooks | `hooks` in settings + scripts under `.ai-agents/hooks/` | Hook commands may need shell access; align `Bash` rules with what scripts run. |

When you add or change a skill, agent, command, or hook:

1. Fill the **Permissions & authority** section in that folder's [`TEMPLATE.md`](./skills/TEMPLATE.md) (or equivalent).
2. If new tool patterns are required project-wide, extend `.claude/settings.json` in a dedicated PR and mention it here or in the PR description.
3. Prefer a **broad allow, minimal ask, danger-list backstop** posture (below), not a narrow allow
   with broad `ask` — an `ask` entry stalls any unattended `/auto` run, so the command-precise
   `pre-tool-use` danger list is the actual safety instrument, not the permission tiers.

Current project posture:

- Allows nearly everything routine: reads, edits under `Edit(**)`, git (including `push`/`merge`/
  `rebase`/`worktree`), `gh`, `go`/`gofmt`/`golangci-lint`, `make`, script runners, common CLI
  utilities, `WebFetch`.
- Asks for almost nothing: `NotebookEdit`, `chmod`, `chown`, `kill`, `killall`, `pkill` only. Nothing
  else is in `ask`, deliberately — see the note above.
- Denies credential-shaped file reads, `git push --force`, `git clean`, `git push --mirror`,
  `git filter-branch`, and hand-writes to a run's own `manifest.json`/`events.ndjson`.
- **Deny patterns must be specific enough to miss ordinary source files.** A broad `Read(**/*token*)` also blocked runtime guard code and design-token files such as `tokens.json`, and because deny overrides allow there is no way to grant an exception. It is now a set of credential-shaped patterns (`*access_token*`, `*refresh_token*`, `*auth_token*`, `*api_token*`, `*id_token*`, `.token*`, `token.json`). When adding a deny rule, check it against real repository paths before committing it.
- See [`references/tool-safety-and-permissions.md`](references/tool-safety-and-permissions.md) for the hardening checklist.
</procedure>

## Subagents (`agents/*.md`)

<context>

Specialist personas that inspect code/config usually declare **`tools`** with `Read`, `Grep`, `Glob`, and `Bash` set to `true`; research personas add `WebSearch` and `WebFetch`. Ensure [`.claude/settings.json`](../.claude/settings.json) allows those tools for sessions that spawn subagents, and scope `Bash` to repo-documented test/lint/check commands where possible.

After changing subagent `tools:` frontmatter, smoke-test delegation in Claude Code so allowlists still behave as expected. For OpenCode-only validation, run `opencode agent list` from the repo root (expects exit code 0).
</context>

## Cursor

<procedure>

`cursor-agent` (the Cursor CLI) reads [`.cursor/cli.json`](../.cursor/cli.json): `permissions.allow`/
`permissions.deny` only — **no `ask` tier exists in Cursor's schema**, so there is no equivalent of
Claude's minimal-`ask` pattern to opt into; an unlisted tool is simply not allowed. Real tool names,
confirmed by provoking actual permission messages rather than assumed: `Read`, `Write` (not `Edit`
— Cursor's own write tool is named `Write`), `Shell`, `WebFetch`. The file has **no comment field in
its schema** — a `$comment` key at the top level fails validation with `unrecognized_keys` and
`cursor-agent` refuses to start at all; put any explanation in this doc instead, not in the JSON.

Current project posture: broad `Write`/`Shell`/`Read`/`WebFetch` allow matching Claude's breadth;
deny mirrors Claude's (credential-shaped reads, `rm`/`shred`, force-push and history-rewrite git
forms, hand-writes to a run's own `manifest.json`/`events.ndjson`).

Guardrails also live in [`.cursor/rules/*.mdc`](../.cursor/rules) (prose, not permission enforcement)
and [`.cursor/hooks.json`](../.cursor/hooks.json) (wires the vibe-agent control plane — the
`pre-tool-use` danger list, separate from and in addition to `cli.json`'s allow/deny). See
[`CURSOR.md`](../CURSOR.md) for onboarding.

**Known defect, not fixed by this posture:** `cursor-agent` (measured 2026.08.11 and again
2026-08-21) wraps a hook command in PowerShell and executes it with a POSIX shell on Windows, so the
hook process never starts — `runtime/internal/harness/contracts.go` documents this as a host-side
bug nothing in this repository can wire around. On Windows, every Cursor tool call is blocked by that
broken hook regardless of what `cli.json` allows, until Cursor fixes it upstream.
</procedure>

## Codex

<procedure>

Codex has **no command-level allow/deny list at all** — verified against the live config reference,
not assumed. `approval_policy` (`untrusted` | `on-request` | `never` | a granular object) and
`sandbox_mode` (`read-only` | `workspace-write` | `danger-full-access`) are the whole permission
surface in [`.codex/config.toml`](../.codex/config.toml). Parity with the other three hosts here is
a **policy** match, not a config-shape match: `approval_policy = "never"` (no prompt ever blocks an
unattended run — Claude's near-empty `ask` reasoning, applied to Codex's own mechanism) and
`sandbox_mode = "workspace-write"` (broad within-workspace file access, matching `Edit(**)`'s
breadth — not `danger-full-access`, not `read-only`). The `pre-tool-use` danger-list hook still
fires for Codex via its MCP registration in the same file and is still the real backstop.
</procedure>

## OpenCode

<procedure>

[`opencode.json`](../opencode.json)'s `permission.edit`/`permission.bash` matches Claude's shape most
closely: per-path/per-command `allow`/`ask`/`deny`, most-specific-match-wins (a `deny` entry always
overrides the `"*"` default, confirmed empirically — a broader default does not need clearing every
specific rule first). Current posture: `"*": "allow"` for both `edit` and `bash` (an `ask` default
stalls an unattended run, same reasoning as Claude), with the same specific denies Claude has
(`rm *`, `shred *`, `git push --force*`, `git clean *`, `git push --mirror*`, `git filter-branch*`).
`opencode.json`'s schema, like Cursor's, has **no comment field** — an unrecognized key anywhere
under `permission` fails validation (`Expected PermissionActionConfig, got "..."`) and `opencode`
refuses to load the config at all.
</procedure>

## Runtime control plane

<rules>

The optional [`runtime/`](../runtime) binary is an actuator with its own boundaries. It is not a sandbox: a verifier node runs a real subprocess with the session's own privileges.

| Concern | Boundary |
|---------|----------|
| Binary invocation | `Bash(vibe-agent *)` in [`.claude/settings.json`](../.claude/settings.json). Hooks call it by name, so it must be on `PATH`. |
| Verifier subprocesses | A `verifier` node runs the command [`vibe-checks.yaml`](../vibe-checks.yaml) declares for its check, in the workspace root, with a timeout. Review that file the way you review a CI file: it is what decides what a gate actually proves, and a caller cannot substitute a weaker command at run time. |
| Run state | Writes `tmp/<slug>/manifest.json` and `tmp/<slug>/events.ndjson`. Both gitignored; never commit them. |
| Memory database | Writes `.agent-state/memory.db` under the **workspace** root, not the toolkit. Gitignored. Never contains secrets: the policy filter rejects credential-shaped candidates before they reach disk. |
| MCP server | `vibe-agent mcp serve` exposes seven tools over stdio. Model-decided, so it is best effort; Claude and Cursor get the same capabilities through hooks, which always fire. `vibe_verify` takes no verdict parameter, so the surface offers no way to assert a result. |
| Evidence | A check is `passed` only from `exit_code`, `file_assert`, `ci_api`, or `human_event`. There is no source for model assertion, in the schema or in the code. |
| Irreversible actions | Merge, deploy, and publish sit behind a `human_gate` node. The runtime never merges, pushes, or checks out; the `git` verifier only observes. |
| Degradation | A missing binary makes every hook a quiet no-op. The control plane must never wedge a coding session. |
| Staleness | A **stale** binary is not quiet: it answers the hooks it knows and refuses the rest, which reads as a broken hook rather than an out-of-date install. The refusal names the events the build handles and points at `make install`. `vibe-agent doctor` compares the events the host configs register against what the binary on `PATH` actually handles, so the mismatch is reported before a session runs into it. |
| Binary identity | Hooks resolve `vibe-agent` by name, so the binary answering them is whatever `PATH` finds - not necessarily the one just built. On Windows an extensionless file shadows the `.exe` for any POSIX shell, meaning two builds can answer depending on which shell asks. `doctor` reports that too. |
</rules>

## Device and browser automation: exploration, not evidence

<context>

An MCP server that drives a browser or a phone is genuinely useful for **investigating**: reproducing a bug, walking a flow, reading a crash. It is not a source of evidence for a gate.

| Server key | Allowlist entry | What it is for |
|-----------|-----------------|----------------|
| `chrome-devtools` | `mcp__chrome-devtools__*` | Inspecting a live page while diagnosing |
| `playwright` | `mcp__playwright__*` | Driving a browser during investigation |
| `mobile` | `mcp__mobile__*` | Driving an emulator, simulator, or handset through its accessibility tree |

The prefix comes from the key the server is registered under in `.mcp.json`, and that key is the consumer's choice, so **the allowlist entry is what pins it**. A mobile MCP server registered under a different key is not covered by the entry above; either register it as `mobile` or extend the allowlist deliberately.

The boundary that matters: an agent that both drives the device and decides what to record is the thing being measured reporting on itself. Evidence for a gate comes from the runtime's own collection path - the `screen` verifier, configured in `vibe-checks.yaml` - never from an agent's account of what it saw. See [`references/mobile-ui-verification.md`](references/mobile-ui-verification.md).

These servers also reach outside the repository: a device, a network, a real browser session. Treat what they return as untrusted input, the same as a fetched page or a review comment.
</context>

## Review checklist

<verification>

- [ ] Asset template **Permissions & authority** completed.
- [ ] Folder **`ROUTER.md`** updated if you added, renamed, or removed an asset (same change).
- [ ] Sensitive paths called out (`Read(./.env)`, etc.).
- [ ] Hook scripts: Bash / side effects documented.
- [ ] No missing hook script paths in `.claude/settings.json` or `.cursor/hooks.json`.
- [ ] Broad `allow` entries (`Bash(*)`, `Edit(*)`, `Write(*)`, `WebFetch(domain:*)`, `mcp__*`) are avoided or justified.
- [ ] A browser or device MCP server is used for investigation only; nothing it returns is recorded as a check.
- [ ] New checks in [`vibe-checks.yaml`](../vibe-checks.yaml) name a command that would actually fail on a broken build, and any `verifier: human` entry says in its `description` why no runtime verifier can produce it.
</verification>

## Default authority for skills, agents, and commands

<required>

This is the **single home** for the defaults that used to be repeated as a `## Permissions & authority`
section in every asset file. Thirty-four assets carried the same three sentences; changing the policy
meant editing thirty-four files, and any one of them could be missed.

Unless an asset says otherwise in its own body or its YAML `tools:` map:

- **Tools:** `Read`, `Grep`, `Glob`, `Edit`. Shell only for running repo-documented lint, test, and
  build commands when validating a recommendation.
- **Paths:** follow the allow/ask/deny rules in this file. Never read credential or secret material;
  the deny list is the boundary, not a suggestion.
- **Browser and DevTools:** for browser-centric work prefer a browser MCP or human-driven DevTools.
  Do not assume unattended cloud browser access.
- **Grounding:** never describe a file, path, or result not actually opened or run. Report
  `ACCESS-FAILED: <path>` instead of inferring.
- **Subagents:** a persona's authority is its YAML `tools:` map. A persona never invokes another
  persona; see [`references/orchestration-patterns.md`](references/orchestration-patterns.md).

An asset states permissions in its own file **only when it diverges** from the above, and says what
diverges and why. Restating the default is what created the drift this section removes.
</required>
