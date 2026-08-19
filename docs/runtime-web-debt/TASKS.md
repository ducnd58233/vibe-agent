# TASKS: Runtime web layer debt

Slug: `runtime-web-debt`. Spec: [SPEC.md](./SPEC.md). Plan: [PLAN.md](./PLAN.md).

## Progress summary

| Task | Status | Notes |
|------|--------|-------|
| 1 Typed errors | **done** | PR #54 |
| 2 HTMX helper | **done** | PR #54 |
| 3 handler_http | **done** | PR #54 |
| 4 files.go | **done** | PR #54 |
| 5 workspace.go | **done** | PR #54 |
| 6 catalog.go | **done** | PR #54 |
| 7 sessionread port | **done** | PR #55; fake in `internal/testutil` (pending commit) |
| 8 view no I/O | **done** | PR #55 |
| 9 panic HTML policy | **partial** | Decision in `runtime/AGENTS.md` (PR #55); HTMX panic test not added |
| 10 redact imports | **done** | PR #55 |
| 11 validate DRY | **done** | PR #55 |
| 12 SSE hub phase 2 | **deferred** | No shared hub needed; see Task 12 |

---

## Task 1: Typed session and not-found errors

**Status:** done (PR #54)

**Description:** Replace stringly "not found" errors from session replay and run event reads with sentinel errors in `web/infra` or `internal/session` that `errors.Is` can match.

**Acceptance criteria:**

- [x] Exported `ErrSessionLogNotFound` (or equivalent) used when log path missing.
- [x] `app/isNotFoundErr` deleted; handlers use `errors.Is`.
- [x] Unit tests for error wrapping from replay/read paths.

**Verification:**

- [x] `cd runtime && go test ./internal/web/... ./web/view/...`

**Dependencies:** None

**Files likely touched:**

- `runtime/internal/session/replay.go`
- `runtime/internal/web/app/tail.go`, `sse.go`
- `runtime/web/view/tail.go`

**Estimated scope:** Medium

**Delivery branch:** `refactor/runtime-web-debt-task-1-typed-errors`

---

## Task 2: HTMX partial error helper

**Status:** done (PR #54)

**Description:** Add `respondHTMXError` (name TBD) in `web/app` that sets `Content-Type: text/html`, writes a safe fragment, and respects `HX-Request`. Do not replace all call sites yet.

**Acceptance criteria:**

- [x] Helper used by at least one handler and documented in `runtime/AGENTS.md`.
- [x] HTMX and non-HTMX paths tested.
- [x] No credential or raw path leakage in fragment body.

**Verification:**

- [x] `cd runtime && go test ./internal/web/app/...`

**Dependencies:** None

**Estimated scope:** Small

**Delivery branch:** `refactor/runtime-web-debt-task-2-htmx-helper`

---

## Task 3: Migrate `handler_http.go` error helpers

**Status:** done (PR #54)

**Description:** Point `writeBadForm`, `writeTemplateError`, `writeBadAfter`, method-not-allowed through Task 2 helper or `renderError` where appropriate.

**Acceptance criteria:**

- [x] No `http.Error` in `handler_http.go`.
- [x] Existing handler tests updated.

**Verification:**

- [x] `cd runtime && make check`

**Dependencies:** Task 2

**Files likely touched:**

- `runtime/internal/web/app/handler_http.go`
- `runtime/internal/web/app/*_test.go`

**Estimated scope:** Small

**Delivery branch:** `refactor/runtime-web-debt-task-3-handler-http`

---

## Task 4: Migrate `files.go` HTMX errors

**Status:** done (PR #54)

**Description:** Replace ~17 `http.Error` calls in file picker and preview handlers with Task 2 helper; preserve status semantics via fragment or redirect where full-page fits better.

**Acceptance criteria:**

- [x] Zero `http.Error` in `files.go`.
- [x] File preview still redacts via `redact.Text` before HTML.
- [x] Manual: open file modal, bad path shows fragment not plain text dump.

**Verification:**

- [x] `cd runtime && make check`
- [x] Manual loopback check on `/files` routes

**Dependencies:** Task 2

**Estimated scope:** Medium

**Delivery branch:** `refactor/runtime-web-debt-task-4-files-htmx`

---

## Task 5: Migrate `workspace.go` errors

**Status:** done (PR #54)

**Description:** Replace workspace switcher `http.Error` with redirect+query or HTMX fragment per existing POST patterns.

**Acceptance criteria:**

- [x] Zero `http.Error` in `workspace.go`.
- [x] Failed workspace save surfaces user-visible error on settings page.

**Verification:**

- [x] `cd runtime && make check`

**Dependencies:** Task 2

**Estimated scope:** Small

**Delivery branch:** `refactor/runtime-web-debt-task-5-workspace-errors`

---

## Task 6: Migrate `catalog.go` errors

**Status:** done (PR #54)

**Description:** Catalog search partial returns HTMX-safe error fragment instead of `http.Error`.

**Acceptance criteria:**

- [x] Zero `http.Error` in `catalog.go`.
- [x] Composer catalog search failure shows inline message.

**Verification:**

- [x] `cd runtime && make check`

**Dependencies:** Task 2

**Estimated scope:** Small

**Delivery branch:** `refactor/runtime-web-debt-task-6-catalog-errors`

---

## Task 7: Session read infra port

**Status:** done (PR #55)

**Description:** Create `web/infra/sessionread` (or extend `web/infra/persistence`) with interfaces for tail replay, ambient stat, peekHost line scan. Implement with existing `session` + `os` calls.

**Acceptance criteria:**

- [x] Port interface documented; fake usable in tests (`internal/testutil.SessionReadFake`).
- [x] No new I/O in `web/view` from this task alone.

**Verification:**

- [x] `cd runtime && go test ./internal/web/infra/...`

**Dependencies:** Task 1

**Estimated scope:** Medium

**Delivery branch:** `refactor/runtime-web-debt-task-7-sessionread-port`

---

## Task 8: Remove view layer direct I/O

**Status:** done (PR #55)

**Description:** Refactor `web/view/sessions.go`, `pages.go`, `tail.go` to use Task 7 port via parameters or app-layer injection; view remains pure projection.

**Acceptance criteria:**

- [x] No `os.Open` / `os.Stat` in `web/view` for session/workspace data.
- [x] All view tests pass without filesystem in view package tests (fakes at boundary).

**Verification:**

- [x] `cd runtime && make check`

**Dependencies:** Task 7

**Estimated scope:** Large

**Delivery branch:** `refactor/runtime-web-debt-task-8-view-no-io`

---

## Task 9: Panic recovery HTML policy

**Status:** partial (PR #55 documented; test gap remains)

**Description:** Decide and implement how `middleware.Recover` serves HTML for `HX-Request` and full page requests. Options: optional render hook on `StandardStack`, or document API-only plain text as intentional.

**Acceptance criteria:**

- [x] Decision recorded in `runtime/AGENTS.md`.
- [ ] Test covers panic on HTMX route with expected content type.
- [x] Panic value never logged raw (`LogPanicRecovered` unchanged).

**Verification:**

- [ ] `cd runtime && go test ./internal/shared/infra/httpserver/...` (add panic+HTMX test)

**Dependencies:** None (product decision)

**Estimated scope:** Medium

**Delivery branch:** `refactor/runtime-web-debt-task-9-panic-html`

**Decision (2026-08-20):** Keep `middleware.Recover` on `httpserver.RespondError` (HTMX gets HTML fragment via existing helper; no `renderError` hook). Remaining work: one middleware test that panics on an HTMX request.

---

## Task 10: Direct `shared/redact` imports in web

**Status:** done (PR #55)

**Description:** Replace `session.RedactText` call sites in `web/app` and `web/view` with `redact.Text` where session adds no value; keep `session.RedactText` as thin delegate or deprecate in comment.

**Acceptance criteria:**

- [x] Web packages import `shared/redact` for redaction at boundary.
- [x] No behavior change in redaction tests.

**Verification:**

- [x] `cd runtime && make check`

**Dependencies:** None

**Estimated scope:** Small

**Delivery branch:** `chore/runtime-web-debt-task-10-redact-imports`

---

## Task 11: Shared validate for graph asset IDs

**Status:** done (PR #55)

**Description:** Move or wrap `graph/domain` identifier patterns into `shared/validate` where they overlap with slug rules; keep graph-specific patterns local.

**Acceptance criteria:**

- [x] No duplicate regex for run slug (already in `shared/validate`).
- [x] Graph validation tests unchanged in behavior.

**Verification:**

- [x] `cd runtime && go test ./internal/graph/...`

**Dependencies:** None

**Estimated scope:** Small

**Delivery branch:** `refactor/runtime-web-debt-task-11-validate-dry`

---

## Task 12 (optional): SSE hub phase 2

**Status:** deferred

**Description:** Consolidate session SSE polling with shared hub if repeated connection logic remains after Tasks 1–6.

**Acceptance criteria:**

- [x] Documented in spec addendum or closed as wont-fix with reason.
- [ ] If implemented: one hub owner, tests for multi-subscriber cleanup.

**Verification:**

- [ ] `cd runtime && make check`
- [ ] Manual SSE session stream check

**Dependencies:** Tasks 2, 4

**Estimated scope:** Large

**Delivery branch:** `refactor/runtime-web-debt-task-12-sse-hub`

**Defer reason:** Session SSE already uses `shared/streaming/sse.Poll` with one poll loop per connection; no duplicated hub logic worth a phase-2 refactor in this slug. Reopen under a future `runtime-sse-hub` slug if multi-subscriber sharing becomes a requirement.

---

## Checkpoints

| After | Action | State |
|-------|--------|-------|
| Task 2 | Human approves HTMX error fragment shape | passed (shipped) |
| Task 6 | Manual loopback pass: files, workspace, catalog | passed (browser smoke) |
| Task 8 | Architecture review: view has zero filesystem imports | passed (PR #55) |
| Task 9 | Human approves panic HTML vs API-only policy | passed (document-only) |

## Remaining work (this slug)

1. **Task 9:** Add `middleware.Recover` + HTMX panic integration test.
2. **Follow-up (convention):** Commit `internal/testutil` layout (paths + `SessionReadFake`) and keep fakes out of production packages.
