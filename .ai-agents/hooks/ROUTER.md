# Hooks router

<routing>

Lookup table for hook scripts in this folder. **After you add, rename, or remove a hook script, update this table in the same change.** Runtime-owned hooks are not listed in the table because they are commands inside `vibe-agent`, not scripts on disk.

| Event / concern / use case | Script | Permission notes |
|----------------------------|--------|------------------|
| Source-driven `WebFetch` cache revalidation (pre) | [`sdd-cache-pre.py`](sdd-cache-pre.py) | Python 3 stdlib (`urllib`, `hashlib`); reads/writes the host SDD cache directory |
| Source-driven `WebFetch` cache write (post) | [`sdd-cache-post.py`](sdd-cache-post.py) | Python 3 stdlib (`urllib`, `hashlib`); reads/writes the host SDD cache directory |
| Strip AI/agent co-author attribution from commit messages (POSIX) | [`strip-ai-attribution.sh`](strip-ai-attribution.sh) | `sh` + `awk`; git `prepare-commit-msg` hook installed by `scripts/link-ai-agents.*` calls this; edits the commit-message file in place |
| Strip AI/agent co-author attribution from commit messages (PowerShell) | [`strip-ai-attribution.ps1`](strip-ai-attribution.ps1) | PowerShell 5.1 and 7+ equivalent of the `.sh`; for PowerShell-driven environments and manual runs |

**Authoring:** [`TEMPLATE.md`](TEMPLATE.md) - document stdin/stdout JSON and wiring in [`.cursor/hooks.json`](../../.cursor/hooks.json), [`.codex/hooks.json`](../../.codex/hooks.json), or [`.claude/settings.json`](../../.claude/settings.json).

**Runtime hooks:** the control plane supplies the shared lifecycle hooks as `vibe-agent hook <event>`, wired by the host config files. Keep their behavior in `runtime/internal/harness`, not in this folder.

- `session-start` reports active runs, loaded rules, and confirmed memories.
- `user-prompt-submit` injects run and routing context where the host allows it.
- `post-tool-use` journals successful tool calls and runs the advisory file guards: `sensitive-data-guard`, `design-token-guard`, `ui-slop-guard`, and `core-logic-test-guard`.
- `post-tool-use-failure` journals failed tool calls for hosts that split success and failure events.
- `subagent-stop` checks whether a subagent cited paths that do not appear in its transcript.
- `pre-tool-use` exits 2 on protected-branch pushes, unapproved PR merges, writes to run state files, and live credential literals.
- `stop` blocks ending a turn while a run sits mid-graph with no evidence recorded, at most once per turn, and never for a run awaiting a human.

Look in [`runtime/README.md`](../../runtime/README.md) when a shell command is refused or a turn will not end with no script to blame.
</routing>