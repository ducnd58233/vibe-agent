# Commands router

Slash-style prompts live in this folder as `*.md`. **Claude Code** resolves them via `.claude/commands` → junction/symlink to here; **Cursor** resolves **`.cursor/commands`** → same target (after [`scripts/link-ai-agents`](../../scripts/link-ai-agents.ps1)).

**After you add, rename, or remove a command file, update this table in the same change.**

| User goal / use case | Command file | Preconditions / notes |
|----------------------|--------------|----------------------|
| Five-axis review | [`review.md`](review.md) | Diff or paths scoped |
| Ship decision, parallel personas | [`ship.md`](ship.md) | Non-trivial blast radius |
| Write spec first | [`spec.md`](spec.md) | Agree spec path |
| Plan tasks from spec | [`plan.md`](plan.md) | Spec exists |
| Implement next task (TDD) | [`build.md`](build.md) | Tasks/plan exist |
| TDD / Prove-It | [`test.md`](test.md) | — |
| Simplify safely | [`code-simplify.md`](code-simplify.md) | Tests protect behavior |

**Authoring:** [`TEMPLATE.md`](TEMPLATE.md).
