# Stack profile: Backend Golang

## Scope

<routing>

Applies to consumer repositories implementing Go services, APIs, and CLIs with idiomatic project/module boundaries.

## When to load

- Go service/API feature work
- Handler/service/repository layering decisions
- Module, package, and command structure decisions
</routing>

## Detection

<context>

- `go.mod` exists
- `cmd/` and `internal/` directories present (common signal)
- Optional framework signals (`gin`, `chi`, `cobra`, `gorm`, `pgx`, `sqlc`)

## Framework and tooling

- Go modules
- Optional: Cobra, Gin/Chi/Echo, Gorm/pgx/sqlc

## Repo layout conventions

- Read `README.md`, `go.mod`, and key entrypoints (`cmd/*/main.go`) first
- Keep handlers thin, business logic in services/use-cases
- Keep storage adapters behind interfaces
</context>

## Commands

<procedure>

- `go test ./...`
- `go test -cover ./...`
- `go vet ./...`
- `go build ./...`

## Scaffolding & command surface (CLI-first)

Initialize and add deps via the official toolchain; do not hand-write `go.mod` or module layout from memory ([`source-driven-development`](../skills/source-driven-development/SKILL.md)):

- Init: `go mod init <module-path>`; deps: `go get <pkg>`; tidy: `go mod tidy`
- Migrations: use the repo's documented tool (for example `golang-migrate`, `goose`, `atlas`); confirm commands from its docs.

Provide a root **`Makefile`** wiring these targets: `docker-up`/`docker-down` (compose deps), `run`, `build`, `test`, `lint` (`go vet` + `golangci-lint`), and `migrate-new name=<x>` / `migrate-up` / `migrate-down`.
</procedure>

## Boundaries

<required>

- Avoid leaking transport/request models into domain logic
- Avoid global mutable state; use dependency injection via constructors
- Keep context propagation explicit across boundaries
</required>

## References

<references>

- https://go.dev/doc/effective_go
- https://github.com/golang-standards/project-layout
- https://cobra.dev
- https://gin-gonic.com
- https://go.dev/ref/mod
</references>
