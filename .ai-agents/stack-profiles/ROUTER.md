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
| *— Add rows here as you author `*.md` files.* | | | For example later: `frontend-nextjs.md`, `backend-fastapi.md`, `devops-docker.md`. |
