---
name: backend-engineering
description: >-
  Structures backend code as modular bounded packages with layered clean architecture—controllers/transports, services, repositories, infra, config—transactions in services only, one table per repository, explicit DTOs at edges. Use when designing or refactoring HTTP/gRPC/async workers or server-side domains.
disable-model-invocation: true
---

# Backend Engineering

## Stack profile for current workspace

Do not assume a global stack — inspect manifests and existing patterns. When working **in a repository that includes this toolkit**, open [`stack-profiles/ROUTER.md`](../../stack-profiles/ROUTER.md) when present, select every applicable profile row for your task, and read those files. Product/domain expectations: root [`AGENTS.md`](../../../AGENTS.md).

## Overview

Apply **modular bounded contexts** with **clear layering**: transport code stays thin, orchestration lives in services, persistence is isolated behind repositories, and infrastructure (DB drivers, queues, outbound HTTP) stays at the rim. Loose coupling favors **explicit contracts** between modules—not shared singletons or cross-imports into sibling feature internals.

**Relationship:** [`api-and-interface-design`](../api-and-interface-design/SKILL.md) defines **external contracts** (HTTP shapes, versioned APIs, boundary validation). **This skill** owns **internal module layout**, **transaction borders**, **repository scope**, and **type mapping between layers.**

## When to Use

- New service, bounded context, or API module skeleton.
- Refactoring a “fat handler” into services and repositories.
- Introducing persistence, messaging, or background jobs alongside HTTP/gRPC surfaces.
- Code review targeting coupling, misplaced transactions, or repository sprawl.

**When NOT to use:** Solely choosing REST vs GraphQL (see `api-and-interface-design`); client-only concerns (see `frontend-ui-engineering`).

## Target module layout

Adapt naming to repo conventions (Go packages, Rust modules, Java packages, Nest modules, FastAPI routers, …). Canonical **shape**:

```text
backend/
├── config/                 # Loaded settings, typed config structs
├── infra/                  # DB engine, pooled clients, message buses, outbound HTTP wrappers
├── modules/
│   └── <bounded_context>/  # Feature / domain boundary
│       ├── controllers/    # or handlers/
│       │   ├── http/       # REST/JSON, server actions adapters, ...
│       │   └── grpc/       # protobuf services (if applicable)
│       ├── services/       # Use cases orchestration ONLY here
│       ├── repositories/   # Persistence per table rule (below)
│       └── dto/ or domain/ # Optional: exported types scoped to module
└── ...
```

Rules:

1. **`infra/`**: technology adapters only—no business rules leaking upward without interfaces.
2. **`config/`**: no feature logic—reads env/files and supplies typed config to composition root.
3. **`modules/<context>/`**: one **bounded context** per folder; avoid importing another module’s **internal** subpackages—depend on **published ports** (interfaces) or shared **kernel** types when truly cross-cutting.

## Layer responsibilities

| Layer | Owns | Must not |
|-------|------|----------|
| **Controllers** (http/grpc/…) | Parse/validate transport, map to service DTOs, status codes, auth wiring | Orchestrate transactions; call repositories directly |
| **Services** | Use cases; transaction boundaries; call one or many repositories | Raw SQL/driver calls; protocol details |
| **Repositories** | Read/write **one persisted table / collection** via ORM/driver | Cross-table orchestration without going through services |
| **Infra** | Connections, retries, serializers at the metal | Domain rules |

## Mandatory rules

### Modular structure and coupling

- **Bounded modules:** group by capability or subdomain; keep **internals private** (`internal/` in Go, package-private patterns elsewhere).
- **Loose coupling:** depend on **stable interfaces** (ports) declared next to consumers or shared domain—not concrete infra types from unrelated modules.

### Transactions (NON-NEGOTIABLE)

- **Begin/commit/rollback (or equivalent unit-of-work) live ONLY in the service layer** orchestrating multi-step work.
- **Repositories never start or end transactions** for business flows; they accept a **session/connection/tx handle** supplied by the service or unit-of-work if the stack requires it.
- If one use case needs **multiple repositories**, the **service** coordinates them inside **one** transaction scope.

### Repository scope (NON-NEGOTIABLE)

- **Exactly one primary persisted table (or one clearly defined document/collection) per repository class** / type.
- No “god repository” that reads/writes unrelated tables. **Joins across tables** for a use case are orchestrated in the **service** through **multiple** repositories (or a read model pattern—still not one mega-repository).

### Types at boundaries (NON-NEGOTIABLE)

- **Transport → service:** use **request/response DTOs** (structs, records, Pydantic models, …) defined for the application boundary—not ORM entities leaking from handlers.
- **Service → repository:** use **persistence types** (ORM models, row structs, DB schema shapes) **inside** the repository adapter; **map explicitly** between service-level types and persistence types in the service or in dedicated mappers colocated with the repository package.
- **Never** return ORM entities directly from HTTP/gRPC handlers to clients.

## Data flow (happy path)

```text
Client
  → Controller: validate transport → Command/Query DTO
    → Service: open transaction → call Repo A (+ Repo B if needed)
      → Repo: single-table operations using persistence structs
    → Service: commit → map to response DTO
  → Controller: serialize response
```

## Anti-patterns

| Smell | Fix |
|-------|-----|
| Handler invokes ORM/session directly | Lift into service + repo |
| `Repository.save(order, order_lines, invoice)` spanning tables | Split repos; transactional service |
| `begin_transaction()` inside repo | Move to service or UoW injected into service |
| Shared mutable statics between modules | Inject dependencies; interfaces |
| DTO ≡ ORM entity | Introduce mapping layer |

## Verification

After structural work:

- [ ] Every inbound transport calls **one entry** method on a service for the use case—not scattered repo calls from controllers.
- [ ] No transactional API on repositories except accepting an existing session/tx from caller.
- [ ] Each repository maps to **one** table/collection naming doc or invariant comment.
- [ ] Modules do not circular-import sibling feature internals.
- [ ] DTO types used at controller boundary differ from persistence types where ORM bleed was a risk.

## Related references

- [`api-and-interface-design`](../api-and-interface-design/SKILL.md) — public contracts and evolution.
- [`security-and-hardening`](../security-and-hardening/SKILL.md), [`references/security-checklist.md`](../../references/security-checklist.md).
- [`test-driven-development`](../test-driven-development/SKILL.md) — fixtures for services with fake repos.
