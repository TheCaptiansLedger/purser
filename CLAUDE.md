# Purser

Purser is a self-hosted metadata manager for the media you care about. It maintains a rich, browsable library enriched from external sources (TMDB, TVDB, MusicBrainz, StashDB, and others) and gives users tools to identify gaps in their collection and fill them. The primary value is clean, deep metadata across every content type; the acquisition pipeline (Phase 2) is a disciplined way to go get what's missing.

Designed to run as a standalone binary, a container, or a Kubernetes workload. No cloud account required.

**Module path:** `github.com/purser-app/purser` (update when repo is published)

---

## Development Phases

Build in this order. Do not begin Phase 2 work until Phase 1 is solid.

### Phase 1 — Metadata & Browse
- Domain model + database schema
- Metadata ingestion from external sources (StashDB, TPDB, TMDB, TVDB, MusicBrainz, etc.)
- REST API for browsing: library entries, groups, items, people, tags
- React UI: browse the full hierarchy, see performers/cast, filter by any metadata dimension
- Search within the local database

### Phase 2 — Acquisition & Disk Management
- Indexer integration (Prowlarr adapter)
- Download client integration (qBittorrent, Transmission, etc.)
- Release matching and grab pipeline
- File identification (OSHash-first, then configurable parser chain)
- On-disk organization via user-defined naming templates
- Monitoring: wanted/grabbed/imported state machine

---

## What Purser Does (and Does Not Do)

**Does:**
- Maintain a rich, browsable metadata database for multiple content types
- Enrich metadata from external sources (StashDB, TPDB, TMDB, TVDB, MusicBrainz, etc.)
- Track monitored items at every level of the content hierarchy
- Search indexers via Prowlarr for matching releases
- Dispatch grabs to download clients
- Identify files by OSHash first, then fallback parser chain
- Rename and organize files on disk using user-defined Go templates
- Expose a REST API that drives everything — the UI has no privileged access

