---
name: engineering-principles
description: >-
  Enforces core software-design principles (SOLID, DRY, KISS, YAGNI, composition over inheritance, separation of concerns), judicious design-pattern use, and plain human writing in code and comments so code stays extensible and maintainable. Use when implementing or refactoring non-trivial logic, designing module/class boundaries, or reviewing code for coupling, duplication, over-engineering, or noisy comments.
---

# Engineering Principles

## Stack profile for current workspace

Apply these principles within the conventions of the detected stack; do not impose foreign idioms.

## Overview

Code is read and changed far more than it is written. These principles exist to keep change cheap and safe: small surfaces, single reasons to change, no duplicated truth, and patterns applied only where they remove real friction. This skill is a default coding baseline alongside [`karpathy-guardrails`](../karpathy-guardrails/SKILL.md) (simplicity, surgical diffs). When a principle and "simplest thing that works" appear to conflict, resolve it explicitly rather than silently picking gold-plating.

## When to Use

- Implementing non-trivial logic, a new module, or a class/interface boundary.
- Refactoring tangled or duplicated code, or splitting a "god" type/function.
- Reviewing code for coupling, duplication, leaky abstractions, over-engineering, or noisy comments.

**When NOT to use:** trivial one-line changes, typo/format fixes, or moves with identical behavior. Do not invoke patterns as a checklist on tiny code.

## Core principles (MUST follow when implementing)

### SOLID

| Principle | Rule of thumb | Smell it fixes |
|-----------|---------------|----------------|
| **S** Single Responsibility | One module/class/function has **one reason to change**. | "and"-named units; edits for unrelated features touch the same file. |
| **O** Open/Closed | Extend behavior by adding code (new impl/strategy), not by editing stable call sites. | growing `switch`/`if-else` on a type tag across the codebase. |
| **L** Liskov Substitution | Subtypes honor the base contract (no surprising exceptions, narrowed inputs, weakened outputs). | `isinstance`/type checks to special-case a "subtype". |
| **I** Interface Segregation | Many small role interfaces over one fat interface; clients depend only on what they use. | implementers stubbing methods with `NotImplemented`. |
| **D** Dependency Inversion | Depend on **abstractions/ports**, inject concretions at the composition root. | high-level logic importing concrete DB/HTTP clients directly. |

### DRY, KISS, YAGNI

- **DRY:** one authoritative source for each piece of knowledge (logic, constants, schemas). Deduplicate **knowledge**, not coincidentally-similar lines. In tests, prefer **DAMP** over DRY (see [`test-driven-development`](../test-driven-development/SKILL.md)).
- **KISS:** choose the simplest design that satisfies current requirements and is clear to the next reader. Complexity must earn its place.
- **YAGNI:** build for today's known requirements; do not add hooks, config, or generality for hypothetical futures. This bounds the patterns below.

### Boundaries and coupling

- **Separation of concerns:** keep transport, business logic, and persistence in distinct layers (see [`backend-engineering`](../backend-engineering/SKILL.md)).
- **Composition over inheritance:** prefer assembling small collaborators over deep inheritance trees.
- **Law of Demeter:** talk to immediate collaborators; avoid `a.b().c().d()` reach-through chains.
- **High cohesion, loose coupling:** related things together, dependencies explicit and minimal.

## Design patterns (use to extend and maintain, not to decorate)

Reach for a pattern when it removes a concrete, present pain (branching on type, hard-to-test construction, cross-cutting behavior). Name the pattern and the need it resolves; if you cannot name the need, do not add the pattern (YAGNI wins).

| Need you actually have | Candidate pattern(s) |
|-------------------------|----------------------|
| Swap one of several interchangeable algorithms/behaviors | Strategy, Policy |
| Construction is complex, branches on input, or is hard to test | Factory, Builder, Abstract Factory |
| Decouple high-level logic from concrete infra (DB, HTTP, queue) | Repository, Adapter, Ports and Adapters (Hexagonal), Dependency Injection |
| Add behavior without editing the core type | Decorator, Strategy, Observer |
| One pipeline with pluggable steps | Chain of Responsibility, Pipeline, Template Method |
| Single point for shared, expensive, or lifecycle-bound resource | Singleton (sparingly, prefer DI scope), Object Pool |
| React to state changes or events | Observer, Pub/Sub, State |

