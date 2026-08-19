# SPEC: Runtime web layer debt

<scope>

## Objective

Pay down intentional deferrals in the loopback web UI and shared HTTP stack so browser-facing behavior matches `runtime/AGENTS.md`: HTMX-safe errors, clean architecture boundaries, typed errors instead of string matching, and one redaction import path. Work is split into small, behavior-preserving PRs because each area carries UX and regression risk.

## Out of scope

- Product redesign of error pages (copy, layout) beyond matching existing `renderError` patterns.
- TruffleHog / Betterleaks migration (track separately; gitleaks stays for inline detection).
- Full SSE hub redesign (phase 2 noted as optional follow-up task).

## Stack

- Go 1.22+ (`runtime/`), stdlib `net/http`, HTMX partials, existing `httpserver` middleware.
- Verification: `cd runtime && make check`.

## Current violations (inventory)

Updated after PR #54 and PR #55. Remaining gaps are called out explicitly.

| Area | Location | Status |
|------|----------|--------|
| `http.Error` in browser handlers | `files.go`, `workspace.go`, `catalog.go`, `handler_http.go` | **fixed** (PR #54) |
| `http.NotFound` on session routes | `tail.go`, `sse.go`, `actions.go` | **open** (not in original task list) |
| View layer filesystem I/O | `web/view/sessions.go`, `pages.go`, `tail.go` | **fixed** via `sessionread.Reader` (PR #55) |
| Redact indirection | `web/view/*`, `web/app/*` | **fixed** direct `shared/redact` at boundaries (PR #55) |
| Panic recovery UX | `middleware/recover.go` → `httpserver.RespondError` | **documented**; HTMX panic test still open (Task 9) |
| Not-found detection | `app/tail.go` substring hack | **fixed** typed `session.IsNotFound` (PR #54) |
| Shared validate drift | `graph/domain/validate.go` asset patterns | **fixed** `validate.AssetID` (PR #55) |
| Test fakes in prod packages | `sessionread/fake.go` | **fixed** → `internal/testutil.SessionReadFake` (pending commit) |

## Boundaries

**Always**

- Loopback bind only (`127.0.0.1`).
- Redact with `shared/redact.Text` before any response, log, or evidence file.
- One planned task = one branch = one PR.

**Ask**

- Whether HTMX partial errors should render a shared fragment template or inline `<p class="empty">` (current SSE/events pattern).
- Whether panic recovery should inject `renderError` via middleware option or stay API-only for non-HTML.

**Never**

- Big-bang replace all `http.Error` in one PR.
- Move business rules into `web/view`; view stays projection-only after I/O moves out.

## Data classification

| Data | Allowed | Not allowed |
|------|---------|-------------|
| Credentials in file preview / chat | Redacted via `redact.Text` | Raw token in HTML partial or log |
| Workspace paths | Loopback UI, user-initiated | Paths in public logs unredacted |
| Session log content | Redacted rows in HTMX partials | Full NDJSON in error bodies |

## Success criteria

1. Zero `http.Error` in the migrated handler groups (`handler_http`, `files`, `workspace`, `catalog`) — **met** (PR #54). Residual `http.NotFound` on a few session routes remains out of scope for this slug.
2. `web/view` has no direct `os.Open` / `os.Stat` for session data — **met** (PR #55).
3. `isNotFoundErr` replaced by typed errors — **met** (PR #54).
4. Panic recovery documented with chosen HTML vs JSON behavior — **partial** (documented PR #55; HTMX panic test open, Task 9).
5. `web/*` imports `shared/redact` directly where only redaction is needed — **met** (PR #55).

## Open questions

1. HTMX partial error fragment: reuse `renderError` subset or standardize on `<p class="empty">` + `HX-Retarget`? **Resolved for now:** `writeHTMXOrError` inline fragment (PR #54).
2. Inject HTML renderer into `middleware.Recover` via functional option on `StandardStack`? **Resolved:** no hook; `RespondError` handles HTMX (PR #55 docs).
3. SSE hub phase 2: worth scheduling in this slug or a later `runtime-sse-hub` slug? **Deferred** (Task 12); see TASKS.md.

</scope>
