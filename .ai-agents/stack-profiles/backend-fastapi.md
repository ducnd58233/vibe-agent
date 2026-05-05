# Stack profile: Backend FastAPI

## Scope

Applies to consumer repositories implementing Python HTTP APIs with FastAPI and common ecosystem libraries.

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
