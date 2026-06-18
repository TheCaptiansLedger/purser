# Testing Standards

## Coverage Targets

| Layer | Target |
|---|---|
| Core domain (`internal/domain`) | 95% unit test coverage |
| Adapters (`internal/adapters`) | 80% coverage |
| Aggregation/glue (`main.go`, router setup) | 50% coverage |

## Package Selection

Prefer `package name_test` (external test package) for unit tests. This forces testing of the public API/interface rather than internal structure. Use the same package name only when absolutely necessary to verify internal logic.

## Backend Rules

- Unit tests alongside code (`_test.go` files)
- App service tests use hand-rolled mock port implementations — no mockgen
- Adapter tests use real databases (SQLite in-memory; testcontainers for PostgreSQL) — never mock the database
- HTTP handler tests use `httptest` with real app services + in-memory SQLite

## Frontend Rules

The frontend must include unit tests. No shipping frontend code without test coverage.

## Linting

Before every commit:
- `go test ./...` must pass
- `golangci-lint run` must pass with zero issues
- The codebase must compile
