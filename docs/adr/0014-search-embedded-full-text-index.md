# 0014. Search: An Embedded Full-Text Index (Bleve), Not a Search Server

Status: Accepted

## Context

The `Datastore` design ([0012](0012-datastore-persistence.md)) gives every
entity efficient *exact-match* filtered `List` via `Document.Index` — "all
`LibraryEntry` rows where `kind=studio`," "all `Image` rows where
`owner_id=p1`." That is not search. A UI need like "find performers named
roughly 'Rileigh'" or "search my library for 'gonzo'" needs fuzzy matching
and relevance ranking, neither of which a key-value secondary index (or a
generic `documents` table in any of the three SQL dialects) provides. No
capability like this exists anywhere in the codebase today.

Three real options were weighed, not just Bleve in isolation:

- **Elasticsearch** — the default association with "search," but JVM-based,
  cluster-oriented, and heavy (realistically ≥2 GiB just to run) for what
  this project is: a self-hosted app aimed at single or small-household
  deployments, not a team running a search cluster. Rejected as the
  default; not planned as an adapter at all unless a specific future need
  argues for it.
- **Meilisearch / Typesense** — purpose-built, single-binary, typo-tolerant
  search servers, dramatically lighter than Elasticsearch. Real, credible
  options — but both are still a *separate process* to deploy and operate
  alongside Purser, same operational category as choosing Postgres over
  Badger for the datastore.
- **Bleve** (`github.com/blevesearch/bleve`) — a pure-Go, embeddable
  full-text search *library*, not a server. It runs inside the Purser
  binary and persists its index to disk; no second process, no extra
  deployment step.

This project has already made this exact trade-off twice: BadgerDB (pure
Go, no CGo, no separate process) is the default `Database.Driver` over
Postgres/MySQL; `modernc.org/sqlite` (pure Go, no CGo) was chosen
specifically to avoid a CGo SQLite driver. Bleve is the same philosophy
applied to search — the zero-extra-infrastructure option is the default,
consistent with everything else this codebase has already decided.

## Decision

- **`ports.SearchIndex`** is a new, narrow port — separate from
  `ports.ImageRepository`/`ImageStore`'s split reasoning applied again
  here: exact-match structured filtering (`Datastore`) and ranked
  free-text search (`SearchIndex`) are different capabilities with
  different reasons to change, not one interface trying to do both.
  Shape (conceptual, not final): `Index(ctx, doc SearchDocument) error`,
  `Search(ctx, query string, opts...) ([]SearchResult, error)`,
  `Delete(ctx, id string) error` — a `SearchDocument` carries whatever
  free-text fields an entity wants searchable (name, title, aliases,
  overview) plus enough identifying data (collection/type, ID) to resolve
  a hit back to a real record via the normal `ports.XRepository.Get`.
- **`internal/adapters/searchindex/bleve`** is the first and default
  implementation, storing its index under `<Paths.DataDir>/search` (same
  derivation pattern `Database.Badger.DataDir`/`Media.Path` already use).
  No new configuration surface beyond that path; no separate process.
- **Index population is a write-time side effect, not derived lazily.**
  Whatever writes an entity (an entity repo or, more likely, a composing
  indexing step above it) is responsible for pushing the searchable
  fields into `SearchIndex` at Create/Update time and removing them at
  Delete — the same "who's responsible for keeping this in sync"
  question [0015](0015-deletion-impact-and-composing-services.md) raises
  for cross-entity cleanup, not solved differently here.
- **Meilisearch/Typesense remain a legitimate future adapter** behind the
  same port, for a user who wants a standalone search server — mirroring
  `Database.Driver`'s badger-default/postgres-mysql-sqlite-opt-in
  relationship. Not built now; the port is designed so it can be.
- **Elasticsearch is not planned as an adapter.** If a specific need
  argues for it later, that's a new decision, not an oversight here.

## Consequences

- New dependency (`bleve`), pure Go, no CGo — consistent with every other
  storage dependency already chosen in this codebase.
- Search results and the entity's authoritative record can drift if the
  index isn't updated in lockstep with `Datastore` writes — there is no
  transactional guarantee tying the two together (same category of
  eventual-consistency risk `Datastore`/`ImageStore` already have with
  each other per [0013](0013-image-blob-storage.md)). Whatever composing
  layer writes both is responsible for ordering/retrying sensibly; not
  solved automatically by either port.
- Search stays purely additive to browsing: `List` with `Document.Index`
  filters remains the mechanism for "show me just the Studios"; `Search`
  is for "find something by rough name," a different UI affordance
  entirely, not a replacement for filtered lists.

## Self-Audit Checklist

1. Does any code call into Bleve (or any future search adapter) directly
   instead of through `ports.SearchIndex`? If yes — fix it.
2. Does `ports.SearchIndex` grow structured-filter methods that belong on
   `Datastore` instead (mission creep back toward one interface doing
   both jobs)? If yes — split it back out.
3. Does an entity get written to `Datastore` without a corresponding
   `SearchIndex` update (or vice versa on delete), leaving a searchable
   ghost or an unsearchable real record? If yes — fix the write path that
   introduced the gap.
4. Is a full-collection scan-and-filter being used anywhere as a
   workaround for "search," instead of either a `Datastore` index filter
   (exact match) or `SearchIndex` (fuzzy/ranked)? If yes — use the right
   tool for which capability is actually needed.
