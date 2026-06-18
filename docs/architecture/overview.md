# Architecture Overview

Purser uses hexagonal architecture. Business logic lives in `internal/domain` and `internal/app`. All external systems are accessed only through ports in `internal/ports`.

## Directory Layout

```
cmd/purser/              # main; wires adapters to ports, starts server
internal/
  domain/                # pure domain types — no imports from app/ports/adapters
  ports/                 # Go interfaces only, no implementations
  app/                   # application services (use cases); depend only on ports
  adapters/
    db/                  # SQLite adapter (default)
    postgres/            # PostgreSQL adapter (optional)
    prowlarr/            # Prowlarr indexer adapter
    stashdb/             # StashDB metadata adapter
    tpdb/                # TPDB metadata adapter
    tmdb/                # TMDB metadata adapter
    tvdb/                # TVDB metadata adapter
    mbz/                 # MusicBrainz metadata adapter
    qbittorrent/         # qBittorrent download client adapter
    fs/                  # filesystem adapter (rename, move, oshash)
  api/                   # HTTP handlers + router (Chi); adapts HTTP → app services
  web/                   # embedded React build (go:embed)
  config/                # config loading (YAML file + env vars)
```

## Dependency Rule

```
domain ← ports ← app ← adapters / api
```

Nothing in `domain` or `ports` imports from `app`, `adapters`, or `api`. Violations are bugs.

## Technology Stack

| Layer | Choice | Notes |
|---|---|---|
| Language | Go 1.23+ | |
| HTTP router | Chi | stdlib-compatible, no magic |
| Database (default) | SQLite via `modernc.org/sqlite` | no CGo, works in distroless |
| Database (optional) | PostgreSQL via `pgx` | configured via `DATABASE_URL` |
| DB migrations | `golang-migrate/migrate` | embedded SQL files |
| Query layer | `sqlc` | type-safe Go generated from SQL |
| Config | struct + `envconfig` | 12-factor; YAML file + env overrides |
| Logging | `log/slog` (stdlib) | JSON in prod, text in dev |
| Frontend | React 18 + TypeScript + Vite | embedded via `go:embed` |
| UI data fetching | TanStack Query | all data from API |
| UI routing | React Router v6 | |
| UI styling | Tailwind CSS | |

## File Layout Conventions

- One file per type group in `domain/` — no god files
- Each adapter implements exactly one port
- SQL in `internal/adapters/db/queries/*.sql` (sqlc source)
- Migrations in `internal/adapters/db/migrations/` as numbered `.sql` files
- API handlers grouped by resource in `internal/api/`
- React source in `web/src/`, build output in `web/dist/` (git-ignored), embedded via `go:embed`
