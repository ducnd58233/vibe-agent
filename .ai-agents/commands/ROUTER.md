# Commands router

<routing>

Slash-style prompts live in this folder as `*.md`. **Claude Code** commonly resolves them via `.claude/commands` linked to this folder; **Cursor** can resolve **`.cursor/commands`** to the same target when link-based discovery is configured (for example via [`scripts/link-ai-agents`](../../scripts/link-ai-agents.ps1)).

**After you add, rename, or remove a command file, update this table in the same change.**

| User goal / use case | Command file | Preconditions / notes |
|----------------------|--------------|----------------------|
| End-to-end objective to shippable work | [`goal.md`](goal.md) | Clarify-first; composes spec/plan/build/test/review/ship; waits on PR CI + external reviews; E2E when in scope; records `tmp/<slug>/` |
| Same objective, unattended | [`auto.md`](auto.md) | `/goal` with the approval gates answered by evidence. Requires the runtime and a workspace opt-in; the danger list and an ambiguous spec still stop it |
| Five-axis review | [`review.md`](review.md) | Diff or paths scoped |
| Build or audit UI registry-first | [`design.md`](design.md) | Modes: build / audit / registry; audit is read-only; needs browser or MCP for render evidence |
| Ship decision, parallel personas | [`ship.md`](ship.md) | Non-trivial blast radius; only phase that may authorize merge to `main` after GO + human approval |
| Write spec first | [`spec.md`](spec.md) | Agree spec path |
| Citation-first research digest command | [`research.md`](research.md) | Topic + scope; MUST Applicability + Refine + Mermaid; reusable behavior is `research-with-citations`; multi-lane uses `investigate.md` |
| Host/CI experiment + STATUS.md updates | [`experiment.md`](experiment.md) | `researcher-delivery` `experiment_run`; no in-process GPU sandbox |
| Findings citing STATUS and run artifacts | [`findings.md`](findings.md) | After experiment terminal STATUS |
| Analyze evidence into recommendation | [`analyze.md`](analyze.md) | Digest/evidence available |
| Parallel evidence investigation with audit | [`investigate.md`](investigate.md) | Multi-faceted citation question; merge required; single-lane research uses `research.md` |
| Plan tasks from spec | [`plan.md`](plan.md) | Spec exists |
| Implement next task (TDD) | [`build.md`](build.md) | One branch/PR per planned task; same-task feedback on same branch; new task = new branch; never merge to `main` |
| TDD / Prove-It | [`test.md`](test.md) | - |
| Simplify safely | [`code-simplify.md`](code-simplify.md) | Tests protect behavior |
| Audit AI asset health, deterministically | [`doctor.md`](doctor.md) | Validate routers, hooks, links, permissions; a judgment-level harness review is `agent-systems-auditor` |
| Harden AI asset safety | [`harden.md`](harden.md) | Review permissions, hooks, tool boundaries |

**Authoring:** [`TEMPLATE.md`](TEMPLATE.md).
</routing>
