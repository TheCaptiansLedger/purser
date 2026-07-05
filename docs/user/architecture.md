# Purser — Architecture

Purser is built in layers. Each layer has one job. No layer talks to anything below its own level. This document explains what each layer does and how data moves through the system.

## The Layers

```
┌──────────────────────────────────────────────────┐
│  React UI  (web/src/)                            │
│  — API consumer, no privileged access            │
└───────────────────────┬──────────────────────────┘
                        │ HTTP / JSON
┌───────────────────────▼──────────────────────────┐
│  API Handlers  (internal/api/)                   │
│  — translate HTTP ↔ app services                 │
└───────────────────────┬──────────────────────────┘
                        │ Go function calls
┌───────────────────────▼──────────────────────────┐
│  App Services  (internal/app/)                   │
│  — orchestrate domain logic, call ports          │
│  library/  metadata/  scan/  people/             │
└───────────────────────┬──────────────────────────┘
                        │ port interfaces
┌───────────────────────▼──────────────────────────┐
│  Ports  (internal/ports/)                        │
│  — Go interfaces only, no implementations        │
│  MetadataSource  FileIdentifier  Repository…     │
└──────────┬────────────┬──────────────────────────┘
           │            │
┌──────────▼──┐  ┌──────▼─────────────────────────┐
│  Adapters   │  │  Adapters                       │
│  (storage)  │  │  (external services)            │
│  badger/    │  │  mbz/  stashdb/  fanart/        │
│  db/        │  │  theaudiodb/  identifier/       │
└─────────────┘  └────────────────────────────────┘
```

**The rule:** dependencies only point upward. Domain knows nothing about ports. Ports know nothing about adapters. App services never import adapter packages — they always call through an interface.

---

## Domain  (`internal/domain/`)

Pure Go types. No network calls, no database calls, no imports from any other Purser package. This is the vocabulary the whole system speaks.

Key types:

| Type | What it is |
|---|---|
| `LibraryEntry` | Root node in the content tree: artist, studio, series, author |
| `Group` | Mid-level grouping: album, season, adult series |
| `Item` | Leaf content: track, episode, scene, movie |
| `Person` | Cross-cutting person record: performer, artist, actor |
| `MediaFile` | A physical file on disk linked to an Item |
| `ScannedFile` | A file the scanner found but hasn't identified yet |
| `UnmatchedFile` | A file the pipeline couldn't auto-import; sits in the queue |
| `MatchCandidate` | One possible identity for a scanned file, with a computed confidence score |
| `Fingerprint` | All computed fingerprints for a file: OSHash, AcoustID, embedded tags, ISBN |
| `ContentType` | `music` \| `tv` \| `movie` \| `adult` \| `jav` \| `book` |
| `Kind` | `artist` \| `studio` \| `series` \| `network` \| `author` \| `movie` \| `book` |

Content hierarchy by module:

```
Music:   artist (LibraryEntry) → album (Group) → track (Item) → file (MediaFile)
TV:      series (LibraryEntry) → season (Group) → episode (Item) → file (MediaFile)
Adult:   studio (LibraryEntry) → [series (Group)] → scene (Item) → file (MediaFile)
Movie:   [studio (LibraryEntry)] → movie (Item, collapsed) → file (MediaFile)
Book:    author (LibraryEntry) → book (Item, collapsed) → file (MediaFile)
```

`ContentType.ParentEntryKind()` returns the correct `Kind` for any content type. No code outside the domain module should hardcode a kind string for a given content type.

---

## Ports  (`internal/ports/`)

Go interfaces. Nothing else. No implementations live here.

The important ones:

