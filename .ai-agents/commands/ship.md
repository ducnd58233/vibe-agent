---
description: Pre-ship parallel review — code-reviewer + security-auditor + test-engineer, then GO/NO-GO
---

Follow [`shipping-and-launch`](../skills/shipping-and-launch/SKILL.md) and [`references/orchestration-patterns.md`](../references/orchestration-patterns.md).

`/ship` **fans out** three personas on the same change, then merges in the main session:

## Phase A — Parallel fan-out

Spawn three subagents in **one turn** when using Claude Code’s Agent tool (`subagent_type` matches YAML `name`):

1. **`code-reviewer`** — Five-axis review; template in [`agents/code-reviewer.md`](../agents/code-reviewer.md); grounded in [`code-review-and-quality`](../skills/code-review-and-quality/SKILL.md).
2. **`security-auditor`** — [`agents/security-auditor.md`](../agents/security-auditor.md) + [`security-and-hardening`](../skills/security-and-hardening/SKILL.md).
3. **`test-engineer`** — [`agents/test-engineer.md`](../agents/test-engineer.md) + [`test-driven-development`](../skills/test-driven-development/SKILL.md).

Without Agent tool: run the three perspectives sequentially but merge as below.

Personas **must not** delegate to each other.

## Phase B — Merge

Synthesize: quality blockers, security blockers, coverage gaps, accessibility ([`references/accessibility-checklist.md`](../references/accessibility-checklist.md) if UI changed), infra/env/feature flags.

## Phase C — Decision

Emit **Ship Decision: GO | NO-GO** with blockers, recommended fixes, acknowledged risks, **rollback plan**, and appended specialist reports.

**Skip fan-out** only if the change is trivial: ≤2 files, ~≤50 lines, and no auth/payments/data/config — otherwise default to full `/ship`.
