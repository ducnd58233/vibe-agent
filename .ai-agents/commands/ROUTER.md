# Commands router

Slash-style prompts live in this folder as `*.md`. **Claude Code** commonly resolves them via `.claude/commands` linked to this folder; **Cursor** can resolve **`.cursor/commands`** to the same target when link-based discovery is configured (for example via [`scripts/link-ai-agents`](../../scripts/link-ai-agents.ps1)).

**After you add, rename, or remove a command file, update this table in the same change.**

| User goal / use case | Command file | Preconditions / notes |
|----------------------|--------------|----------------------|
| Five-axis review | [`review.md`](review.md) | Diff or paths scoped |
| Ship decision, parallel personas | [`ship.md`](ship.md) | Non-trivial blast radius |
| Write spec first | [`spec.md`](spec.md) | Agree spec path |
| Research with citations | [`research.md`](research.md) | Topic + scope provided |
| Analyze evidence into recommendation | [`analyze.md`](analyze.md) | Digest/evidence available |
| Parallel investigation with audit | [`investigate.md`](investigate.md) | Multi-faceted question; merge required |
| Plan tasks from spec | [`plan.md`](plan.md) | Spec exists |
| Implement next task (TDD) | [`build.md`](build.md) | Tasks/plan exist |
| TDD / Prove-It | [`test.md`](test.md) | — |
| Simplify safely | [`code-simplify.md`](code-simplify.md) | Tests protect behavior |
| Audit AI asset health | [`doctor.md`](doctor.md) | Validate routers, hooks, links, permissions |
| Harden AI asset safety | [`harden.md`](harden.md) | Review permissions, hooks, tool boundaries |

**Authoring:** [`TEMPLATE.md`](TEMPLATE.md).
