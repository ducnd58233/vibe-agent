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
    testutil/             test helpers and port fakes (import only from tests)
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
- Default port: **`app.DefaultPort`** (1411). Do not scatter `1411` literals in production code; tests may use the constant.
- State file: `.agent-state/web.json` via `web/infra/persistence`; remove on shutdown.

**Browser-facing HTTP (MUST):**

- Never return **`http.Error`** or bare **`http.NotFound`** to the browser. Use **`renderError`** or redirect with an `error` query param.
- Every handler that writes HTML must set **`Content-Type: text/html; charset=utf-8`** before writing.
- POST handlers that fail should redirect (**303 See Other**) back to a meaningful page with an `error` query parameter, not render an error body inline.

**Secrets and logging (MUST):**

- Redact before any log or file sink: `redact.Text` (or `session.RedactText`, which delegates to it).
- Never log raw panic values (may contain secrets); use `LogPanicRecovered` and log `panic_type` only.

**Run and session state (MUST):**

- Run state lives under **`.agent-state/runs/<date>/<slug>/<version>/manifest.json`**. Do not write it directly; use **`checkpoint`** and **`run`** packages.
- Session logs are append-only NDJSON. Do not truncate or rewrite them.

**Stack-blind quality and checks (MUST):**

These rules come from research slug `improve-inner-outer-loops`. They apply whenever you add or change verifiers, checkplan wiring, graphs, or sandbox/runner code under `runtime/`.

- **Keep the runtime stack-blind.** Graph nodes and Go check keys name quality *roles* (`unit`, `lint`, `expectation_ok`, …), never languages or frameworks. Do not add `if language == …` gate logic in the control plane.
- **Workspace owns commands.** Stack-specific tools live in the consumer (or toolkit) **`vibe-checks.yaml`**. The runtime runs what the checkplan declares; it does not ship a default analyzer per language.
- **Evidence provenance is closed.** Checks use only **`exit_code`**, **`file_assert`**, **`ci_api`**, **`human_event`**. Do not treat a judge-LLM score or model assertion as `Passed` evidence. Do not invent a fifth `--source`.
- **Fail-closed skip.** Optional quality cells (structure, security/deps, coverage, and future matrix rows) that the workspace never declared must **skip**, not invent a tool or pass vacuously with a hardcoded command.
- **Two matrices, do not conflate.** Agent-eval scoreboards (SWE-bench, OpenHands Index, and similar) measure the coding agent. Codebase quality cells measure the consumer repo. Do not wire agent benches into delivery graphs or checkplans.
- **No in-process analyzers or GPU/container sandbox.** Do not embed language-specific static analysis, SWE harness runners, or an in-process container/GPU runtime in Go. Isolation for checks that need it uses the workspace-opted **sandbox runner port** (`.agent-state/sandbox.yaml`, `vibe-agent sandbox`, optional checkplan `runner:`). Embedded container/GPU inside the Go process stays declined (root [`AGENTS.md`](../AGENTS.md)).
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
- Panic recovery uses **`httpserver.RespondError`**, which returns an HTMX HTML fragment when **`HX-Request`** is set and plain text otherwise. No separate panic HTML template; keep panic values out of logs via **`LogPanicRecovered`**.
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
- Benchmarks (optional, not in **`check`**): **`make bench`** from **`runtime/`** runs **`go test -bench=. -benchmem`**. See **Benchmark tests** below.

### Benchmark tests

Benchmarks measure CPU and allocation cost of hot paths. They are **not** regression gates in CI unless a task explicitly adds one.

**Add or extend a benchmark when:**

- The code sits on a **request, hook, or verifier hot path** (for example `redact.Text`, `session.Replay`, graph load, catalog load, slop scan).
- You are **optimizing** a function and need a before/after number, not a gut feel.
- A change **touches allocation-heavy logic** (regex, JSON, tree-sitter, markdown parsing) and you need to confirm you did not regress it.
- The package already has **`bench_test.go`** and your change modifies the timed path; update the bench in the same PR.

**Do not add a benchmark when:**

- The path is **cold** (one-off CLI, install, release) or dominated by **network/disk** without a stable **`testdata`** fixture.
- A **unit test** already covers correctness and the function is trivial (getters, thin wrappers).
- You would benchmark **third-party libraries** instead of our wrapper or call pattern.
- The only goal is to satisfy coverage; use **`go test`**, not **`go test -bench`**.

**Authoring rules (MUST):**

