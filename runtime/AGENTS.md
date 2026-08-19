# Runtime development rules

Rules for the Go control plane and loopback web UI under `runtime/`. For project-wide policy, see the root [`AGENTS.md`](../AGENTS.md).

When your task touches files here, read this file before editing. Root charter still applies; this file adds runtime-specific layout, boundaries, and UI rules.

<context>

## How harnesses load these rules

| Harness | Loads `runtime/AGENTS.md`? | Mechanism |
|---------|---------------------------|-----------|
| **Cursor** | Yes, when editing `runtime/**` | [`.cursor/rules/001-runtime-rules.mdc`](../.cursor/rules/001-runtime-rules.mdc) (`globs: runtime/**`) points here |
| **Claude Code** | Indirectly | [`runtime/CLAUDE.md`](CLAUDE.md) points here; subdirectory `CLAUDE.md` loads **on demand** when Claude **reads** a file under `runtime/` (not at session start). Run `/context` to confirm |
| **Codex** | No (automatic) | [`.codex/config.toml`](../.codex/config.toml) sets `model_instructions_file = "../AGENTS.md"` only. Root [`AGENTS.md`](../AGENTS.md) links here for runtime work; read this file yourself |
| **opencode** | Yes (when listed) | [`opencode.json`](../opencode.json) `instructions` includes `runtime/AGENTS.md` alongside root `AGENTS.md` |

Nested `AGENTS.md` does **not** replace root `AGENTS.md`. Both apply; runtime rules win for paths under `runtime/` when they add detail.

## Module layout and boundaries

```
runtime/
  cmd/                    one file per CLI command; main.go is dispatch only
  internal/
    shared/               cross-cutting infra (no domain logic)
      workspace/          path constants (.agent-state, tmp)
      redact/             credential-shaped text replacement
      markdown/           markdown table and link parsing for routers
      validate/           shared format checks (slug, etc.)
      observability/      structured logging port + dual-sink logger
      infra/
        database/         SQLite open helper (driver registration)
        httpserver/       Serve, JSON/errors, request context
          middleware/     Chain, RequestID, AccessLog, Recover, StandardStack
        streaming/
          sse/            SSE headers, frames, poll loop (stdlib only)
    web/                  loopback UI
      app/                composition root (bootstrap, routes, handlers)
      domain/             registry, path rules (no I/O)
      infra/              catalog parsers, persistence, host picker
    run/, graph/, memory/, harness/, mcp/, ...   other control-plane modules
```

**Ports:** use small interfaces at boundaries (`observability.Logger`, repository interfaces in each module). Handlers stay thin; business rules stay in domain or application packages.
</context>

<required>

**Dependency direction (MUST):**

- `internal/shared/*` depends on **nothing** under `internal/web`, `internal/memory`, `internal/harness`, etc.
- Domain modules (`web`, `memory`, `run`, ...) may import `shared`; never the reverse.
- `web/app` is the web composition root: wires shared middleware, domain, and infra adapters.
- Do not import concrete types across domain modules (e.g. `web` must not reach into `memory` persistence). Shared path names live in `shared/workspace` only.

**Loopback web server (MUST):**

- Bind **`127.0.0.1` only** (`app.ListenHost`). Refuse `0.0.0.0` and non-loopback hosts.
- Default port: **`app.DefaultPort`** (3080). Do not scatter `3080` literals in production code; tests may use the constant.
- State file: `.agent-state/web.json` via `web/infra/persistence`; remove on shutdown.

**Browser-facing HTTP (MUST):**

- Never return **`http.Error`** or bare **`http.NotFound`** to the browser. Use **`renderError`** or redirect with an `error` query param.
- Every handler that writes HTML must set **`Content-Type: text/html; charset=utf-8`** before writing.
- POST handlers that fail should redirect (**303 See Other**) back to a meaningful page with an `error` query parameter, not render an error body inline.

**Secrets and logging (MUST):**

- Redact before any log or file sink: `redact.Text` (or `session.RedactText`, which delegates to it).
- Never log raw panic values (may contain secrets); use `LogPanicRecovered` and log `panic_type` only.

**Run and session state (MUST):**

- Run state lives under **`tmp/<slug>/manifest.json`**. Do not write it directly; use **`checkpoint`** and **`run`** packages.
- Session logs are append-only NDJSON. Do not truncate or rewrite them.
</required>

<rules>

## Shared infrastructure

### Logging (`internal/shared/observability`)

- Long-running commands (`web`, `hook`, `mcp serve`) use dual-sink slog: tinted console + JSON file.
- Log directory defaults to a sibling `logs/` next to the install `bin/` (see `ResolveLogDir`). Override with `VIBE_LOG_DIR`; level with `VIBE_LOG_LEVEL` (`observability.EnvLogDir`, `EnvLogLevel`, `DefaultLogLevel`).
- Use `observability.LogError` / `LogErrorContext` for errors; `LogPanicRecovered` for panics.

### HTTP server (`internal/shared/infra/httpserver`)

- **`httpserver.Serve(ctx, addr, handler, logger)`** owns listen + graceful shutdown. Callers cancel `ctx` on SIGINT/SIGTERM; do not reimplement shutdown loops in feature code.
- Wrap handlers with **`middleware.StandardStack(h, log)`** (RequestID, AccessLog, Recover). Do not duplicate middleware wiring.
- JSON responses: **`httpserver.JSON`**. API errors: **`httpserver.RespondError`** (JSON when `Accept: application/json`; plain text for HTMX via `HX-Request`).
- HTML browser errors: **`renderError`** in `web/app/http.go`, not `http.Error`.

### SSE streaming (`internal/shared/infra/streaming/sse`)

