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

| Area | Location | Count / note |
|------|----------|--------------|
| `http.Error` in browser handlers | `files.go`, `workspace.go`, `catalog.go`, `handler_http.go` | ~31 call sites |
| View layer filesystem I/O | `web/view/sessions.go`, `pages.go`, `tail.go` (via session replay) | reads logs, stat ambient |
| Redact indirection | `web/view/*`, `web/app/*` import `session.RedactText` | cosmetic; delegates to `shared/redact` |
| Panic recovery UX | `middleware/recover.go` → `httpserver.RespondError` | plain text / JSON, not HTML shell |
| Not-found detection | `app/tail.go` `isNotFoundErr` | substring `"not found"` on `err.Error()` |
| Shared validate drift | `graph/domain/validate.go` asset patterns | separate from run slug in `shared/validate` |

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

1. Zero `http.Error` / bare `http.NotFound` in `internal/web/app` browser handlers (helpers may remain for non-HTMX internal use if documented).
2. `web/view` has no direct `os.Open` / `os.Stat` for session data; reads go through `web/infra` adapters.
3. `isNotFoundErr` replaced by typed errors (`errors.Is`) from infra/view boundary.
4. Panic recovery documented with chosen HTML vs JSON behavior; tests cover HTMX request path.
5. Optional: `web/*` imports `shared/redact` directly where only redaction is needed.

## Open questions

1. HTMX partial error fragment: reuse `renderError` subset or standardize on `<p class="empty">` + `HX-Retarget`?
2. Inject HTML renderer into `middleware.Recover` via functional option on `StandardStack`?
3. SSE hub phase 2: worth scheduling in this slug or a later `runtime-sse-hub` slug?

</scope>
