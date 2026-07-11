# 0003. Go Testing Standards

Status: Accepted

## Context

Coverage targets already exist per-component in `codecov.yml`, but nothing
documented *what kind* of test satisfies each layer, so coverage numbers were
sometimes hit with tests that mocked away the exact thing they should have
been proving. This ADR ties the testing approach to the layers defined in
[0001](0001-hexagonal-architecture.md).

## Decision

Coverage targets (enforced by `codecov.yml`, component-scoped):

| Component | Path | Target |
|---|---|---|
| Domain | `internal/domain/**` | 95% |
| Adapters | `internal/adapters/**` | 80% |
| API | `internal/api/**` | 80% |
| CMD | `cmd/**` | 50% |
| Pkg | `pkg/**` | 80% |

Test style per layer:

- **Domain** (`internal/domain/**`): table-driven unit tests, no I/O, no
  network, no filesystem. Pure function/behavior tests. This layer has the
  highest bar (95%) because it has zero excuse for being hard to test — it
  has no external dependencies by construction.
- **Ports**: no test files of their own (they're interfaces). Ports are
  validated indirectly: a fake implementation used across domain/service
  tests, and a contract test (see below) run against every real adapter.
- **Adapters** (`internal/adapters/**`): every adapter implementing a port
  runs the same shared **contract test** for that port — one shared test
  suite defined once, invoked per adapter with that adapter's constructor
  (file convention: `<port>_contract_test.go` next to the port, exercised by
  each adapter package). This is what guarantees Liskov substitutability
  from [0002](0002-solid-design-principles.md).
  Adapter-specific tests beyond the contract test may use recorded
  HTTP fixtures (golden files) rather than live network calls; they must
  not require network access to run in CI.
- **Services**: unit tests against fake port implementations, not real
  adapters. A service test that spins up a real database or makes a real
  HTTP call is testing the wrong layer — that behavior belongs in an
  adapter contract test.
- **API/handlers** (`internal/api/**`): request/response tests against the
  handler with faked services underneath — assert routing, status codes,
  and payload shape, not business logic (business logic is a service/domain
  test's job).
- **CMD** (`cmd/**`): lowest bar (50%) — mostly wiring/composition root. Test
  what's testable (flag parsing, config loading) without demanding coverage
  of `main()` glue that a compile already proves is wired correctly.
- **Pkg** (`pkg/**`): shared infrastructure libraries used across the
  codebase (HTTP client, cache, etc.), held to the same bar as adapters
  (80%) for the same reason — they're the thing everything else depends on.
  Where a `pkg/**` package defines its own port/adapter pair for
  swappability (e.g. a cache interface with an in-memory and, later, a Redis
  implementation), it follows the same shared contract-test convention as
  `internal/adapters/**` above, even though it lives outside `internal/`.

Mocks are only for ports. Never mock a concrete struct — if you need to fake
something that isn't behind a port yet, that's a signal a port is missing
(see [0001](0001-hexagonal-architecture.md)).

## Consequences

- A green coverage number on `internal/domain/**` actually means the business
  logic was exercised, not that a mock made the number go up.
- Writing a new adapter requires writing (or reusing) that port's contract
  test — this is the mechanical check for Liskov substitution, not just a
  code-review reminder.
- Slower to write the first adapter for a new port (contract test has to be
  designed well); every adapter after that is cheap.

## Self-Audit Checklist

1. Does any domain test touch the network, filesystem, or a real database?
   If yes — it's misplaced or the domain layer has a leaked dependency.
2. Does every adapter for a given port run that port's shared contract test?
   If a new adapter skips it — add it before merging.
3. Does any service test construct a real adapter instead of a fake port
   implementation? If yes — fix the test, not the coverage number.
4. Are coverage targets in `codecov.yml` still matched to the actual package
   layout (`internal/domain`, `internal/adapters`, `internal/api`, `cmd`,
   `pkg`)? If the layout changed and the config didn't, fix the config.