- Use **`sse.Begin`** for headers and flush setup; **`sse.Poll`** for ticker-driven server push; **`sse.WriteEvent`** / **`sse.Event`** for wire format.
- Domain handlers (`web/app/sse.go`) own slug parsing, templates, and event production; shared package owns transport only.
- Set **`X-Accel-Buffering: no`** via `sse.Begin` when a proxy sits in front (harmless on loopback).

### Database (`internal/shared/infra/database`)

- Use **`database.Open`** for SQLite so driver import stays in one place. Schema and migrations remain in each module's `infra/persistence`.

## Go backend

### HTTP handlers (HTML UI)

- Use helpers in **`handler_http.go`**: `requireMethod`, `writeBadForm`, `writeTemplateError`, `writeBadAfter`. Keep user-visible message strings in that file's constants.
- HTMX partial errors: **`writeHTMXFragment`** / **`writeHTMXOrError`** in **`htmx.go`**. Escape user-visible text; never return raw paths or tool output in fragments.

### Context and timeouts

- Do not add fixed timeouts to GenAI host calls. Host response time varies by model and load. Use **`context.WithoutCancel`** for host print calls.
- Timeout-based status messages (like "Host timed out") are ephemeral and must not be replayed as conversation context. Filter them in **`ComposePrefix`**.

### Form handling

- When a form depends on which button was clicked, use a hidden input set via **`onclick`** on each button before submit.
- Never disable submit buttons synchronously in **`onsubmit`** when the button's `name`/`value` is needed server-side. Use **`setTimeout(0)`** to defer disabling.

### Commands and verification

- One file per command under **`cmd/`**; shared flags in **`cmd/common.go`**.

## HTML/CSS frontend

### CSS token usage

- Use design tokens from **`web/static/tokens.css`** for colors, spacing, radii, and font sizes. Never hard-code hex colors or pixel sizes outside tokens.
- New semantic colors: define in **`tokens.css`**, use the variable everywhere.

### Composer input alignment

- Stacked grid: **`<input>`** and **`.composer-preview`** share one grid cell (`grid-area: 1 / 1`).
- Both layers must share identical **`padding`**, **`line-height`**, **`height`**, **`font`**, and **`white-space`**. Canonical rules: **`.composer-field-wrap .composer-preview, .composer-field-wrap .composer-field`**.

### Layout and grid

- CSS grid for 2D layouts; flexbox for 1D.
- Do not set **`overflow: hidden`** on containers with absolutely positioned dropdowns/tooltips unless portaled outside.
- Test at desktop (`>68.75em`) and mobile (`<48em`) per **`shell.css`**.

### Dark/light theme

- Layout and spacing match across themes; only color variables differ under **`[data-theme="light"]`**.

### Forms and dialogs

- Slug inputs: validate via **`/session/check-slug`** (debounced) before submit.
- Dialogs: **`<dialog>`** + **`.showModal()`**, not `display: block`.

### JavaScript

- Vanilla JS only in templates or **`web/static/*.js`**.
- Listeners on specific elements; delegate only when targets are dynamic.
- Debounce input-triggered network calls (300ms minimum).
</rules>

<verification>

## Testing

- Go: **`go test ./...`** or **`make check`** from **`runtime/`**.
- After changing runtime code: **`make check`** from **`runtime/`** (gofmt, vet, golangci-lint, tests, e2e).
- Reinstall for hook testing: **`make install`** then **`vibe-agent doctor`**. Passing `go test` does not update the binary on PATH.

### Web and UI changes (MUST)

Any change under **`runtime/web/`**, **`runtime/internal/web/`**, or **`web/static/`** is not done until browser verification passes.

1. **Run the loopback server:** `vibe-agent web --workspace .` (binds **`127.0.0.1:3080`** only).
2. **Exercise affected flows in a real browser** (Cursor browser MCP, Playwright, or manual). At minimum hit every route or HTMX partial you touched, plus one happy path and one error path when the change affects errors or forms.
3. **Record evidence** under **`tmp/<slug>/browser/`** at the workspace root (gitignored). For each session include:
   - `RECORD.md` with date, branch, flows tested, pass/fail, and notes.
   - Screenshots or short notes for before/after when the change is visual.
   - Redact before write; no credentials or full file contents in evidence.
4. **Do not skip browser checks** because unit tests passed. HTMX partials, SSE, composer alignment, and dialog behavior are not fully covered by `go test`.
5. **`/test` and `/review`** on web tasks must cite the browser evidence path. **`/ship`** treats missing browser evidence on a web/UI task as **NO-GO**.

CSS/HTML-only tweaks: verify at desktop (`>68.75em`) and mobile (`<48em`) per **`shell.css`**. UI bugfixes: screenshot before and after in the browser.
</verification>

<antipatterns>

## Common mistakes to avoid

1. **Padding mismatch** between composer input and preview overlay.
2. **`font: inherit` resets `line-height`**. Declare **`font`** before **`line-height`** in shared composer rules.
3. **`http.Error` in browser-facing handlers.** Use **`renderError`** or redirect.
4. **Disabling submit buttons before form data is captured.**
5. **Replaying ephemeral status messages** in **`ComposePrefix`**.
6. **`shared` importing domain packages** (e.g. `session` in middleware). Use **`shared/redact`** instead.
7. **Logging panic values or unredacted paths/tokens.** Use **`LogPanicRecovered`** and **`redact.Text`**.
8. **Duplicate middleware or shutdown logic** outside **`httpserver.Serve`** / **`StandardStack`**.
9. **Magic port/timeouts** instead of **`DefaultPort`**, **`readHeaderTimeout`**, **`shutdownTimeout`** in shared httpserver.
10. **Committing generated plugin manifests** under **`.claude-plugin/`**, **`.cursor-plugin/`**, etc. If tracked by mistake, **`git rm --cached`**.
</antipatterns>