| Interface | What it declares |
|---|---|
| `MetadataSource` | Base contract every external source adapter must satisfy |
| `StudioSearchSource` | Optional: search for artists/studios by name |
| `ItemSearchSource` | Optional: search for tracks/scenes/episodes by name |
| `EntryContentSource` | Optional: page through all albums/scenes for an artist/studio |
| `GroupContentSource` | Optional: page through all tracks in an album |
| `ExternalIDSource` | Optional: fetch full entity by external ID |
| `ItemSource` | Optional: fetch a single track/scene/episode by its source-native ID |
| `HashLookupSource` | Optional: identify a file by hash (StashDB does this) |
| `FileIdentifier` | Match a fingerprinted file against library items and external sources |
| `FileFingerprinter` | Compute fingerprints (AcoustID, OSHash, embedded tags) for a file |
| `LibraryEntryRepository` | CRUD for LibraryEntry records |
| `GroupRepository` | CRUD for Group records |
| `ItemRepository` | CRUD for Item records |
| `PersonRepository` | CRUD for Person records |
| `MediaFileRepository` | CRUD for MediaFile records |
| `UnmatchedFileRepository` | CRUD for the import queue |
| `ExternalIDRepository` | Map external IDs (MBIDs, StashDB IDs) to internal IDs |
| `JobQueue` | Submit and track background jobs |
| `NotificationDispatcher` | Emit lifecycle events (scan started, file matched, etc.) |
| `ThumbnailCache` | Download and store remote images for queue display |

