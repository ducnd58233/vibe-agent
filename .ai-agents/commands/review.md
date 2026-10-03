---
description: Five-axis code review - correctness, readability, architecture, security, performance
---

Review the current change across five axes: correctness, readability, architecture, security, and performance.

<prerequisites>

**Runtime required (MUST).** Run `vibe-agent doctor` first. Rules: [`goal.md`](goal.md) section "Runtime is required".
</prerequisites>

<procedure>

Follow the [`code-review-and-quality`](../skills/code-review-and-quality/SKILL.md) skill.

**Run the deterministic pass first (MUST).** These commands are the mechanical
half of this review. Do not redo their work by eye, and do not choose for
yourself which files or functions to read:

```sh
vibe-agent review scan --changed        # a branch or PR: the change's findings, changed blocks marked *
vibe-agent review scan <path>...        # paths the user named; no path scans the whole workspace
vibe-agent slop audit <path>            # AI-slop signals: unfinished markers, ignored results, filler
```

Add `--json` to either for a record under `tmp/<slug>/`. `review scan` reads
every source file (installed packages, build output, virtual environments,
vendored and generated files are excluded), cuts each file into blocks that
together cover every line, and reports:

- **References:** unused imports; definitions nothing references; definitions
  this change left without a caller (`orphaned-by-change`); helpers only tests call.
- **Bug shapes:** self-comparison, NaN comparison, identical branches, repeated
  conditions, unreachable statements, duplicate keys, assignment in a
  condition, swallowed errors, return in finally, defer in a loop, mutable
  defaults, `is` with a literal, leftover debugger calls, redefinitions.

**Then walk its BLOCKS list in order, one block at a time (MUST):**

1. `vibe-agent review block <path>:<line>` prints the block with line numbers,
   its findings, and the files that reference it. Nested blocks are folded to
   one line; print each one separately.
2. Settle every finding in the block: fix it, or state in the review why it is
   wrong (a framework or reflection call, a deliberate idiom). Never drop one
   silently. Before deleting an unreferenced definition, check the listed
   callers and search for its name; ask before deleting public API.
3. Review the block on the five axes below. Review a changed block (`*`) in
   full; read an unchanged block only for context a changed one needs.

The scan resolves no types, so it cannot see a call through reflection, a
framework, or another repository. Treat its findings as leads you verify, not
verdicts. A deliberate exception is recorded in the code: a `review:ignore
<reason>` comment, or the linter's own directive (`noqa`, `nolint`,
`eslint-disable`), on or above the line silences it. A machine can say a
condition repeats; it cannot say a boundary is wrong. Spend your attention on
the half only a reviewer can do.

Review current changes (staged diff, branch, or paths the user specifies) across:

1. **Correctness** - Spec alignment, edge cases, adequate tests.
2. **Readability** - Names, structure, clarity.
3. **Architecture** - Patterns, boundaries, coupling.
4. **Security** - Deep pass references [`security-and-hardening`](../skills/security-and-hardening/SKILL.md); query/injection safety for your persistence layer. **Also review the sinks, not only the logic:** every log call, response body, client-storage write, error path, and build-time env var the diff touched, against [`sensitive-data-exposure.md`](../references/sensitive-data-exposure.md). Disclosure is the defect class review misses most often, because the code works.
5. **Performance** - N+1, bundle, caching; see [`performance-optimization`](../skills/performance-optimization/SKILL.md).

Categorize findings as **Critical**, **Important**, or **Suggestion**. Include `file:line` and concrete fixes.

**Grounding, when the diff touches a `docs/**/*.md` deliverable (MUST):** run `vibe-agent docs check-claims <path>` before writing findings for that file. It flags a backtick-quoted file path that does not resolve in the repo tree - the same repo-fact discipline `docs/ROUTER.md` generation and the slug-continuation lookup already require. A finding is a warning, not a `vibe-checks.yaml` gate: decide whether it is a genuinely fabricated path or a reasonable shorthand worth tightening, the same way you already judge a `slop audit` finding. For a claim the checker cannot mechanically verify (a factual assertion about the codebase, not just a path), use the [`source-auditor`](../agents/source-auditor.md) agent - already built for RESEARCH's external citations - on SPEC/PLAN/RESEARCH claims about the codebase too, not RESEARCH alone.

Optional: spawn the **`code-reviewer`** subagent ([`agents/code-reviewer.md`](../agents/code-reviewer.md)) for a dedicated review session.
</procedure>

## REVIEW.md contract (MUST on the auto path)

<required>

On the auto path, `review_ok` in `goal-delivery.yaml` `file_assert`s this file before the run
continues to `experiment_run`: a self-report accepted with no check at all is exactly the gap
`docs/2026-09-24/agent-code-quality-hardening` closed (self-report is weak evidence precisely
because generator and evaluator share the same failure modes - `arXiv:2606.05976`). The check is
mechanical presence of the five axis names, not a quality judgment: it cannot tell a real review from
a perfunctory one, only that one was written down. Write:

`.agent-state/runs/<date>/<slug>/<version>/review/REVIEW.md`

```markdown
# Review

## Correctness
<findings, or "No issues found.">

## Readability
...

## Architecture
...

## Security
...

## Performance
...
```

Every axis heading must appear even when a section has no findings - the check reads for the axis
names, not for content under them. On `/goal` (no auto flag), write the same file as a matter of
discipline; nothing in the graph verifies it there today.
</required>

## Routing & discovery

<routing>

- Use when review intent is explicit.
- Do not use as a replacement for implementation commands.

Invoke before merge, ship, or when quality concerns are raised.
</routing>
