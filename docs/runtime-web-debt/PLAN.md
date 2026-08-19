# PLAN: Runtime web layer debt

<procedure>

## Dependency graph

```text
Task 1 (typed errors) ──► Task 3 (isNotFoundErr removal)
Task 2 (HTMX error helper) ──► Task 4–6 (handler migrations by file)
Task 7 (infra read ports) ──► Task 8 (view I/O removal)
Task 9 (panic recovery) — independent; needs product decision
Task 10 (redact imports) — independent, cosmetic
Task 11 (validate DRY) — independent, low risk
Task 12 (SSE hub phase 2) — optional, after Task 2
```

## Phases

### Phase A — Foundations (Tasks 1–2)

Typed not-found and session-read errors; HTMX-aware helper used by new code before migrating old call sites.

**Checkpoint:** human approves error fragment shape for HTMX partials. **Complete.**

### Phase B — HTMX error migration (Tasks 3–6)

Migrate one file group per PR: `handler_http.go` helpers, `files.go`, `workspace.go`, `catalog.go`. Each PR keeps behavior; only response shape changes.

**Checkpoint:** manual browser check on file picker and workspace switcher. **Complete** (PR #54).

### Phase C — Clean architecture (Tasks 7–8)

Introduce `web/infra/sessionread` (or extend existing persistence) for log tail / peekHost / ambient stat. View functions accept data or call ports injected from `web/app`.

**Checkpoint:** `make check` plus existing view tests green without `os.*` in `web/view`. **Complete** (PR #55).

### Phase D — Cross-cutting (Tasks 9–11)

Panic recovery policy, redact import cleanup, graph validate sharing.

**Status:** Tasks 10–11 complete (PR #55). Task 9 partial (doc only).

### Phase E — Optional (Task 12)

SSE hub consolidation if spec open question 3 answers yes.

**Status:** deferred (see TASKS.md Task 12).

## Risk controls

- No single PR touches more than one handler file group.
- Disclosure check on every error-path change: partial HTML must not widen stderr or file contents.
- Table-driven test for each migrated handler (HTMX request header vs full page).

## Verification rhythm

After each task: `cd runtime && make check`. After Phase B: loopback manual pass on `/files`, workspace picker. Evidence under `tmp/runtime-web-debt/` when executing `/goal` on this slug.

</procedure>
