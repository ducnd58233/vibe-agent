---
name: source-driven-development
description: >-
  Grounds framework and library decisions in official docs and pinned versions. Use when implementing web, HTTP API, agent, or database client patterns where APIs drift; cite URLs for non-obvious choices.
disable-model-invocation: true
---

# Source-Driven Development

## Stack profile for this repository

When working **in this monorepo**, open [`stack-profiles/ROUTER.md`](../../stack-profiles/ROUTER.md), select applicable profiles, and read those files. Product/domain expectations: root [`AGENTS.md`](../../../AGENTS.md).

## Overview

Framework-specific decisions should trace to **authoritative, version-appropriate documentation**, not memory. State detected versions from lockfiles / manifests, then implement and **cite** sources.

## When to Use

- Building or changing patterns that depend on framework/library APIs.
- User asks for “correct” or documented behavior.
- Reviewing code that may use deprecated APIs.

**When NOT to use:** Pure refactors with no API surface change; typos; moves with identical behavior.

## Process

```text
DETECT → FETCH → IMPLEMENT → CITE
```

### 1. Detect stack and versions

Read:

- `package.json` / lockfile → SPA/SSR frameworks, UI libs, runners, bundlers (name what you detect).
- `pyproject.toml` / `uv.lock` / `requirements*.txt` → HTTP frameworks, drivers, ML/agent libs (name what you detect).
- `environment.yml` → Python pin

State explicitly:

```text
STACK DETECTED:
- web: <names@versions from manifests>
- backend: <names@versions from manifests>
```

If versions are ambiguous, ask — do not guess.

### 2. Fetch official documentation

**Authority order:** official docs → official changelog/blog → web standards (MDN) → compatibility tables.

**Not primary sources:** random tutorials, uncited Stack Overflow, unverified AI summaries.

Fetch the **specific page** for the feature (e.g. your framework’s route/API handler docs, dependency injection guide, aggregation/query API), not only the product homepage.

### 3. Implement from documented patterns

- Match signatures and recommended patterns from current docs.
- If docs deprecate a pattern, avoid it unless the repo standardizes otherwise via ADR.

**Conflict with existing code:** surface it — do not silently diverge.

```text
CONFLICT: Docs recommend X; codebase uses Y (file Z).
Options: A) adopt X, B) stay consistent with Y until refactor.
```

### 4. Cite sources

For non-obvious framework choices, add comments or reply text with **full URLs** (deep links with anchors when helpful).

```text
// Example — Image LCP tuning — https://<your-framework-docs>/...
```

If something cannot be verified in official docs, label it **UNVERIFIED**.

## Verification

- [ ] Versions identified from repo manifests
- [ ] Official docs consulted for non-trivial API usage
- [ ] Citations included for non-obvious decisions
- [ ] Deprecated APIs avoided or explicitly justified
- [ ] Unverified items labeled clearly
