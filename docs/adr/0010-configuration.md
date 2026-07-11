# 0010. Configuration: Viper

Status: Accepted

## Context

Purser is configured from a mix of a YAML file (`ops/purser.yaml`), env vars
(`.env.example` already documents a `PURSER_` prefix with nested keys like
`PURSER_SOURCES_STASHDB_API_KEY` and `PURSER_DATABASE_BADGER_DATA_DIR`), and
eventually CLI flags on top of both. Nothing yet in the codebase (no
`internal/config` exists as of this ADR) actually loads any of it — this ADR
fixes the mechanism before that package is built, so it's built once, the
right way, instead of accreting ad hoc `os.Getenv` calls across adapters.

## Decision

- **Viper** (`github.com/spf13/viper`, already a dependency) is the only
  configuration-loading library. No package reads `os.Getenv` directly for
  application configuration outside of Viper's own env binding.
- **Precedence** (highest to lowest, Viper's standard order): explicit
  Cobra flag (see [0009](0009-cli-stack.md)) > environment variable > config
  file (`ops/purser.yaml` or `--config` path) > documented default.
- **Env var convention:** prefix `PURSER_`, nested keys joined by `_`,
  matching the structure already established in `.env.example` and
  `ops/purser.yaml` (e.g. `database.badger.data_dir` ↔
  `PURSER_DATABASE_BADGER_DATA_DIR`). This is a continuation of existing
  practice, not a new one.
- **One config struct tree, assembled from component structs.** Each
  package that needs configuration (e.g. `pkg/httpclient.Config`,
  `pkg/cache.Config`) owns its own struct with `mapstructure` tags and a
  `DefaultConfig()` — those packages do not import Viper themselves. A
  single `internal/config` package (composition-root-adjacent, built when
  first needed) owns the Viper instance, embeds the component structs into
  one top-level struct, and is the only place `viper.Unmarshal` is called.
  This keeps Viper itself out of `pkg/**` and `internal/domain|ports|
  adapters`, consistent with [0001](0001-hexagonal-architecture.md) — those
  layers receive an already-populated `Config` value via constructor
  injection, they don't fetch their own config.
- **Validation happens once, after unmarshal,** in `internal/config` (or
  each component's own `Validate() error` method called from there) —
  invalid config fails fast at startup, not partway through a request.
- **Defaults live in code** (`DefaultConfig()` per component), not
  duplicated into the shipped `ops/purser.yaml`/`.env.example` — those files
  document *available* keys and show non-default examples; they are not the
  source of truth for what happens when a key is absent.

## Consequences

- Adding a new configurable component means adding its `Config` struct and
  registering it in the one composition point — no new ad hoc env-reading
  code scattered through adapters.
- `internal/config` becomes a required stop for anyone changing what's
  configurable, which is the point: one place to check precedence and
  defaults instead of grepping for `os.Getenv`.
- Slightly more indirection to trace "where does this value ultimately come
  from" versus reading `os.Getenv` inline at the call site — acceptable
  given the alternative is inconsistent config sourcing per package.

## Self-Audit Checklist

1. Does any package outside `internal/config` call `os.Getenv`/`os.LookupEnv`
   for application configuration, or import `viper` directly? If yes — fix
   it.
2. Does every component `Config` struct have a `DefaultConfig()` and
   `mapstructure` tags matching the `PURSER_<NESTED>_<KEY>` convention? If
   no — fix it.
3. Is config validated once at startup (fail fast) rather than checked
   ad hoc at first use deep in a call path? If no — fix it.
4. Do `ops/purser.yaml` and `.env.example` stay documentation of available
   keys (with defaults implied by code), not a second source of truth that
   can drift from `DefaultConfig()`? If they've drifted, fix it.