Adapters implement whichever interfaces fit their capabilities. The MBZ adapter implements `MetadataSource`, `EntryContentSource`, `GroupContentSource`, and `ExternalIDSource` but not `HashLookupSource` (MBZ doesn't look up by file hash). StashDB implements `HashLookupSource` and `StudioSearchSource`. No adapter needs to implement everything. Unimplemented optional interfaces return `ErrNotSupported`.

---

## App Services  (`internal/app/`)

Where the business logic lives. Each service accepts port interfaces in its constructor and only talks through those interfaces.

| Service | What it does |
|---|---|
| `library.Service` | CRUD for the library: create entries, groups, items; apply tag filters; manage monitor state |
| `metadata.Service` | Fan out searches to all registered sources; import from external sources into the library with full enrichment (performers, tags, images) |
| `scan.Service` | Orchestrate the file pipeline: fingerprint → identify → auto-import or enqueue; manage the unmatched queue; handle manual resolution |
| `people.Service` | Search and manage Person records; import people from external sources |

The `metadata.Service` uses an internal `Aggregator` that fans searches and fetches out to all registered sources simultaneously. Sources that don't support a given operation are skipped automatically via interface type assertion.

**`ImportEntry`** is the core import operation in `metadata.Service`. It creates the correct `LibraryEntry` kind for the content type (`ContentType.ParentEntryKind()`), then its parent if one exists, then the group (album/season), then all items within that group. The operation is fully idempotent — calling it twice with the same external ID returns the existing record without modification.

---

## Adapters  (`internal/adapters/`)

Each adapter implements one or more port interfaces and translates between the port contract and one specific external system.

### Storage adapters

| Package | What it implements |
|---|---|
| `adapters/badger/` | All repository ports using BadgerDB (default; embedded, no CGo) |
| `adapters/db/` | All repository ports using SQLite or PostgreSQL |

### Metadata source adapters

| Package | Implements | Talks to |
|---|---|---|
| `adapters/mbz/` | Recording, artist, album lookups | MusicBrainz REST API |
| `adapters/stashdb/` | Hash lookup, studio/scene search and import | StashDB GraphQL API |
| `adapters/fanart/` | Group and entry images | Fanart.tv REST API |
| `adapters/theaudiodb/` | Artist hero images keyed by MBID | TheAudioDB REST API |

### Scan pipeline adapters

| Package | Implements | What it does |
|---|---|---|
| `adapters/fs/` | `FileScanner`, `FileWatcher`, `ThumbnailCache`, `ImageDownloader` | Walk directories, watch filesystem events, cache thumbnails |
| `adapters/fingerprint/` | `FileFingerprinter` (per content type) | Compute OSHash; read FLAC/ID3 tags; compute AcoustID fingerprints |
| `adapters/identifier/` | `FileIdentifier` (per content type) | Run identification strategies and return ranked candidates |

### Response cache

All external HTTP adapters carry a `*cache.Cache` from `pkg/cache/` — a thread-safe in-memory LRU store with per-entry TTL and byte accounting. Responses are cached by request URL (or fingerprint+duration for AcoustID). Named caches: `mbz` (24h), `audiodb` (24h), `fanart` (24h), `stashdb` (24h), `github` (24h), `acoustid` (7d). Cache stats and flush are exposed at `/api/v1/cache/`.

---

## The Scan Pipeline

This is the path a file takes from appearing on disk to being in the library.

```
File appears on disk (new file or filesystem event)
       │
       ▼
FileScanner / FileWatcher
  Emits ScannedFile{Path, Size, ContentType}
  ContentType is determined by which module root owns the path —
  never by file extension alone, so adult and JAV files are never
  misclassified as movies.
       │
       ▼
FileFingerprinter chain
  All registered fingerprinters that handle this ContentType run in sequence.
  Results are merged into a single Fingerprint{}:
    OSHash          — fast content hash, used for duplicate detection
    AcoustID        — audio fingerprint (music only), submitted to AcoustID API
    EmbeddedTags    — FLAC/ID3 metadata: title, artist, album, duration, MBIDs, etc.
    ISBN            — extracted from ebook metadata (books only)
       │
       ▼
FileIdentifier chain
  All registered identifiers that handle this ContentType run in sequence.
  Each produces []MatchCandidate. Results are merged and deduplicated.

  Music identification strategy order:
    1. Embedded MusicBrainz track ID tag (musicbrainz_track_id)
       → fetch full recording from MBZ, check ExternalIDRepository for local item
    2. AcoustID fingerprint → MBZ recording IDs (may return several for same audio)
       → for each recording ID, check ExternalIDRepository for local item
    3. Full tag-set fuzzy match (title + artist + album + duration within 5s)
       → search ItemRepository
    4. Filename parse → search ItemRepository by extracted title

  Adult/video identification strategy order:
    1. OSHash → StashDB hash lookup
    2. Provider hash → StashDB
    3. Filename parse → search ItemRepository

  Confidence is computed per candidate, not per strategy:
    base score from the strategy that found the candidate
    + bonus if candidate title matches embedded title tag
    + bonus if candidate artist/studio matches embedded artist tag
    + bonus if candidate album/series matches embedded album tag
    + bonus if candidate duration is within 5 seconds of embedded duration
  Maximum score: 1.0
       │
       ▼
Auto-import decision
  confidence >= configured threshold AND candidate links to an existing Item?
    YES → create MediaFile, set item.status = "imported", remove from queue
    NO  → write UnmatchedFile to queue

  Queue entry carries:
    - All candidates ranked by confidence (not just the top one)
    - The file's full Fingerprint (embedded tags, AcoustID, OSHash)
    - Status: pending | matched | dismissed
    - Grouped by candidate album (GroupExternalID) for music
```

---

## Import Queue

When a file cannot be auto-imported it sits in the queue at `GET /api/v1/unmatched-files`. The UI groups music files by their best candidate's album MBID so that all tracks from the same album appear together.

For each queued file the user can:

- **Accept** — the file is linked to the top candidate's existing library item (`POST /api/v1/unmatched-files/{id}/match`)
- **Select a different candidate** — pick any ranked candidate and import that one instead
- **Import & Create** — the content tree doesn't exist yet; the server imports the full tree (artist → album → all tracks) then matches this file to the correct track (`POST /api/v1/metadata/items/import` followed by `POST /api/v1/unmatched-files/{id}/match`)
- **Import Album** — import all tracks from the same album in one operation and match all queued files for that album at once
- **Search Provider** — re-run the identifier chain with a custom query to get fresh candidates
- **Dismiss** — remove from the pending queue without importing

Long-running import operations return a job ID. The UI polls `GET /api/v1/commands/{id}` until the job reports `done` or `error`, then proceeds to the match step.

---

## External ID System

Every entity can be linked to one or more external IDs stored in the `external_ids` table:

```
entity_type  TEXT  -- library_entry | group | item | person
entity_id    TEXT  -- internal UUID
source       TEXT  -- musicbrainz | stashdb | tmdb | tvdb | ...
external_id  TEXT  -- the ID within that source
```

`ExternalIDRepository.FindEntity(entityType, source, externalID)` returns the internal ID for any entity given its external ID. This is how `ImportEntry` and all import operations are idempotent: they check for an existing record first and return it if found, so calling the same import twice is always safe.

---

## API Handlers  (`internal/api/`)

Thin translation layer. Each handler group maps HTTP verbs to app service calls, translates between JSON and domain types, and writes responses. No business logic lives here.

The React UI is just another API client — it has no special access. Every operation the UI performs can be replicated with `curl`. See `docs/user/api.md` for full examples.
