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
- Adapter tests use the real backend for that adapter — never mock the storage layer:
  - BadgerDB adapter: open with `WithInMemory(true)`
  - SQLite adapter: open an in-memory DSN (`:memory:`)
  - PostgreSQL adapter: testcontainers
- HTTP handler tests use `httptest` with real app services wired to an in-memory BadgerDB instance

## Adapter Tests — HTTP Metadata Sources

Any adapter that makes off-box HTTP calls (metadata sources, indexers, download clients) requires **two layers** of testing. Both layers must exist before a PR is merged.

### Layer 1 — Unit Tests (httptest mocks)

**Never make real network calls in `go test ./...`.**  Every HTTP adapter must intercept calls via `net/http/httptest`.

Conventions:
- JSON response fixtures as `const` strings at the top of the relevant test file
- One `_test.go` file per logical method group (e.g., `scenes_test.go`, `studios_test.go`, `people_test.go`)
- A `newTestAdapter(srv *httptest.Server) *pkg.Adapter` constructor helper per file or in a shared `adapter_test.go`
- Cover: happy path field mapping, `ErrNotFound`, `ErrNotSupported` stubs, HTTP non-200 propagation, invalid JSON body
- Use `package foo_test` (black-box); access internals only when necessary

### Layer 2 — Integration Tests (real API calls)

Integration tests run against the live external service. They are **excluded from the default `go test ./...` run** via a build tag and are run manually or in a dedicated CI job that has credentials available.

Conventions:
- Filename: `adapter_integration_test.go` inside the adapter package
- First non-blank, non-comment line: `//go:build integration`
- Skip immediately when the required credential env var is absent:

  ```go
  //go:build integration

  package stashdb_test

  import (
      "os"
      "testing"
      ...
  )

  func TestIntegration_SearchStudios(t *testing.T) {
      apiKey := os.Getenv("PURSER_SOURCES_STASHDB_API_KEY")
      if apiKey == "" {
          t.Skip("PURSER_SOURCES_STASHDB_API_KEY not set")
      }
      // ... real call
  }
  ```

- Use a timeout context — never block forever on a flaky network:

  ```go
  ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
  defer cancel()
  ```

- **Read-only calls only** — never mutate external state
- Test one representative call per method group (not exhaustive fixture coverage); the goal is to catch field-mapping bugs that fixtures cannot
- Assert only what the API guarantees (e.g., non-empty title, valid ExternalID); avoid asserting on volatile data (e.g., scene counts)

Run integration tests for a single adapter:

```bash
PURSER_SOURCES_STASHDB_API_KEY=xxx go test -tags integration -timeout 120s ./internal/adapters/stashdb/
```

Run all integration tests:

```bash
go test -tags integration -timeout 300s ./internal/adapters/...
```

### Credential env vars per adapter

| Adapter | Env Var | Notes |
|---|---|---|
| `stashdb` | `PURSER_SOURCES_STASHDB_API_KEY` | |
| `tpdb` | `PURSER_SOURCES_TPDB_API_KEY` | |
| `tmdb` | `PURSER_SOURCES_TMDB_API_KEY` | Bearer token (API Read Access Token) |
| `tvdb` | `PURSER_SOURCES_TVDB_API_KEY` | API key; adapter exchanges for JWT |
| `mbz` | _(none)_ | Public API; integration tests always run |
| `fanart` | `PURSER_SOURCES_FANART_API_KEY` | |
| `lastfm` | `PURSER_SOURCES_LASTFM_API_KEY` | |
| `theaudiodb` | `PURSER_SOURCES_THEAUDIODB_API_KEY` | Free tier uses key `123` |

## k6 API Tests

End-to-end API tests live in `tests/k6/`. They require a running server (`make up`) and k6 installed (`brew install k6`).

```
tests/k6/
  config.js              # shared base URL, options, and helpers
  endpoints/             # one file per endpoint group — verify each route works correctly
  flows/                 # multi-step scenarios — verify complete user-facing workflows
```

### Running

```bash
# Single endpoint group
k6 run tests/k6/endpoints/music-releases.js
k6 run tests/k6/endpoints/music-queue.js

# Complete flow
k6 run tests/k6/flows/music-import-flow.js

# All endpoint tests
k6 run tests/k6/endpoints/*.js

# Override the base URL (e.g. staging)
BASE_URL=http://staging:7474/api/v1 k6 run tests/k6/flows/music-import-flow.js
```

### Load testing

The same scripts double as load tests — pass `--vus` and `--duration` to override the default single-iteration run:

```bash
k6 run --vus 20 --duration 30s tests/k6/endpoints/music-releases.js
k6 run --vus 10 --duration 60s tests/k6/flows/music-import-flow.js
```

### Conventions

- `config.js` sets `vus: 1, iterations: 1` so the default run is a functional smoke test.
- Each endpoint file tests one logical group of routes (create, read, patch, delete, list).
- Each flow file tests a complete user-facing scenario using `group()` blocks.
- When a new endpoint is added, add or update the corresponding file in `endpoints/`. When a new user flow is implemented, add a file in `flows/`.

## Frontend Rules

The frontend must include unit tests. No shipping frontend code without test coverage.

## Linting

Before every commit:
- `go test ./...` must pass
- `golangci-lint run` must pass with zero issues
- The codebase must compile
