# Stack profile: Backend Rust Axum

## Scope

<routing>

Applies to consumer repositories implementing Rust HTTP APIs and services with Axum on Tokio, including routers, extractors, middleware layers, async persistence clients, and observability.

Compose with [`lang-rust.md`](lang-rust.md) for language-level concerns: ownership and lifetimes, error-type design, `unsafe`, cargo workspaces and features.

## When to load

- Rust API or service work using Axum
- Router, extractor, middleware, or handler boundary decisions
- Tokio task, cancellation, timeout, or async I/O decisions in an Axum service
- SQLx, SeaORM, Diesel, Redis, NATS, Kafka, or outbound HTTP integration inside Rust services
</routing>

## Detection

<context>

- `Cargo.toml` exists with `axum`
- `tokio`, `tower`, `tower-http`, `tracing`, `serde`, or `sqlx` dependencies
- Source paths such as `src/main.rs`, `src/lib.rs`, `src/routes/`, `src/handlers/`, `src/services/`, `src/repositories/`

## Framework and tooling

- Rust stable toolchain and Cargo workspaces
- Axum for HTTP routing, extractors, responses, and WebSocket extraction
- Tokio for async runtime, tasks, timers, channels, and non-blocking I/O
- Tower / Tower HTTP for middleware layers such as trace, timeout, compression, CORS, and request limits
- Serde for transport DTOs; SQLx / SeaORM / Diesel depending on manifest
- Tracing / tracing-subscriber for structured spans and request correlation

## Repo layout conventions

- Read `Cargo.toml`, `Cargo.lock`, `rust-toolchain*`, `README.md`, and service entrypoints first
- Keep handlers thin: extract request state, validate/deserialize, call one service use case, map response/error
- Keep business orchestration in service modules; keep database calls in repositories or adapters
- Prefer typed app state (`Router::with_state`) and constructor-injected dependencies over globals
- Keep error mapping explicit via a shared application error type implementing `IntoResponse`
</context>

## Commands

<procedure>

- `cargo fmt --all --check`
- `cargo clippy --all-targets --all-features -- -D warnings`
- `cargo test --all`
- `cargo check --all-targets --all-features`

## Scaffolding & command surface (CLI-first)

Initialize and add deps via Cargo; do not hand-write `Cargo.toml` or crate layout from memory ([`source-driven-development`](../skills/source-driven-development/SKILL.md)):

- Init: `cargo new <name>` (or `cargo init` in an existing dir); deps: `cargo add axum tokio --features tokio/full` (adjust to docs)
- Migrations (SQLx example): `sqlx migrate add <name>`, `sqlx migrate run`, `sqlx migrate revert`, or the repo's chosen tool (SeaORM/Diesel); confirm from its docs.

Provide a root **`Makefile`** wiring these targets: `docker-up`/`docker-down` (compose deps), `run` (`cargo run`), `build` (`cargo build --release`), `test`, `lint` (`cargo fmt --check` + `clippy`), and `migrate-new name=<x>` / `migrate-up` / `migrate-down`.
</procedure>

## Boundaries

<required>

- Do not hold mutex guards, database transactions, or borrowed request data across unrelated `.await` points
- Do not call blocking filesystem, CPU-heavy, or sync database work on Tokio worker threads; isolate with `spawn_blocking`, a bounded worker pool, or a dedicated service
- Do not leak Axum extractor types or persistence row types into domain logic
- Keep middleware concerns in Tower layers; keep business authorization decisions in services or policy modules
- Propagate cancellation and deadlines rather than spawning detached background tasks without ownership and shutdown paths

## Security / performance appendix

- Add request body limits, timeouts, CORS, compression, and tracing layers deliberately
- Use bounded channels for fan-out and backpressure; never let per-connection buffers grow unbounded
- Instrument hot handlers with spans and latency/error metrics before optimizing
- For WebSockets, split read/write loops, handle close frames, heartbeat/liveness, and slow-consumer eviction
</required>

## References

<references>

- https://docs.rs/axum/latest/axum/
- https://docs.rs/axum/latest/axum/extract/ws/
- https://tokio.rs/tokio/tutorial
- https://tokio.rs/tokio/tutorial/spawning
- https://tokio.rs/tokio/tutorial/shared-state
- https://tokio.rs/tokio/tutorial/channels
- https://docs.rs/tower-http/latest/tower_http/
- https://docs.rs/tracing/latest/tracing/
</references>
