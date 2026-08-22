---
name: researcher-harness
description: >-
  Domain-agnostic research loop for vibe-agent: literature with mandatory
  Applicability and Mermaid, experiment plans with Mermaid, host/CI STATUS
  monitoring until done or failed, findings that cite runs. Use for AI, eng,
  finance, or non-code research on researcher-delivery. Not for product ship
  (use goal-driven-delivery).
disable-model-invocation: true
---

# Researcher harness

## How

<procedure>

1. **Literature** - citation-first digest. MUST include:
   - **Applicability** - how each source maps to *this* topic (reuse / reject / gap).
   - **Refine** - what to change before experiments.
   - A fenced `mermaid` literature or claim→method diagram.
2. **Hypothesis** - testable questions derived from Refine.
3. **Experiment design** - PLAN with Mermaid setup (data → protocol → metrics → stop), plus TASKS.
4. **Run** - host or CI only. Keep `experiment/STATUS.md` (`running|done|failed`).
5. **Monitor** - `vibe_verify` / `vibe_experiment_status` until terminal.
6. **Findings + writeup** - cite STATUS and artifacts; no orphan claims.

Anti-fabrication: no model assertion as check evidence. Gates use `file_assert` / `human_event` / `exit_code` / `ci_api` only.

GPU/sandbox: unsupported in-process. Document host/CI as the compute port.
</procedure>

## Routing & discovery

<routing>

- Graph: [`researcher-delivery`](../../graphs/researcher-delivery.yaml)
- Commands: [`research.md`](../../commands/research.md), [`experiment.md`](../../commands/experiment.md), [`findings.md`](../../commands/findings.md)
- Cursor rule: research Applicability + Mermaid MUST
- Prefer [`ai-research-methodology`](../ai-research-methodology/SKILL.md) only for AI/ML method detail overlays

Use for researcher workflows. Avoid when shipping product code through `goal-delivery`.
</routing>

## Permissions & authority

<required>

- Tools: Read, Grep, Glob, WebSearch, WebFetch, Bash (host experiment commands), MCP `vibe_experiment_status` / `vibe_verify`
- No forging `human_event`; auto gates use document structure tests
</required>
