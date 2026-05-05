# Stack profile: Backend Golang

## Scope

Applies to consumer repositories implementing Go services, APIs, and CLIs with idiomatic project/module boundaries.

## When to load

- Go service/API feature work
- Handler/service/repository layering decisions
- Module, package, and command structure decisions

## Detection

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

## Commands

- `go test ./...`
- `go test -cover ./...`
- `go vet ./...`
- `go build ./...`

## Boundaries

- Avoid leaking transport/request models into domain logic
- Avoid global mutable state; use dependency injection via constructors
- Keep context propagation explicit across boundaries

## References

- https://go.dev/doc/effective_go
- https://github.com/golang-standards/project-layout
- https://cobra.dev
- https://gin-gonic.com
- https://go.dev/ref/mod
