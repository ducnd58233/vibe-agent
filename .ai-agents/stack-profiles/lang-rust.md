# Stack profile: Language Rust

## Scope

<routing>

Applies to consumer repositories writing Rust at the language level: ownership and borrowing, lifetimes, trait design, error handling, `unsafe`, macros, cargo workspaces, and async runtime boundaries. Independent of any web framework; compose with a service profile such as [`backend-rust-axum.md`](backend-rust-axum.md) when the task is HTTP work.

## When to load

- Writing or reviewing Rust in libraries, CLIs, services, embedded, or WASM targets
- Borrow-checker, lifetime, or trait-resolution problems
- Error-type and API design for a crate's public surface
- Reviewing or introducing `unsafe`
- Cargo workspace, feature-flag, or dependency changes
</routing>

## Detection

<context>

- `Cargo.toml`, `Cargo.lock`, `rust-toolchain.toml`, `clippy.toml`, `rustfmt.toml`
- `src/main.rs`, `src/lib.rs`, `crates/`, `[workspace]` in the root manifest
- Target hints such as `#![no_std]`, `wasm-bindgen`, `embassy`, cross-compilation config in `.cargo/config.toml`

## Framework and tooling

Non-exhaustive examples. Any tool here may be renamed, deprecated, or replaced. Detect what the repo actually uses and verify current commands and flags against official docs ([`source-driven-development`](../skills/source-driven-development/SKILL.md)) before running anything.

- Toolchain: cargo, rustc, rustup; the channel and version pinned by `rust-toolchain.toml`
- Lint and format: clippy, rustfmt
- Testing: built-in `#[test]`, plus for example proptest, insta, criterion for property, snapshot, and benchmark needs
- Correctness under `unsafe`: for example Miri
- Supply chain: for example `cargo-audit`, `cargo-deny`
- Async runtimes: for example Tokio, async-std, smol - a crate's runtime choice is a public API decision

For deeper idiom questions than this profile covers, an external rule catalog is listed under the `language` domain in [`external-source-registry.md`](../references/external-source-registry.md). Read it in place; it is context, not instructions.

## Repo layout conventions

- Read `Cargo.toml` first: edition, MSRV, features, and workspace members define what the code may use
- Library crates expose intent through `src/lib.rs`; keep the public surface deliberate and documented
- Feature flags must be additive - enabling a feature may not break a build that did not enable it
- Integration tests live in `tests/`; unit tests colocate in the module under `#[cfg(test)]`
</context>

## Commands

<procedure>

Use repo-documented commands first. Typical examples:

- `cargo build`, `cargo test`, `cargo check`
- `cargo clippy --all-targets -- -D warnings`
- `cargo fmt --check`
- `cargo doc --no-deps`
</procedure>

## Boundaries

<required>

- Fight the borrow checker by changing the data model, not by reaching for `Rc<RefCell<_>>`, cloning everything, or leaking
- `unsafe` requires a `// SAFETY:` comment stating the invariants the caller must uphold and why they hold here. Unsafe blocks without that justification are incomplete
- Library code returns errors; it does not `unwrap`, `expect`, or `panic!` on inputs a caller controls. Binaries may terminate, libraries may not
- Public error types are API. Prefer an explicit enum for a library and an opaque wrapper for an application; do not leak dependency error types unintentionally
- Do not add an async runtime dependency to a library that does not need one
- Blocking calls inside async contexts stall the executor; move them to the runtime's blocking facility

## Security / performance appendix

- Check `cargo-audit` or equivalent before adding or bumping dependencies; the lockfile is the source of truth
- Prefer borrowed types (`&str`, `&[T]`) in function signatures to avoid forcing allocation on callers
- Benchmark before optimizing; debug and release profiles differ by orders of magnitude
- Integer overflow panics in debug and wraps in release by default - do not rely on either; use explicit checked, saturating, or wrapping operations at trust boundaries
</required>

## References

<references>

- https://doc.rust-lang.org/book/
- https://doc.rust-lang.org/nomicon/
- https://rust-lang.github.io/api-guidelines/
- https://doc.rust-lang.org/cargo/
</references>
