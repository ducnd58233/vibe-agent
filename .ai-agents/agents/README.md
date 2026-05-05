# Agent personas (Claude subagents)

Markdown files in this folder define **single-role** specialists consumed as Claude Code subagents (via junction from `.claude/agents` after [`scripts/link-ai-agents`](../../scripts/link-ai-agents.ps1)). Cursor does not load these automatically; use the same instructions by `@`-referencing a file in chat.

## Three-layer model

| Layer | Role | Location |
|-------|------|----------|
| **Skill** | Workflow (how) | [`skills/`](../skills/) |
| **Persona** | Perspective (who) | This folder |
| **Command** | Entry point (when) | [`commands/`](../commands/) |

Personas **do not** call other personas. Composition is via user or slash commands — see [`references/orchestration-patterns.md`](../references/orchestration-patterns.md).

## Personas

| Persona | File | Best for |
|---------|------|----------|
| Code review (five axes) | [`code-reviewer.md`](code-reviewer.md) | Pre-merge PR review |
| Security audit | [`security-auditor.md`](security-auditor.md) | Auth, data paths, dependencies |
| Tests / coverage | [`test-engineer.md`](test-engineer.md) | TDD, gaps, Prove-It bugs |

## Authoring

Use [`TEMPLATE.md`](TEMPLATE.md). After adding or renaming a file, update [`ROUTER.md`](ROUTER.md).
