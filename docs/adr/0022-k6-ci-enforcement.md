# 0022. k6 CI Enforcement: Failing Thresholds, a Standalone-Badger CI Job

Status: Accepted

## Context

`test/k6/grpc/afterdark_browse_test.js` and `test/k6/http/afterdark_browse_test.js`
broke silently after [0020](0020-server-generated-kernel-entity-ids.md)
made every kernel `Create` RPC discard a caller-supplied `id` — both
scripts still threaded a pre-generated client-side id through every
downstream call, so every `BrowseService` list came back empty and every
terminal `Delete` 404'd. Two independent gaps let this sit unnoticed:

1. No k6 script anywhere sets `thresholds`. `k6 run` only fails (non-zero
   exit) on a transport-level error — a failed `check()` prints a red `✗`
   in the summary and the process still exits `0`. Confirmed live: `make
   k6` printed the failures and returned exit code 0 regardless, which
   means the Makefile's per-file `|| exit 1` guard in `_k6-endpoint`/
   `_k6-flow` is dead code against assertion failures specifically — it
   only catches script/connection errors.
2. `make k6` requires a running server and is entirely manual. It's not in
   `.github/workflows/pr.yml` (lint, `go test -race`, build+`--version`
   smoke test only) or `.pre-commit-config.yaml` (golangci-lint, `go
   test`, vitest). Nothing automated runs it at all.

## Decision

### Every k6 script fails its own process on a failed check

`test/k6/lib/options.js` exports one shared `options` object:

```js
export const options = {
  thresholds: {
    checks: ['rate==1.0'],
  },
};
```

`checks` is k6's built-in metric aggregating every `check()` call in a
script under one name — `rate==1.0` means any single failed assertion
fails the threshold, which fails the k6 process's exit code. Every file
under `test/k6/{grpc,http,flow}/*.js` re-exports it:

```js
import { options } from '../lib/options.js';
export { options };
```

k6 resolves `options` from a re-export the same as a local declaration, so
this is equivalent to duplicating the object 34 times without the drift
risk. One shared file, one place to change the policy later (e.g. per-check
tagging, a lower threshold for a known-flaky check) instead of a sweep.

### CI runs k6 against a standalone Badger process, not `ops/compose.yml`

A new `k6` job in `pr.yml` builds `cmd/purser` and runs it directly — no
Postgres, no Grafana/Prometheus/Tempo, no `docker compose`. This is safe
because of what the server's own defaults already are, not a CI-only
carve-out:

- `internal/config/database.go`'s `DefaultConfig` is `Database{Driver:
  "badger"}` — Badger, not Postgres, is the *default* driver.
  [0018](0018-local-development-environment.md)'s Postgres requirement is
  for the shared local-dev stack (Grafana's own backend, plus Postgres as
  one of several supported `database.driver` choices) — it's not a
  statement that Purser itself needs Postgres to run.
- `internal/config/telemetry.go`'s `DefaultConfig` is `Enabled: false` — no
  Tempo/Prometheus dependency unless explicitly turned on.
- `cmd/purser/serve.go`'s `newServeMux` never references `web/dist`, and
  the only `go:embed` outside `_test.go` files in the repo is
  `internal/adapters/datastore/sql/sql.go` (SQL migration files) — the
  server binary needs no frontend build to compile or run.
- Default `server.listen_addr` is `:7474`, matching every k6 script's
  `PURSER_GRPC_ADDR`/`PURSER_HTTP_URL` default, so no k6-side
  configuration is needed either.

Three new Makefile targets make this runnable identically in CI and
locally, following the existing `k6`/`_k6-endpoint`/`_k6-flow` dispatch
pattern:

- `_k6-app-start` — `rm -rf .cidata` **first**, then `mkdir`, `go build -o
  .cidata/purser ./cmd/purser`, runs it with
  `PURSER_PATHS_DATA_DIR=$(CURDIR)/.cidata/data` (everything else stock
  defaults), waits (bounded, 30s) for `:7474` to accept connections.
- `_k6-app-stop` — kills the tracked PID, polls (bounded, 10s) until it's
  actually gone before `rm -rf .cidata` — otherwise the killed process's
  own graceful-shutdown datastore close can race the directory removal and
  log a spurious "closing datastore" error.
- `k6-ci` — `_k6-app-start`, runs `make k6` (both endpoint and flow
  suites), always runs `_k6-app-stop` after (via captured exit status, not
  `&&`, so teardown happens on failure too).

**`.cidata/` is a new top-level directory, deliberately *not* under
`.local/`.** `.local/` ([0018](0018-local-development-environment.md)) is
the developer's persistent dev-loop state — it's meant to accumulate real
data across many `make run`/`make compose up` sessions, and is exactly the
kind of state that caused the flake this ADR is responding to (a stale
`LibraryEntry` count from earlier debugging runs made an unfiltered,
`pageSize: 10` k6 `List` check miss the entry it had just created — not a
code bug, dirty box state). `.cidata/` exists only for the k6-against-a-
standalone-app flow and `_k6-app-start` unconditionally wipes and recreates
it *before* building/starting anything — not just torn down after — so a
run is hermetic even if a previous run's teardown never happened (Ctrl-C,
a CI runner killed mid-job, a crashed build). Local box state, or a
previous CI attempt's leftovers, can never leak into a `k6-ci` run.
`k6-ci` is additive — bare `make k6` is unchanged and still means "run
against whatever's already listening on `:7474`" (the compose stack,
today).

## Consequences

- A regression like the one that prompted this ADR now fails `make k6`'s
  own exit code, not just its printed output — and fails the `k6` CI job
  on every PR.
- CI's `k6` job has no service containers, no `npm ci`, and builds only
  `cmd/purser` — fast and has no dependency on `ops/compose.yml` staying
  healthy. It also means CI never exercises the SQL/Postgres datastore
  backend end-to-end; that gap is accepted for now since Badger is the
  default and the `Datastore` interface ([0012](0012-datastore-persistence.md))
  is backend-agnostic above the adapter boundary — a future ADR can add a
  Postgres-backed k6 job if a Postgres-specific regression ever slips
  through for a reason Badger coverage wouldn't have caught.
- Anyone changing what a k6 script's `check()` set covers is now changing
  a release gate, not just a manual smoke-test aid — failed checks block
  merges. Flaky or environment-dependent checks need to be fixed or
  removed, not left red.
- `k6-ci` binds `:7474` locally the same as the containerized `app`
  service does — running both at once will conflict. Not a new problem
  (native `make run` already conflicts with the containerized app today),
  but worth stating since `k6-ci` is a new way to trip it.
- If `_k6-app-stop` never runs (process killed, CI runner terminated), a
  `.cidata/` directory and an orphaned `purser` process can be left behind.
  Harmless by design: the directory is wiped on the *next* `_k6-app-start`
  regardless, and CI runners are ephemeral so an orphaned process there
  dies with the VM. A local developer who wants it gone immediately can
  just `rm -rf .cidata` — no `make reset` subcommand needed since it isn't
  part of that tree.

## Self-Audit Checklist

1. Does any new or changed k6 script under `test/k6/**` omit the
   `import { options } from '.../lib/options.js'; export { options };`
   pair? If yes — add it; a script without it can fail checks silently
   again.
2. Does `test/k6/lib/options.js` still resolve at the same relative depth
   from every subdirectory that imports it? If a new `test/k6/<subdir>/`
   is added at a different nesting level, fix the relative import path.
3. Does the CI `k6` job's Makefile target still avoid Postgres/compose
   entirely? If a future change makes the app require Postgres/SQL by
   default, this ADR's premise breaks and the job needs to either gain a
   Postgres service container or explicitly force `PURSER_DATABASE_DRIVER=badger`.
4. Does `make k6`'s (or `k6-ci`'s) exit code actually go non-zero when a
   check fails? If a new k6 script is added without inheriting the shared
   `thresholds`, this silently regresses per-script.