**Anti-pattern guardrails:** no speculative generality, no pattern for a single call site, no framework-within-the-framework. Prefer the language/stack idiomatic mechanism (Go interfaces, Rust traits, Python protocols, DI containers) over hand-rolled machinery.

## Anti-patterns and fixes

| Smell | Fix |
|-------|-----|
| God class/function doing many things | Split by responsibility (SRP); extract collaborators |
| Copy-pasted logic in N places | Extract one source of truth (DRY) |
| `switch`/`if` on a type code, repeated | Polymorphism or Strategy (OCP) |
| High-level module imports concrete infra | Introduce a port; inject the impl (DIP) |
| Deep inheritance for reuse | Compose small units instead |
| Pattern added "to be flexible" with one impl | Remove it; inline the simple code (YAGNI/KISS) |

## Comments and writing style (MUST)

Code and comments are read by humans. Write them like a human engineer would.

- **Comment only what earns it.** Add a comment when it explains **why** (intent, tradeoff, non-obvious constraint, link to a ticket/spec). Do not narrate **what** the code already says. Delete comments that restate the line below them.
- **Plain words.** Use direct, concrete language. Avoid AI-tell filler words such as: ensure, enhance, simplify, leverage, utilize, robust, seamless, comprehensive, delve, facilitate, streamline, effortless. Say what the code does in plain terms instead.
- **No decorative characters.** Do not use emojis, icons, or the em-dash character in code, comments, or commit messages. Use a normal hyphen, a comma, or two sentences. Avoid ASCII art and banner comments.
- **Names over comments.** Prefer a clear identifier to a comment that explains a vague one. A good name removes the need for the note.
- **No dead weight.** Do not leave commented-out code, TODO dumps without an owner/context, or auto-generated boilerplate comments.

This style also applies to commit messages and to the agent's own replies (see [`AGENTS.md`](../../../AGENTS.md) plain-writing rule and [`git-workflow-and-versioning`](../git-workflow-and-versioning/SKILL.md) for commit format).

## Verification

After non-trivial implementation or refactor:

- [ ] Each new/changed unit has a single clear responsibility.
- [ ] No duplicated knowledge introduced; shared logic has one source of truth.
- [ ] New extension points exist only where a real, present need justified them (not speculative).
- [ ] High-level logic depends on abstractions, not concrete infra, where it crosses a boundary.
- [ ] Any design pattern used names the need it resolves; no single-use ceremony.
- [ ] Design stays the simplest that meets current requirements (KISS/YAGNI honored).
- [ ] Comments explain why and only where needed; no AI-tell filler words, emojis, icons, or em-dash characters in code or comments.

## Related skills

- [`karpathy-guardrails`](../karpathy-guardrails/SKILL.md): simplicity bias, surgical diffs, clarify-first; bounds pattern enthusiasm.
- [`backend-engineering`](../backend-engineering/SKILL.md): layering, transactions, repository scope as applied SoC/DIP.
- [`api-and-interface-design`](../api-and-interface-design/SKILL.md): contract design at module/service edges.
- [`code-simplification`](../code-simplification/SKILL.md): reduce complexity once behavior is covered by tests.
- [`source-driven-development`](../source-driven-development/SKILL.md): scaffold and use framework APIs from official docs/CLIs.

## What

Provides a concrete, enforceable set of design principles, a pattern-selection guide, and a comment/writing-style rule for implementation and refactoring work.

## Why

Keeps code extensible and maintainable by reducing coupling and duplication, preventing over-engineering, and keeping code and comments readable in plain language, so future change stays cheap and safe.

## How

Apply the principles during implementation, select patterns only against a named need, write comments and names in plain human language, and run the verification checklist before declaring work done.

## When

Use for non-trivial implementation, module/boundary design, and reviews focused on coupling, duplication, or comment noise; skip for trivial edits.

## Routing & discovery

- **Use when:** implementing or refactoring non-trivial logic, designing boundaries, or reviewing for coupling/duplication/over-engineering/comment noise.
- **Do not use when:** trivial one-line changes, formatting, or behavior-preserving moves.

## Permissions & authority

| Topic | Notes |
|-------|--------|
| **Tools** | Read, Grep, Glob, Edit; Shell only for repo-documented lint/tests. |
| **Paths** | Follow [`.ai-agents/PERMISSIONS.md`](../../PERMISSIONS.md); no secret material. |
| **Cursor** | No `settings.json` matrix; relies on workspace trust and [`.cursor/rules`](../../../.cursor/rules). |
