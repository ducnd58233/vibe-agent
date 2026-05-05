# Stack profiles router

Lookup table for **repo-pinned stack** markdown files in this folder. Generic skills and references stay portable; profiles name **frameworks, layout, and tooling** here.

**After you add, rename, or remove a profile `*.md`, update this table in the same change.**

## Composing profiles

Many tasks span several layers:

1. Open this table.
2. Select **every** row whose **When to load** fits the current task (e.g. UI change + HTTP API change + compose two profiles).
3. Read each matching `*.md` file listed in **Profile**.
4. If no row fits, fall back to **manifest + directory scanning** (`package.json`, `pyproject.toml`, `go.mod`, `apps/`, `backend/`, …) until you author a matching profile ([`TEMPLATE.md`](TEMPLATE.md)).

**Authoring:** [`TEMPLATE.md`](TEMPLATE.md).

| Profile | Layer / concern | When to load | Detection / notes |
|---------|-----------------|--------------|-------------------|
| [`frontend-nextjs-ts.md`](frontend-nextjs-ts.md) | Frontend web (Next.js + TypeScript) | UI/routing/component boundary work | `package.json` has `next`; `tsconfig.json`; `app/` or `pages/` present |
| [`backend-fastapi.md`](backend-fastapi.md) | Backend HTTP APIs (Python/FastAPI) | Endpoint/service/validation persistence work | FastAPI in dependency manifests; `uv.lock`/`alembic.ini` may exist |
| [`backend-golang.md`](backend-golang.md) | Backend services (Go) | Go API/service layering and module changes | `go.mod` present; `cmd/` + `internal/` common |
| [`finance-analyzer.md`](finance-analyzer.md) | Finance research and metrics analysis | Public-company, valuation, fundamentals tasks | Mentions 10-K/10-Q/8-K, EDGAR, FRED, market metrics |
| [`finance-advisor.md`](finance-advisor.md) | Advisory-safe finance responses | User requests action-oriented investment guidance | Same as analyzer plus suitability/disclaimer constraints |
| [`datascience.md`](datascience.md) | Data science / ML workflows | Dataset analysis, model training/evaluation, notebooks | Data/ML libs in manifests, notebooks present, `data/`/`models/` hints |
