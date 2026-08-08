# Hooks router

Lookup table for hook scripts in this folder. **After you add, rename, or remove a hook script, update this table in the same change.**

| Event / concern / use case | Script | Permission notes |
|----------------------------|--------|------------------|
| Session bootstrap message with meta-skill | [`session-start.py`](session-start.py) | Python 3 runtime + file read access |
| Source-driven `WebFetch` cache revalidation (pre) | [`sdd-cache-pre.py`](sdd-cache-pre.py) | Python 3 stdlib (`urllib`, `hashlib`); reads/writes `.claude/sdd-cache/` |
| Source-driven `WebFetch` cache write (post) | [`sdd-cache-post.py`](sdd-cache-post.py) | Python 3 stdlib (`urllib`, `hashlib`); reads/writes `.claude/sdd-cache/` |
| Protect simplify-ignore blocks during read/edit | [`simplify-ignore.py`](simplify-ignore.py) | Python 3 stdlib; edits files + `.claude/.simplify-ignore-cache/` |
| Hook behavior smoke test for simplify-ignore | [`simplify-ignore-test.py`](simplify-ignore-test.py) | Python 3 local test helper script |
| Warn on raw colors in UI files | [`design-token-guard.py`](design-token-guard.py) | Python 3 hook wired as `PostToolUse` on `Edit\|Write` in [`.claude/settings.json`](../../.claude/settings.json); warns when tokenizable raw colors appear; reports as JSON on stdout with both `systemMessage` and `additionalContext`, since stderr on exit 0 reaches neither the person nor the model |
| Inject precedence/routing reminders on asset-authoring prompts | [`precedence-reminder.py`](precedence-reminder.py) | Python 3 stdlib; `UserPromptSubmit`; returns `additionalContext`; fires only on asset-authoring or routing intent (English and Vietnamese verbs); may also fire on unrelated uses of the word "hook"; always exits 0 |
| Flag paths a subagent cited but never opened | [`subagent-grounding-guard.py`](subagent-grounding-guard.py) | Python 3 stdlib; `SubagentStop`; reads `transcript_path` and reports via `systemMessage`; **warn-only by design** — `SubagentStop` supports exit 2 to block, but a heuristic path check would strand real work; honors `stop_hook_active` |
| Warn on AI-aesthetic signatures in UI files | [`ui-slop-guard.py`](ui-slop-guard.py) | Python 3 stdlib; `PostToolUse` on `Edit\|Write`; binary pattern checks for slop gradients, arbitrary scale-escaping values, default font stacks, radius/shadow monotony; per-line opt-out via `ui-slop-guard: allow`; reports both `systemMessage` and `additionalContext`; always exits 0 |
| Warn on tests that cannot fail when behavior changes | [`core-logic-test-guard.py`](core-logic-test-guard.py) | Python 3 stdlib; `PostToolUse` on `Edit\|Write\|NotebookEdit` and Cursor `afterFileEdit`; Go, Python, and JS/TS test files only. Flags four things the text alone settles: no assertion at all, a lone environment-variable assertion, a lone "the mock was called", and a discovery or health-check name with a matching lone assertion. Deliberately does **not** flag filesystem reads on their own, because the same call is behavior in "saving writes a manifest" and discovery in "the config file exists". Opt out via `core-logic-test-guard: allow`; always exits 0 |
| Contract test for the core-logic test guard | [`core-logic-test-guard-test.py`](core-logic-test-guard-test.py) | Python 3 local test script; 15 cases, 8 of them false-positive guards drawn from this repository's own suite |
| Strip AI/agent co-author attribution from commit messages (POSIX) | [`strip-ai-attribution.sh`](strip-ai-attribution.sh) | `sh` + `awk`; git `prepare-commit-msg` hook installed by `scripts/link-ai-agents.*` calls this; edits the commit-message file in place |
| Strip AI/agent co-author attribution from commit messages (PowerShell) | [`strip-ai-attribution.ps1`](strip-ai-attribution.ps1) | PowerShell 5.1 and 7+ equivalent of the `.sh`; for PowerShell-driven environments and manual runs |

**Authoring:** [`TEMPLATE.md`](TEMPLATE.md) - document stdin/stdout JSON and wiring in [`.cursor/hooks.json`](../../.cursor/hooks.json) / [`.claude/settings.json`](../../.claude/settings.json).

**Not in this folder:** the optional runtime binary supplies six more hooks as `vibe-agent hook <event>`, wired in the same two config files. Two of them **refuse**, and no script in this folder does:

- `pre-tool-use` exits 2 on a push to `main`/`master`, on a `gh pr merge` while no active run has recorded `merge_approved`, and on any write to a run's own `manifest.json` or `events.ndjson`.
- `stop` returns `decision: "block"` while a run sits mid-graph with nothing recorded, at most once per turn, and never for a run awaiting a human.

Look there, not here, when a shell command is refused or a turn will not end with no script to blame. See [`runtime/README.md`](../../runtime/README.md) under "What is deterministic, and what is not".
