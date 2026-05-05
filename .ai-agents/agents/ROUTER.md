# Subagents router

Lookup table for Claude subagent files in this folder. **After you add, rename, or remove a `*.md` subagent, update this table in the same change.**

Subagents are primarily for **Claude Code** (junction `.claude/agents` → here after `scripts/link-ai-agents`). Cursor users may `@`-reference the same files as prompts.

| Task type / use case | Subagent file | Tool scope (YAML `tools`) |
|----------------------|---------------|---------------------------|
| Five-axis code review | [`code-reviewer.md`](code-reviewer.md) | Read, Grep, Glob, Bash |
| Security audit, OWASP + document DB + LLM surfaces | [`security-auditor.md`](security-auditor.md) | Read, Grep, Glob, Bash |
| Tests, coverage, Prove-It | [`test-engineer.md`](test-engineer.md) | Read, Grep, Glob, Bash |

**Authoring:** [`TEMPLATE.md`](TEMPLATE.md). Overview: [`README.md`](README.md).
