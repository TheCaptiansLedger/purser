# 0018. Local Development Environment: Shared Directories, One Compose File, One Postgres

Status: Accepted

## Context

Purser can run two ways during development: the natively-built binary
(`go run ./cmd/purser serve` / `make run`), or the containerized app via
`ops/compose.yml`. Before this ADR, these two run modes didn't share
on-disk state — the container used named Docker volumes
(`purser-data`, `purser-media`) while the native binary defaulted to an
OS-appropriate per-user data directory — so switching between "run it
locally" and "run it in the container" meant working against two
different, unrelated datasets. Gitignore entries for this state were
scattered (`data/`, `purser-data/`, `images/`, listed twice in one case),
making it unclear at a glance what was actually dev-local state.

Separately, [0007](0007-telemetry.md) commits to OTLP traces and
Prometheus-scraped metrics but leaves SDK/exporter wiring for later,
which this ADR's companion work (see the telemetry task tracked
alongside this one) now fills in — which means there needs to be
somewhere locally to send that data and look at it: Prometheus to scrape
metrics, Grafana Tempo to receive traces, and Grafana to browse both.
Postgres is already a supported `database.driver` ([0012](0012-datastore-persistence.md))
and is also a natural backend for Grafana's own storage (dashboards,
users, datasource config) instead of Grafana's default embedded SQLite —
running one Postgres container for both, rather than two, avoids a
second stateful container whose only job is to be "a database, but only
for Grafana."

## Decision

- **One top-level dev-state directory: `.local/`.** Every *new* directory
  that holds local, disposable, gitignored runtime state — datastore
  files, downloaded media, Postgres's data directory, Grafana's
  config/state, Prometheus's TSDB, Tempo's storage — lives under
  `.local/<component>/`. The gitignore entry is a single `/.local/`
  line, replacing the previous scattered `data/`, `purser-data/`,
  `images/` entries. `test-data/` (scanner test fixtures, bind-mounted
  to `/media/content`) is a deliberate exception, not folded into
  `.local/`: it predates this ADR, is partially tracked (`.gitkeep` plus
  whatever fixture files a developer drops in), and isn't disposable
  state in the same sense — moving it would just be churn.
- **The native binary and the containerized app point at the same
  `.local/` paths.** `ops/compose.yml` bind-mounts `.local/data` and
  `.local/media` into the container at the same paths it already used
  (`/data`, `/media/purser`); the native binary is pointed at the
  host-side `.local/data`/`.local/media` paths via the same
  `PURSER_PATHS_DATA_DIR`/`PURSER_MEDIA_PATH` env vars documented in
  `.env.example`. Stopping the container and continuing against the same
  data natively (or vice versa) is a supported, intended workflow, not
  an accident of bind-mount plumbing.
- **One `ops/compose.yml`, not one file per concern.** The app, Postgres,
  Grafana, Prometheus, and Tempo are services in a single compose file on
  the default network it creates — no second `compose.observability.yml`
  to keep in sync, no second `make` subcommand namespace. `make compose
  <args>` forwards to this one file for the whole stack.
- **One Postgres container, two roles, two databases.** An init script
  under `ops/postgres/` (mounted to `/docker-entrypoint-initdb.d/`, which
  the official `postgres` image runs once, only when its data directory
  is empty) creates a `purser` role+database and a separate `grafana`
  role+database, each with its own password sourced from `.env`. The app
  connects via `PURSER_DATABASE_SQL_DSN` as the `purser` role; Grafana
  connects via its own `GF_DATABASE_*` env vars as the `grafana` role.
  Neither service can see the other's database — separate roles, not a
  shared login reused across two schemas.
- **`.env` is the single source of secrets/credentials for both run
  modes.** `make run` (native) sources it explicitly (`set -a; . ./.env;
  set +a`) before exec'ing the binary, rather than assuming the
  developer's shell already has `direnv` loaded; `ops/compose.yml`
  continues to read it via `env_file`. Both paths see the same values —
  documented once in `.env.example`, not duplicated into
  `ops/purser.yaml` (per [0010](0010-configuration.md)'s "defaults live
  in code, files are documentation" rule).

## Consequences

- Switching between native and containerized runs during development no
  longer means re-seeding or losing data — both read/write the same
  files on disk.
- A single `/.local/` gitignore entry and a single compose file are
  easier to keep correct than several scattered ones; a new dev-state
  subdirectory is just a new subdirectory under `.local/`, not a new
  gitignore line to remember.
- Postgres becomes a required container for full local observability
  (Grafana's backend) even for developers who otherwise run Purser
  itself on the default Badger driver — accepted because it's one
  container either way, and Purser's own `database.driver` stays
  independently configurable (Badger, SQLite, Postgres, MySQL all still
  work; Postgres is not required for Purser's own data just because
  Grafana uses it).
- Bind mounts (vs. the previous named volumes) mean `.local/`'s
  ownership/permissions are whatever the container's process UID leaves
  behind on the host — a known tradeoff of bind mounts, acceptable for
  disposable dev state that `make reset` already wipes.

## Self-Audit Checklist

1. Does any new dev-local state directory get its own top-level gitignore
   entry instead of living under `.local/`? If yes — move it.
2. Does the native run path and the container run path point at
   different underlying directories for the same kind of state (data,
   media, content)? If yes — fix the env var/bind-mount mismatch.
3. Is there more than one compose file, or a `make` target that only
   brings up part of the stack under a different subcommand namespace? If
   yes — that's the split this ADR rejected; fold it back into
   `ops/compose.yml`.
4. Does Grafana's Postgres role have access to the `purser` database, or
   vice versa? If either role can reach the other's database, fix the
   init script's grants — they must stay isolated.
5. Are Postgres/Grafana/Prometheus/Tempo credentials or endpoints
   hardcoded anywhere outside `.env`/`.env.example` and the compose
   file's env-var references? If yes — fix it per
   [0010](0010-configuration.md).
