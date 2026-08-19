# Runtime development rules

Rules for the Go backend and HTML/CSS/JS frontend under `runtime/`. Every agent editing files here must follow these. For project-wide policy, see the root [`AGENTS.md`](../AGENTS.md).

## Go backend

### Error handling in HTTP handlers

- Never return `http.Error` or `http.NotFound` to the browser. These produce blank pages with no navigation. Use `renderError` (defined in `http.go`) to render the styled error template with a "Back to home" link.
- Every handler that writes HTML must set `Content-Type: text/html; charset=utf-8` before writing.
- POST handlers that fail must redirect (303 See Other) back to a meaningful page with an `error` query parameter, not render an error body inline.

### Context and timeouts

- Do not add fixed timeouts to GenAI host calls. Host response time is unpredictable and varies by model, task complexity, and provider load. Use `context.WithoutCancel` for host print calls.
- Timeout-based status messages (like "Host timed out") are ephemeral and must not be replayed as conversation context. Filter them in `ComposePrefix`.

### Form handling

- When a form submission depends on which button was clicked (multiple submit buttons), use a hidden input to carry the value. Set it via an `onclick` handler on each button before the form submits.
- Never disable submit buttons synchronously in an `onsubmit` handler when the disabled button's `name`/`value` pair is needed by the server. Use `setTimeout(0)` to defer disabling until after the browser captures form data.

### Session and run state

- Run state lives under `tmp/<slug>/manifest.json`. Do not write to it directly; use the checkpoint and run packages.
- Session logs are append-only NDJSON. Do not truncate or rewrite them.

## HTML/CSS frontend

### CSS token usage

- Use design tokens from `tokens.css` for all colors, spacing, radii, and font sizes. Never hard-code hex colors or pixel sizes outside of `tokens.css`.
- When adding a new semantic color (e.g., `--color-success`), define it in `tokens.css` and use the variable everywhere.

### Composer input alignment

- The composer uses a stacked grid: an `<input>` and a `.composer-preview` div occupy the same grid cell (`grid-area: 1 / 1`). The preview renders highlighted text while the input stays transparent.
- Both layers must share identical `padding`, `line-height`, `height`, `font`, and `white-space`. A mismatch between any of these causes text to appear shifted.
- The canonical values are in `.composer-field-wrap .composer-preview, .composer-field-wrap .composer-field`. Do not override these in `.composer .composer-field` without updating the preview rule to match.

### Layout and grid

- Use CSS grid for two-dimensional layouts, flexbox for one-dimensional.
- Do not set `overflow: hidden` on containers that have absolutely-positioned children (dropdowns, tooltips, autocomplete panels) unless those children are portaled outside.
- Test all layout changes at both desktop (`>68.75em`) and mobile (`<48em`) breakpoints. The responsive rules are at the bottom of `shell.css`.

### Dark/light theme

- Both themes share the same layout and spacing. Only color variables differ (defined in `tokens.css` under `[data-theme="light"]`).
- After adding or changing colors, verify both themes render correctly.

### Forms and dialogs

- Slug inputs must validate against the server (`/session/check-slug`) with debounced fetch before allowing submission. This prevents blank screens from duplicate slugs.
- All dialogs use the `<dialog>` element with `.showModal()`. Never use `display: block` to open them.

### JavaScript

- No framework. Vanilla JS only, embedded in templates or in `/static/*.js`.
- Event listeners go on specific elements, not on `document`. Use event delegation only when the target set is dynamic.
- Debounce network requests triggered by user input (300ms minimum).

## Testing

- Go tests use `go test ./...` from the `runtime/` directory.
- CSS/HTML changes must be verified visually via `vibe-agent web` before considering them done. A passing Go build does not prove the UI renders correctly.
- When fixing a UI bug, take a browser screenshot before and after. The "before" confirms the bug exists; the "after" confirms the fix works.

## Common mistakes to avoid

These are patterns that have caused bugs in this codebase. Check for them in every review.

1. **Padding mismatch between input and preview overlay.** The composer input and its preview div must have identical padding. A 0.125rem difference shifts highlighted text visually.
2. **`font: inherit` resets `line-height`.** The CSS `font` shorthand resets `line-height` to `normal`. Always declare `font` BEFORE `line-height` when both appear in the same rule. The shared `.composer-field-wrap` rule must keep this order so both the preview and the field get the correct line-height.
3. **`http.Error` in browser-facing handlers.** Produces a blank page. Use `renderError` or redirect.
4. **Disabling submit buttons before form data is captured.** The browser drops disabled button values from the submission. Defer disabling with `setTimeout(0)`.
5. **Replaying ephemeral status messages as conversation context.** Timeout or error messages logged to the session must be filtered out of `ComposePrefix` to avoid duplicate prompts.
6. **Committing generated files.** Plugin manifests under `.claude-plugin/`, `.cursor-plugin/`, `.codex-plugin/`, and `docs/` are gitignored. If they appear in `git status` as tracked, run `git rm --cached`.