- File name **`bench_test.go`**, functions **`BenchmarkXxx`**, package under test (not a separate `benchmark` package unless the subject is a fixture suite).
- **Setup outside the timed loop**: build fixtures, warm one-shot init (for example gitleaks), read files, with **`b.ResetTimer()`** when setup is inside the benchmark function.
- Prefer **`b.Loop()`** (Go 1.22+) or a classic **`for i := 0; i < b.N; i++`** loop with **no work** outside it.
- Use **`b.ReportAllocs()`** for paths that allocate.
- Split inputs with **`b.Run("case", ...)`** when cost differs materially (clean vs redacted text, short vs long input).
- Repo paths: **`testutil.ToolkitRoot(tb)`** / **`RuntimeRoot(tb)`** from **`internal/testutil`** (`testing.TB` works for tests and benchmarks); do not duplicate module-root walks.
- Keep **`slopaudit/fixture/`** regression fixtures (`TestSeededFixtures...`) separate from Go benchmarks (`BenchmarkAudit...` in **`bench_test.go`**).

**Where benches live today:** `internal/shared/redact`, `internal/shared/markdown`, `internal/graph`, `internal/session`, `internal/web/infra/catalog`, `internal/slopaudit/fixture`.

### Test helpers and fakes (MUST)

Production packages under **`internal/**`** and **`web/**`** must not ship **`Fake`** types or test-only helpers.

| Kind | Location | Example |
|------|----------|---------|
| Paths, port fakes, shared test doubles | **`internal/testutil`** | `testutil.ToolkitRoot(t)`, `testutil.SessionReadFake` |
| Helpers used by one module only | **`<module>/.../*_test.go`** | `web/app` HTTP harness |

Rules:

- **`internal/testutil`** is for tests only: production packages must not import it.
- Fakes implement production **interfaces**; name them **`SessionReadFake`**, not `Fake` in prod packages.
- Do not add **`Fake`** types or test helpers under **`internal/shared/*`** (shared stays shippable infra).

### Web and UI changes (MUST)

Any change under **`runtime/web/`**, **`runtime/internal/web/`**, or **`web/static/`** is not done until browser verification passes.

1. **Run the loopback server:** `vibe-agent web --workspace .` (binds **`127.0.0.1:1411`** only).
2. **Exercise affected flows in a real browser** (Cursor browser MCP, Playwright, or manual). At minimum hit every route or HTMX partial you touched, plus one happy path and one error path when the change affects errors or forms.
   - **No browser automation available?** Drive the running server directly with an HTTP/SSE client (`curl`, or a small Go test) as the minimum bar for wire-level/functional behavior: status codes, headers, streaming (e.g. does an SSE endpoint actually deliver events, not just accept the connection). This is not a substitute for a visual check when the change is CSS/layout-only, but skipping verification entirely because no browser tool is connected is not an option either.
3. **Record evidence** under **`.agent-state/runs/<date>/<slug>/<version>/browser/`** at the workspace root (gitignored). For each session include:
   - `RECORD.md` with date, branch, flows tested, pass/fail, and notes. Note explicitly when the fallback HTTP/SSE client stood in for a browser, and what it did and did not cover.
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
11. **Wrapping `http.ResponseWriter` in a struct silently drops `http.Flusher`** (and similarly `http.Hijacker`, `io.ReaderFrom`) unless the wrapper forwards it explicitly - embedding an interface only promotes that interface's own method set, not extra methods the concrete value underneath happens to have. This broke every SSE request (`stream unsupported`) when the access-log middleware's status-capturing wrapper had no `Flush()` passthrough. Any `http.ResponseWriter` wrapper needs an explicit `Flush() { if f, ok := w.ResponseWriter.(http.Flusher); ok { f.Flush() } }`-shaped method (and the `Hijacker`/`ReaderFrom` equivalents if those matter to the route).
12. **Hardcoding a language analyzer or test runner in Go** "so every repo works." Put the command in **`vibe-checks.yaml`**; keep the graph stack-blind.
13. **Importing SWE-bench / OpenHands Index (or any agent leaderboard) as a delivery check.** Those score agents, not consumer codebases.
14. **Passing an optional quality check by inventing a default tool** when the workspace omitted it. Skip fail-closed instead.
15. **Recording a judge-LLM or model opinion as check evidence.** Only `exit_code` / `file_assert` / `ci_api` / `human_event`.
16. **Embedding a container or GPU sandbox inside the Go process.** Use the external runner port or host/CI only.
</antipatterns>
