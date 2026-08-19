# TASKS: Runtime web layer debt

Slug: `runtime-web-debt`. Spec: [SPEC.md](./SPEC.md). Plan: [PLAN.md](./PLAN.md).

---

## Task 1: Typed session and not-found errors

**Description:** Replace stringly "not found" errors from session replay and run event reads with sentinel errors in `web/infra` or `internal/session` that `errors.Is` can match.

**Acceptance criteria:**

- [ ] Exported `ErrSessionLogNotFound` (or equivalent) used when log path missing.
- [ ] `app/isNotFoundErr` deleted; handlers use `errors.Is`.
- [ ] Unit tests for error wrapping from replay/read paths.

**Verification:**

- [ ] `cd runtime && go test ./internal/web/... ./web/view/...`

**Dependencies:** None

**Files likely touched:**

- `runtime/internal/session/replay.go`
- `runtime/internal/web/app/tail.go`, `sse.go`
- `runtime/web/view/tail.go`

**Estimated scope:** Medium

**Delivery branch:** `refactor/runtime-web-debt-task-1-typed-errors`

---

## Task 2: HTMX partial error helper

**Description:** Add `respondHTMXError` (name TBD) in `web/app` that sets `Content-Type: text/html`, writes a safe fragment, and respects `HX-Request`. Do not replace all call sites yet.

**Acceptance criteria:**

- [ ] Helper used by at least one handler and documented in `runtime/AGENTS.md`.
- [ ] HTMX and non-HTMX paths tested.
- [ ] No credential or raw path leakage in fragment body.

**Verification:**

- [ ] `cd runtime && go test ./internal/web/app/...`

**Dependencies:** None

**Estimated scope:** Small

**Delivery branch:** `refactor/runtime-web-debt-task-2-htmx-helper`

---

## Task 3: Migrate `handler_http.go` error helpers

**Description:** Point `writeBadForm`, `writeTemplateError`, `writeBadAfter`, method-not-allowed through Task 2 helper or `renderError` where appropriate.

**Acceptance criteria:**

- [ ] No `http.Error` in `handler_http.go`.
- [ ] Existing handler tests updated.

**Verification:**

- [ ] `cd runtime && make check`

**Dependencies:** Task 2

**Files likely touched:**

- `runtime/internal/web/app/handler_http.go`
- `runtime/internal/web/app/*_test.go`

**Estimated scope:** Small

**Delivery branch:** `refactor/runtime-web-debt-task-3-handler-http`

---

## Task 4: Migrate `files.go` HTMX errors

**Description:** Replace ~17 `http.Error` calls in file picker and preview handlers with Task 2 helper; preserve status semantics via fragment or redirect where full-page fits better.

**Acceptance criteria:**

- [ ] Zero `http.Error` in `files.go`.
- [ ] File preview still redacts via `redact.Text` before HTML.
- [ ] Manual: open file modal, bad path shows fragment not plain text dump.

**Verification:**

- [ ] `cd runtime && make check`
- [ ] Manual loopback check on `/files` routes

**Dependencies:** Task 2

**Estimated scope:** Medium

**Delivery branch:** `refactor/runtime-web-debt-task-4-files-htmx`

---

## Task 5: Migrate `workspace.go` errors

**Description:** Replace workspace switcher `http.Error` with redirect+query or HTMX fragment per existing POST patterns.

**Acceptance criteria:**

- [ ] Zero `http.Error` in `workspace.go`.
- [ ] Failed workspace save surfaces user-visible error on settings page.

**Verification:**

- [ ] `cd runtime && make check`

**Dependencies:** Task 2

**Estimated scope:** Small

**Delivery branch:** `refactor/runtime-web-debt-task-5-workspace-errors`

---

## Task 6: Migrate `catalog.go` errors

**Description:** Catalog search partial returns HTMX-safe error fragment instead of `http.Error`.

**Acceptance criteria:**

- [ ] Zero `http.Error` in `catalog.go`.
- [ ] Composer catalog search failure shows inline message.

**Verification:**

- [ ] `cd runtime && make check`

**Dependencies:** Task 2

**Estimated scope:** Small

