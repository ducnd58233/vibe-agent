# Performance Checklist

Quick reference for web and API performance. Use alongside the [`performance-optimization`](../skills/performance-optimization/SKILL.md) skill.

**Workspace-specific stack notes** for the current project: [`stack-profiles/ROUTER.md`](../stack-profiles/ROUTER.md).

## Core Web Vitals Targets

| Metric | Good | Needs Work | Poor |
|--------|------|------------|------|
| LCP (Largest Contentful Paint) | ≤ 2.5s | ≤ 4.0s | > 4.0s |
| INP (Interaction to Next Paint) | ≤ 200ms | ≤ 500ms | > 500ms |
| CLS (Cumulative Layout Shift) | ≤ 0.1 | ≤ 0.25 | > 0.25 |

## TTFB Diagnosis

When TTFB is slow (> 800ms), check each component in DevTools Network waterfall:

- [ ] **DNS resolution** slow → consider DNS prefetch / preconnect for known origins
- [ ] **TCP/TLS handshake** slow → HTTP/2 or HTTP/3, keep-alive, edge deployment
- [ ] **Server processing** slow → profile handlers, persistence layer, caching

## Frontend Checklist

### Images

- [ ] Responsive images with modern formats where the toolchain supports it
- [ ] Responsive sizing (`sizes`, width/height or equivalent) to reduce CLS
- [ ] Below-the-fold images deferred; prioritize LCP asset per framework guidance

### JavaScript

- [ ] Code splitting / dynamic import for heavy client-only features
- [ ] Avoid shipping large libs on initial route unnecessarily
- [ ] Long tasks (> 50ms) broken up for INP (`scheduler.yield()` where available)
- [ ] Cached server/async state libraries: stable keys, sane stale intervals to avoid refetch storms

### CSS

- [ ] Prefer purged/minimal CSS in production (configure content paths correctly)
- [ ] Prefer CSS variables and minimal runtime CSS-in-JS in hot paths

### Fonts

- [ ] Limit font families/weights; subset and load with non-blocking patterns where supported

### Network

- [ ] Static assets cached with long `max-age` + content hashing
- [ ] API responses: `Cache-Control` where safe (mostly GET, non-user-specific)
- [ ] HTTP/2 or HTTP/3 enabled for production

### Rendering

- [ ] No layout thrashing (batch DOM reads/writes)
- [ ] Long lists virtualized where needed
- [ ] Preserve bfcache eligibility (avoid `unload`, careful `no-store` on HTML)

## Backend Checklist

### Persistence (document drivers and ORMs)

- [ ] No N+1 patterns — batch loads, `IN`-style queries, aggregation when appropriate
- [ ] Indexes on filter/sort fields used in list endpoints
- [ ] List endpoints paginated (cursor or skip/limit with caps); never unbounded full scans by default
- [ ] Projection / field selection — avoid returning oversized documents by default
- [ ] Connection pool sized for deployment; monitor slow query logs

### HTTP API Layer

- [ ] p95 latency targets met for hot routes; async I/O in handlers where applicable
- [ ] Heavy CPU off request thread (worker pool or queue) if needed
- [ ] Response compression at reverse proxy or middleware
- [ ] Bulk operations instead of per-row loops for external calls

### LLM orchestration

- [ ] Tool calls bounded (timeouts, max iterations); streaming where UX needs it
- [ ] Avoid loading huge contexts — summarize or retrieve selectively

### Infrastructure

- [ ] CDN for static assets; API region aligned with persistence when possible
- [ ] Health check endpoint for load balancers

## Measurement Commands

```bash
npx lighthouse https://localhost:3000 --output json --output-path ./report.json
# Bundle: analyze stats if configured in repo
```

Use RUM where available; DevTools Performance for long tasks on interactions.

## Common Anti-Patterns

| Anti-Pattern | Impact | Fix |
|--------------|--------|-----|
| N+1 persistence calls | Linear load growth | Batch, aggregate, reshape in service layer |
| Unbounded scans | Memory, timeouts | Pagination + limits |
| Missing indexes | Slow reads as data grows | Index for query shape |
| Refetch/cache storms | Janky UI, API load | Keys, stale TTL, prefetch discipline |
| Blocking main thread | Poor INP | Yield, split work, workers |

---

Concrete framework and driver reminders for the **current workspace**: authored in profile `*.md` files listed under [`ROUTER.md`](../stack-profiles/ROUTER.md).
