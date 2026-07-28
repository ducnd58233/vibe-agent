# Stack profile: Language Python

## Scope

Applies to consumer repositories writing Python at the language level: typing, the async and concurrency model, packaging and environments, imports, and data-model semantics. Independent of any framework; compose with [`backend-fastapi.md`](backend-fastapi.md), [`datascience.md`](datascience.md), or [`mlops.md`](mlops.md) when the task is framework or pipeline work.

## When to load

- Writing or reviewing Python in libraries, services, CLIs, scripts, or notebooks
- Type annotation, generics, protocol, or type-checker configuration work
- Packaging, dependency resolution, virtual environments, or lockfile changes
- `asyncio` design, blocking-call problems, threading, or multiprocessing decisions
- Import structure, circular imports, or namespace package issues

## Detection

- `pyproject.toml`, `setup.py`, `setup.cfg`, `requirements*.txt`
- Lockfiles such as `uv.lock`, `poetry.lock`, `Pipfile.lock`, `pdm.lock`
- `.python-version`, `tox.ini`, `noxfile.py`, `mypy.ini`, `ruff.toml`, `.pre-commit-config.yaml`
- `src/` layout or a top-level package directory with `__init__.py`

## Framework and tooling

Non-exhaustive examples. Any tool here may be renamed, deprecated, or replaced. Detect what the repo actually uses from its manifests and lockfile, and verify current commands and flags against official docs ([`source-driven-development`](../skills/source-driven-development/SKILL.md)) before running anything.

- Environment and dependencies: for example uv, Poetry, PDM, pip with venv, conda
- Type checking: for example mypy, Pyright, ty
- Lint and format: for example Ruff, Black, isort, flake8
- Test: for example pytest, unittest, Hypothesis for property tests
- Profiling and debugging: built-in `cProfile`, `tracemalloc`, `pdb`; for example py-spy, memray

## Repo layout conventions

- Read `pyproject.toml` first: it declares the Python version floor, dependencies, entry points, and tool configuration
- Prefer the `src/` layout when present; it prevents accidentally importing the working directory instead of the installed package
- Never install into the system interpreter; use the environment the repo defines
- Keep the package's public surface explicit; a leading underscore marks internal, and `__all__` documents intent

## Commands

Use repo-documented commands first. Typical examples:

- Sync the environment with the tool matching the committed lockfile
- Test: `pytest`
- Typecheck and lint: the repo's configured type checker and linter

## Boundaries

- Mutable default arguments are evaluated once at definition. Use `None` and construct inside the function
- Type hints are not enforced at runtime. Data crossing a trust boundary must be validated, not merely annotated
- In `asyncio`, any blocking call — synchronous I/O, CPU-bound work, a non-async database driver — stalls the entire event loop. Move it to a thread or process executor
- Do not mix async and sync database or HTTP clients in the same request path without an explicit executor boundary
- The GIL means threads help I/O-bound work and not CPU-bound work; reach for processes when the work is CPU-bound
- Catch specific exceptions. A bare `except:` swallows `KeyboardInterrupt` and `SystemExit`
- Do not restructure imports to break a cycle without checking the dependency direction; a cycle usually signals a layering problem
- Prefer explicit dependency injection over import-time side effects; module-level work runs on first import, in an order you do not control

## Security / performance appendix

- Never `eval`, `exec`, or `pickle.loads` untrusted input; pickle executes arbitrary code by design
- Use parameterized queries; string-formatted SQL is injectable regardless of the driver
- Never interpolate untrusted input into a shell command; pass an argument list and avoid `shell=True`
- Bound external calls with explicit timeouts
- Profile before optimizing; the cost is usually I/O, serialization, or an accidental O(n²), not the interpreter

## References

- https://docs.python.org/3/
- https://typing.python.org/en/latest/
- https://packaging.python.org/en/latest/
- https://docs.python.org/3/library/asyncio.html