**Delivery branch:** `refactor/runtime-web-debt-task-6-catalog-errors`

---

## Task 7: Session read infra port

**Description:** Create `web/infra/sessionread` (or extend `web/infra/persistence`) with interfaces for tail replay, ambient stat, peekHost line scan. Implement with existing `session` + `os` calls.

**Acceptance criteria:**

- [ ] Port interface documented; fake usable in tests.
- [ ] No new I/O in `web/view` from this task alone.

**Verification:**

- [ ] `cd runtime && go test ./internal/web/infra/...`

**Dependencies:** Task 1

**Estimated scope:** Medium

**Delivery branch:** `refactor/runtime-web-debt-task-7-sessionread-port`

---

## Task 8: Remove view layer direct I/O

**Description:** Refactor `web/view/sessions.go`, `pages.go`, `tail.go` to use Task 7 port via parameters or app-layer injection; view remains pure projection.

**Acceptance criteria:**

- [ ] No `os.Open` / `os.Stat` in `web/view` for session/workspace data.
- [ ] All view tests pass without filesystem in view package tests (fakes at boundary).

**Verification:**

- [ ] `cd runtime && make check`

**Dependencies:** Task 7

**Estimated scope:** Large

**Delivery branch:** `refactor/runtime-web-debt-task-8-view-no-io`

---

## Task 9: Panic recovery HTML policy

**Description:** Decide and implement how `middleware.Recover` serves HTML for `HX-Request` and full page requests. Options: optional render hook on `StandardStack`, or document API-only plain text as intentional.

**Acceptance criteria:**

- [ ] Decision recorded in `runtime/AGENTS.md`.
- [ ] Test covers panic on HTMX route with expected content type.
- [ ] Panic value never logged raw (`LogPanicRecovered` unchanged).

**Verification:**

- [ ] `cd runtime && go test ./internal/shared/infra/httpserver/...`

**Dependencies:** None (product decision)

**Estimated scope:** Medium

**Delivery branch:** `refactor/runtime-web-debt-task-9-panic-html`

---

## Task 10: Direct `shared/redact` imports in web

**Description:** Replace `session.RedactText` call sites in `web/app` and `web/view` with `redact.Text` where session adds no value; keep `session.RedactText` as thin delegate or deprecate in comment.

**Acceptance criteria:**

- [ ] Web packages import `shared/redact` for redaction at boundary.
- [ ] No behavior change in redaction tests.

**Verification:**

- [ ] `cd runtime && make check`

**Dependencies:** None

**Estimated scope:** Small

**Delivery branch:** `chore/runtime-web-debt-task-10-redact-imports`

---

## Task 11: Shared validate for graph asset IDs

**Description:** Move or wrap `graph/domain` identifier patterns into `shared/validate` where they overlap with slug rules; keep graph-specific patterns local.

**Acceptance criteria:**

- [ ] No duplicate regex for run slug (already in `shared/validate`).
- [ ] Graph validation tests unchanged in behavior.

**Verification:**

- [ ] `cd runtime && go test ./internal/graph/...`

**Dependencies:** None

**Estimated scope:** Small

**Delivery branch:** `refactor/runtime-web-debt-task-11-validate-dry`

---

## Task 12 (optional): SSE hub phase 2

**Description:** Consolidate session SSE polling with shared hub if repeated connection logic remains after Tasks 1–6.

**Acceptance criteria:**

- [ ] Documented in spec addendum or closed as wont-fix with reason.
- [ ] If implemented: one hub owner, tests for multi-subscriber cleanup.

**Verification:**

- [ ] `cd runtime && make check`
- [ ] Manual SSE session stream check

**Dependencies:** Tasks 2, 4

**Estimated scope:** Large

**Delivery branch:** `refactor/runtime-web-debt-task-12-sse-hub`

---

## Checkpoints

| After | Action |
|-------|--------|
| Task 2 | Human approves HTMX error fragment shape |
| Task 6 | Manual loopback pass: files, workspace, catalog |
| Task 8 | Architecture review: view has zero filesystem imports |
| Task 9 | Human approves panic HTML vs API-only policy |
