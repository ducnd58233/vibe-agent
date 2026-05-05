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

This skill operationalizes four guardrails:

1. Think before coding.
2. Simplicity first.
3. Surgical changes.
4. Goal-driven execution with explicit verification.

It is designed to reduce costly assistant failure patterns:
unchecked assumptions, overengineering, orthogonal edits, and unverified claims.

## Inputs

- User request and constraints
- Relevant code context
- Existing tests/build commands and quality gates

## Outputs

- Minimal, correct implementation aligned to request
- Explicit assumptions/tradeoffs when ambiguity exists
- Verifiable completion evidence (tests/checks/runtime proof)

## When to use

Use when:

- Requirements are non-trivial or ambiguous.
- Code changes could impact correctness or maintainability.
- You need robust guardrails against scope creep and over-design.

Do not use when:

- Task is a trivial one-line cosmetic fix with no ambiguity.
- User explicitly requests brainstorming-only discussion without execution.

## Guardrails

### 1) Think before coding

- State assumptions before non-trivial execution.
- Surface ambiguity instead of silently choosing an interpretation.
- Ask for clarification when blocked by conflicting constraints.

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

## Execution flow

1. Restate goal and declare assumptions.
2. Identify minimal change set.
3. Implement with strict scope boundaries.
4. Run verification for requested behavior.
5. Report findings and residual risk succinctly.

## Verification

- [ ] Assumptions/tradeoffs were surfaced when needed.
- [ ] No unnecessary abstractions or speculative logic added.
- [ ] Diff is scoped to task intent.
- [ ] Completion is backed by concrete verification evidence.

## What

A reusable behavioral guardrail skill for high-quality, low-regression AI coding.

## Why

LLM failure modes often stem from silent assumptions, unnecessary complexity,
broad edits, and weak completion criteria.

## How

Apply explicit assumption management, simplicity bias, surgical diff discipline,
and verification-first completion rules throughout execution.

## When

Invoke for implementation, bug fixing, and refactoring work where disciplined
execution and evidence-based completion are required.

## Routing & discovery

- Use when reliability and disciplined change management are priorities.
- Do not use as heavyweight ceremony for clearly trivial edits.

## Permissions & authority

- Tools: standard repo tools (Read/Glob/rg/Shell) as required.
- Paths: no extra path permissions beyond task scope.
- Cursor: reinforce via `.cursor/rules` and repository charter.
