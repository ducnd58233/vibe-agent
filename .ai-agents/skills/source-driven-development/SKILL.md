---
name: source-driven-development
description: >-
  Grounds framework and library decisions in official docs and pinned versions. Use when implementing web, HTTP API, agent, or database client patterns where APIs drift; cite URLs for non-obvious choices.
disable-model-invocation: true
---

# Source-Driven Development

## Overview

<context>

Framework-specific decisions should trace to **authoritative, version-appropriate documentation**, not memory. State detected versions from lockfiles / manifests, then implement and **cite** sources.
</context>

## When to Use

<routing>

- Building or changing patterns that depend on framework/library APIs.
- User asks for “correct” or documented behavior.
- Reviewing code that may use deprecated APIs.

**When NOT to use:** Pure refactors with no API surface change; typos; moves with identical behavior.

## Routing & discovery

- Use when correctness depends on external docs/versioned APIs.
- Do not use for purely local refactors with no API decisions.

Use when framework/library API behavior influences implementation.
</routing>

## Permissions & authority

<required>

- Tools: repository read/search plus web retrieval for authoritative sources.
- Authority: every non-obvious API choice should be source-backed.
</required>

## Process

<context>

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

Read the documentation that matches the **pinned version** in the manifests/lockfiles, not memory and not a newer/older version. For a brand-new dependency, read the current official docs. If the version or intended behavior is ambiguous, ask the user; do not guess.

### 2. Fetch official documentation

**Authority order:** official docs → official changelog/blog → web standards (MDN) → compatibility tables.

**Not primary sources:** random tutorials, uncited Stack Overflow, unverified AI summaries.

Fetch the **specific page** for the feature (e.g. your framework’s route/API handler docs, dependency injection guide, aggregation/query API), not only the product homepage.

### 3. Implement from documented patterns

- Match signatures and recommended patterns from current docs.
- If docs deprecate a pattern, avoid it unless the repo standardizes otherwise via ADR.

**Conflict with existing code:** surface it - do not silently diverge.

```text
CONFLICT: Docs recommend X; codebase uses Y (file Z).
Options: A) adopt X, B) stay consistent with Y until refactor.
```

### 4. Cite sources

For non-obvious framework choices, add comments or reply text with **full URLs** (deep links with anchors when helpful).

```text
// Example - Image LCP tuning - https://<your-framework-docs>/...
```

If something cannot be verified in official docs, label it **UNVERIFIED**.
</context>

## Scaffolding and dependencies (CLI-first), MUST

<procedure>

When **initializing a project/tool** or **adding a package/library**, do **not** fabricate the file/folder layout, config, or lockfile from memory. Project generators evolve; hand-written scaffolds drift from the current template and miss required wiring.

1. **Read the official docs** for the framework/library/language at the detected (or latest stable) version.
2. **Run the canonical CLI** so the tool generates its own structure and updates manifests/lockfiles.

   The commands below are **illustrative examples, not a closed list**. The authoritative, current commands for this workspace live in the matching [`stack-profiles/`](../../stack-profiles/ROUTER.md) entry. Any generator can change its name or flags, so confirm against current official docs before running. Examples seen in common stacks:

   | Stack | Init and add (verify current command in docs) |
   |-------|-----------------------------------------------|
   | Next.js | `npx create-next-app@latest` |
   | shadcn/ui | `npx shadcn@latest init`, then `npx shadcn@latest add <component>` |
   | React (Vite) | `npm create vite@latest` |
   | Node deps | `npm install <pkg>` or `pnpm add <pkg>` |
   | Python (uv) | `uv init`, then `uv add <pkg>` |
   | Go | `go mod init <module>`, then `go get <pkg>` |
   | Rust | `cargo new <name>` or `cargo init`, then `cargo add <crate>` |

3. **Let the tool write its files**, then edit generated output. If a CLI step needs approval or a destructive flag, surface it rather than reproducing its output by hand.

### Makefile command surface, MUST

Capture the project's operational commands in a **`Makefile`** at the repo root so humans and agents share one entry point. **Exception:** Node/JS projects that already centralize commands in `package.json` `scripts`; use those instead of duplicating into a Makefile.

Provide targets (adapt names to the stack/tooling), at minimum:

```makefile
docker-up / docker-down   # local dependencies (db, cache, queues) via docker compose
run                        # run the app locally
build                      # compile/build artifacts
test                       # run the test suite
lint                       # lint + format check (and typecheck where applicable)
migrate-new name=<x>       # create a new migration
migrate-up                 # apply migrations
migrate-down               # roll back the last migration
```

Wire each target to the **documented** CLI for the migration/runtime tool the repo actually uses. Confirm the current commands from that tool's docs; the specific migration tool belongs in the stack-profile, not hardcoded here.
</procedure>

## Verification

<verification>

- [ ] Versions identified from repo manifests
- [ ] Official docs consulted for non-trivial API usage
- [ ] Citations included for non-obvious decisions
- [ ] Deprecated APIs avoided or explicitly justified
- [ ] Unverified items labeled clearly
- [ ] Project/package scaffolding done via the official CLI (no hand-fabricated init files/folders)
- [ ] Operational commands captured in a `Makefile` (docker, run, build, test, lint, migrate new/up/down), or `package.json` scripts for Node projects
</verification>
