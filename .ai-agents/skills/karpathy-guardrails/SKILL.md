---
name: karpathy-guardrails
description: >-
  Enforces assumption-checking, simplicity, surgical diffs, and goal-driven
  verification to avoid common LLM coding failure modes. Use for non-trivial
  implementation, refactors, and bugfixes where correctness and change
  discipline matter.
---

# Karpathy Guardrails

## Overview

<context>

This skill operationalizes five guardrails:

1. Think before coding.
2. Simplicity first.
3. Surgical changes.
4. Goal-driven execution with explicit verification.
5. Grounded claims (no fabrication).

It is designed to reduce costly assistant failure patterns:
unchecked assumptions, overengineering, orthogonal edits, unverified claims, and
fabricated files/paths/results.

These guardrails are **harness-agnostic**: apply them identically across Claude, Codex,
Cursor, opencode, and any other tool, and to both primary agents and subagents.
</context>

## Inputs

<inputs>

- User request and constraints
- Relevant code context
- Existing tests/build commands and quality gates
</inputs>

## Outputs

<outputs>

- Minimal, correct implementation aligned to request
- Explicit assumptions/tradeoffs when ambiguity exists
- Verifiable completion evidence (tests/checks/runtime proof)
</outputs>

## When to use

<routing>

Use when:

- Requirements are non-trivial or ambiguous.
- Code changes could impact correctness or maintainability.
- You need robust guardrails against scope creep and over-design.

Do not use when:

- Task is a trivial one-line cosmetic fix with no ambiguity.
- User explicitly requests brainstorming-only discussion without execution.
</routing>

## Guardrails

<rules>

### 1) Think before coding (ask first)

- **MUST ask the user a focused question** when the request is ambiguous, underspecified, or has conflicting constraints, before changing code. Do not guess an interpretation and execute. Running ahead on a wrong guess and making the code worse is the failure this guardrail exists to prevent.
- State assumptions before non-trivial execution when you do proceed.
- Surface ambiguity instead of silently choosing an interpretation.
- Before using or upgrading a framework/library, read the docs for the version pinned in the repo manifests/lockfiles (not memory or a different version). If the version or intended behavior is unclear, ask.

### 2) Simplicity first

- Prefer the smallest design that satisfies requirements.
- Avoid speculative abstractions and future-proofing not requested.
- Rewrite overcomplicated drafts to simpler equivalents.

### 3) Surgical changes

- Limit edits to directly relevant files and lines.
- Do not “cleanup sweep” unrelated code/comments.
- If unrelated issues are noticed, report separately; do not auto-refactor.

### 4) Goal-driven execution

- Define success criteria in observable terms.
- Use verification loops (tests/build/runtime checks) before claiming done.
- Treat “looks correct” as insufficient without evidence.

### 5) Grounded claims (no fabrication)

- Never describe a file, directory, path, command result, or source you have not actually opened, listed, or run — quote the observed tool output as the basis.
- If a provided path or resource is inaccessible (not found, empty, out of sandbox), report `ACCESS-FAILED: <path>` and stop, rather than inferring or guessing structure.
- Applies to every harness and to both primary agents and subagents; never let an unreachable input become an invented answer.
</rules>

## Execution flow

<procedure>

1. Restate goal and declare assumptions.
2. Identify minimal change set.
3. Implement with strict scope boundaries.
4. Run verification for requested behavior.
5. Report findings and residual risk succinctly.
</procedure>

## Verification

<verification>

- [ ] Assumptions/tradeoffs were surfaced when needed.
- [ ] No unnecessary abstractions or speculative logic added.
- [ ] Diff is scoped to task intent.
- [ ] Completion is backed by concrete verification evidence.
- [ ] Every file/path/result described was actually observed; inaccessible inputs were reported as `ACCESS-FAILED`, not inferred.
</verification>

## Routing & discovery

<routing>

- Use when reliability and disciplined change management are priorities.
- Do not use as heavyweight ceremony for clearly trivial edits.

Invoke for implementation, bug fixing, and refactoring work where disciplined
execution and evidence-based completion are required.
</routing>
