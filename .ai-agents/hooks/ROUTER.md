# Hooks router

Lookup table for hook scripts in this folder. **After you add, rename, or remove a hook script, update this table in the same change.**

| Event / concern / use case | Script | Permission notes |
|----------------------------|--------|------------------|
| Session bootstrap message with meta-skill | [`session-start.py`](session-start.py) | Python 3 runtime + file read access |
| Source-driven `WebFetch` cache revalidation (pre) | [`sdd-cache-pre.py`](sdd-cache-pre.py) | Python 3 stdlib (`urllib`, `hashlib`); reads/writes `.claude/sdd-cache/` |
| Source-driven `WebFetch` cache write (post) | [`sdd-cache-post.py`](sdd-cache-post.py) | Python 3 stdlib (`urllib`, `hashlib`); reads/writes `.claude/sdd-cache/` |
| Protect simplify-ignore blocks during read/edit | [`simplify-ignore.py`](simplify-ignore.py) | Python 3 stdlib; edits files + `.claude/.simplify-ignore-cache/` |
| Hook behavior smoke test for simplify-ignore | [`simplify-ignore-test.py`](simplify-ignore-test.py) | Python 3 local test helper script |
| Warn on raw colors in UI files | [`design-token-guard.py`](design-token-guard.py) | Python 3 hook wired as `PostToolUse` on `Edit\|Write` in [`.claude/settings.json`](../../.claude/settings.json); warns when tokenizable raw colors appear |
| Warn on AI-aesthetic signatures in UI files | [`ui-slop-guard.py`](ui-slop-guard.py) | Python 3 stdlib; `PostToolUse` on `Edit\|Write`; binary pattern checks for slop gradients, arbitrary scale-escaping values, default font stacks, radius/shadow monotony; per-line opt-out via `ui-slop-guard: allow`; always exits 0 |
| Strip AI/agent co-author attribution from commit messages (POSIX) | [`strip-ai-attribution.sh`](strip-ai-attribution.sh) | `sh` + `awk`; git `prepare-commit-msg` hook installed by `scripts/link-ai-agents.*` calls this; edits the commit-message file in place |
| Strip AI/agent co-author attribution from commit messages (PowerShell) | [`strip-ai-attribution.ps1`](strip-ai-attribution.ps1) | PowerShell 5.1 and 7+ equivalent of the `.sh`; for PowerShell-driven environments and manual runs |

**Authoring:** [`TEMPLATE.md`](TEMPLATE.md) - document stdin/stdout JSON and wiring in [`.cursor/hooks.json`](../../.cursor/hooks.json) / [`.claude/settings.json`](../../.claude/settings.json).