**Does not:**
- Manage indexers (Prowlarr's job, injected via interface)
- Serve or transcode media
- Replace Stash — it can ingest metadata from Stash but is not a Stash replacement
- Handle user authentication (deferred)

---

## Architecture

Hexagonal architecture. Business logic lives in `internal/domain` and `internal/app`. All external systems are accessed only through ports in `internal/ports`.

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

### Dependency Rule
`domain` ← `ports` ← `app` ← `adapters` / `api`

Nothing in `domain` or `ports` imports from `app`, `adapters`, or `api`. Violations are bugs.

---

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

---

## Data Model

### Content Types

User selects content type when adding anything to the library. Content type drives terminology, metadata sources, and hierarchy depth.

| `content_type` | Display | Hierarchy | Leaf name | Person role |
|---|---|---|---|---|
| `movie` | Movie | entry only | Movie | Cast |
| `tv` | TV Show | entry → group (Season) → item (Episode) | Episode | Cast |
| `music` | Music | entry → group (Album) → item (Track) | Track | Artist |
| `adult` | Adult | entry → [group (Series)] → item (Scene) | Scene | Performer |
| `jav` | JAV | entry → [group (Series)] → item (Title) | Title | Actress |

### `library_entries` — the content hierarchy

Self-referential tree. Every node can be independently monitored.

```sql
id            TEXT PRIMARY KEY
content_type  TEXT NOT NULL   -- movie|tv|music|adult|jav
kind          TEXT NOT NULL   -- network|studio|series|artist|movie
name          TEXT NOT NULL
sort_name     TEXT
overview      TEXT
parent_id     TEXT REFERENCES library_entries(id)  -- network → studio, etc.
monitored     BOOLEAN DEFAULT false
monitor_mode  TEXT DEFAULT 'all'  -- all|future|none
status        TEXT              -- continuing|ended|active
quality_profile_id   TEXT
metadata_profile_id  TEXT
path          TEXT              -- root path on disk
metadata      JSON              -- type-specific fields (rating, label, etc.)
added_at      TIMESTAMP
updated_at    TIMESTAMP
```

`kind` values and their typical parent:
- `network` → no parent (HBO, Naughty America parent brand, Columbia Records)
- `studio` → parent is `network` (production company, adult site, JAV studio)
- `series` → parent is `studio` or `network` (a TV show, an adult site's named series)
- `artist` → no parent (Fleetwood Mac; music top-level)
- `movie` → parent is `studio` (collapsed: no separate leaf item)

**Movies are collapsed**: `kind=movie` in `library_entries` is the movie itself. One `item` record is auto-created as its leaf (holds file/status state). No manual group creation.

### `groups` — intermediate groupings

Season (TV), Album (music), JAV/Adult Series.

```sql
id                TEXT PRIMARY KEY
library_entry_id  TEXT REFERENCES library_entries(id)
title             TEXT
sort_name         TEXT
number            INTEGER    -- season 1, album 2
year              INTEGER
overview          TEXT
monitored         BOOLEAN DEFAULT true
monitor_mode      TEXT DEFAULT 'all'
metadata          JSON
```

### `items` — leaf content

Episode, Scene, Track, Movie (auto-created), JAV Title.

```sql
id                TEXT PRIMARY KEY
content_type      TEXT NOT NULL
library_entry_id  TEXT REFERENCES library_entries(id)
group_id          TEXT REFERENCES groups(id)  -- nullable
title             TEXT
overview          TEXT
date              DATE
sequence          TEXT        -- "S01E05", "3", "SSIS-001", NULL for movies
runtime_seconds   INTEGER
monitored         BOOLEAN DEFAULT true
status            TEXT DEFAULT 'wanted'
                  -- wanted|grabbed|downloading|imported|missing|skipped
metadata          JSON        -- explicit, censored, BPM, content rating, etc.
added_at          TIMESTAMP
updated_at        TIMESTAMP
```

### `people` — cross-cutting persons

Performers, cast, artists, actresses. Independently monitorable across any studio.

```sql
id            TEXT PRIMARY KEY
name          TEXT NOT NULL
sort_name     TEXT
overview      TEXT
monitored     BOOLEAN DEFAULT false
monitor_mode  TEXT DEFAULT 'all'
metadata      JSON
added_at      TIMESTAMP
```

```sql
-- Name aliases (stage names, maiden names, etc.)
people_aliases (person_id, alias)

-- Who appears in what
item_people (item_id, person_id, role)
  role: performer|actress|director|actor|artist|producer
```

### `external_ids` — links to external databases

```sql
entity_type  TEXT  -- library_entry|group|item|person
entity_id    TEXT
source       TEXT  -- stashdb|tpdb|tmdb|tvdb|mbid|javlibrary|r18|discogs|mal|anilist
external_id  TEXT
PRIMARY KEY (entity_type, entity_id, source)
```

### `tags`

Two scopes: `user` (organizational labels) and `metadata` (genres, content tags from sources).

```sql
tags (id, name, scope)
item_tags (item_id, tag_id)
entry_tags (library_entry_id, tag_id)
```

### `media_files` — files on disk

```sql
id          TEXT PRIMARY KEY
item_id     TEXT REFERENCES items(id) UNIQUE
path        TEXT NOT NULL
size        INTEGER
oshash      TEXT
md5         TEXT
quality     TEXT   -- 4K|1080p|720p|480p|SD
resolution  TEXT   -- "1920x1080"
codec       TEXT
container   TEXT
added_at    TIMESTAMP
```

### `releases` — indexer search results

```sql
id                TEXT PRIMARY KEY
title             TEXT NOT NULL   -- raw indexer title
size              INTEGER
seeders           INTEGER
leechers          INTEGER
indexer_config_id TEXT
guid              TEXT
download_url      TEXT
info_url          TEXT
published_at      TIMESTAMP
item_id           TEXT REFERENCES items(id)  -- set when matched
grabbed           BOOLEAN DEFAULT false
grabbed_at        TIMESTAMP
```

### `downloads` — in-flight and completed

```sql
id                TEXT PRIMARY KEY
release_id        TEXT REFERENCES releases(id)
item_id           TEXT REFERENCES items(id)
client_config_id  TEXT
client_job_id     TEXT
status            TEXT  -- queued|downloading|seeding|completed|failed|imported
output_path       TEXT
created_at        TIMESTAMP
updated_at        TIMESTAMP
completed_at      TIMESTAMP
```

### Config tables

```sql
quality_profiles        (id, name, content_type, config JSON)
metadata_profiles       (id, name, content_type, config JSON)
indexer_configs         (id, name, type, enabled, priority, config JSON)
download_client_configs (id, name, type, enabled, config JSON)
metadata_source_configs (id, name, type, content_types JSON, priority, enabled, config JSON)
parser_configs          (id, name, content_type, type, priority, enabled, config JSON)
naming_configs          (id, content_type UNIQUE, folder_template, file_template)
```

### Monitoring Logic

An item is grabbed when:
```
item.monitored = true
AND item.status = 'wanted'
AND (
  -- hierarchy path: all ancestors monitored
  (library_entry.monitored AND (group.monitored OR group IS NULL))
  OR
  -- person monitoring: any performer/cast has monitored=true
  EXISTS(item_people JOIN people WHERE people.monitored = true)
)
```

`monitor_mode` on any node drives the initial `monitored` state of newly discovered children:
- `all` → new children default to `monitored=true`
- `future` → new children after `added_at` default to `monitored=true`, backfill defaults to `false`
- `none` → new children default to `monitored=false` (manual selection only)

### UI hierarchy browse → DB mapping

| User action | DB effect |
|---|---|
| Monitor all Fleetwood Mac | `library_entries[artist].monitored=true, monitor_mode=all` |
| Monitor Rumours album only | artist monitored + `groups[Rumours].monitored=true`, others false |
| Monitor specific tracks | group monitored + cherry-picked `items[track].monitored=true` |
| All videos with Alex Coal | `people[Alex Coal].monitored=true` |
| All episodes of a series | `library_entries[series].monitored=true, monitor_mode=all` |
| Future episodes only | `library_entries[series].monitored=true, monitor_mode=future` |
| Specific episodes | series NOT monitored + cherry-picked `items.monitored=true` |
| All movies with Tom Cruise | `people[Tom Cruise].monitored=true` |
| This specific movie | `library_entries[kind=movie].monitored=true` |

---

## Key Ports (Interfaces)

```go
// Metadata sources — StashDB, TPDB, TMDB, TVDB, MusicBrainz, etc.
type MetadataSource interface {
    Name() string
    ContentTypes() []string
    FindByExternalID(ctx context.Context, id string) (*domain.ExternalItem, error)
    FindByHash(ctx context.Context, hash string) (*domain.ExternalItem, error)
    SearchByTitle(ctx context.Context, title string, limit int) ([]*domain.ExternalItem, error)
}

// Indexers — Prowlarr
type Indexer interface {
    Search(ctx context.Context, query string, cats []Category) ([]Release, error)
}

// Download clients — qBittorrent, Transmission, etc.
type DownloadClient interface {
    Add(ctx context.Context, r Release) (jobID string, err error)
    Status(ctx context.Context, jobID string) (DownloadStatus, error)
    Remove(ctx context.Context, jobID string, deleteFiles bool) error
}

// Repositories — one per aggregate root, swappable (SQLite ↔ PostgreSQL)
type LibraryEntryRepository interface {
    Get(ctx context.Context, id string) (*domain.LibraryEntry, error)
    List(ctx context.Context, filter LibraryFilter) ([]*domain.LibraryEntry, error)
    Save(ctx context.Context, e *domain.LibraryEntry) error
    Delete(ctx context.Context, id string) error
}
// Separate repository interfaces for: GroupRepository, ItemRepository,
// PersonRepository, ReleaseRepository, DownloadRepository

// Filesystem
type FileSystem interface {
    Stat(ctx context.Context, path string) (*FileInfo, error)
    Move(ctx context.Context, src, dst string) error
    OSHash(ctx context.Context, path string) (string, error)
}
```

---

## API Design

All functionality at `/api/v1/`. The React UI is just another API consumer.

Standard CRUD:
```
GET    /api/v1/{resource}          list with filter/sort/pagination
GET    /api/v1/{resource}/:id      single
POST   /api/v1/{resource}          create
PUT    /api/v1/{resource}/:id      full update
PATCH  /api/v1/{resource}/:id      partial update
DELETE /api/v1/{resource}/:id      delete
```

Resources: `library-entries`, `groups`, `items`, `people`, `tags`, `releases`, `downloads`, `commands`

Async operations via commands:
```
POST /api/v1/commands  { "name": "SearchItem", "ids": [...] }
GET  /api/v1/commands/:id
```

Errors: `{ "error": "human message", "code": "SNAKE_CASE_CODE" }`

---

## Configuration

YAML file (default: `purser.yaml`, override with `$CONFIG_PATH`) + environment variable overrides.

```yaml
server:
  port: 7474

database:
  driver: sqlite        # sqlite | postgres
  dsn: purser.db        # file path for sqlite, connection string for postgres

library:
  root: /media          # base path for organized media

log:
  level: info           # debug | info | warn | error
  format: text          # text | json
```

---

## File Layout Conventions

- One file per type group in `domain/` — no god files
- Each adapter implements exactly one port
- SQL in `internal/adapters/db/queries/*.sql` (sqlc source)
- Migrations in `internal/adapters/db/migrations/` as numbered `.sql` files
- API handlers grouped by resource in `internal/api/`
- React source in `web/src/`, build output in `web/dist/` (git-ignored), embedded via `go:embed`

---

## Build and Run

```bash
# Backend only
go run ./cmd/purser

# Full build (UI + embed)
cd web && npm run build && cd ..
go build -o purser ./cmd/purser

# Tests
go test ./...

# Regenerate sqlc types after SQL changes
sqlc generate

# Run migrations
go run ./cmd/purser migrate
```

---

## Testing Conventions

- **Coverage Targets**: Core domain must maintain at least **95%** unit test coverage. Adapters must maintain at least **80%** coverage. Aggregation/glue code (such as `main.go` and router setup) must maintain at least **50%** coverage.
- **Package Selection**: Prefer `package name_test` format (external tests) for unit tests so they do not rely on internal structure, forcing testing of the public API/interface. Test internal structures (same package name) only when absolutely necessary to verify internal logic.
- Unit tests alongside code (`_test.go` files).
- App service tests use hand-rolled mock port implementations (no mockgen).
- Adapter tests use real databases (SQLite in-memory; testcontainers for PostgreSQL) — no mocking the database.
- HTTP handler tests use `httptest` with real app services + in-memory SQLite.
- **UI Testing**: Going forward, the frontend/UI must also include unit tests.

---

## Code & Design Conventions

- **Pre-Write Code Review**: Always present the code or diff in the chat and wait for explicit user confirmation/approval before writing, editing, or creating files.
- **SOLID Principles**: Adhere to SOLID design principles across the codebase.
- **Go Best Practices**: Follow Go community best practices for all backend development.
- **UI Best Practices**: Follow React, Vite, TypeScript, and CSS best practices for all frontend development.
- No comments explaining what code does — names do that.
- Comments only for non-obvious WHY (hidden constraint, workaround, subtle invariant).
- No global state — everything injected via constructor.
- Errors wrapped with `fmt.Errorf("context: %w", err)` at each layer boundary.
- Context propagated everywhere — no `context.Background()` inside business logic.
- All time values in UTC; never `time.Local`.
- `metadata JSON` columns hold type-specific fields that don't need indexed querying.
- Fields that need filtering (performers, tags, quality) always get their own columns or junction tables.
- **Linting**: The codebase must always compile successfully, all unit tests must pass, and the project must always pass `golangci-lint` check.

---

## Architectural Discipline

### Cross-Cutting Feature Design (REQUIRED)

Before implementing any new feature, reason through how it applies to **every content type**: `movie`, `tv`, `music`, `adult`, `jav`. If a feature only touches one module (e.g., `afterdark`), that is a red flag — either the feature belongs in shared infrastructure or the design needs to be reconsidered.

When a new feature is proposed, explicitly ask:
- How does this work for movies? TV? Music? Books? Adult? JAV?
- Does it belong in a shared component/API/service, or does each module need its own implementation?
- If the answer differs per content type, is the difference encoded in data (content_type field, config) rather than duplicated code?

**Never snowflake a module.** Logic that appears in `afterdark/` but would also be needed in `movies/` or `music/` must live in shared code. The content type is a data dimension, not a reason to fork the architecture.

### Hexagonal Architecture (ENFORCE)

The dependency rule is absolute:

```
domain ← ports ← app ← adapters / api
```

Violations to watch for:
- An adapter importing from another adapter (cross-adapter coupling)
- An API handler containing business logic (should be in `app/`)
- A domain type importing anything from outside `domain/`
- Content-type-specific logic in `app/` instead of being driven by the domain model

### SOLID in Practice

- **Single Responsibility**: each file/type does one thing. API handlers translate HTTP ↔ app service. App services orchestrate domain logic. Adapters translate external systems ↔ ports.
- **Open/Closed**: new content types, metadata sources, and download clients are added by implementing a port — not by editing existing switch statements throughout the codebase.
- **Interface Segregation**: ports are narrow. A metadata source adapter implements `MetadataSource`; it does not also implement repository methods.
- **Dependency Inversion**: app services depend on port interfaces, never on concrete adapters.

### UI Architecture

The React UI is an API consumer with no privileged access. The same REST API that the UI uses must be sufficient for any other client.

- Shared UI patterns (cards, detail pages, search dialogs) must be built as reusable components, not duplicated per content type.
- Content-type-specific behavior is driven by props/config, not by forked component trees.
- A component built for `afterdark` that would be needed identically in `movies` must be extracted to `components/` before landing.
