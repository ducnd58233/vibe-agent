# Stack profile: Backend FastAPI

## Scope

Applies to consumer repositories implementing Python HTTP APIs with FastAPI and common ecosystem libraries.

Compose with [`lang-python.md`](lang-python.md) for language-level concerns: typing, asyncio and blocking-call boundaries, packaging and environments.

## When to load

- New or changed FastAPI endpoints
- Dependency injection, validation, persistence, and async I/O boundaries
- API service layering and error mapping decisions

## Detection

- `pyproject.toml` or `requirements*.txt` includes `fastapi`
- Optional: `uv.lock`, `alembic.ini`, `app/` or `src/` API modules

## Framework and tooling

- FastAPI
- Pydantic v2
- SQLAlchemy 2.0 async patterns
- Optional: Alembic, uv

## Repo layout conventions

- Read `README.md`, dependency manifests, and env examples first
- Keep route handlers thin; move business logic to services/use-cases
- Keep persistence details behind repository interfaces

## Commands

- `uv run pytest`
- `uv run ruff check .`
- `uv run mypy .`
- `uv run python -m app.main` (or project-specific run command)

## Scaffolding & command surface (CLI-first)

Initialize and add deps via `uv`; do not hand-write `pyproject.toml`/`uv.lock` or app layout from memory ([`source-driven-development`](../skills/source-driven-development/SKILL.md)):

- Init: `uv init`; deps: `uv add fastapi[standard] sqlalchemy alembic` (adjust to docs); dev deps: `uv add --dev pytest ruff mypy`
- Migrations (Alembic): `alembic init`, `alembic revision --autogenerate -m "<x>"`, `alembic upgrade head`, `alembic downgrade -1`; confirm against current docs.

Provide a root **`Makefile`** wiring these targets: `docker-up`/`docker-down` (compose deps), `run`, `build`, `test`, `lint` (`ruff` + `mypy`), and `migrate-new name=<x>` / `migrate-up` / `migrate-down`.

## Boundaries

- Validate inbound/outbound DTOs at API boundary
- Keep domain/application code framework-agnostic
- Avoid ORM model leakage into transport contracts

## References

- https://fastapi.tiangolo.com
- https://docs.pydantic.dev
- https://docs.sqlalchemy.org/en/20/
- https://docs.astral.sh/uv/
- https://github.com/zhanymkanov/fastapi-best-practices
